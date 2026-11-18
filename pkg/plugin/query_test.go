package plugin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func runQueries(t *testing.T, d *Datasource, queries ...backend.DataQuery) *backend.QueryDataResponse {
	resp, err := d.QueryData(t.Context(), &backend.QueryDataRequest{Queries: queries})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func builderQuery(t *testing.T, refID string, model map[string]any) backend.DataQuery {
	model["editorMode"] = "builder"
	model["format"] = "table"
	raw, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	return backend.DataQuery{
		RefID:     refID,
		JSON:      raw,
		TimeRange: backend.TimeRange{From: time.Unix(0, 0), To: time.Unix(10000, 0)},
	}
}

func frameRows(t *testing.T, f *data.Frame) [][]any {
	rows := make([][]any, f.Rows())
	for i := range rows {
		for _, field := range f.Fields {
			v, ok := field.ConcreteAt(i)
			if !ok {
				v = nil
			}
			rows[i] = append(rows[i], v)
		}
	}
	return rows
}

func fieldNames(f *data.Frame) []string {
	names := make([]string, len(f.Fields))
	for i, field := range f.Fields {
		names[i] = field.Name
	}
	return names
}

func assertJSONEqual(t *testing.T, got, want any) {
	t.Helper()
	g, _ := json.Marshal(got)
	w, _ := json.Marshal(want)
	if string(g) != string(w) {
		t.Fatalf("got %s, want %s", g, w)
	}
}

func assertQueryError(t *testing.T, res backend.DataResponse, want string) {
	t.Helper()
	if res.Error == nil || res.Error.Error() != want || res.Status != backend.StatusBadRequest {
		t.Fatalf("got error %v status %v, want %q", res.Error, res.Status, want)
	}
}

func TestQueryBuilderTableReturnsHitsFrameNamedAfterCollection(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			resp := runQueries(t, testDatasource(url, adminKey), builderQuery(t, "A", map[string]any{"collection": collection, "q": "acme", "queryBy": "brand", "sortBy": "ts:desc", "perPage": 1}))
			res := resp.Responses["A"]
			if res.Error != nil || len(res.Frames) != 1 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			frame := res.Frames[0]
			if frame.Name != collection {
				t.Fatalf("frame name %q", frame.Name)
			}
			assertJSONEqual(t, fieldNames(frame), []string{"brand", "id", "price", "ts"})
			assertJSONEqual(t, frameRows(t, frame), [][]any{{"acme", "2", 2.5, 2000}})
		})
	}
}

func TestQueryBuilderTablePartialPageAddsShowingNotice(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			res := runQueries(t, testDatasource(url, adminKey), builderQuery(t, "A", map[string]any{"collection": collection, "q": "*", "perPage": 1})).Responses["A"]
			if res.Error != nil || len(res.Frames) != 1 || res.Frames[0].Meta == nil {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			assertJSONEqual(t, res.Frames[0].Meta.Notices, []data.Notice{{Severity: data.NoticeSeverityInfo, Text: "Showing 1 of 3 hits"}})
		})
	}
}

func TestQueryBuilderTableFullPageHasNoNotice(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			res := runQueries(t, testDatasource(url, adminKey), builderQuery(t, "A", map[string]any{"collection": collection, "q": "*"})).Responses["A"]
			if res.Error != nil || len(res.Frames) != 1 || res.Frames[0].Meta != nil {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
		})
	}
}

func TestQueryBuilderTableTimeFieldFiltersByRangeAndBecomesTimeColumn(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			q := builderQuery(t, "A", map[string]any{"collection": collection, "q": "*", "filterBy": "brand:acme", "timeField": "ts"})
			q.TimeRange = backend.TimeRange{From: time.Unix(1500, 0), To: time.Unix(10000, 0)}
			res := runQueries(t, testDatasource(url, adminKey), q).Responses["A"]
			if res.Error != nil || len(res.Frames) != 1 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			ts, _ := res.Frames[0].FieldByName("ts")
			if ts.Type() != data.FieldTypeNullableTime || ts.Len() != 1 {
				t.Fatalf("ts column %v with %d rows", ts.Type(), ts.Len())
			}
			if got, _ := ts.ConcreteAt(0); !got.(time.Time).Equal(time.Unix(2000, 0)) {
				t.Fatalf("ts %v", got)
			}
		})
	}
}

func TestQueryBuilderTableTimeFieldInMillisecondsBecomesTimeColumn(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			q := builderQuery(t, "A", map[string]any{"collection": collection, "q": "*", "timeField": "ts"})
			q.TimeRange = backend.TimeRange{From: time.UnixMilli(1500), To: time.UnixMilli(2500)}
			d := testDatasource(url, adminKey)
			d.settings.TimeUnit = "ms"
			res := runQueries(t, d, q).Responses["A"]
			if res.Error != nil || len(res.Frames) != 1 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			ts, _ := res.Frames[0].FieldByName("ts")
			if got, _ := ts.ConcreteAt(0); ts.Len() != 1 || !got.(time.Time).Equal(time.UnixMilli(2000)) {
				t.Fatalf("ts column %v with %d rows", ts.Type(), ts.Len())
			}
		})
	}
}

func TestQueryBuilderTableFloatTimeFieldBecomesTimeColumn(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			q := builderQuery(t, "A", map[string]any{"collection": collection, "q": "*", "timeField": "price", "sortBy": "price:asc"})
			res := runQueries(t, testDatasource(url, adminKey), q).Responses["A"]
			if res.Error != nil || len(res.Frames) != 1 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			price, _ := res.Frames[0].FieldByName("price")
			if price.Type() != data.FieldTypeNullableTime || price.Len() != 3 {
				t.Fatalf("price column %v with %d rows", price.Type(), price.Len())
			}
			if got, _ := price.ConcreteAt(0); !got.(time.Time).Equal(time.Unix(1, 500_000_000)) {
				t.Fatalf("price %v", got)
			}
		})
	}
}

func TestQueryBuilderTableHitWithoutTimeFieldGetsNullTime(t *testing.T) {
	url := stubSearch(t, `{"found":2,"hits":[{"document":{"id":"1","ts":1000}},{"document":{"id":"2"}}]}`)
	res := runQueries(t, testDatasource(url, adminKey), builderQuery(t, "A", map[string]any{"collection": "books", "timeField": "ts"})).Responses["A"]
	if res.Error != nil || len(res.Frames) != 1 {
		t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
	}
	ts, _ := res.Frames[0].FieldByName("ts")
	if ts.Type() != data.FieldTypeNullableTime || ts.At(1).(*time.Time) != nil {
		t.Fatalf("ts column %v, second row %v", ts.Type(), ts.At(1))
	}
	if got, _ := ts.ConcreteAt(0); !got.(time.Time).Equal(time.Unix(1000, 0)) {
		t.Fatalf("ts %v", got)
	}
}

func TestQueryBuilderTableNonNumericTimeFieldFailsQuery(t *testing.T) {
	for _, value := range []string{`"acme"`, `true`} {
		t.Run(value, func(t *testing.T) {
			url := stubSearch(t, `{"found":1,"hits":[{"document":{"id":"1","ts":`+value+`}}]}`)
			res := runQueries(t, testDatasource(url, adminKey), builderQuery(t, "A", map[string]any{"collection": "books", "timeField": "ts"})).Responses["A"]
			assertQueryError(t, res, `Time field "ts" is not a numeric timestamp`)
		})
	}
}

func TestQueryBuilderTableTimeFieldWithNoMatchingHitsReturnsEmptyFrame(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			q := builderQuery(t, "A", map[string]any{"collection": collection, "q": "*", "timeField": "ts"})
			q.TimeRange = backend.TimeRange{From: time.Unix(5000, 0), To: time.Unix(6000, 0)}
			res := runQueries(t, testDatasource(url, adminKey), q).Responses["A"]
			if res.Error != nil || len(res.Frames) != 1 || len(res.Frames[0].Fields) != 0 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
		})
	}
}

func TestQueryBuilderTableFacetsAddFacetFrames(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			res := runQueries(t, testDatasource(url, adminKey), builderQuery(t, "A", map[string]any{"collection": collection, "q": "*", "facetBy": "brand"})).Responses["A"]
			if res.Error != nil || len(res.Frames) != 3 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			if res.Frames[0].Name != collection || res.Frames[0].Rows() != 3 {
				t.Fatalf("first frame %q with %d rows", res.Frames[0].Name, res.Frames[0].Rows())
			}
			assertJSONEqual(t, frameRows(t, res.Frames[1]), [][]any{{"acme", 2}, {"zeta", 1}})
		})
	}
}

func TestQueryBuilderTableGroupByReplacesHitsWithGroupsFrame(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			res := runQueries(t, testDatasource(url, adminKey), builderQuery(t, "A", map[string]any{"collection": collection, "q": "*", "sortBy": "ts:asc", "groupBy": " brand "})).Responses["A"]
			if res.Error != nil || len(res.Frames) != 1 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			assertJSONEqual(t, fieldNames(res.Frames[0]), []string{"brand", "found"})
			assertJSONEqual(t, frameRows(t, res.Frames[0]), [][]any{{"acme", 2}, {"zeta", 1}})
		})
	}
}

func TestSplitTrimmedTrimsAndDropsEmptyNames(t *testing.T) {
	assertJSONEqual(t, splitTrimmed("a, b,, c"), []string{"a", "b", "c"})
}

func TestQuerySearchErrorFailsOnlyThatQuery(t *testing.T) {
	want := map[string]string{"http://localhost:8107": "Not found.", "http://localhost:8109": "Collection not found"}
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			resp := runQueries(t, testDatasource(url, adminKey),
				builderQuery(t, "A", map[string]any{"collection": "test_query_missing", "q": "*"}),
				builderQuery(t, "B", map[string]any{"collection": collection, "q": "*"}),
			)
			assertQueryError(t, resp.Responses["A"], `Search on "test_query_missing" failed: `+want[url])
			if b := resp.Responses["B"]; b.Error != nil || len(b.Frames) != 1 || b.Frames[0].Rows() != 3 {
				t.Fatalf("B: got error %v, %d frames", b.Error, len(b.Frames))
			}
		})
	}
}

func TestQueryWrongKeyReportsRejectedKey(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			res := runQueries(t, testDatasource(url, "wrong-key"), builderQuery(t, "A", map[string]any{"collection": collection, "q": "*"})).Responses["A"]
			assertQueryError(t, res, `API key was rejected or cannot search "`+collection+`"`)
		})
	}
}

func TestQueryPerSearchUnauthorizedReportsRejectedKey(t *testing.T) {
	url := stubSearch(t, `{"code":401,"error":"Forbidden - a valid x-typesense-api-key header must be sent."}`)
	res := runQueries(t, testDatasource(url, adminKey), builderQuery(t, "A", map[string]any{"collection": "books"})).Responses["A"]
	assertQueryError(t, res, `API key was rejected or cannot search "books"`)
}

func TestQueryEmptyURLReportsMissingURL(t *testing.T) {
	res := runQueries(t, testDatasource("", adminKey), builderQuery(t, "A", map[string]any{"collection": "books", "q": "*"})).Responses["A"]
	assertQueryError(t, res, "URL is missing")
}

func TestQueryMalformedJSONFailsQuery(t *testing.T) {
	res := runQueries(t, testDatasource("http://localhost:8109", adminKey), backend.DataQuery{RefID: "A", JSON: []byte(`{`)}).Responses["A"]
	if res.Error == nil || res.Status != backend.StatusBadRequest {
		t.Fatalf("got error %v status %v", res.Error, res.Status)
	}
}

func TestQueryTimeFieldWithEmptyOrInvertedRangeReturnsNoFrames(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			for _, to := range []int64{2000, 1000} {
				q := builderQuery(t, "A", map[string]any{"collection": collection, "q": "*", "timeField": "ts"})
				q.TimeRange = backend.TimeRange{From: time.Unix(2000, 0), To: time.Unix(to, 0)}
				res := runQueries(t, testDatasource(url, adminKey), q).Responses["A"]
				if res.Error != nil || len(res.Frames) != 0 {
					t.Fatalf("to=%d: got error %v, %d frames", to, res.Error, len(res.Frames))
				}
			}
		})
	}
}

func TestQuerySearchCutoffAddsWarningNotice(t *testing.T) {
	url := stubSearch(t, `{"found":1,"hits":[{"document":{"id":"1"}}],"search_cutoff":true}`)
	res := runQueries(t, testDatasource(url, adminKey), builderQuery(t, "A", map[string]any{"collection": "books", "q": "*"})).Responses["A"]
	if res.Error != nil || len(res.Frames) != 1 || res.Frames[0].Meta == nil {
		t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
	}
	assertJSONEqual(t, res.Frames[0].Meta.Notices, []data.Notice{searchCutoffNotice})
}

func TestQueryBlankQSearchesEverything(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			for _, q := range []string{"", "  "} {
				res := runQueries(t, testDatasource(url, adminKey), builderQuery(t, "A", map[string]any{"collection": collection, "q": q})).Responses["A"]
				if res.Error != nil || len(res.Frames) != 1 || res.Frames[0].Rows() != 3 {
					t.Fatalf("q=%q: got error %v, %d frames", q, res.Error, len(res.Frames))
				}
			}
		})
	}
}

func TestQueryDataRunsUpToTenQueriesAtOnce(t *testing.T) {
	var mu sync.Mutex
	inFlight, peak := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		inFlight++
		peak = max(peak, inFlight)
		mu.Unlock()
		time.Sleep(200 * time.Millisecond)
		mu.Lock()
		inFlight--
		mu.Unlock()
		w.Write([]byte(`{"results":[{"found":0,"hits":[]}]}`))
	}))
	t.Cleanup(srv.Close)
	queries := make([]backend.DataQuery, 20)
	for i := range queries {
		queries[i] = builderQuery(t, fmt.Sprint(i), map[string]any{"collection": "books"})
	}
	resp := runQueries(t, testDatasource(srv.URL, adminKey), queries...)
	if len(resp.Responses) != 20 || peak != 10 {
		t.Fatalf("%d responses, peak %d concurrent searches", len(resp.Responses), peak)
	}
}

func TestQueryBuilderTablePartialGroupPageAddsShowingGroupsNotice(t *testing.T) {
	want := map[string][]data.Notice{
		"http://localhost:8107": {{Severity: data.NoticeSeverityInfo, Text: "Showing 1 group; more may exist"}},
		"http://localhost:8109": {{Severity: data.NoticeSeverityInfo, Text: "Showing 1 of 2 groups"}},
	}
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			res := runQueries(t, testDatasource(url, adminKey), builderQuery(t, "A", map[string]any{"collection": collection, "q": "*", "groupBy": "brand", "perPage": 1})).Responses["A"]
			if res.Error != nil || len(res.Frames) != 1 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			var notices []data.Notice
			if res.Frames[0].Meta != nil {
				notices = res.Frames[0].Meta.Notices
			}
			assertJSONEqual(t, notices, want[url])
		})
	}
}

func TestQueryUnknownFormatOrEditorModeFailsQuery(t *testing.T) {
	url := stubSearch(t, `{"found":0,"hits":[]}`)
	cases := []struct {
		model map[string]any
		want  string
	}{
		{map[string]any{"format": "graph"}, "Unknown format graph"},
		{map[string]any{"editorMode": "visual"}, "Unknown editor mode visual"},
	}
	for _, c := range cases {
		c.model["collection"] = "books"
		raw, _ := json.Marshal(c.model)
		res := runQueries(t, testDatasource(url, adminKey), backend.DataQuery{RefID: "A", JSON: raw}).Responses["A"]
		assertQueryError(t, res, c.want)
	}
}

func TestQueryBuilderTableFullDefaultGroupPageSaysMoreMayExist(t *testing.T) {
	groups := strings.TrimSuffix(strings.Repeat(`{"group_key":["acme"],"found":1,"hits":[]},`, 10), ",")
	url := stubSearch(t, `{"found":10,"grouped_hits":[`+groups+`]}`)
	res := runQueries(t, testDatasource(url, adminKey), builderQuery(t, "A", map[string]any{"collection": "books", "groupBy": "brand"})).Responses["A"]
	if res.Error != nil || len(res.Frames) != 1 || res.Frames[0].Meta == nil {
		t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
	}
	assertJSONEqual(t, res.Frames[0].Meta.Notices, []data.Notice{{Severity: data.NoticeSeverityInfo, Text: "Showing 10 groups; more may exist"}})
}

func timeSeriesQuery(t *testing.T, url string, model map[string]any) backend.DataResponse {
	model["editorMode"] = "builder"
	model["format"] = "timeseries"
	raw, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	dq := groupByDataQuery(4, time.Hour)
	dq.JSON = raw
	return testDatasource(url, adminKey).query(t.Context(), dq)
}

func TestQueryTimeSeriesGroupByModeNeedsNoTimeField(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := groupByCollection(t, url, groupByHourDocs())
			frame := singleFrame(t, timeSeriesQuery(t, url, map[string]any{"bucketMode": "groupBy", "collection": collection, "groupBy": "hour"}))
			assertJSONEqual(t, seriesCounts(frame), []int64{3, 1, 0, 2})
		})
	}
}

func TestQueryTimeSeriesEmptyBucketModeMeansHitsMode(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := groupByCollection(t, url, groupByHourDocs())
			empty := singleFrame(t, timeSeriesQuery(t, url, map[string]any{"bucketMode": "", "collection": collection, "timeField": "hour"}))
			hits := singleFrame(t, timeSeriesQuery(t, url, map[string]any{"bucketMode": "hits", "collection": collection, "timeField": "hour"}))
			assertJSONEqual(t, empty, hits)
			assertJSONEqual(t, seriesCounts(empty), []int64{3, 1, 0, 2})
		})
	}
}

func TestQueryTimeSeriesSharedChecks(t *testing.T) {
	cases := []struct {
		name  string
		model map[string]any
		want  string
	}{
		{"hits needs time field", map[string]any{"bucketMode": "hits"}, "Time field is required for time series"},
		{"empty bucket mode needs time field", map[string]any{}, "Time field is required for time series"},
		{"range facet needs time field", map[string]any{"bucketMode": "rangeFacet"}, "Time field is required for time series"},
		{"hits stats need value field", map[string]any{"bucketMode": "hits", "timeField": "hour", "stats": true}, "Value field is required for stats"},
		{"range facet stats need value field", map[string]any{"bucketMode": "rangeFacet", "timeField": "hour", "stats": true}, "Value field is required for stats"},
		{"unknown bucket mode", map[string]any{"bucketMode": "bogus", "timeField": "hour"}, "Unknown bucket mode bogus"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.model["collection"] = "x"
			res := timeSeriesQuery(t, "http://localhost:8107", c.model)
			if res.Error == nil || res.Error.Error() != c.want || len(res.Frames) != 0 {
				t.Fatalf("got %v, %d frames", res.Error, len(res.Frames))
			}
		})
	}
}

func TestQueryTimeSeriesGroupByModeIgnoresStatsAndValueField(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := groupByCollection(t, url, groupByHourDocs())
			frame := singleFrame(t, timeSeriesQuery(t, url, map[string]any{"bucketMode": "groupBy", "collection": collection, "groupBy": "hour", "stats": true, "valueField": "brand"}))
			assertJSONEqual(t, fieldNames(frame), []string{"time", "count"})
			assertJSONEqual(t, seriesCounts(frame), []int64{3, 1, 0, 2})
		})
	}
}

func TestQueryTimeSeriesHitsModeBucketsHits(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			name := hitsCollection(t, url, hitsDocs)
			raw, _ := json.Marshal(map[string]any{"editorMode": "builder", "format": "timeseries", "bucketMode": "hits", "collection": name, "timeField": "ts", "stats": true, "valueField": "price"})
			res := runQueries(t, testDatasource(url, adminKey), backend.DataQuery{
				RefID:     "A",
				JSON:      raw,
				Interval:  10 * time.Second,
				TimeRange: backend.TimeRange{From: time.Unix(0, 0), To: time.Unix(30, 0)},
			}).Responses["A"]
			frame := singleFrame(t, res)
			assertJSONEqual(t, seriesCounts(frame), []int64{2, 0, 2})
			assertJSONEqual(t, fieldNames(frame), []string{"time", "count", "sum", "avg", "min", "max"})
		})
	}
}

func TestQueryTimeSeriesRangeFacetModeReturnsSeriesFrame(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := rangeFacetCollection(t, url)
			_, counts := rangeFacetQuery(t, map[string]any{"collection": collection, "timeField": "ts"}, 180, time.Minute)
			_, stats := rangeFacetQuery(t, map[string]any{"collection": collection, "timeField": "ts", "valueField": "price", "stats": true}, 180, time.Minute)
			_, empty := rangeFacetQuery(t, map[string]any{"collection": collection, "timeField": "ts"}, 0, time.Minute)
			counts.RefID, stats.RefID, empty.RefID = "A", "B", "C"
			resp := runQueries(t, testDatasource(url, adminKey), counts, stats, empty)
			a, b, c := resp.Responses["A"], resp.Responses["B"], resp.Responses["C"]
			if a.Error != nil || b.Error != nil || c.Error != nil || len(a.Frames) != 1 || len(b.Frames) != 1 || len(c.Frames) != 0 {
				t.Fatalf("got A %v %d frames, B %v %d frames, C %v %d frames", a.Error, len(a.Frames), b.Error, len(b.Frames), c.Error, len(c.Frames))
			}
			if a.Frames[0].Name != collection {
				t.Fatalf("got name %q", a.Frames[0].Name)
			}
			start := rangeFacetStart
			assertJSONEqual(t, frameRows(t, a.Frames[0]), [][]any{{start, 2}, {start.Add(time.Minute), 1}, {start.Add(2 * time.Minute), 1}})
			assertJSONEqual(t, frameRows(t, b.Frames[0]), [][]any{{start, 2, 3, 1.5, 0, 3}, {start.Add(time.Minute), 1, 1, 1, 1, 1}, {start.Add(2 * time.Minute), 1, 2, 2, 2, 2}})
		})
	}
}

func TestQueryTimeSeriesRangeFacetModeNoticesReachTheFrame(t *testing.T) {
	url := stubTypesense(t, map[string]stubResponse{"/multi_search": {200, `{"results":[{"found":1,"search_cutoff":true,"facet_counts":[{"field_name":"ts","counts":[{"value":"b0","count":1}],"stats":{}}]}]}`}})
	_, dq := rangeFacetQuery(t, map[string]any{"collection": "books", "timeField": "ts"}, 60, time.Minute)
	res := runQueries(t, testDatasource(url, adminKey), dq).Responses["A"]
	if res.Error != nil || len(res.Frames) != 1 {
		t.Fatalf("got %v, %d frames", res.Error, len(res.Frames))
	}
	assertJSONEqual(t, res.Frames[0].Meta.Notices, []data.Notice{searchCutoffNotice})
}

func TestQueryDataPanicFailsOnlyThatQuery(t *testing.T) {
	d := testDatasource(stubTypesense(t, map[string]stubResponse{"/multi_search": {200, `{"results":[]}`}}), adminKey)
	empty := builderQuery(t, "B", map[string]any{"collection": "books", "timeField": "ts"})
	empty.TimeRange = backend.TimeRange{From: time.Unix(10, 0), To: time.Unix(10, 0)}
	resp := runQueries(t, d, builderQuery(t, "A", map[string]any{"collection": "books"}), empty)
	assertQueryError(t, resp.Responses["A"], "Internal error: runtime error: index out of range [0] with length 0")
	if b := resp.Responses["B"]; b.Error != nil || len(b.Frames) != 0 {
		t.Fatalf("B: got error %v, %d frames", b.Error, len(b.Frames))
	}
}

func TestQueryTimeSeriesRangeWithinOneTimeUnitGivesNoFrames(t *testing.T) {
	url := stubSearch(t, `{"found":0,"hits":[],"grouped_hits":[],"facet_counts":[{"field_name":"ts","counts":[],"stats":{}}]}`)
	for _, mode := range []string{"hits", "rangeFacet", "groupBy"} {
		raw, _ := json.Marshal(map[string]any{"editorMode": "builder", "format": "timeseries", "bucketMode": mode, "collection": "books", "timeField": "ts", "groupBy": "ts"})
		q := backend.DataQuery{RefID: "A", JSON: raw, Interval: 100 * time.Millisecond, TimeRange: backend.TimeRange{From: time.Unix(50, 200_000_000), To: time.Unix(50, 700_000_000)}}
		res := runQueries(t, testDatasource(url, adminKey), q).Responses["A"]
		if res.Error != nil || len(res.Frames) != 0 {
			t.Fatalf("%s: got error %v, %d frames", mode, res.Error, len(res.Frames))
		}
	}
}
