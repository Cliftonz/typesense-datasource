package typesense

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

const apiKey = "test-key"

var servers = []string{"http://localhost:8107", "http://localhost:8109"}

func TestHealthOK(t *testing.T) {
	for _, url := range servers {
		t.Run(url, func(t *testing.T) {
			c := New(url, apiKey, http.DefaultClient)
			h, err := c.Health(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if !h.OK || h.ResourceError != "" {
				t.Fatalf("got %+v", h)
			}
		})
	}
}

func stubServer(t *testing.T, status int, body string) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, apiKey, srv.Client())
}

func TestHealthResourceError(t *testing.T) {
	h, err := stubServer(t, 200, `{"ok":true,"resource_error":"OUT_OF_DISK"}`).Health(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !h.OK || h.ResourceError != "OUT_OF_DISK" {
		t.Fatalf("got %+v", h)
	}
}

func TestHealthUnavailable(t *testing.T) {
	h, err := stubServer(t, 503, `{"ok":false,"resource_error":"OUT_OF_MEMORY"}`).Health(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if h.OK || h.ResourceError != "OUT_OF_MEMORY" {
		t.Fatalf("got %+v", h)
	}
}

func TestHealthUnexpectedStatus(t *testing.T) {
	_, err := stubServer(t, 500, `{"ok":true}`).Health(t.Context())
	if err == nil {
		t.Fatal("expected error")
	}
}

func adminDo(t *testing.T, method, url, body string) {
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-TYPESENSE-API-KEY", apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: %d %s", method, url, resp.StatusCode, b)
	}
}

func createCollection(t *testing.T, server, docsJSONL string) string {
	name := fmt.Sprintf("test_%d", time.Now().UnixNano())
	adminDo(t, "POST", server+"/collections", `{"name":"`+name+`","fields":[
		{"name":"title","type":"string"},
		{"name":"category","type":"string","facet":true},
		{"name":"price","type":"int32","facet":true},
		{"name":"big","type":"int64"}]}`)
	t.Cleanup(func() { adminDo(t, "DELETE", server+"/collections/"+name, "") })
	if docsJSONL != "" {
		adminDo(t, "POST", server+"/collections/"+name+"/documents/import", docsJSONL)
	}
	return name
}

func TestListCollections(t *testing.T) {
	for _, url := range servers {
		t.Run(url, func(t *testing.T) {
			name := createCollection(t, url, "")
			names, err := New(url, apiKey, http.DefaultClient).ListCollections(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(names, name) {
				t.Fatalf("%s not in %v", name, names)
			}
		})
	}
}

func TestListCollectionsUnauthorized(t *testing.T) {
	for _, url := range servers {
		t.Run(url, func(t *testing.T) {
			_, err := New(url, "wrong-key", http.DefaultClient).ListCollections(t.Context())
			if !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

const fruitDocs = `{"id":"1","title":"apple","category":"fruit","price":2,"big":9007199254740993}
{"id":"2","title":"banana","category":"fruit","price":4,"big":1}
{"id":"3","title":"carrot","category":"veg","price":6,"big":2}`

func TestMultiSearchTypedResults(t *testing.T) {
	for _, url := range servers {
		t.Run(url, func(t *testing.T) {
			name := createCollection(t, url, fruitDocs)
			body := `{"searches":[
				{"collection":"` + name + `","q":"*","facet_by":"category,price","sort_by":"price:asc"},
				{"collection":"` + name + `","q":"*","group_by":"category","sort_by":"price:asc"}]}`
			results, err := New(url, apiKey, http.DefaultClient).MultiSearch(t.Context(), []byte(body))
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 2 {
				t.Fatalf("got %d results", len(results))
			}

			flat := results[0]
			if flat.Found != 3 || len(flat.Hits) != 3 || flat.Hits[0].Document["title"] != "apple" {
				t.Fatalf("flat: %+v", flat)
			}
			if len(flat.FacetCounts) != 2 {
				t.Fatalf("facets: %+v", flat.FacetCounts)
			}
			category, price := flat.FacetCounts[0], flat.FacetCounts[1]
			if category.FieldName != "category" || category.Counts[0] != (FacetCount{Value: "fruit", Count: 2}) {
				t.Fatalf("category facet: %+v", category)
			}
			if price.FieldName != "price" || *price.Stats.TotalValues != 3 {
				t.Fatalf("price facet: %+v", price)
			}

			grouped := results[1]
			if len(grouped.GroupedHits) != 2 {
				t.Fatalf("grouped: %+v", grouped)
			}
			fruit := grouped.GroupedHits[0]
			if len(fruit.GroupKey) != 1 || fruit.GroupKey[0] != "fruit" || fruit.Found != 2 || len(fruit.Hits) != 2 {
				t.Fatalf("fruit group: %+v", fruit)
			}
		})
	}
}

func TestMultiSearchPerItemError(t *testing.T) {
	for _, url := range servers {
		t.Run(url, func(t *testing.T) {
			name := createCollection(t, url, fruitDocs)
			body := `{"searches":[
				{"collection":"` + name + `","q":"*"},
				{"collection":"` + name + `_missing","q":"*"}]}`
			results, err := New(url, apiKey, http.DefaultClient).MultiSearch(t.Context(), []byte(body))
			if err != nil {
				t.Fatal(err)
			}
			if results[0].Code != 0 || results[0].Error != "" || results[0].Found != 3 {
				t.Fatalf("ok item: %+v", results[0])
			}
			if results[1].Code != 404 || results[1].Error == "" {
				t.Fatalf("failed item: %+v", results[1])
			}
		})
	}
}

func TestMultiSearchInt64Precision(t *testing.T) {
	for _, url := range servers {
		t.Run(url, func(t *testing.T) {
			name := createCollection(t, url, fruitDocs)
			body := `{"searches":[{"collection":"` + name + `","q":"*","filter_by":"id:=1"}]}`
			results, err := New(url, apiKey, http.DefaultClient).MultiSearch(t.Context(), []byte(body))
			if err != nil {
				t.Fatal(err)
			}
			big := results[0].Hits[0].Document["big"]
			if big != json.Number("9007199254740993") {
				t.Fatalf("got %#v", big)
			}
		})
	}
}

func TestCancelledContext(t *testing.T) {
	for _, url := range servers {
		t.Run(url, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			c := New(url, apiKey, http.DefaultClient)
			if _, err := c.Health(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("health: %v", err)
			}
			if _, err := c.ListCollections(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("list: %v", err)
			}
			if _, err := c.MultiSearch(ctx, []byte(`{"searches":[]}`)); !errors.Is(err, context.Canceled) {
				t.Fatalf("multi search: %v", err)
			}
		})
	}
}

func TestTrailingSlashBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/collections" {
			t.Errorf("path %q", r.URL.Path)
		}
		w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	if _, err := New(srv.URL+"/", apiKey, srv.Client()).ListCollections(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestMultiSearchHTTPErrors(t *testing.T) {
	for _, url := range servers {
		t.Run(url, func(t *testing.T) {
			_, err := New(url, "wrong-key", http.DefaultClient).MultiSearch(t.Context(), []byte(`{"searches":[]}`))
			if !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("wrong key: %v", err)
			}
			_, err = New(url, apiKey, http.DefaultClient).MultiSearch(t.Context(), []byte(`{bad`))
			if err == nil || !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "Bad JSON.") {
				t.Fatalf("bad body: %v", err)
			}
		})
	}
}

func TestListCollectionsHTTPError(t *testing.T) {
	_, err := stubServer(t, 500, `{"message":"Internal failure."}`).ListCollections(t.Context())
	if err == nil || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "Internal failure.") {
		t.Fatalf("got %v", err)
	}
}

func TestFacetStatsNullWhenOmitted(t *testing.T) {
	for _, url := range servers {
		t.Run(url, func(t *testing.T) {
			name := createCollection(t, url, fruitDocs)
			body := `{"searches":[{"collection":"` + name + `","q":"*","facet_by":"category,price"}]}`
			results, err := New(url, apiKey, http.DefaultClient).MultiSearch(t.Context(), []byte(body))
			if err != nil {
				t.Fatal(err)
			}
			category, price := results[0].FacetCounts[0].Stats, results[0].FacetCounts[1].Stats
			if category.Min != nil || category.Max != nil || category.Sum != nil || category.Avg != nil {
				t.Fatalf("category stats: %+v", category)
			}
			if *price.Min != 2 || *price.Max != 6 || *price.Sum != 12 || *price.Avg != 4 {
				t.Fatalf("price stats: %+v", price)
			}
		})
	}
}

func TestMultiSearchContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type %q", got)
		}
		w.Write([]byte(`{"results":[]}`))
	}))
	t.Cleanup(srv.Close)
	if _, err := New(srv.URL, apiKey, srv.Client()).MultiSearch(t.Context(), []byte(`{"searches":[]}`)); err != nil {
		t.Fatal(err)
	}
}

func TestMultiSearchSearchCutoff(t *testing.T) {
	results, err := stubServer(t, 200, `{"results":[{"found":0,"hits":[],"search_cutoff":true}]}`).MultiSearch(t.Context(), []byte(`{"searches":[{}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !results[0].SearchCutoff {
		t.Fatalf("got %+v", results[0])
	}
}

func TestNonJSONErrorBody(t *testing.T) {
	body := "<html>" + strings.Repeat("a", 194) + "TRUNCATED</html>"
	c := stubServer(t, 502, body)
	_, listErr := c.ListCollections(t.Context())
	_, searchErr := c.MultiSearch(t.Context(), []byte(`{"searches":[]}`))
	for _, err := range []error{listErr, searchErr} {
		if err == nil || !strings.Contains(err.Error(), "502") || !strings.Contains(err.Error(), body[:200]) || strings.Contains(err.Error(), "TRUNCATED") {
			t.Fatalf("got %v", err)
		}
	}
}

func TestErrorBodyReadFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(500)
		w.Write([]byte("short"))
	}))
	t.Cleanup(srv.Close)
	_, err := New(srv.URL, apiKey, srv.Client()).ListCollections(t.Context())
	if err == nil || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("got %v", err)
	}
}

func TestErrorBodyCutByRunes(t *testing.T) {
	body := strings.Repeat("a", 199) + "éé" + "TRUNCATED"
	_, err := stubServer(t, 502, body).ListCollections(t.Context())
	want := "typesense: status 502: " + strings.Repeat("a", 199) + "é"
	if err == nil || err.Error() != want {
		t.Fatalf("got %q", err)
	}
}

func TestErrorBodyJSONWithoutMessage(t *testing.T) {
	_, err := stubServer(t, 500, `{"detail":"proxy says no"}`).ListCollections(t.Context())
	if err == nil || err.Error() != `typesense: status 500: {"detail":"proxy says no"}` {
		t.Fatalf("got %v", err)
	}
}

func TestHealthErrorMessages(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   string
	}{
		{503, "<html>Service Unavailable</html>", "Typesense health returned status 503: <html>Service Unavailable</html>"},
		{500, `{"ok":true}`, `Typesense health returned status 500: {"ok":true}`},
	}
	for _, tc := range cases {
		_, err := stubServer(t, tc.status, tc.body).Health(t.Context())
		if err == nil || err.Error() != tc.want {
			t.Errorf("got %v", err)
		}
	}
}

func TestNoPartialResultsOnError(t *testing.T) {
	h, err := stubServer(t, 200, `{"ok":true,"resource_error":5}`).Health(t.Context())
	if err == nil || h != (Health{}) {
		t.Errorf("health: %+v, %v", h, err)
	}
	names, err := stubServer(t, 200, `[{"name":"a"},{"name":5}]`).ListCollections(t.Context())
	if err == nil || names != nil {
		t.Errorf("list: %v, %v", names, err)
	}
	results, err := stubServer(t, 200, `{"results":[{"found":"x"},{"found":1}]}`).MultiSearch(t.Context(), []byte(`{}`))
	if err == nil || results != nil {
		t.Errorf("multi search: %+v, %v", results, err)
	}
}

func TestFacetStatsTotalValuesNullWhenOmitted(t *testing.T) {
	results, err := stubServer(t, 200, `{"results":[{"facet_counts":[{"field_name":"f","counts":[],"stats":{}}]}]}`).MultiSearch(t.Context(), []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := results[0].FacetCounts[0].Stats.TotalValues; got != nil {
		t.Fatalf("got %v", *got)
	}
}

func TestHealthBodyReadFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(503)
		w.Write([]byte("short"))
	}))
	t.Cleanup(srv.Close)
	_, err := New(srv.URL, apiKey, srv.Client()).Health(t.Context())
	if err == nil || !strings.Contains(err.Error(), "503") || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("got %v", err)
	}
}
