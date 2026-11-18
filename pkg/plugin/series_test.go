package plugin

import (
	"testing"
	"time"

	"github.com/cliftonz/typesense-datasource/pkg/timeseries"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func singleFrame(t *testing.T, res backend.DataResponse) *data.Frame {
	t.Helper()
	if res.Error != nil || len(res.Frames) != 1 {
		t.Fatalf("error %v, %d frames", res.Error, len(res.Frames))
	}
	return res.Frames[0]
}

func seriesCounts(frame *data.Frame) []int64 {
	field, _ := frame.FieldByName("count")
	counts := make([]int64, field.Len())
	for i := range counts {
		counts[i] = field.At(i).(int64)
	}
	return counts
}

func seriesNotices(frame *data.Frame) []data.Notice {
	if frame.Meta == nil {
		return nil
	}
	return frame.Meta.Notices
}

func seriesSum(counts []int64) int64 {
	var sum int64
	for _, c := range counts {
		sum += c
	}
	return sum
}

var seriesPlan = timeseries.Plan{Buckets: []timeseries.Bucket{
	{Start: time.Unix(0, 0).UTC(), From: 0, To: 10},
	{Start: time.Unix(10, 0).UTC(), From: 10, To: 20},
	{Start: time.Unix(20, 0).UTC(), From: 20, To: 30},
}}

func TestSeriesFrameZeroFillsEveryBucket(t *testing.T) {
	frame := seriesFrame("events", seriesPlan, []seriesPoint{{Count: 2}, {}, {Count: 1}}, false)
	if frame.Name != "events" {
		t.Fatalf("got name %q", frame.Name)
	}
	assertJSONEqual(t, fieldNames(frame), []string{"time", "count"})
	assertJSONEqual(t, frameRows(t, frame), [][]any{
		{time.Unix(0, 0).UTC(), 2},
		{time.Unix(10, 0).UTC(), 0},
		{time.Unix(20, 0).UTC(), 1},
	})
}

func TestSeriesFrameStatsColumnsAreNullForBucketsWithoutValues(t *testing.T) {
	sum, avg, minimum, maximum := 6.0, 3.0, 1.0, 5.0
	frame := seriesFrame("events", seriesPlan, []seriesPoint{{Count: 2, Sum: &sum, Avg: &avg, Min: &minimum, Max: &maximum}, {Count: 1}, {}}, true)
	assertJSONEqual(t, fieldNames(frame), []string{"time", "count", "sum", "avg", "min", "max"})
	assertJSONEqual(t, frameRows(t, frame), [][]any{
		{time.Unix(0, 0).UTC(), 2, 6, 3, 1, 5},
		{time.Unix(10, 0).UTC(), 1, nil, nil, nil, nil},
		{time.Unix(20, 0).UTC(), 0, nil, nil, nil, nil},
	})
}

func planFor(t *testing.T, minInterval, timezone string) (timeseries.Plan, error) {
	d := testDatasource("", "")
	return d.timeSeriesPlan(queryModel{MinInterval: minInterval, Timezone: timezone}, backend.DataQuery{
		TimeRange: backend.TimeRange{From: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), To: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)},
		Interval:  time.Hour,
	})
}

func TestTimeSeriesPlanDefaultsToNoMinIntervalAndUTC(t *testing.T) {
	plan, err := planFor(t, "", "")
	if err != nil || len(plan.Buckets) != 24 || !plan.Buckets[0].Start.Equal(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("got error %v, %d buckets", err, len(plan.Buckets))
	}
}

func TestTimeSeriesPlanAlignsDayBucketsToTimezone(t *testing.T) {
	plan, err := planFor(t, "1d", "America/New_York")
	if err != nil || len(plan.Buckets) == 0 || !plan.Buckets[0].Start.Equal(time.Date(2024, 1, 1, 5, 0, 0, 0, time.UTC)) {
		t.Fatalf("got error %v, buckets %+v", err, plan.Buckets)
	}
}

func TestTimeSeriesPlanInvalidMinInterval(t *testing.T) {
	if _, err := planFor(t, "abc", ""); err == nil || err.Error() != "Invalid min interval: abc" {
		t.Fatalf("got error %v", err)
	}
}

func TestTimeSeriesPlanUnknownTimezone(t *testing.T) {
	if _, err := planFor(t, "", "Mars/Base"); err == nil || err.Error() != "Unknown time zone Mars/Base" {
		t.Fatalf("got error %v", err)
	}
}

func TestUnixUsesConfiguredTimeUnit(t *testing.T) {
	at := time.Unix(1700000000, 900_000_000)
	for unit, want := range map[string]int64{"s": 1700000000, "ms": 1700000000900} {
		d := testDatasource("", "")
		d.settings.TimeUnit = unit
		if got := d.unix(at); got != want {
			t.Errorf("unit %s: got %d, want %d", unit, got, want)
		}
	}
}
