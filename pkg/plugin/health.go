package plugin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cliftonz/typesense-datasource/pkg/typesense"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.settings.URL == "" {
		return healthError("URL is missing"), nil
	}
	if d.settings.Secrets.ApiKey == "" {
		return healthError("API key is missing"), nil
	}
	health, err := d.client.Health(ctx)
	if err != nil && strings.HasPrefix(err.Error(), "Typesense health returned status") {
		return healthError(err.Error()), nil
	}
	if err != nil {
		return healthError("Cannot reach Typesense: " + err.Error()), nil
	}
	if health.ResourceError != "" {
		return healthError("Typesense reports " + health.ResourceError), nil
	}
	if !health.OK {
		return healthError("Typesense is not healthy"), nil
	}
	collection := d.settings.DefaultCollection
	collections, err := d.client.ListCollections(ctx)
	unauthorized := errors.Is(err, typesense.ErrUnauthorized)
	if unauthorized && collection == "" {
		return healthError("API key cannot list collections; set a default collection to test a search-only key"), nil
	}
	if err != nil && !unauthorized {
		return healthError(err.Error()), nil
	}
	message := "Connected to Typesense"
	if !unauthorized {
		message += ", " + collectionCount(len(collections))
	}
	if collection != "" {
		if failure := d.searchFailure(ctx, collection); failure != "" {
			return healthError(failure), nil
		}
		message += `, search on "` + collection + `" works`
	}
	return healthOK(message), nil
}

func (d *Datasource) searchFailure(ctx context.Context, collection string) string {
	if _, err := d.search(ctx, map[string]any{"collection": collection, "q": "*", "per_page": 0}); err != nil {
		return err.Error()
	}
	return ""
}

func collectionCount(n int) string {
	if n == 1 {
		return "1 collection"
	}
	return fmt.Sprintf("%d collections", n)
}

func healthOK(message string) *backend.CheckHealthResult {
	return &backend.CheckHealthResult{Status: backend.HealthStatusOk, Message: message}
}

func healthError(message string) *backend.CheckHealthResult {
	return &backend.CheckHealthResult{Status: backend.HealthStatusError, Message: message}
}
