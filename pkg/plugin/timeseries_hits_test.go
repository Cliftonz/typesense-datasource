package plugin

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func hitsCollection(t *testing.T, url string, docs []map[string]any) string {
	return testCollection(t, url, "hits", []map[string]any{
		{"name": "ts", "type": "int64"},
		{"name": "price", "type": "float", "optional": true},
		{"name": "brand", "type": "string"},
	}, docs...)
}

var hitsDocs = []map[string]any{
	{"ts": 5, "price": 1.0, "brand": "acme"},
	{"ts": 7, "price": 3.0, "brand": "zeta"},
	{"ts": 25, "price": 5.0, "brand": "acme"},
	{"ts": 26, "brand": "acme"},
	{"ts": 99, "price": 2.5, "brand": "zeta"},
	{"ts": 150, "price": 9.0, "brand": "acme"},
}

func hitsManyDocs(n int) []map[string]any {
	docs := make([]map[string]any, n)
	for i := range docs {
		docs[i] = map[string]any{"ts": i % 100, "brand": "acme"}
	}
	return docs
}

func hitsRun(t *testing.T, d *Datasource, model map[string]any, from, to int64) backend.DataResponse {
	model["timeField"] = "ts"
	raw, _ := json.Marshal(model)
	var qm queryModel
	if err := json.Unmarshal(raw, &qm); err != nil {
		t.Fatal(err)
	}
	return d.hitsSeries(t.Context(), qm, backend.DataQuery{
		JSON:      raw,
		Interval:  10 * time.Second,
		TimeRange: backend.TimeRange{From: time.Unix(from, 0), To: time.Unix(to, 0)},
	})
}

func hitsStub(t *testing.T, found, hits int, document string, cutoff bool) string {
	documents := strings.TrimSuffix(strings.Repeat(`{"document":`+document+`},`, hits), ",")
	return stubSearch(t, fmt.Sprintf(`{"found":%d,"hits":[%s],"search_cutoff":%t}`, found, documents, cutoff))
}

func TestHitsSeriesCountsHitsPerBucket(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			name := hitsCollection(t, url, hitsDocs)
			frame := singleFrame(t, hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": name}, 0, 100))
			assertJSONEqual(t, seriesCounts(frame), []int64{2, 0, 2, 0, 0, 0, 0, 0, 0, 1})
			if frame.Name != name || !frame.Fields[0].At(9).(time.Time).Equal(time.Unix(90, 0)) {
				t.Fatalf("got frame %q ending %v", frame.Name, frame.Fields[0].At(9))
			}
			assertJSONEqual(t, fieldNames(frame), []string{"time", "count"})
		})
	}
}

func TestHitsSeriesStatsPerBucketIgnoreHitsWithoutValue(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			name := hitsCollection(t, url, hitsDocs)
			frame := singleFrame(t, hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": name, "stats": true, "valueField": "price"}, 0, 30))
			assertJSONEqual(t, frameRows(t, frame), [][]any{
				{time.Unix(0, 0).UTC(), 2, 4, 2, 1, 3},
				{time.Unix(10, 0).UTC(), 0, nil, nil, nil, nil},
				{time.Unix(20, 0).UTC(), 2, 5, 5, 5, 5},
			})
		})
	}
}

func TestHitsSeriesAppliesFilterBy(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			name := hitsCollection(t, url, hitsDocs)
			frame := singleFrame(t, hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": name, "filterBy": "brand:=acme"}, 0, 100))
			assertJSONEqual(t, seriesCounts(frame), []int64{1, 0, 2, 0, 0, 0, 0, 0, 0, 0})
		})
	}
}

func TestHitsSeriesPagesThroughAllHits(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			name := hitsCollection(t, url, hitsManyDocs(600))
			frame := singleFrame(t, hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": name}, 0, 100))
			assertJSONEqual(t, seriesCounts(frame), []int64{60, 60, 60, 60, 60, 60, 60, 60, 60, 60})
			if notices := seriesNotices(frame); notices != nil {
				t.Fatalf("got notices %v", notices)
			}
		})
	}
}

func TestHitsSeriesMaxPagesReachedAddsNotice(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			name := hitsCollection(t, url, hitsManyDocs(600))
			frame := singleFrame(t, hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": name, "maxPages": 1}, 0, 100))
			if total := seriesSum(seriesCounts(frame)); total != 250 {
				t.Fatalf("bucketed %d hits", total)
			}
			assertJSONEqual(t, seriesNotices(frame), []data.Notice{{Severity: data.NoticeSeverityWarning, Text: "Bucketed 250 of 600 hits (max pages reached)"}})
		})
	}
}

func TestHitsSeriesFallsBackToSettingsMaxPages(t *testing.T) {
	url := typesenseServers[0]
	name := hitsCollection(t, url, hitsManyDocs(600))
	d := testDatasource(url, adminKey)
	d.settings.MaxPages = 2
	frame := singleFrame(t, hitsRun(t, d, map[string]any{"collection": name}, 0, 100))
	assertJSONEqual(t, seriesNotices(frame), []data.Notice{{Severity: data.NoticeSeverityWarning, Text: "Bucketed 500 of 600 hits (max pages reached)"}})
}

func TestHitsSeriesEmptyOrInvertedRangeGivesNoFrames(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			name := hitsCollection(t, url, hitsDocs)
			for _, r := range [][2]int64{{50, 50}, {60, 50}} {
				res := hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": name}, r[0], r[1])
				if res.Error != nil || len(res.Frames) != 0 {
					t.Fatalf("range %v: got error %v, %d frames", r, res.Error, len(res.Frames))
				}
			}
		})
	}
}

func TestHitsSeriesPlanErrorFailsQuery(t *testing.T) {
	res := hitsRun(t, testDatasource(typesenseServers[0], adminKey), map[string]any{"collection": "unused", "minInterval": "soon"}, 0, 100)
	assertQueryError(t, res, "Invalid min interval: soon")
}

func TestHitsSeriesSearchErrorFailsQuery(t *testing.T) {
	res := hitsRun(t, testDatasource(typesenseServers[0], adminKey), map[string]any{"collection": "test_hits_missing"}, 0, 100)
	if res.Error == nil || res.Status != backend.StatusBadRequest {
		t.Fatalf("got error %v, status %d", res.Error, res.Status)
	}
}

func TestHitsSeriesSearchCutoffNoticeOnce(t *testing.T) {
	url := hitsStub(t, 500, 250, `{"ts":5}`, true)
	frame := singleFrame(t, hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": "books"}, 0, 20))
	assertJSONEqual(t, seriesCounts(frame), []int64{500, 0})
	assertJSONEqual(t, seriesNotices(frame), []data.Notice{searchCutoffNotice})
}

func TestHitsSeriesStopsPagingOnShortPage(t *testing.T) {
	url := hitsStub(t, 1000, 1, `{"ts":5}`, false)
	frame := singleFrame(t, hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": "books"}, 0, 20))
	assertJSONEqual(t, seriesCounts(frame), []int64{1, 0})
}

func TestHitsSeriesNonNumericValueFailsQuery(t *testing.T) {
	for _, value := range []string{`"cheap"`, `true`, `{"amount":1}`, `1e400`} {
		url := hitsStub(t, 1, 1, `{"ts":5,"price":`+value+`}`, false)
		res := hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": "books", "stats": true, "valueField": "price"}, 0, 20)
		assertQueryError(t, res, `Value field "price" is not numeric`)
	}
}

func TestHitsSeriesNonNumericTimeFailsQuery(t *testing.T) {
	for _, document := range []string{`{"price":1}`, `{"ts":"soon"}`, `{"ts":1e400}`} {
		url := hitsStub(t, 1, 1, document, false)
		res := hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": "books"}, 0, 20)
		assertQueryError(t, res, `Time field "ts" is not a numeric timestamp`)
	}
}

func TestHitsSeriesNullValueCountsWithoutStats(t *testing.T) {
	url := hitsStub(t, 1, 1, `{"ts":5,"price":null}`, false)
	frame := singleFrame(t, hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": "books", "stats": true, "valueField": "price"}, 0, 20))
	assertJSONEqual(t, frameRows(t, frame), [][]any{
		{time.Unix(0, 0).UTC(), 1, nil, nil, nil, nil},
		{time.Unix(10, 0).UTC(), 0, nil, nil, nil, nil},
	})
}

func TestHitsSeriesIgnoresBuilderGroupFacetAndSort(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			name := hitsCollection(t, url, hitsDocs)
			frame := singleFrame(t, hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": name, "groupBy": "brand", "facetBy": "missing_field", "sortBy": "missing_field:desc"}, 0, 100))
			assertJSONEqual(t, seriesCounts(frame), []int64{2, 0, 2, 0, 0, 0, 0, 0, 0, 1})
		})
	}
}

func TestHitsSeriesSkipsHitsOutsideTheRange(t *testing.T) {
	url := stubSearch(t, `{"found":3,"hits":[{"document":{"ts":5}},{"document":{"ts":25}},{"document":{"ts":150}}]}`)
	frame := singleFrame(t, hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": "books"}, 10, 30))
	assertJSONEqual(t, seriesCounts(frame), []int64{0, 1})
}

func TestHitsSeriesNonPositiveMaxPagesFallsBackToSettings(t *testing.T) {
	url := typesenseServers[0]
	name := hitsCollection(t, url, hitsManyDocs(600))
	d := testDatasource(url, adminKey)
	d.settings.MaxPages = 2
	frame := singleFrame(t, hitsRun(t, d, map[string]any{"collection": name, "maxPages": -1}, 0, 100))
	assertJSONEqual(t, seriesNotices(frame), []data.Notice{{Severity: data.NoticeSeverityWarning, Text: "Bucketed 500 of 600 hits (max pages reached)"}})
}

func TestHitsSeriesRequestsOnlyTimeAndValueFields(t *testing.T) {
	for stats, want := range map[bool]string{false: "ts", true: "ts,price"} {
		url, sent := stubSearchCapture(t, `{"found":0,"hits":[]}`)
		singleFrame(t, hitsRun(t, testDatasource(url, adminKey), map[string]any{"collection": "books", "stats": stats, "valueField": "price"}, 0, 20))
		if got := (*sent)[0]["include_fields"]; got != want {
			t.Fatalf("stats %v: include_fields %v, want %q", stats, got, want)
		}
	}
}
