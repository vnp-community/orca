// Package oauth implements usecase.OAuthExchanger against any provider's
// standard OAuth 2.0 authorization-code endpoints — see
// scm-integration-service.md §9.1's decision (a web flow, not the TS
// PTY/CLI-login mechanism it replaces) and cmd/server/main.go's composition
// root for the five provider instances this package is configured with
// (one Client per provider, each pointed at that provider's real
// authorize/token URLs — see internal/config for where those URLs and
// client credentials come from).
//
// A single generic Client — not one hand-written type per provider, unlike
// internal/adapter/{github,gitlab,...} which each need a provider-specific
// REST shape — is enough here because the OAuth 2.0 authorization-code
// grant itself is the same shape across every one of these five providers;
// only the URLs, scope string, and client credentials differ, and those are
// exactly this struct's fields.
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// Config is the provider-specific OAuth 2.0 wiring cmd/server/main.go's
// composition root supplies per provider.
type Config struct {
	AuthorizeURL string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scope        string
}

// Client implements usecase.OAuthExchanger for one provider's OAuth 2.0
// authorization-code endpoints.
type Client struct {
	httpClient   *http.Client
	authorizeURL string
	tokenURL     string
	clientID     string
	clientSecret string
	scope        string
}

// New returns a Client for cfg. A nil httpClient defaults to
// http.DefaultClient, matching every other adapter in this service.
func New(httpClient *http.Client, cfg Config) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		httpClient:   httpClient,
		authorizeURL: cfg.AuthorizeURL,
		tokenURL:     cfg.TokenURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		scope:        cfg.Scope,
	}
}

var _ usecase.OAuthExchanger = (*Client)(nil)

// AuthorizationURL builds the URL the browser is redirected to — pure
// string construction, no HTTP call (§9.1 sequence diagram, "redirect to
// provider authorization URL" step).
func (c *Client) AuthorizationURL(state, redirectURI string) string {
	q := url.Values{}
	q.Set("client_id", c.clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("response_type", "code")
	if c.scope != "" {
		q.Set("scope", c.scope)
	}
	sep := "?"
	if strings.Contains(c.authorizeURL, "?") {
		sep = "&"
	}
	return c.authorizeURL + sep + q.Encode()
}

// oauthTokenResponse mirrors the token-endpoint response shape shared by
// GitHub/GitLab/Bitbucket/Azure DevOps/Gitea's OAuth 2.0 implementations —
// all of them return access_token/scope on success and error/error_description
// on failure (RFC 6749 §5.1/§5.2), so one struct covers every provider this
// service supports.
type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// ExchangeCode calls the provider's token endpoint for real — the only
// place this package ever sees a client secret or the resulting access
// token; never logged, never stored on Client beyond this call's stack,
// per scm-integration-service.md §9's token-handling invariant (see
// internal/adapter/github's ListIssues doc comment for the shared rule).
func (c *Client) ExchangeCode(ctx context.Context, code, redirectURI string) (usecase.OAuthToken, error) {
	form := url.Values{}
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return usecase.OAuthToken{}, fmt.Errorf("oauth: build token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json") // GitHub's token endpoint form-encodes its response unless this is set

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return usecase.OAuthToken{}, fmt.Errorf("oauth: token exchange request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return usecase.OAuthToken{}, fmt.Errorf("oauth: token exchange: unexpected status %d", resp.StatusCode)
	}

	var raw oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return usecase.OAuthToken{}, fmt.Errorf("oauth: decode token exchange response: %w", err)
	}
	if raw.Error != "" {
		return usecase.OAuthToken{}, fmt.Errorf("oauth: provider rejected code exchange: %s: %s", raw.Error, raw.ErrorDesc)
	}
	if raw.AccessToken == "" {
		return usecase.OAuthToken{}, fmt.Errorf("oauth: token exchange response had no access_token")
	}
	return usecase.OAuthToken{AccessToken: raw.AccessToken, Scope: raw.Scope}, nil
}
