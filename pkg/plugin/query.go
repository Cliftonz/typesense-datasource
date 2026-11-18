package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cliftonz/typesense-datasource/pkg/frames"
	"github.com/cliftonz/typesense-datasource/pkg/typesense"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

type queryModel struct {
	Collection  string `json:"collection"`
	Q           string `json:"q"`
	QueryBy     string `json:"queryBy"`
	FilterBy    string `json:"filterBy"`
	SortBy      string `json:"sortBy"`
	FacetBy     string `json:"facetBy"`
	GroupBy     string `json:"groupBy"`
	PerPage     int    `json:"perPage"`
	TimeField   string `json:"timeField"`
	MinInterval string `json:"minInterval"`
	Timezone    string `json:"timezone"`
	EditorMode  string `json:"editorMode"`
	RawBody     string `json:"rawBody"`
	Format      string `json:"format"`
	BucketMode  string `json:"bucketMode"`
	ValueField  string `json:"valueField"`
	Stats       bool   `json:"stats"`
	MaxPages    int    `json:"maxPages"`
}

const (
	maxConcurrentQueries    = 10
	typesenseDefaultPerPage = 10
)

var searchCutoffNotice = data.Notice{Severity: data.NoticeSeverityWarning, Text: "Typesense stopped the search early (search_cutoff); results may be partial"}

func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	response := backend.NewQueryDataResponse()
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, maxConcurrentQueries)
	for _, q := range req.Queries {
		slots <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := d.recoveredQuery(ctx, q)
			mu.Lock()
			response.Responses[q.RefID] = res
			mu.Unlock()
			<-slots
		}()
	}
	wg.Wait()
	return response, nil
}

func (d *Datasource) recoveredQuery(ctx context.Context, dq backend.DataQuery) (res backend.DataResponse) {
	defer func() {
		if r := recover(); r != nil {
			res = backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("Internal error: %v", r))
		}
	}()
	return d.query(ctx, dq)
}

func (d *Datasource) query(ctx context.Context, dq backend.DataQuery) backend.DataResponse {
	if d.settings.URL == "" {
		return backend.ErrDataResponse(backend.StatusBadRequest, "URL is missing")
	}
	var qm queryModel
	if err := json.Unmarshal(dq.JSON, &qm); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	switch qm.EditorMode {
	case "", "builder", "raw":
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, "Unknown editor mode "+qm.EditorMode)
	}
	if qm.EditorMode == "raw" {
		return d.rawTables(ctx, qm.RawBody, dq.TimeRange)
	}
	switch qm.Format {
	case "", "table", "timeseries":
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, "Unknown format "+qm.Format)
	}
	if qm.Format == "timeseries" {
		switch qm.BucketMode {
		case "groupBy":
			return d.groupBySeries(ctx, qm, dq)
		case "", "hits", "rangeFacet":
		default:
			return backend.ErrDataResponse(backend.StatusBadRequest, "Unknown bucket mode "+qm.BucketMode)
		}
		if qm.TimeField == "" {
			return backend.ErrDataResponse(backend.StatusBadRequest, "Time field is required for time series")
		}
		if qm.Stats && qm.ValueField == "" {
			return backend.ErrDataResponse(backend.StatusBadRequest, "Value field is required for stats")
		}
		if qm.BucketMode == "rangeFacet" {
			return d.rangeFacetSeries(ctx, qm, dq)
		}
		return d.hitsSeries(ctx, qm, dq)
	}
	return d.builderTable(ctx, qm, dq.TimeRange)
}

func (d *Datasource) builderTable(ctx context.Context, qm queryModel, timeRange backend.TimeRange) backend.DataResponse {
	if qm.TimeField != "" && !timeRange.From.Before(timeRange.To) {
		return backend.DataResponse{}
	}
	results, err := d.search(ctx, builderSearch(qm, qm.TimeField, d.unix(timeRange.From), d.unix(timeRange.To)))
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	result := results[0]
	var table *data.Frame
	shown, unit := len(result.Hits), "hits"
	if qm.GroupBy != "" {
		table = frames.GroupsFrame(qm.Collection, splitTrimmed(qm.GroupBy), result.GroupedHits)
		shown, unit = len(result.GroupedHits), "groups"
	} else {
		table = frames.HitsFrame(qm.Collection, result.Hits)
		if qm.TimeField != "" {
			if err := d.convertTimeColumn(table, qm.TimeField); err != nil {
				return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
			}
		}
	}
	perPage := qm.PerPage
	if perPage == 0 {
		perPage = typesenseDefaultPerPage
	}
	switch {
	case result.Found > shown:
		table.AppendNotices(data.Notice{Severity: data.NoticeSeverityInfo, Text: fmt.Sprintf("Showing %d of %d %s", shown, result.Found, unit)})
	case qm.GroupBy != "" && shown == perPage && result.Found == shown:
		noun := "groups"
		if shown == 1 {
			noun = "group"
		}
		table.AppendNotices(data.Notice{Severity: data.NoticeSeverityInfo, Text: fmt.Sprintf("Showing %d %s; more may exist", shown, noun)})
	}
	if result.SearchCutoff {
		table.AppendNotices(searchCutoffNotice)
	}
	return backend.DataResponse{Frames: append(data.Frames{table}, frames.FacetFrames(result.FacetCounts)...)}
}

func builderSearch(qm queryModel, timeField string, start, end int64) map[string]any {
	filterBy := qm.FilterBy
	if timeField != "" {
		filterBy = withTimeFilter(filterBy, timeField, start, end)
	}
	q := qm.Q
	if strings.TrimSpace(q) == "" {
		q = "*"
	}
	search := map[string]any{"collection": qm.Collection, "q": q}
	for key, value := range map[string]string{"query_by": qm.QueryBy, "filter_by": filterBy, "sort_by": qm.SortBy, "facet_by": qm.FacetBy, "group_by": qm.GroupBy} {
		if value != "" {
			search[key] = value
		}
	}
	if qm.PerPage != 0 {
		search["per_page"] = qm.PerPage
	}
	return search
}

func (d *Datasource) convertTimeColumn(frame *data.Frame, name string) error {
	field, i := frame.FieldByName(name)
	if i < 0 {
		return nil
	}
	unit := d.timeUnit()
	times := make([]*time.Time, field.Len())
	for row := range times {
		v, ok := field.ConcreteAt(row)
		if !ok {
			continue
		}
		var t time.Time
		switch n := v.(type) {
		case int64:
			t = time.Unix(0, n*int64(unit))
		case float64:
			t = time.Unix(0, int64(n*float64(unit)))
		default:
			return fmt.Errorf("Time field %q is not a numeric timestamp", name)
		}
		times[row] = &t
	}
	frame.Fields[i] = data.NewField(name, nil, times)
	return nil
}

func (d *Datasource) search(ctx context.Context, searches ...map[string]any) ([]typesense.SearchResult, error) {
	body, err := json.Marshal(map[string]any{"searches": searches})
	if err != nil {
		panic(err)
	}
	collections := make([]string, len(searches))
	for i, search := range searches {
		collections[i] = search["collection"].(string)
	}
	return d.multiSearch(ctx, body, collections)
}

func (d *Datasource) multiSearch(ctx context.Context, body []byte, collections []string) ([]typesense.SearchResult, error) {
	results, err := d.client.MultiSearch(ctx, body)
	if err != nil {
		return nil, searchError(collections[0], errors.Is(err, typesense.ErrUnauthorized), err.Error())
	}
	for i, r := range results {
		if r.Code == http.StatusUnauthorized || r.Error != "" {
			return nil, searchError(collections[i], r.Code == http.StatusUnauthorized, r.Error)
		}
	}
	return results, nil
}

func searchError(collection string, unauthorized bool, reason string) error {
	if unauthorized {
		return fmt.Errorf("API key was rejected or cannot search %q", collection)
	}
	return fmt.Errorf("Search on %q failed: %s", collection, reason)
}

func splitTrimmed(list string) []string {
	var names []string
	for _, p := range strings.Split(list, ",") {
		if name := strings.TrimSpace(p); name != "" {
			names = append(names, name)
		}
	}
	return names
}
