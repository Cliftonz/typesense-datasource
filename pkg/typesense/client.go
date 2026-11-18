package typesense

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var ErrUnauthorized = errors.New("typesense: unauthorized")

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func New(baseURL, apiKey string, httpClient *http.Client) *Client {
	return &Client{baseURL: strings.TrimSuffix(baseURL, "/"), apiKey: apiKey, httpClient: httpClient}
}

type Health struct {
	OK            bool   `json:"ok"`
	ResourceError string `json:"resource_error"`
}

func (c *Client) Health(ctx context.Context) (Health, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return Health{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Health{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Health{}, fmt.Errorf("Typesense health returned status %d: %w", resp.StatusCode, err)
	}
	var h Health
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, &h); err != nil {
			return Health{}, err
		}
		return h, nil
	}
	if resp.StatusCode == http.StatusServiceUnavailable && json.Unmarshal(raw, &h) == nil {
		return h, nil
	}
	return Health{}, fmt.Errorf("Typesense health returned status %d: %s", resp.StatusCode, first200(raw))
}

func (c *Client) ListCollections(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/collections", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-TYPESENSE-API-KEY", c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp)
	}
	var collections []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&collections); err != nil {
		return nil, err
	}
	names := make([]string, len(collections))
	for i, col := range collections {
		names[i] = col.Name
	}
	return names, nil
}

func (c *Client) MultiSearch(ctx context.Context, body []byte) ([]SearchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/multi_search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-TYPESENSE-API-KEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp)
	}
	var out struct {
		Results []SearchResult `json:"results"`
	}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

func statusError(resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	var body struct {
		Message *string `json:"message"`
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("typesense: status %d: %w", resp.StatusCode, err)
	}
	if json.Unmarshal(raw, &body) != nil || body.Message == nil {
		return fmt.Errorf("typesense: status %d: %s", resp.StatusCode, first200(raw))
	}
	return fmt.Errorf("typesense: status %d: %s", resp.StatusCode, *body.Message)
}

func first200(raw []byte) string {
	runes := []rune(string(raw))
	return string(runes[:min(len(runes), 200)])
}
