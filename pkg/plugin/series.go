package plugin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/cliftonz/typesense-datasource/pkg/timeseries"
	"github.com/cliftonz/typesense-datasource/pkg/typesense"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/gtime"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

const seriesPageSize = 250

type seriesPoint struct {
	Count              int64
	Sum, Avg, Min, Max *float64
}

func seriesFrame(name string, plan timeseries.Plan, points []seriesPoint, stats bool) *data.Frame {
	n := len(plan.Buckets)
	times, counts := make([]time.Time, n), make([]int64, n)
	sums, avgs, mins, maxes := make([]*float64, n), make([]*float64, n), make([]*float64, n), make([]*float64, n)
	for i, p := range points {
		times[i], counts[i] = plan.Buckets[i].Start, p.Count
		sums[i], avgs[i], mins[i], maxes[i] = p.Sum, p.Avg, p.Min, p.Max
	}
	frame := data.NewFrame(name, data.NewField("time", nil, times), data.NewField("count", nil, counts))
	if stats {
		frame.Fields = append(frame.Fields, data.NewField("sum", nil, sums), data.NewField("avg", nil, avgs), data.NewField("min", nil, mins), data.NewField("max", nil, maxes))
	}
	return frame
}

func (d *Datasource) pageThrough(ctx context.Context, search map[string]any, maxPages int, unit string, foundIsTotal bool, bucket func(typesense.SearchResult) (int, error)) ([]data.Notice, error) {
	if maxPages <= 0 {
		maxPages = d.settings.MaxPages
	}
	search["per_page"] = seriesPageSize
	fetched, found, cutoff, done := 0, 0, false, false
	for page := 1; page <= maxPages && !done; page++ {
		search["page"] = page
		results, err := d.search(ctx, search)
		if err != nil {
			return nil, err
		}
		n, err := bucket(results[0])
		if err != nil {
			return nil, err
		}
		found, fetched, cutoff = results[0].Found, fetched+n, cutoff || results[0].SearchCutoff
		done = n < seriesPageSize || foundIsTotal && fetched >= found
	}
	var notices []data.Notice
	switch {
	case done:
	case found > fetched:
		notices = append(notices, data.Notice{Severity: data.NoticeSeverityWarning, Text: fmt.Sprintf("Bucketed %d of %d %s (max pages reached)", fetched, found, unit)})
	default:
		notices = append(notices, data.Notice{Severity: data.NoticeSeverityWarning, Text: fmt.Sprintf("Bucketed %d %s (max pages reached; more may exist)", fetched, unit)})
	}
	if cutoff {
		notices = append(notices, searchCutoffNotice)
	}
	return notices, nil
}

func (d *Datasource) timeUnit() time.Duration {
	if d.settings.TimeUnit == "ms" {
		return time.Millisecond
	}
	return time.Second
}

func (d *Datasource) unix(t time.Time) int64 {
	if d.timeUnit() == time.Millisecond {
		return t.UnixMilli()
	}
	return t.Unix()
}

func (d *Datasource) timeSeriesPlan(qm queryModel, dq backend.DataQuery) (timeseries.Plan, error) {
	var minInterval time.Duration
	if qm.MinInterval != "" {
		parsed, err := gtime.ParseDuration(qm.MinInterval)
		if err != nil {
			return timeseries.Plan{}, errors.New("Invalid min interval: " + qm.MinInterval)
		}
		minInterval = parsed
	}
	location, err := time.LoadLocation(qm.Timezone)
	if err != nil {
		return timeseries.Plan{}, errors.New("Unknown time zone " + qm.Timezone)
	}
	if d.unix(dq.TimeRange.From) >= d.unix(dq.TimeRange.To) {
		return timeseries.Plan{}, nil
	}
	return timeseries.NewPlan(dq.TimeRange.From, dq.TimeRange.To, dq.Interval, minInterval, dq.MaxDataPoints, d.settings.TimeUnit, location), nil
}

func bucketIndex(plan timeseries.Plan, v float64) (int, bool) {
	i := sort.Search(len(plan.Buckets), func(i int) bool { return float64(plan.Buckets[i].To) > v })
	return i, i < len(plan.Buckets) && v >= float64(plan.Buckets[0].From)
}
