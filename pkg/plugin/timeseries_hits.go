package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cliftonz/typesense-datasource/pkg/typesense"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

type hitsBucket struct {
	count, values int64
	sum, min, max float64
}

func (d *Datasource) hitsSeries(ctx context.Context, qm queryModel, dq backend.DataQuery) backend.DataResponse {
	plan, err := d.timeSeriesPlan(qm, dq)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	if len(plan.Buckets) == 0 {
		return backend.DataResponse{}
	}
	qm.GroupBy, qm.FacetBy, qm.SortBy = "", "", ""
	search := builderSearch(qm, qm.TimeField, d.unix(dq.TimeRange.From), d.unix(dq.TimeRange.To))
	search["include_fields"] = qm.TimeField
	if qm.Stats {
		search["include_fields"] = qm.TimeField + "," + qm.ValueField
	}
	buckets := make([]hitsBucket, len(plan.Buckets))
	notices, err := d.pageThrough(ctx, search, qm.MaxPages, "hits", true, func(result typesense.SearchResult) (int, error) {
		for _, hit := range result.Hits {
			ts, ok := jsonFloat(hit.Document[qm.TimeField])
			if !ok {
				return 0, fmt.Errorf("Time field %q is not a numeric timestamp", qm.TimeField)
			}
			i, inRange := bucketIndex(plan, ts)
			if !inRange {
				continue
			}
			b := &buckets[i]
			b.count++
			if value := hit.Document[qm.ValueField]; qm.Stats && value != nil {
				v, ok := jsonFloat(value)
				if !ok {
					return 0, fmt.Errorf("Value field %q is not numeric", qm.ValueField)
				}
				if b.values == 0 {
					b.min, b.max = v, v
				}
				b.values, b.sum, b.min, b.max = b.values+1, b.sum+v, min(b.min, v), max(b.max, v)
			}
		}
		return len(result.Hits), nil
	})
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	points := make([]seriesPoint, len(buckets))
	for i, b := range buckets {
		points[i].Count = b.count
		if b.values > 0 {
			avg := b.sum / float64(b.values)
			points[i].Sum, points[i].Avg, points[i].Min, points[i].Max = &b.sum, &avg, &b.min, &b.max
		}
	}
	frame := seriesFrame(qm.Collection, plan, points, qm.Stats)
	frame.AppendNotices(notices...)
	return backend.DataResponse{Frames: data.Frames{frame}}
}

func jsonFloat(value any) (float64, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := number.Float64()
	return f, err == nil
}
