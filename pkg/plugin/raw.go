package plugin

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/cliftonz/typesense-datasource/pkg/frames"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

type rawBody struct {
	Union    bool `json:"union"`
	Searches []struct {
		Collection string `json:"collection"`
		GroupBy    string `json:"group_by"`
	} `json:"searches"`
}

func (d *Datasource) rawTables(ctx context.Context, raw string, timeRange backend.TimeRange) backend.DataResponse {
	body := strings.NewReplacer("$__fromTime", strconv.FormatInt(d.unix(timeRange.From), 10), "$__toTime", strconv.FormatInt(d.unix(timeRange.To), 10)).Replace(raw)
	var parsed rawBody
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "Invalid raw body: "+err.Error())
	}
	if parsed.Union {
		return backend.ErrDataResponse(backend.StatusBadRequest, "union search not supported")
	}
	if len(parsed.Searches) == 0 {
		return backend.ErrDataResponse(backend.StatusBadRequest, "Invalid raw body: no searches")
	}
	collections := make([]string, len(parsed.Searches))
	for i, search := range parsed.Searches {
		collections[i] = search.Collection
	}
	results, err := d.multiSearch(ctx, []byte(body), collections)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	var out data.Frames
	for i, result := range results {
		search := parsed.Searches[i]
		var searchFrames data.Frames
		switch {
		case result.GroupedHits != nil:
			searchFrames = data.Frames{frames.GroupsFrame(search.Collection, splitTrimmed(search.GroupBy), result.GroupedHits)}
		case len(result.FacetCounts) > 0:
			searchFrames = frames.FacetFrames(result.FacetCounts)
		default:
			searchFrames = data.Frames{frames.HitsFrame(search.Collection, result.Hits)}
		}
		if result.SearchCutoff {
			searchFrames[0].AppendNotices(searchCutoffNotice)
		}
		out = append(out, searchFrames...)
	}
	return backend.DataResponse{Frames: out}
}
