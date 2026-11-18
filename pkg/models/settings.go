package models

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type PluginSettings struct {
	URL               string                `json:"url"`
	TimeUnit          string                `json:"timeUnit"`
	DefaultCollection string                `json:"defaultCollection"`
	MaxPages          int                   `json:"maxPages"`
	Secrets           *SecretPluginSettings `json:"-"`
}

type SecretPluginSettings struct {
	ApiKey string `json:"apiKey"`
}

func LoadPluginSettings(source backend.DataSourceInstanceSettings) (*PluginSettings, error) {
	settings := PluginSettings{}
	if len(source.JSONData) > 0 {
		err := json.Unmarshal(source.JSONData, &settings)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal PluginSettings json: %w", err)
		}
	}
	if settings.TimeUnit == "" {
		settings.TimeUnit = "s"
	}
	if settings.MaxPages == 0 {
		settings.MaxPages = 40
	}

	settings.URL = strings.TrimSpace(settings.URL)
	settings.DefaultCollection = strings.TrimSpace(settings.DefaultCollection)
	settings.Secrets = loadSecretPluginSettings(source.DecryptedSecureJSONData)

	return &settings, nil
}

func loadSecretPluginSettings(source map[string]string) *SecretPluginSettings {
	return &SecretPluginSettings{
		ApiKey: strings.TrimSpace(source["apiKey"]),
	}
}
