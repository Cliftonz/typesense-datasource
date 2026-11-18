package plugin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

var rawCollectionSeq atomic.Int64

func runRaw(t *testing.T, d *Datasource, body string, from, to time.Time) backend.DataResponse {
	return d.rawTables(t.Context(), body, backend.TimeRange{From: from, To: to})
}

func TestRawMacrosFilterByRangeInSeconds(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			body := `{"searches":[{"collection":"` + collection + `","q":"*","filter_by":"ts:>=$__fromTime && ts:<$__toTime","sort_by":"ts:asc"}]}`
			res := runRaw(t, testDatasource(url, adminKey), body, time.Unix(1500, 900_000_000), time.Unix(3000, 0))
			if res.Error != nil || len(res.Frames) != 1 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			assertJSONEqual(t, frameRows(t, res.Frames[0]), [][]any{{"acme", "2", 2.5, 2000}})
		})
	}
}

func TestRawMacrosFilterByRangeInMilliseconds(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			body := `{"searches":[{"collection":"` + collection + `","q":"*","filter_by":"ts:>=$__fromTime && ts:<$__toTime","sort_by":"ts:asc"}]}`
			d := testDatasource(url, adminKey)
			d.settings.TimeUnit = "ms"
			res := runRaw(t, d, body, time.UnixMilli(1500), time.UnixMilli(3000))
			if res.Error != nil || len(res.Frames) != 1 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			assertJSONEqual(t, frameRows(t, res.Frames[0]), [][]any{{"acme", "2", 2.5, 2000}})
		})
	}
}

func TestRawHitsTableNamedAfterCollection(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			body := `{"searches":[{"collection":"` + collection + `","q":"zeta","query_by":"brand"}]}`
			res := runRaw(t, testDatasource(url, adminKey), body, time.Unix(0, 0), time.Unix(10000, 0))
			if res.Error != nil || len(res.Frames) != 1 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			if res.Frames[0].Name != collection {
				t.Fatalf("frame name %q", res.Frames[0].Name)
			}
			assertJSONEqual(t, fieldNames(res.Frames[0]), []string{"brand", "id", "price", "ts"})
			assertJSONEqual(t, frameRows(t, res.Frames[0]), [][]any{{"zeta", "3", 3.5, 3000}})
		})
	}
}

func TestRawTwoSearchesGiveTwoTables(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			first, second := tableCollection(t, url), tableCollection(t, url)
			body := `{"searches":[{"collection":"` + first + `","q":"zeta","query_by":"brand"},{"collection":"` + second + `","q":"*","filter_by":"ts:=1000"}]}`
			res := runRaw(t, testDatasource(url, adminKey), body, time.Unix(0, 0), time.Unix(10000, 0))
			if res.Error != nil || len(res.Frames) != 2 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			assertJSONEqual(t, []string{res.Frames[0].Name, res.Frames[1].Name}, []string{first, second})
			assertJSONEqual(t, frameRows(t, res.Frames[0]), [][]any{{"zeta", "3", 3.5, 3000}})
			assertJSONEqual(t, frameRows(t, res.Frames[1]), [][]any{{"acme", "1", 1.5, 1000}})
		})
	}
}

func TestRawFacetSearchGivesFacetTables(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			body := `{"searches":[{"collection":"` + collection + `","q":"*","facet_by":"brand"}]}`
			res := runRaw(t, testDatasource(url, adminKey), body, time.Unix(0, 0), time.Unix(10000, 0))
			if res.Error != nil || len(res.Frames) != 2 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			assertJSONEqual(t, []string{res.Frames[0].Name, res.Frames[1].Name}, []string{"brand", "facet stats"})
			assertJSONEqual(t, frameRows(t, res.Frames[0]), [][]any{{"acme", 2}, {"zeta", 1}})
		})
	}
}

func TestRawGroupBySearchGivesGroupsTable(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			body := `{"searches":[{"collection":"` + collection + `","q":"*","group_by":" brand, ","sort_by":"ts:asc"}]}`
			res := runRaw(t, testDatasource(url, adminKey), body, time.Unix(0, 0), time.Unix(10000, 0))
			if res.Error != nil || len(res.Frames) != 1 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			if res.Frames[0].Name != collection {
				t.Fatalf("frame name %q", res.Frames[0].Name)
			}
			assertJSONEqual(t, fieldNames(res.Frames[0]), []string{"brand", "found"})
			assertJSONEqual(t, frameRows(t, res.Frames[0]), [][]any{{"acme", 2}, {"zeta", 1}})
		})
	}
}

func TestRawInvalidJSONFailsQuery(t *testing.T) {
	res := runRaw(t, testDatasource("http://localhost:1", adminKey), `{"searches":`, time.Unix(0, 0), time.Unix(10000, 0))
	if res.Error == nil || res.Error.Error() != "Invalid raw body: unexpected end of JSON input" {
		t.Fatalf("got error %v", res.Error)
	}
}

func TestRawUnionSearchFailsQuery(t *testing.T) {
	res := runRaw(t, testDatasource("http://localhost:1", adminKey), `{"union":true,"searches":[{"collection":"books","q":"*"}]}`, time.Unix(0, 0), time.Unix(10000, 0))
	if res.Error == nil || res.Error.Error() != "union search not supported" {
		t.Fatalf("got error %v", res.Error)
	}
}

func TestRawWrongKeyReportsRejectedKey(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			res := runRaw(t, testDatasource(url, "wrong-key"), `{"searches":[{"collection":"books","q":"*"},{"collection":"films","q":"*"}]}`, time.Unix(0, 0), time.Unix(10000, 0))
			if res.Error == nil || res.Error.Error() != `API key was rejected or cannot search "books"` {
				t.Fatalf("got error %v", res.Error)
			}
		})
	}
}

func TestRawRequestErrorReportsItsText(t *testing.T) {
	url := stubTypesense(t, map[string]stubResponse{"/multi_search": {500, `{"message":"boom"}`}})
	res := runRaw(t, testDatasource(url, adminKey), `{"searches":[{"collection":"books","q":"*"}]}`, time.Unix(0, 0), time.Unix(10000, 0))
	if res.Error == nil || res.Error.Error() != `Search on "books" failed: typesense: status 500: boom` {
		t.Fatalf("got error %v", res.Error)
	}
}

func TestQueryRawEditorModeRunsRawBody(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			model, _ := json.Marshal(map[string]any{"editorMode": "raw", "format": "table", "collection": "ignored", "rawBody": `{"searches":[{"collection":"` + collection + `","q":"*","filter_by":"ts:>=$__fromTime && ts:<$__toTime"}]}`})
			q := backend.DataQuery{RefID: "A", JSON: model, TimeRange: backend.TimeRange{From: time.Unix(2500, 0), To: time.Unix(10000, 0)}}
			res := runQueries(t, testDatasource(url, adminKey), q).Responses["A"]
			if res.Error != nil || len(res.Frames) != 1 || res.Frames[0].Name != collection {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
			assertJSONEqual(t, frameRows(t, res.Frames[0]), [][]any{{"zeta", "3", 3.5, 3000}})
		})
	}
}

func TestRawOneFailingSearchFailsWholeQuery(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := tableCollection(t, url)
			missing := collection + "_missing"
			body := `{"searches":[{"collection":"` + collection + `","q":"*"},{"collection":"` + missing + `","q":"*"}]}`
			res := runRaw(t, testDatasource(url, adminKey), body, time.Unix(0, 0), time.Unix(10000, 0))
			if res.Error == nil || !regexp.MustCompile(`^Search on "`+missing+`" failed: (Not found\.|Collection not found)$`).MatchString(res.Error.Error()) || len(res.Frames) != 0 {
				t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
			}
		})
	}
}

func TestRawSearchRefusedPerSearchNamesCollection(t *testing.T) {
	url := stubTypesense(t, map[string]stubResponse{"/multi_search": {200, `{"results":[{"found":0,"hits":[]},{"code":401,"error":"Forbidden - a valid x-typesense-api-key header must be sent."}]}`}})
	res := runRaw(t, testDatasource(url, adminKey), `{"searches":[{"collection":"books","q":"*"},{"collection":"films","q":"*"}]}`, time.Unix(0, 0), time.Unix(10000, 0))
	if res.Error == nil || res.Error.Error() != `API key was rejected or cannot search "films"` {
		t.Fatalf("got error %v", res.Error)
	}
}

func TestRawSearchCutoffAddsWarningNoticeOnEachCutSearchMainFrame(t *testing.T) {
	url := stubTypesense(t, map[string]stubResponse{"/multi_search": {200, `{"results":[` +
		`{"found":1,"hits":[{"document":{"id":"1"}}],"facet_counts":[],"search_cutoff":true},` +
		`{"found":1,"hits":[{"document":{"id":"1"}}],"facet_counts":[],"search_cutoff":false},` +
		`{"found":1,"hits":[],"facet_counts":[{"field_name":"brand","counts":[{"value":"acme","count":1}],"stats":{}}],"search_cutoff":true}]}`}})
	body := `{"searches":[{"collection":"books","q":"*"},{"collection":"films","q":"*"},{"collection":"songs","q":"*","facet_by":"brand"}]}`
	res := runRaw(t, testDatasource(url, adminKey), body, time.Unix(0, 0), time.Unix(10000, 0))
	if res.Error != nil || len(res.Frames) != 4 {
		t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
	}
	cutoff := []data.Notice{searchCutoffNotice}
	var notices [][]data.Notice
	for _, frame := range res.Frames {
		if frame.Meta == nil {
			notices = append(notices, nil)
			continue
		}
		notices = append(notices, frame.Meta.Notices)
	}
	assertJSONEqual(t, notices, [][]data.Notice{cutoff, nil, cutoff, nil})
}

func TestRawWithoutSearchesFailsBeforeSending(t *testing.T) {
	sent := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent++
		w.Write([]byte(`{"results":[]}`))
	}))
	t.Cleanup(srv.Close)
	for _, body := range []string{`{}`, `{"searches":null}`, `{"searches":[]}`} {
		res := runRaw(t, testDatasource(srv.URL, adminKey), body, time.Unix(0, 0), time.Unix(10000, 0))
		if res.Error == nil || res.Error.Error() != "Invalid raw body: no searches" || sent != 0 {
			t.Fatalf("body %s: got error %v after %d requests", body, res.Error, sent)
		}
	}
}

func TestRawUnionWithoutSearchesReportsUnion(t *testing.T) {
	res := runRaw(t, testDatasource("http://localhost:1", adminKey), `{"union":true}`, time.Unix(0, 0), time.Unix(10000, 0))
	if res.Error == nil || res.Error.Error() != "union search not supported" {
		t.Fatalf("got error %v", res.Error)
	}
}

func TestQueryRawEditorModeIgnoresFormat(t *testing.T) {
	url := stubSearch(t, `{"found":1,"hits":[{"document":{"id":"1"}}]}`)
	model, _ := json.Marshal(map[string]any{"editorMode": "raw", "format": "graph", "rawBody": `{"searches":[{"collection":"books","q":"*"}]}`})
	res := runQueries(t, testDatasource(url, adminKey), backend.DataQuery{RefID: "A", JSON: model}).Responses["A"]
	if res.Error != nil || len(res.Frames) != 1 || res.Frames[0].Name != "books" {
		t.Fatalf("got error %v, %d frames", res.Error, len(res.Frames))
	}
}
