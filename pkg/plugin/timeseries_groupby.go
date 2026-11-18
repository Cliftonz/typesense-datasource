package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cliftonz/typesense-datasource/pkg/typesense"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func (d *Datasource) groupBySeries(ctx context.Context, qm queryModel, dq backend.DataQuery) backend.DataResponse {
	fields := splitTrimmed(qm.GroupBy)
	if len(fields) == 0 {
		return backend.ErrDataResponse(backend.StatusBadRequest, "Group by field is required for group_by mode")
	}
	groupField := fields[0]
	plan, err := d.timeSeriesPlan(qm, dq)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	if len(plan.Buckets) == 0 {
		return backend.DataResponse{}
	}
	qm.GroupBy, qm.FacetBy, qm.SortBy = groupField, "", ""
	search := builderSearch(qm, groupField, d.unix(dq.TimeRange.From), d.unix(dq.TimeRange.To))
	search["group_limit"] = 1
	points := make([]seriesPoint, len(plan.Buckets))
	notices, err := d.pageThrough(ctx, search, qm.MaxPages, "groups", false, func(result typesense.SearchResult) (int, error) {
		for _, group := range result.GroupedHits {
			number, _ := group.GroupKey[0].(json.Number)
			key, err := number.Int64()
			if err != nil {
				return 0, fmt.Errorf("Group by field %q must hold integer timestamps", groupField)
			}
			if i, inRange := bucketIndex(plan, float64(key)); inRange {
				points[i].Count += int64(group.Found)
			}
		}
		return len(result.GroupedHits), nil
	})
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	frame := seriesFrame(qm.Collection, plan, points, false)
	frame.AppendNotices(notices...)
	return backend.DataResponse{Frames: data.Frames{frame}}
}
