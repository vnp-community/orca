// Package apiclient is orca-cli's thin REST client — one method per
// api-gateway endpoint this CLI calls. Never imports any service's
// internal/ package (SOL-CLI-01's dependency-inversion boundary) — only
// stdlib net/http and encoding/json.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client wraps http.Client with bearer-token injection and JSON
// marshal/unmarshal for every apiclient method.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{baseURL: baseURL, token: token, http: &http.Client{}}
}

// do issues one JSON request/response round trip. A non-2xx status maps
// the response body through errors.go's decodeErrorBody rather than
// returning a raw *http.Response for every caller to re-check.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("apiclient: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("apiclient: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("apiclient: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("apiclient: read response body: %w", err)
	}
	if resp.StatusCode >= 300 {
		return decodeErrorBody(resp.StatusCode, respBody)
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("apiclient: decode response: %w", err)
		}
	}
	return nil
}

// newGetRequest is a small helper shared by non-c.do request paths (health
// checks, the raw-text snapshot fetch) that don't want JSON
// marshal/unmarshal or bearer-token injection folded in.
func newGetRequest(ctx context.Context, url string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
}
