package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type collectionsBody struct {
	CanList     bool     `json:"canList"`
	Collections []string `json:"collections"`
}

func callResource(t *testing.T, url, apiKey, path string) *backend.CallResourceResponse {
	t.Helper()
	return callResourceWithMethod(t, url, apiKey, http.MethodGet, path)
}

func callResourceWithMethod(t *testing.T, url, apiKey, method, path string) *backend.CallResourceResponse {
	t.Helper()
	ds := testDatasource(url, apiKey)
	var got *backend.CallResourceResponse
	err := ds.CallResource(context.Background(), &backend.CallResourceRequest{Method: method, Path: path},
		backend.CallResourceResponseSenderFunc(func(resp *backend.CallResourceResponse) error {
			got = resp
			return nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func decodeCollections(t *testing.T, resp *backend.CallResourceResponse) collectionsBody {
	t.Helper()
	if resp.Status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Status, resp.Body)
	}
	var body collectionsBody
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestCollectionsResourceListsCollectionsWithAdminKey(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			name := testCollection(t, url, "resources", []map[string]any{{"name": "title", "type": "string"}})

			body := decodeCollections(t, callResource(t, url, adminKey, "collections"))

			if !body.CanList || !slices.Contains(body.Collections, name) {
				t.Fatalf("expected canList true with %s, got %+v", name, body)
			}
		})
	}
}

func TestCollectionsResourceReportsCannotListWithSearchOnlyKey(t *testing.T) {
	for _, url := range typesenseServers {
		t.Run(url, func(t *testing.T) {
			body := decodeCollections(t, callResource(t, url, createSearchOnlyKey(t, url), "collections"))

			if body.CanList || body.Collections == nil || len(body.Collections) != 0 {
				t.Fatalf("expected canList false with empty list, got %+v", body)
			}
		})
	}
}

func TestCollectionsResourceReturns500WithMessageOnServerError(t *testing.T) {
	resp := callResource(t, stubTypesense(t, map[string]stubResponse{"/collections": {500, `{"message":"boom"}`}}), adminKey, "collections")

	if resp.Status != http.StatusInternalServerError || !strings.Contains(decodeMessage(t, resp), "boom") {
		t.Fatalf("expected 500 with message, got %d: %s", resp.Status, resp.Body)
	}
}

func decodeMessage(t *testing.T, resp *backend.CallResourceResponse) string {
	t.Helper()
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		t.Fatalf("expected JSON message body, got %q: %v", resp.Body, err)
	}
	return body.Message
}

func TestNonGetCollectionsRequestReturns405WithMessage(t *testing.T) {
	resp := callResourceWithMethod(t, typesenseServers[1], adminKey, http.MethodPost, "collections")

	if resp.Status != http.StatusMethodNotAllowed || decodeMessage(t, resp) == "" {
		t.Fatalf("expected 405 with message, got %d: %s", resp.Status, resp.Body)
	}
}

func TestUnknownResourcePathReturns404(t *testing.T) {
	resp := callResource(t, typesenseServers[1], adminKey, "nope")

	if resp.Status != http.StatusNotFound || decodeMessage(t, resp) == "" {
		t.Fatalf("expected 404 with message, got %d: %s", resp.Status, resp.Body)
	}
}

func TestCollectionsResourceReturnsEmptyArrayForEmptyListing(t *testing.T) {
	resp := callResource(t, stubTypesense(t, map[string]stubResponse{"/collections": {200, `[]`}}), adminKey, "collections")

	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"collections":[]`) {
		t.Fatalf("expected 200 with collections [], got %d: %s", resp.Status, resp.Body)
	}
}

func TestCollectionsResourceSortsCollectionsAlphabetically(t *testing.T) {
	resp := callResource(t, stubTypesense(t, map[string]stubResponse{"/collections": {200, `[{"name":"b"},{"name":"c"},{"name":"a"}]`}}), adminKey, "collections")

	body := decodeCollections(t, resp)
	if !slices.Equal(body.Collections, []string{"a", "b", "c"}) {
		t.Fatalf("expected sorted collections, got %v", body.Collections)
	}
}

func TestCollectionsResourceRespondsWithJSONContentType(t *testing.T) {
	resp := callResource(t, stubTypesense(t, map[string]stubResponse{"/collections": {200, `[]`}}), adminKey, "collections")

	if got := resp.Headers["Content-Type"]; !slices.Equal(got, []string{"application/json"}) {
		t.Fatalf("expected Content-Type application/json, got %v", got)
	}
}
