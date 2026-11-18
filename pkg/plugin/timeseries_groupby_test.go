package plugin

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

const groupByT0 = 1699999200

type groupByDoc struct {
	hour  int64
	brand string
}

func groupByCollection(t *testing.T, url string, docs []groupByDoc) string {
	maps := make([]map[string]any, len(docs))
	for i, doc := range docs {
		maps[i] = map[string]any{"hour": doc.hour, "brand": doc.brand}
	}
	return testCollection(t, url, "groupby", []map[string]any{
		{"name": "hour", "type": "int64", "facet": true},
		{"name": "brand", "type": "string", "facet": true},
	}, maps...)
}

func groupByHourDocs() []groupByDoc {
	return []groupByDoc{
		{groupByT0 - 3600, "acme"},
		{groupByT0, "acme"},
		{groupByT0, "acme"},
		{groupByT0, "zeta"},
		{groupByT0 + 3600, "acme"},
		{groupByT0 + 3*3600, "zeta"},
		{groupByT0 + 3*3600, "acme"},
		{groupByT0 + 4*3600, "acme"},
	}
}

func groupByDataQuery(hours int64, interval time.Duration) backend.DataQuery {
	return backend.DataQuery{
		TimeRange: backend.TimeRange{From: time.Unix(groupByT0, 0), To: time.Unix(groupByT0+hours*3600, 0)},
		Interval:  interval,
	}
}

func TestGroupBySeriesCountsPerBucket(t *testing.T) {
	cases := []struct {
		name     string
		qm       queryModel
		interval time.Duration
		want     []int64
	}{
		{"plan interval equals stored buckets with zero fill", queryModel{GroupBy: "hour"}, time.Hour, []int64{3, 1, 0, 2}},
		{"coarser plan interval sums stored buckets", queryModel{GroupBy: " hour , brand"}, 2 * time.Hour, []int64{4, 2}},
		{"builder filter applied", queryModel{GroupBy: "hour", FilterBy: "brand:=acme"}, time.Hour, []int64{2, 1, 0, 1}},
	}
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := groupByCollection(t, url, groupByHourDocs())
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					c.qm.Collection = collection
					frame := singleFrame(t, testDatasource(url, adminKey).groupBySeries(t.Context(), c.qm, groupByDataQuery(4, c.interval)))
					assertJSONEqual(t, fieldNames(frame), []string{"time", "count"})
					assertJSONEqual(t, seriesCounts(frame), c.want)
					if frame.Name != collection || !frame.Fields[0].At(0).(time.Time).Equal(time.Unix(groupByT0, 0)) {
						t.Fatalf("frame %q starts %v", frame.Name, frame.Fields[0].At(0))
					}
					if notices := seriesNotices(frame); len(notices) != 0 {
						t.Fatalf("notices %+v", notices)
					}
				})
			}
		})
	}
}

func TestGroupBySeriesRequiresGroupByField(t *testing.T) {
	res := testDatasource("http://localhost:8107", adminKey).groupBySeries(t.Context(), queryModel{Collection: "x", GroupBy: " , "}, groupByDataQuery(4, time.Hour))
	if res.Error == nil || res.Error.Error() != "Group by field is required for group_by mode" || len(res.Frames) != 0 {
		t.Fatalf("got %v, %d frames", res.Error, len(res.Frames))
	}
}

func TestGroupBySeriesPlanErrorFailsQuery(t *testing.T) {
	res := testDatasource("http://localhost:8107", adminKey).groupBySeries(t.Context(), queryModel{Collection: "x", GroupBy: "hour", Timezone: "Nowhere/Bogus"}, groupByDataQuery(4, time.Hour))
	if res.Error == nil || res.Error.Error() != "Unknown time zone Nowhere/Bogus" {
		t.Fatalf("got %v", res.Error)
	}
}

func TestGroupBySeriesEmptyOrInvertedRangeGivesNoFrames(t *testing.T) {
	for _, hours := range []int64{0, -2} {
		res := testDatasource("http://localhost:8107", adminKey).groupBySeries(t.Context(), queryModel{Collection: "missing", GroupBy: "hour"}, groupByDataQuery(hours, time.Hour))
		if res.Error != nil || len(res.Frames) != 0 {
			t.Fatalf("hours %d: %v, %d frames", hours, res.Error, len(res.Frames))
		}
	}
}

func TestGroupBySeriesPagesPast250Groups(t *testing.T) {
	docs := make([]groupByDoc, 300)
	for i := range docs {
		docs[i] = groupByDoc{groupByT0 + int64(i)*3600, "acme"}
	}
	maxPagesNotice := []data.Notice{{Severity: data.NoticeSeverityWarning, Text: "Bucketed 250 of 300 groups (max pages reached)"}}
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := groupByCollection(t, url, docs)
			qm := queryModel{Collection: collection, GroupBy: "hour"}
			frame := singleFrame(t, testDatasource(url, adminKey).groupBySeries(t.Context(), qm, groupByDataQuery(300, time.Hour)))
			if counts, notices := seriesCounts(frame), seriesNotices(frame); len(notices) != 0 || seriesSum(counts) != 300 || len(counts) != 300 {
				t.Fatalf("all pages: %v sum %d len %d", notices, seriesSum(counts), len(counts))
			}
			settingsLimited := testDatasource(url, adminKey)
			settingsLimited.settings.MaxPages = 1
			frame = singleFrame(t, settingsLimited.groupBySeries(t.Context(), qm, groupByDataQuery(300, time.Hour)))
			if sum := seriesSum(seriesCounts(frame)); sum != 250 {
				t.Fatalf("settings max pages: sum %d", sum)
			}
			assertJSONEqual(t, seriesNotices(frame), maxPagesNotice)
			qm.MaxPages = 1
			frame = singleFrame(t, testDatasource(url, adminKey).groupBySeries(t.Context(), qm, groupByDataQuery(300, time.Hour)))
			if sum := seriesSum(seriesCounts(frame)); sum != 250 {
				t.Fatalf("query max pages: sum %d", sum)
			}
			assertJSONEqual(t, seriesNotices(frame), maxPagesNotice)
		})
	}
}

func groupByPages(t *testing.T, pages ...string) string {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`{"results":[` + pages[page] + `]}`)); err != nil {
			t.Error(err)
		}
		page++
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func groupByPage(found, groups int, key string, cutoff bool) string {
	hits := make([]string, groups)
	for i := range hits {
		hits[i] = `{"group_key":[` + key + `],"found":2,"hits":[]}`
	}
	return fmt.Sprintf(`{"found":%d,"search_cutoff":%t,"grouped_hits":[%s]}`, found, cutoff, strings.Join(hits, ","))
}

var groupByKey = fmt.Sprint(groupByT0)

func TestGroupBySeriesSearchCutoffNoticeOnce(t *testing.T) {
	url := groupByPages(t, groupByPage(500, 250, groupByKey, true), groupByPage(500, 1, groupByKey, true))
	frame := singleFrame(t, testDatasource(url, adminKey).groupBySeries(t.Context(), queryModel{Collection: "x", GroupBy: "hour", MaxPages: 3}, groupByDataQuery(2, time.Hour)))
	assertJSONEqual(t, seriesCounts(frame), []int64{502, 0})
	assertJSONEqual(t, seriesNotices(frame), []data.Notice{searchCutoffNotice})
}

func TestGroupBySeriesStopsPagingOnShortPage(t *testing.T) {
	url := groupByPages(t, groupByPage(5, 1, groupByKey, false))
	frame := singleFrame(t, testDatasource(url, adminKey).groupBySeries(t.Context(), queryModel{Collection: "x", GroupBy: "hour", MaxPages: 3}, groupByDataQuery(2, time.Hour)))
	assertJSONEqual(t, seriesCounts(frame), []int64{2, 0})
	if notices := seriesNotices(frame); len(notices) != 0 {
		t.Fatalf("notices %+v", notices)
	}
}

func TestGroupBySeriesNonIntegerGroupKeyFailsQuery(t *testing.T) {
	for _, key := range []string{`"abc"`, `1.5`} {
		url := groupByPages(t, groupByPage(1, 1, key, false))
		res := testDatasource(url, adminKey).groupBySeries(t.Context(), queryModel{Collection: "x", GroupBy: "hour"}, groupByDataQuery(2, time.Hour))
		if res.Error == nil || res.Error.Error() != `Group by field "hour" must hold integer timestamps` || len(res.Frames) != 0 {
			t.Fatalf("key %s: %v, %d frames", key, res.Error, len(res.Frames))
		}
	}
}

func TestGroupBySeriesSearchErrorFailsQuery(t *testing.T) {
	url := stubTypesense(t, map[string]stubResponse{"/multi_search": {500, `{"message":"boom"}`}})
	res := testDatasource(url, adminKey).groupBySeries(t.Context(), queryModel{Collection: "x", GroupBy: "hour"}, groupByDataQuery(2, time.Hour))
	if res.Error == nil || res.Error.Error() != `Search on "x" failed: typesense: status 500: boom` {
		t.Fatalf("got %v", res.Error)
	}
}

func TestGroupBySeriesKeepsPagingWhenFoundCountsOnlyFetchedGroups(t *testing.T) {
	url := groupByPages(t, groupByPage(250, 250, groupByKey, false), groupByPage(300, 50, groupByKey, false))
	frame := singleFrame(t, testDatasource(url, adminKey).groupBySeries(t.Context(), queryModel{Collection: "x", GroupBy: "hour", MaxPages: 3}, groupByDataQuery(2, time.Hour)))
	assertJSONEqual(t, seriesCounts(frame), []int64{600, 0})
	if notices := seriesNotices(frame); len(notices) != 0 {
		t.Fatalf("notices %+v", notices)
	}
}

func TestGroupBySeriesMaxPagesOnFullPageWithoutLargerFoundSaysMoreMayExist(t *testing.T) {
	url := groupByPages(t, groupByPage(250, 250, groupByKey, false))
	frame := singleFrame(t, testDatasource(url, adminKey).groupBySeries(t.Context(), queryModel{Collection: "x", GroupBy: "hour", MaxPages: 1}, groupByDataQuery(2, time.Hour)))
	assertJSONEqual(t, seriesNotices(frame), []data.Notice{{Severity: data.NoticeSeverityWarning, Text: "Bucketed 250 groups (max pages reached; more may exist)"}})
}

func TestGroupBySeriesIgnoresBuilderFacetAndSort(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := groupByCollection(t, url, groupByHourDocs())
			qm := queryModel{Collection: collection, GroupBy: "hour", FacetBy: "nope", SortBy: "nope:desc"}
			frame := singleFrame(t, testDatasource(url, adminKey).groupBySeries(t.Context(), qm, groupByDataQuery(4, time.Hour)))
			assertJSONEqual(t, seriesCounts(frame), []int64{3, 1, 0, 2})
		})
	}
}

func TestGroupBySeriesSkipsGroupsOutsideTheRange(t *testing.T) {
	page := `{"found":3,"grouped_hits":[` + strings.Join([]string{
		fmt.Sprintf(`{"group_key":[%d],"found":1,"hits":[]}`, groupByT0-3600),
		fmt.Sprintf(`{"group_key":[%d],"found":1,"hits":[]}`, groupByT0),
		fmt.Sprintf(`{"group_key":[%d],"found":1,"hits":[]}`, groupByT0+2*3600),
	}, ",") + `]}`
	frame := singleFrame(t, testDatasource(groupByPages(t, page), adminKey).groupBySeries(t.Context(), queryModel{Collection: "x", GroupBy: "hour"}, groupByDataQuery(2, time.Hour)))
	assertJSONEqual(t, seriesCounts(frame), []int64{1, 0})
}

func TestGroupBySeriesRequestsOneHitPerGroup(t *testing.T) {
	url, sent := stubSearchCapture(t, `{"found":0,"grouped_hits":[]}`)
	singleFrame(t, testDatasource(url, adminKey).groupBySeries(t.Context(), queryModel{Collection: "x", GroupBy: "hour"}, groupByDataQuery(2, time.Hour)))
	if got := (*sent)[0]["group_limit"]; got != 1.0 {
		t.Fatalf("group_limit %v, want 1", got)
	}
}
