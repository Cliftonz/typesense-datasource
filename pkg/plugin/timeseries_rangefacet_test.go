package plugin

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

const rangeFacetT0 = 1699999980

var rangeFacetStart = time.Unix(rangeFacetT0, 0).UTC()

func rangeFacetCollection(t *testing.T, url string) string {
	var docs []map[string]any
	for i := range 12 {
		if i != 5 {
			docs = append(docs, map[string]any{"ts": rangeFacetT0 + i*60 + 10, "seen": rangeFacetT0 + i*60 + 10, "price": i, "brand": "acme"})
		}
	}
	docs = append(docs,
		map[string]any{"ts": rangeFacetT0 + 30, "seen": rangeFacetT0 + 30, "price": 3, "brand": "zeta"},
		map[string]any{"ts": rangeFacetT0 + 720, "seen": rangeFacetT0 + 720, "price": 99, "brand": "acme"},
	)
	return testCollection(t, url, "rangefacet", []map[string]any{
		{"name": "ts", "type": "int64", "facet": true},
		{"name": "seen", "type": "int64"},
		{"name": "price", "type": "float", "facet": true},
		{"name": "brand", "type": "string"},
		{"name": "label", "type": "string", "facet": true, "optional": true},
		{"name": "weight", "type": "float", "facet": true, "optional": true},
	}, docs...)
}

func rangeFacetQuery(t *testing.T, model map[string]any, seconds int64, interval time.Duration) (queryModel, backend.DataQuery) {
	model["editorMode"] = "builder"
	model["format"] = "timeseries"
	model["bucketMode"] = "rangeFacet"
	raw, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	var qm queryModel
	if err := json.Unmarshal(raw, &qm); err != nil {
		t.Fatal(err)
	}
	return qm, backend.DataQuery{
		RefID:     "A",
		JSON:      raw,
		Interval:  interval,
		TimeRange: backend.TimeRange{From: time.Unix(rangeFacetT0, 0), To: time.Unix(rangeFacetT0+seconds, 0)},
	}
}

func rangeFacetResponse(t *testing.T, url, apiKey string, model map[string]any, seconds int64, interval time.Duration) backend.DataResponse {
	qm, dq := rangeFacetQuery(t, model, seconds, interval)
	return testDatasource(url, apiKey).rangeFacetSeries(t.Context(), qm, dq)
}

func rangeFacetRun(t *testing.T, url string, model map[string]any, seconds int64, interval time.Duration) *data.Frame {
	t.Helper()
	return singleFrame(t, rangeFacetResponse(t, url, adminKey, model, seconds, interval))
}

func rangeFacetError(t *testing.T, url, apiKey string, model map[string]any) string {
	t.Helper()
	res := rangeFacetResponse(t, url, apiKey, model, 360, time.Minute)
	if res.Error == nil || res.Status != backend.StatusBadRequest || len(res.Frames) != 0 {
		t.Fatalf("got error %v, status %d, %d frames", res.Error, res.Status, len(res.Frames))
	}
	return res.Error.Error()
}

func TestRangeFacetSeriesCountsEveryBucketWithZeroFill(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := rangeFacetCollection(t, url)
			frame := rangeFacetRun(t, url, map[string]any{"collection": collection, "timeField": "ts"}, 720, time.Minute)
			assertJSONEqual(t, fieldNames(frame), []string{"time", "count"})
			assertJSONEqual(t, seriesCounts(frame), []int64{2, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1})
			if frame.Name != collection || !frame.Fields[0].At(0).(time.Time).Equal(rangeFacetStart) {
				t.Fatalf("frame %q starts %v", frame.Name, frame.Fields[0].At(0))
			}
			if notices := seriesNotices(frame); len(notices) != 0 {
				t.Fatalf("got notices %v", notices)
			}
		})
	}
}

func TestRangeFacetSeriesAppliesBuilderFilter(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := rangeFacetCollection(t, url)
			frame := rangeFacetRun(t, url, map[string]any{"collection": collection, "timeField": "ts", "filterBy": "brand:=zeta"}, 720, time.Minute)
			assertJSONEqual(t, seriesCounts(frame), []int64{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
		})
	}
}

func TestRangeFacetSeriesStatsPerBucket(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := rangeFacetCollection(t, url)
			frame := rangeFacetRun(t, url, map[string]any{"collection": collection, "timeField": "ts", "valueField": "price", "stats": true}, 360, time.Minute)
			minute := func(i int) time.Time { return rangeFacetStart.Add(time.Duration(i) * time.Minute) }
			assertJSONEqual(t, frameRows(t, frame), [][]any{
				{minute(0), 2, 3, 1.5, 0, 3},
				{minute(1), 1, 1, 1, 1, 1},
				{minute(2), 1, 2, 2, 2, 2},
				{minute(3), 1, 3, 3, 3, 3},
				{minute(4), 1, 4, 4, 4, 4},
				{minute(5), 0, nil, nil, nil, nil},
			})
			if notices := seriesNotices(frame); len(notices) != 0 {
				t.Fatalf("got notices %v", notices)
			}
		})
	}
}

func TestRangeFacetSeriesStatsWidensToFiftyBuckets(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := rangeFacetCollection(t, url)
			frame := rangeFacetRun(t, url, map[string]any{"collection": collection, "timeField": "ts", "valueField": "price", "stats": true}, 720, time.Second)
			if counts := seriesCounts(frame); len(counts) != 48 || seriesSum(counts) != 12 {
				t.Fatalf("got %d buckets with %d documents", len(counts), seriesSum(counts))
			}
			assertJSONEqual(t, seriesNotices(frame), []data.Notice{{Severity: data.NoticeSeverityInfo, Text: "Buckets widened to 15s to stay within 50 searches"}})
		})
	}
}

func TestRangeFacetSeriesEmptyOrInvertedRangeGivesNoFrames(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := rangeFacetCollection(t, url)
			for _, seconds := range []int64{0, -120} {
				for _, stats := range []bool{false, true} {
					res := rangeFacetResponse(t, url, adminKey, map[string]any{"collection": collection, "timeField": "ts", "valueField": "price", "stats": stats}, seconds, time.Minute)
					if res.Error != nil || len(res.Frames) != 0 {
						t.Fatalf("seconds %d stats %v: got error %v, %d frames", seconds, stats, res.Error, len(res.Frames))
					}
				}
			}
		})
	}
}

func TestRangeFacetSeriesPlanErrorFailsQuery(t *testing.T) {
	got := rangeFacetError(t, "http://localhost:8109", adminKey, map[string]any{"collection": "books", "timeField": "ts", "minInterval": "soon"})
	assertJSONEqual(t, got, "Invalid min interval: soon")
}

func TestRangeFacetSeriesUnfacetedTimeFieldFailsWithTypesenseMessage(t *testing.T) {
	url := "http://localhost:8109"
	collection := rangeFacetCollection(t, url)
	got := rangeFacetError(t, url, adminKey, map[string]any{"collection": collection, "timeField": "seen"})
	if !strings.Contains(got, "Could not find a facet field named `seen` in the schema.") {
		t.Fatalf("got %q", got)
	}
}

func TestRangeFacetSeriesStatsSearchFailureNamesCollection(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := rangeFacetCollection(t, url)
			got := rangeFacetError(t, url, adminKey, map[string]any{"collection": collection, "timeField": "ts", "valueField": "brand", "stats": true})
			assertJSONEqual(t, got, `Search on "`+collection+"\" failed: Could not find a facet field named `brand` in the schema.")
		})
	}
}

func TestRangeFacetSeriesStatsWrongKeyIsRejected(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := rangeFacetCollection(t, url)
			got := rangeFacetError(t, url, "wrong-key", map[string]any{"collection": collection, "timeField": "ts", "valueField": "price", "stats": true})
			assertJSONEqual(t, got, `API key was rejected or cannot search "`+collection+`"`)
		})
	}
}

func TestRangeFacetSeriesStatsSearchRefusedPerSearch(t *testing.T) {
	url := stubTypesense(t, map[string]stubResponse{"/multi_search": {200, `{"results":[{"code":401,"error":"Forbidden"}]}`}})
	got := rangeFacetError(t, url, adminKey, map[string]any{"collection": "books", "timeField": "ts", "valueField": "price", "stats": true})
	assertJSONEqual(t, got, `API key was rejected or cannot search "books"`)
}

func TestRangeFacetSeriesStatsTransportFailureNamesCollection(t *testing.T) {
	url := stubTypesense(t, map[string]stubResponse{"/multi_search": {500, `{"message":"boom"}`}})
	got := rangeFacetError(t, url, adminKey, map[string]any{"collection": "books", "timeField": "ts", "valueField": "price", "stats": true})
	assertJSONEqual(t, got, `Search on "books" failed: typesense: status 500: boom`)
}

func TestRangeFacetSeriesStatsSearchesAreExhaustive(t *testing.T) {
	url, sent := stubSearchCapture(t, `{"found":0,"facet_counts":[{"field_name":"price","counts":[],"stats":{}}]}`)
	rangeFacetRun(t, url, map[string]any{"collection": "books", "timeField": "ts", "valueField": "price", "stats": true}, 180, time.Minute)
	if len(*sent) != 3 {
		t.Fatalf("got %d searches", len(*sent))
	}
	for _, s := range *sent {
		if s["facet_strategy"] != "exhaustive" {
			t.Fatalf("got search %v", s)
		}
	}
}

func TestRangeFacetSeriesCountsEveryMatchingDocument(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			docs := make([]map[string]any, 3000)
			for i := range docs {
				docs[i] = map[string]any{"ts": rangeFacetT0 + i%120}
			}
			collection := testCollection(t, url, "rangefacet_many", []map[string]any{{"name": "ts", "type": "int64", "facet": true}}, docs...)
			frame := rangeFacetRun(t, url, map[string]any{"collection": collection, "timeField": "ts"}, 120, time.Minute)
			assertJSONEqual(t, seriesCounts(frame), []int64{1500, 1500})
		})
	}
}

func TestRangeFacetSeriesWideningNoticeShowsExactInterval(t *testing.T) {
	url := "http://localhost:8109"
	collection := rangeFacetCollection(t, url)
	frame := rangeFacetRun(t, url, map[string]any{"collection": collection, "timeField": "ts", "valueField": "price", "stats": true}, 60*3600, 30*time.Minute)
	if step := frame.Fields[0].At(2).(time.Time).Sub(frame.Fields[0].At(1).(time.Time)); step != 90*time.Minute {
		t.Fatalf("got bucket width %v", step)
	}
	assertJSONEqual(t, seriesNotices(frame), []data.Notice{{Severity: data.NoticeSeverityInfo, Text: "Buckets widened to 90m to stay within 50 searches"}})
}

func TestExactIntervalNamesLargestWholeUnit(t *testing.T) {
	for interval, want := range map[time.Duration]string{
		48 * time.Hour:          "2d",
		2 * time.Hour:           "2h",
		90 * time.Minute:        "90m",
		15 * time.Second:        "15s",
		1500 * time.Millisecond: "1500ms",
	} {
		if got := exactInterval(interval); got != want {
			t.Errorf("%v: got %q, want %q", interval, got, want)
		}
	}
}

func TestRangeFacetSeriesStatsCoverMoreThanTenDistinctValues(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := rangeFacetCollection(t, url)
			var docs []map[string]any
			for price := 10; price <= 20; price++ {
				docs = append(docs, map[string]any{"ts": rangeFacetT0 + 40, "seen": 0, "price": price, "brand": "acme"})
			}
			importDocs(t, url, collection, docs...)
			frame := rangeFacetRun(t, url, map[string]any{"collection": collection, "timeField": "ts", "valueField": "price", "stats": true}, 60, time.Minute)
			assertJSONEqual(t, frameRows(t, frame), [][]any{{rangeFacetStart, 13, 168, 168.0 / 13, 0, 20}})
		})
	}
}

func TestRangeFacetSeriesIgnoresBuilderGroupFacetAndSort(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := rangeFacetCollection(t, url)
			importDocs(t, url, collection, map[string]any{"ts": rangeFacetT0 + 70, "seen": 0, "price": 7, "brand": "acme"})
			for _, model := range []map[string]any{{"groupBy": "brand"}, {"sortBy": "brand:asc"}, {"facetBy": "brand"}} {
				for _, stats := range []bool{false, true} {
					model["collection"], model["timeField"], model["valueField"], model["stats"] = collection, "ts", "price", stats
					frame := rangeFacetRun(t, url, model, 180, time.Minute)
					assertJSONEqual(t, seriesCounts(frame), []int64{2, 2, 1})
				}
			}
		})
	}
}

func TestRangeFacetSeriesStatsOnNonNumericFieldFailsQuery(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := rangeFacetCollection(t, url)
			importDocs(t, url, collection, map[string]any{"ts": rangeFacetT0 + 70, "seen": 0, "price": 1, "brand": "acme", "label": "red"})
			got := rangeFacetError(t, url, adminKey, map[string]any{"collection": collection, "timeField": "ts", "valueField": "label", "stats": true})
			assertJSONEqual(t, got, `Value field "label" is not numeric`)
		})
	}
}

func TestRangeFacetSeriesSearchCutoffNoticeOnce(t *testing.T) {
	results := strings.Repeat(`{"found":1,"search_cutoff":true,"facet_counts":[{"field_name":"price","counts":[],"stats":{"sum":1,"avg":1,"min":1,"max":1}}]},`, 6)
	url := stubTypesense(t, map[string]stubResponse{"/multi_search": {200, `{"results":[` + strings.TrimSuffix(results, ",") + `]}`}})
	for _, stats := range []bool{false, true} {
		frame := rangeFacetRun(t, url, map[string]any{"collection": "books", "timeField": "ts", "valueField": "price", "stats": stats}, 360, time.Minute)
		assertJSONEqual(t, seriesNotices(frame), []data.Notice{searchCutoffNotice})
	}
}

func TestRangeFacetSeriesStatsOnOptionalFieldWithoutValuesGivesNullStats(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := rangeFacetCollection(t, url)
			importDocs(t, url, collection, map[string]any{"ts": rangeFacetT0 + 75, "seen": 0, "price": 1, "brand": "acme", "weight": 4})
			frame := rangeFacetRun(t, url, map[string]any{"collection": collection, "timeField": "ts", "valueField": "weight", "stats": true}, 180, time.Minute)
			minute := func(i int) time.Time { return rangeFacetStart.Add(time.Duration(i) * time.Minute) }
			assertJSONEqual(t, frameRows(t, frame), [][]any{
				{minute(0), 2, nil, nil, nil, nil},
				{minute(1), 2, 4, 4, 4, 4},
				{minute(2), 1, nil, nil, nil, nil},
			})
		})
	}
}

func TestRangeFacetSeriesRequestsNoHits(t *testing.T) {
	for _, stats := range []bool{false, true} {
		url, sent := stubSearchCapture(t, `{"found":0,"facet_counts":[{"field_name":"ts","counts":[],"stats":{}}]}`)
		rangeFacetRun(t, url, map[string]any{"collection": "books", "timeField": "ts", "valueField": "price", "stats": stats}, 120, time.Minute)
		for _, search := range *sent {
			if search["per_page"] != 0.0 {
				t.Fatalf("stats %v: per_page %v, want 0", stats, search["per_page"])
			}
		}
	}
}
