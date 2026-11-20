package plugin

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cliftonz/typesense-datasource/pkg/timeseries"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

const rangeFacetMaxSearches = 50

func (d *Datasource) rangeFacetSeries(ctx context.Context, qm queryModel, dq backend.DataQuery) backend.DataResponse {
	plan, err := d.timeSeriesPlan(qm, dq)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	if len(plan.Buckets) == 0 {
		return backend.DataResponse{}
	}
	qm.GroupBy, qm.SortBy = "", ""
	var notices []data.Notice
	bucketSearch := d.rangeFacetCounts
	if qm.Stats {
		var widened bool
		if plan, widened = plan.Widen(rangeFacetMaxSearches); widened {
			notices = append(notices, data.Notice{Severity: data.NoticeSeverityInfo, Text: fmt.Sprintf("Buckets widened to %s to stay within %d searches", exactInterval(plan.Interval()), rangeFacetMaxSearches)})
		}
		bucketSearch = d.rangeFacetStats
	}
	points, cutoff, err := bucketSearch(ctx, qm, plan)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	if cutoff {
		notices = append(notices, searchCutoffNotice)
	}
	frame := seriesFrame(qm.Collection, plan, points, qm.Stats)
	frame.AppendNotices(notices...)
	return backend.DataResponse{Frames: data.Frames{frame}}
}

func (d *Datasource) rangeFacetCounts(ctx context.Context, qm queryModel, plan timeseries.Plan) ([]seriesPoint, bool, error) {
	ranges := make([]string, len(plan.Buckets))
	for i, b := range plan.Buckets {
		ranges[i] = fmt.Sprintf("b%d:[%d, %d]", i, b.From, b.To)
	}
	search := builderSearch(qm, qm.TimeField, plan.Buckets[0].From, plan.Buckets[len(plan.Buckets)-1].To)
	search["per_page"] = 0
	search["facet_by"] = qm.TimeField + "(" + strings.Join(ranges, ", ") + ")"
	search["max_facet_values"] = len(ranges)
	search["facet_strategy"] = "exhaustive"
	results, err := d.search(ctx, search)
	if err != nil {
		return nil, false, err
	}
	points := make([]seriesPoint, len(plan.Buckets))
	for _, c := range results[0].FacetCounts[0].Counts {
		i, err := strconv.Atoi(strings.TrimPrefix(c.Value, "b"))
		if err != nil {
			panic(err)
		}
		points[i].Count = int64(c.Count)
	}
	return points, results[0].SearchCutoff, nil
}

func (d *Datasource) rangeFacetStats(ctx context.Context, qm queryModel, plan timeseries.Plan) ([]seriesPoint, bool, error) {
	searches := make([]map[string]any, len(plan.Buckets))
	for i, b := range plan.Buckets {
		search := builderSearch(qm, qm.TimeField, b.From, b.To)
		search["per_page"] = 0
		search["facet_by"] = qm.ValueField
		search["facet_strategy"] = "exhaustive"
		searches[i] = search
	}
	results, err := d.search(ctx, searches...)
	if err != nil {
		return nil, false, err
	}
	points := make([]seriesPoint, len(plan.Buckets))
	cutoff := false
	for i, r := range results {
		stats := r.FacetCounts[0].Stats
		if len(r.FacetCounts[0].Counts) > 0 && stats.Avg == nil {
			return nil, false, fmt.Errorf("Value field %q is not numeric", qm.ValueField)
		}
		points[i] = seriesPoint{Count: int64(r.Found), Sum: stats.Sum, Avg: stats.Avg, Min: stats.Min, Max: stats.Max}
		cutoff = cutoff || r.SearchCutoff
	}
	return points, cutoff, nil
}

func exactInterval(interval time.Duration) string {
	for _, unit := range []struct {
		size time.Duration
		name string
	}{{24 * time.Hour, "d"}, {time.Hour, "h"}, {time.Minute, "m"}, {time.Second, "s"}} {
		if interval%unit.size == 0 {
			return fmt.Sprintf("%d%s", interval/unit.size, unit.name)
		}
	}
	return fmt.Sprintf("%dms", interval/time.Millisecond)
}
