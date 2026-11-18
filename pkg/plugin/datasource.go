package plugin

import (
	"context"

	"github.com/cliftonz/typesense-datasource/pkg/models"
	"github.com/cliftonz/typesense-datasource/pkg/typesense"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
)

var (
	_ backend.QueryDataHandler   = (*Datasource)(nil)
	_ backend.CheckHealthHandler = (*Datasource)(nil)
)

func NewDatasource(ctx context.Context, instance backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	settings, err := models.LoadPluginSettings(instance)
	if err != nil {
		return nil, err
	}
	opts, err := instance.HTTPClientOptions(ctx)
	if err != nil {
		return nil, err
	}
	httpClient, err := httpclient.New(opts)
	if err != nil {
		return nil, err
	}
	return &Datasource{
		settings: settings,
		client:   typesense.New(settings.URL, settings.Secrets.ApiKey, httpClient),
	}, nil
}

type Datasource struct {
	settings *models.PluginSettings
	client   *typesense.Client
}
