package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cliftonz/typesense-datasource/pkg/models"
	"github.com/cliftonz/typesense-datasource/pkg/typesense"
)

const adminKey = "test-key"

var typesenseServers = []string{"http://localhost:8107", "http://localhost:8109"}

var testCollections atomic.Int64

func testDatasource(url, apiKey string) *Datasource {
	return &Datasource{
		settings: &models.PluginSettings{URL: url, TimeUnit: "s", MaxPages: 40, Secrets: &models.SecretPluginSettings{ApiKey: apiKey}},
		client:   typesense.New(url, apiKey, http.DefaultClient),
	}
}

func testCollection(t *testing.T, url, kind string, fields []map[string]any, docs ...map[string]any) string {
	name := fmt.Sprintf("test_%s_%d_%d", kind, time.Now().UnixNano(), testCollections.Add(1))
	adminRequest(t, http.MethodPost, url+"/collections", map[string]any{"name": name, "fields": fields})
	t.Cleanup(func() { adminRequest(t, http.MethodDelete, url+"/collections/"+name, nil) })
	if len(docs) > 0 {
		importDocs(t, url, name, docs...)
	}
	return name
}

func importDocs(t *testing.T, url, name string, docs ...map[string]any) {
	lines := make([]string, len(docs))
	for i, doc := range docs {
		line, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		lines[i] = string(line)
	}
	req, err := http.NewRequest(http.MethodPost, url+"/collections/"+name+"/documents/import?action=create", strings.NewReader(strings.Join(lines, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-TYPESENSE-API-KEY", adminKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}

func tableCollection(t *testing.T, url string) string {
	return testCollection(t, url, "table", []map[string]any{
		{"name": "brand", "type": "string", "facet": true},
		{"name": "ts", "type": "int64"},
		{"name": "price", "type": "float"},
	},
		map[string]any{"id": "1", "brand": "acme", "ts": 1000, "price": 1.5},
		map[string]any{"id": "2", "brand": "acme", "ts": 2000, "price": 2.5},
		map[string]any{"id": "3", "brand": "zeta", "ts": 3000, "price": 3.5},
	)
}

func adminRequest(t *testing.T, method, url string, body any) []byte {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-TYPESENSE-API-KEY", adminKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out bytes.Buffer
	if _, err := out.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode >= 300 {
		t.Fatalf("%s %s: status %d: %s", method, url, resp.StatusCode, out.String())
	}
	return out.Bytes()
}

type stubResponse struct {
	status int
	body   string
}

func stubTypesense(t *testing.T, routes map[string]stubResponse) string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(routes[r.URL.Path].status)
		if _, err := w.Write([]byte(routes[r.URL.Path].body)); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func stubSearch(t *testing.T, result string) string {
	return stubTypesense(t, map[string]stubResponse{"/multi_search": {200, `{"results":[` + result + `]}`}})
}

func stubSearchCapture(t *testing.T, result string) (string, *[]map[string]any) {
	var sent []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Searches []map[string]any `json:"searches"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		sent = append(sent, body.Searches...)
		if _, err := w.Write([]byte(`{"results":[` + strings.TrimSuffix(strings.Repeat(result+",", len(body.Searches)), ",") + `]}`)); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &sent
}

func createSearchOnlyKey(t *testing.T, url string) string {
	var key struct {
		ID    int    `json:"id"`
		Value string `json:"value"`
	}
	created := adminRequest(t, http.MethodPost, url+"/keys", map[string]any{
		"description": "t", "actions": []string{"documents:search"}, "collections": []string{"*"},
	})
	if err := json.Unmarshal(created, &key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		adminRequest(t, http.MethodDelete, fmt.Sprintf("%s/keys/%d", url, key.ID), nil)
	})
	return key.Value
}
