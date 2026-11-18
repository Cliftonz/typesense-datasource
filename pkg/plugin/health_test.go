package plugin

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func checkHealth(t *testing.T, url, apiKey, defaultCollection string) *backend.CheckHealthResult {
	d := testDatasource(url, apiKey)
	d.settings.DefaultCollection = defaultCollection
	res, err := d.CheckHealth(t.Context(), &backend.CheckHealthRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestCheckHealthAdminKeyReportsCollectionCount(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			res := checkHealth(t, url, adminKey, "")
			if res.Status != backend.HealthStatusOk || !regexp.MustCompile(`^Connected to Typesense, \d+ collections?$`).MatchString(res.Message) {
				t.Fatalf("got %v %q", res.Status, res.Message)
			}
		})
	}
}

func assertHealth(t *testing.T, res *backend.CheckHealthResult, status backend.HealthStatus, message string) {
	t.Helper()
	if res.Status != status || res.Message != message {
		t.Fatalf("got %v %q, want %v %q", res.Status, res.Message, status, message)
	}
}

func TestCheckHealthBlankKey(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			assertHealth(t, checkHealth(t, url, "", ""), backend.HealthStatusError, "API key is missing")
		})
	}
}

func TestCheckHealthUnexpectedHealthStatus(t *testing.T) {
	res := checkHealth(t, stubTypesense(t, map[string]stubResponse{"/health": {500, `{}`}}), adminKey, "")
	assertHealth(t, res, backend.HealthStatusError, "Typesense health returned status 500: {}")
}

func TestCheckHealthResourceError(t *testing.T) {
	res := checkHealth(t, stubTypesense(t, map[string]stubResponse{"/health": {503, `{"ok":false,"resource_error":"OUT_OF_MEMORY"}`}}), adminKey, "")
	assertHealth(t, res, backend.HealthStatusError, "Typesense reports OUT_OF_MEMORY")
}

func TestCheckHealthNotOK(t *testing.T) {
	res := checkHealth(t, stubTypesense(t, map[string]stubResponse{"/health": {503, `{"ok":false}`}}), adminKey, "")
	assertHealth(t, res, backend.HealthStatusError, "Typesense is not healthy")
}

func TestCheckHealthSearchOnlyKeyWithoutDefaultCollection(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			res := checkHealth(t, url, createSearchOnlyKey(t, url), "")
			assertHealth(t, res, backend.HealthStatusError, "API key cannot list collections; set a default collection to test a search-only key")
		})
	}
}

func TestCheckHealthSearchOnlyKeyWithDefaultCollection(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := testCollection(t, url, "health", []map[string]any{{"name": "title", "type": "string"}})
			res := checkHealth(t, url, createSearchOnlyKey(t, url), collection)
			assertHealth(t, res, backend.HealthStatusOk, `Connected to Typesense, search on "`+collection+`" works`)
		})
	}
}

func TestCheckHealthWrongKeyWithDefaultCollection(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := testCollection(t, url, "health", []map[string]any{{"name": "title", "type": "string"}})
			res := checkHealth(t, url, "wrong-key", collection)
			assertHealth(t, res, backend.HealthStatusError, `API key was rejected or cannot search "`+collection+`"`)
		})
	}
}

func TestCheckHealthSearchOnlyKeyWithMissingDefaultCollection(t *testing.T) {
	want := map[string]string{"http://localhost:8107": "Not found.", "http://localhost:8109": "Collection not found"}
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			res := checkHealth(t, url, createSearchOnlyKey(t, url), "test_health_missing")
			assertHealth(t, res, backend.HealthStatusError, `Search on "test_health_missing" failed: `+want[url])
		})
	}
}

func TestCheckHealthSearchError(t *testing.T) {
	url := stubTypesense(t, map[string]stubResponse{
		"/health":       {200, `{"ok":true}`},
		"/collections":  {401, `{"message":"Forbidden - a valid x-typesense-api-key header must be sent."}`},
		"/multi_search": {500, `{"message":"boom"}`},
	})
	assertHealth(t, checkHealth(t, url, adminKey, "books"), backend.HealthStatusError, `Search on "books" failed: typesense: status 500: boom`)
}

func TestCheckHealthListCollectionsError(t *testing.T) {
	url := stubTypesense(t, map[string]stubResponse{
		"/health":      {200, `{"ok":true}`},
		"/collections": {500, `{"message":"boom"}`},
	})
	assertHealth(t, checkHealth(t, url, adminKey, ""), backend.HealthStatusError, "typesense: status 500: boom")
}

func TestCheckHealthBlankURL(t *testing.T) {
	assertHealth(t, checkHealth(t, "", "", ""), backend.HealthStatusError, "URL is missing")
}

func TestCheckHealthSingleCollection(t *testing.T) {
	url := stubTypesense(t, map[string]stubResponse{
		"/health":      {200, `{"ok":true}`},
		"/collections": {200, `[{"name":"books"}]`},
	})
	assertHealth(t, checkHealth(t, url, adminKey, ""), backend.HealthStatusOk, "Connected to Typesense, 1 collection")
}

func TestCheckHealthUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	res := checkHealth(t, url, adminKey, "")
	if res.Status != backend.HealthStatusError || !strings.HasPrefix(res.Message, "Cannot reach Typesense: ") {
		t.Fatalf("got %v %q", res.Status, res.Message)
	}
}

func TestCheckHealthUnreadableHealthBody(t *testing.T) {
	url := stubTypesense(t, map[string]stubResponse{"/health": {200, `not json`}})
	assertHealth(t, checkHealth(t, url, adminKey, ""), backend.HealthStatusError, "Cannot reach Typesense: invalid character 'o' in literal null (expecting 'u')")
}

func TestCheckHealthSearchRefusedPerSearch(t *testing.T) {
	url := stubTypesense(t, map[string]stubResponse{
		"/health":       {200, `{"ok":true}`},
		"/collections":  {401, `{}`},
		"/multi_search": {200, `{"results":[{"code":401,"error":"Forbidden"}]}`},
	})
	assertHealth(t, checkHealth(t, url, adminKey, "books"), backend.HealthStatusError, `API key was rejected or cannot search "books"`)
}

func TestCheckHealthAdminKeyWithDefaultCollectionAlsoSearches(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			collection := testCollection(t, url, "health", []map[string]any{{"name": "title", "type": "string"}})
			res := checkHealth(t, url, adminKey, collection)
			want := regexp.MustCompile(`^Connected to Typesense, \d+ collections?, search on "` + collection + `" works$`)
			if res.Status != backend.HealthStatusOk || !want.MatchString(res.Message) {
				t.Fatalf("got %v %q", res.Status, res.Message)
			}
		})
	}
}

func TestCheckHealthAdminKeyWithMissingDefaultCollection(t *testing.T) {
	want := map[string]string{"http://localhost:8107": "Not found.", "http://localhost:8109": "Collection not found"}
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			res := checkHealth(t, url, adminKey, "test_health_missing")
			assertHealth(t, res, backend.HealthStatusError, `Search on "test_health_missing" failed: `+want[url])
		})
	}
}

func TestCheckHealthThroughGrafanaInstanceSettings(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			inst, err := NewDatasource(t.Context(), backend.DataSourceInstanceSettings{
				JSONData:                []byte(`{"url":"  ` + url + `/  ","timeUnit":"ms"}`),
				DecryptedSecureJSONData: map[string]string{"apiKey": adminKey},
			})
			if err != nil {
				t.Fatal(err)
			}
			res, err := inst.(*Datasource).CheckHealth(t.Context(), &backend.CheckHealthRequest{})
			if err != nil || res.Status != backend.HealthStatusOk {
				t.Fatalf("got error %v, %v %q", err, res.Status, res.Message)
			}
		})
	}
}
