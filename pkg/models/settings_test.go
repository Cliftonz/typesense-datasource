package models

import (
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestLoadPluginSettingsAppliesDefaultsWhenKeysMissing(t *testing.T) {
	settings, err := LoadPluginSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"url":"http://localhost:8108"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.URL != "http://localhost:8108" || settings.TimeUnit != "s" || settings.MaxPages != 40 || settings.DefaultCollection != "" {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestLoadPluginSettingsKeepsExplicitValues(t *testing.T) {
	settings, err := LoadPluginSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"url":"http://ts:8108","timeUnit":"ms","defaultCollection":"events","maxPages":7}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.URL != "http://ts:8108" || settings.TimeUnit != "ms" || settings.MaxPages != 7 || settings.DefaultCollection != "events" {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestLoadPluginSettingsReadsApiKeyFromDecryptedSecureData(t *testing.T) {
	settings, err := LoadPluginSettings(backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{}`),
		DecryptedSecureJSONData: map[string]string{"apiKey": "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.Secrets.ApiKey != "secret" {
		t.Fatalf("got api key %q", settings.Secrets.ApiKey)
	}
}

func TestLoadPluginSettingsTrimsURLAndDefaultCollection(t *testing.T) {
	settings, err := LoadPluginSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"url":"  http://ts:8108 \n","defaultCollection":"\tevents  "}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.URL != "http://ts:8108" || settings.DefaultCollection != "events" {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestLoadPluginSettingsTreatsZeroValuesAsMissing(t *testing.T) {
	settings, err := LoadPluginSettings(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"timeUnit":"","maxPages":0}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.TimeUnit != "s" || settings.MaxPages != 40 {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestLoadPluginSettingsAppliesDefaultsWhenJSONDataEmpty(t *testing.T) {
	settings, err := LoadPluginSettings(backend.DataSourceInstanceSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if settings.TimeUnit != "s" || settings.MaxPages != 40 {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestLoadPluginSettingsTrimsApiKey(t *testing.T) {
	settings, err := LoadPluginSettings(backend.DataSourceInstanceSettings{
		DecryptedSecureJSONData: map[string]string{"apiKey": " \t\n "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.Secrets.ApiKey != "" {
		t.Fatalf("got api key %q", settings.Secrets.ApiKey)
	}
}
