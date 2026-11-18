package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"github.com/cliftonz/typesense-datasource/pkg/typesense"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type collectionsResponse struct {
	CanList     bool     `json:"canList"`
	Collections []string `json:"collections"`
}

func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	if req.Path != "collections" {
		return sendJSON(sender, http.StatusNotFound, map[string]string{"message": "resource not found: " + req.Path})
	}
	if req.Method != http.MethodGet {
		return sendJSON(sender, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed: " + req.Method})
	}
	names, err := d.client.ListCollections(ctx)
	if errors.Is(err, typesense.ErrUnauthorized) {
		return sendJSON(sender, http.StatusOK, collectionsResponse{CanList: false, Collections: []string{}})
	}
	if err != nil {
		return sendJSON(sender, http.StatusInternalServerError, map[string]string{"message": err.Error()})
	}
	slices.Sort(names)
	return sendJSON(sender, http.StatusOK, collectionsResponse{CanList: true, Collections: names})
}

func sendJSON(sender backend.CallResourceResponseSender, status int, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return sender.Send(&backend.CallResourceResponse{
		Status:  status,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    body,
	})
}
