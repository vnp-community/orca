package httpgateway

import (
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

const (
	oauthFormMaxBytes     = 16 << 10
	oauthRegisterMaxBytes = 8 << 10
)

// OAuthRoutes are the public (cookie-less) OAuth 2.1 endpoints of the gateway
// (BE-MCP-SOL-005 section E): the gateway only translates HTTP <-> auth-service
// RPCs; every decision lives in auth-service / mcp-service.
type OAuthRoutes struct {
	Auth            authv1.AuthServiceClient
	Consent         consentCreator
	CookieValidator CookieSessionValidator
	// Issuer is the public base URL echoed as `iss` (RFC 9207); from config, never a request header.
	Issuer string
	// DCREnabled mirrors OAUTH_DCR_ENABLED: off removes POST /oauth/register.
	DCREnabled bool
	// RegisterLimiter / TokenLimiter default to conservative per-IP limiters.
	RegisterLimiter *usecase.RateLimiter
	TokenLimiter    *usecase.RateLimiter
}

// NewOAuthRateLimiters returns the register (10/hour, burst 5) and token
// (2/s, burst 30) limiters the spec calls for.
func NewOAuthRateLimiters() (register, token *usecase.RateLimiter) {
	return usecase.NewRateLimiter(10.0/3600, 5), usecase.NewRateLimiter(2, 30)
}

func (o *OAuthRoutes) mount(r chi.Router) {
	if o.RegisterLimiter == nil || o.TokenLimiter == nil {
		reg, tok := NewOAuthRateLimiters()
		if o.RegisterLimiter == nil {
			o.RegisterLimiter = reg
		}
		if o.TokenLimiter == nil {
			o.TokenLimiter = tok
		}
	}
	if o.DCREnabled {
		r.Post("/oauth/register", o.handleRegister)
	}
	r.Get("/oauth/authorize", o.handleAuthorize)
	r.Post("/oauth/token", o.handleToken)
	r.Post("/oauth/revoke", o.handleRevoke)
}

// remoteHost is the peer address without the ephemeral port, so a limiter key
// is per host and not per connection.
func remoteHost(r *http.Request) string {
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return h
	}
	return r.RemoteAddr
}

func (o *OAuthRoutes) handleRegister(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	if !o.RegisterLimiter.Allow("oauth-register:" + remoteHost(r)) {
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusTooManyRequests, oauthErrorBody{Error: "temporarily_unavailable", Description: "rate limit exceeded"})
		return
	}
	var body struct {
		ClientName              string   `json:"client_name"`
		ClientURI               string   `json:"client_uri"`
		RedirectURIs            []string `json:"redirect_uris"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
		GrantTypes              []string `json:"grant_types"`
		ResponseTypes           []string `json:"response_types"`
	}
	// Unknown RFC 7591 metadata (scope, software_id, ...) is ignored on purpose.
	if err := decodeOAuthJSON(w, r, oauthRegisterMaxBytes, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, oauthErrorBody{Error: "invalid_client_metadata", Description: "request body must be a JSON object of at most 8 KiB"})
		return
	}
	resp, err := o.Auth.OAuthRegisterClient(r.Context(), &authv1.OAuthRegisterClientRequest{
		ClientName: body.ClientName, ClientUri: body.ClientURI, RedirectUris: body.RedirectURIs,
		TokenEndpointAuthMethod: body.TokenEndpointAuthMethod, GrantTypes: body.GrantTypes, ResponseTypes: body.ResponseTypes,
	})
	if err != nil {
		code, msg, unavailable := splitCoded(err)
		switch {
		case unavailable:
			writeOAuthJSONError(w, err)
		case code == "OAUTH_DCR_DISABLED":
			writeJSON(w, http.StatusForbidden, oauthErrorBody{Error: "access_denied", Description: "dynamic client registration is disabled"})
		case code == "OAUTH_DCR_LIMIT_REACHED":
			w.Header().Set("Retry-After", "3600")
			writeJSON(w, http.StatusTooManyRequests, oauthErrorBody{Error: "temporarily_unavailable", Description: "client registry is full"})
		case code == "OAUTH_INVALID_REDIRECT_URI", code == "OAUTH_INVALID_CLIENT_METADATA":
			writeJSON(w, http.StatusBadRequest, oauthErrorBody{Error: oauthErrorName(code), Description: msg})
		case code == "OAUTH_INVALID_REQUEST":
			writeJSON(w, http.StatusBadRequest, oauthErrorBody{Error: "invalid_client_metadata", Description: msg})
		default:
			writeOAuthJSONError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":                  resp.GetClientId(),
		"client_id_issued_at":        resp.GetClientIdIssuedAt(),
		"client_name":                resp.GetClientName(),
		"client_uri":                 resp.GetClientUri(),
		"redirect_uris":              resp.GetRedirectUris(),
		"token_endpoint_auth_method": resp.GetTokenEndpointAuthMethod(),
		"grant_types":                resp.GetGrantTypes(),
		"response_types":             resp.GetResponseTypes(),
	})
}

func (o *OAuthRoutes) handleToken(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	r.Body = http.MaxBytesReader(w, r.Body, oauthFormMaxBytes)
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, oauthErrorBody{Error: "invalid_request", Description: "body must be application/x-www-form-urlencoded"})
		return
	}
	// Only the POST body counts: query-string parameters (which land in
	// access logs) are never an acceptable carrier for code/verifier/refresh_token.
	f := r.PostForm
	if !o.TokenLimiter.Allow("oauth-token:" + f.Get("client_id") + ":" + remoteHost(r)) {
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusTooManyRequests, oauthErrorBody{Error: "temporarily_unavailable", Description: "rate limit exceeded"})
		return
	}
	resp, err := o.Auth.OAuthExchangeToken(r.Context(), &authv1.OAuthExchangeTokenRequest{
		GrantType: f.Get("grant_type"), Code: f.Get("code"), RedirectUri: f.Get("redirect_uri"), CodeVerifier: f.Get("code_verifier"),
		ClientId: f.Get("client_id"), RefreshToken: f.Get("refresh_token"), Resource: f.Get("resource"), Scope: f.Get("scope"),
	})
	if err != nil {
		writeOAuthJSONError(w, err)
		return
	}
	out := map[string]any{
		"access_token": resp.GetAccessToken(), "token_type": "Bearer", "expires_in": resp.GetExpiresIn(), "scope": resp.GetScope(),
	}
	if rt := resp.GetRefreshToken(); rt != "" {
		out["refresh_token"] = rt
	}
	writeJSON(w, http.StatusOK, out)
}

// handleRevoke implements RFC 7009: unknown or already-revoked tokens still
// answer 200. A real infrastructure failure answers 503 (RFC 7009 section
// 2.2.1) so a client does not believe a token was revoked when it was not.
func (o *OAuthRoutes) handleRevoke(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	r.Body = http.MaxBytesReader(w, r.Body, oauthFormMaxBytes)
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, oauthErrorBody{Error: "invalid_request", Description: "body must be application/x-www-form-urlencoded"})
		return
	}
	f := r.PostForm
	if !o.TokenLimiter.Allow("oauth-token:" + f.Get("client_id") + ":" + remoteHost(r)) {
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusTooManyRequests, oauthErrorBody{Error: "temporarily_unavailable", Description: "rate limit exceeded"})
		return
	}
	if _, err := o.Auth.OAuthRevokeToken(r.Context(), &authv1.OAuthRevokeTokenRequest{
		Token: f.Get("token"), TokenTypeHint: f.Get("token_type_hint"), ClientId: f.Get("client_id"),
	}); err != nil {
		if _, _, unavailable := splitCoded(err); unavailable {
			writeOAuthJSONError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}
