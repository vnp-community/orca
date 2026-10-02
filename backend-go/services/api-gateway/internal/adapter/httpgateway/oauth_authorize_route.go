package httpgateway

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"strings"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
	"google.golang.org/grpc"
)

// consentCreator is the slice of McpServiceClient the authorize endpoint needs.
type consentCreator interface {
	CreateConsentRequest(ctx context.Context, in *mcpv1.CreateConsentRequestRequest, opts ...grpc.CallOption) (*mcpv1.CreateConsentRequestResponse, error)
}

// authorizeSecurityHeaders: the authorize response must never be cached,
// leak its URL (it carries state/challenge) via Referer, or be framed.
func authorizeSecurityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'")
}

// handleAuthorize is GET /oauth/authorize. Order matters: client and
// redirect_uri are validated BEFORE anything may redirect to the client, so a
// bad client or redirect_uri is shown as a static page and never redirected to
// (open-redirect guard); only then is the browser session checked.
func (o *OAuthRoutes) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	authorizeSecurityHeaders(w)
	q := r.URL.Query()
	info, err := o.Auth.OAuthValidateAuthorizeRequest(r.Context(), &authv1.OAuthValidateAuthorizeRequestRequest{
		ResponseType: q.Get("response_type"), ClientId: q.Get("client_id"), RedirectUri: q.Get("redirect_uri"), Scope: q.Get("scope"),
		CodeChallenge: q.Get("code_challenge"), CodeChallengeMethod: q.Get("code_challenge_method"), Resource: q.Get("resource"),
	})
	if err != nil {
		o.authorizeFailure(w, r, q, "", err)
		return
	}
	// Prefer the redirect_uri echoed by the authorization server (it is the
	// exact registered value that matched) over the raw query parameter.
	redirectURI := info.GetRedirectUri()
	if redirectURI == "" {
		redirectURI = q.Get("redirect_uri")
	}

	if o.CookieValidator == nil {
		writeAuthorizePage(w, http.StatusServiceUnavailable, "Sign-in is temporarily unavailable. Return to the application and try again.")
		return
	}
	id, err := o.CookieValidator.ValidateCookie(r.Context(), r) // cookie only; Bearer is ignored here
	if err != nil {
		// Relative target only: return_to can never point at another host.
		http.Redirect(w, r, "/login?return_to="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return
	}
	ctx := gatewaygrpc.AttachIdentity(r.Context(), usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
	resp, err := o.Consent.CreateConsentRequest(ctx, &mcpv1.CreateConsentRequestRequest{
		ResponseType: q.Get("response_type"), ClientId: q.Get("client_id"), RedirectUri: q.Get("redirect_uri"), Scope: q.Get("scope"),
		State: q.Get("state"), CodeChallenge: q.Get("code_challenge"), CodeChallengeMethod: q.Get("code_challenge_method"), Resource: q.Get("resource"),
	})
	if err != nil {
		o.authorizeFailure(w, r, q, redirectURI, err)
		return
	}
	if u := resp.GetRedirectUrl(); u != "" {
		http.Redirect(w, r, u, http.StatusFound)
		return
	}
	http.Redirect(w, r, "/oauth/consent?request_id="+url.QueryEscape(resp.GetRequestId()), http.StatusFound)
}

// authorizeFailure decides between a static error page and an error redirect.
// knownRedirect is non-empty once auth-service has already accepted the client
// and redirect_uri; before that, only the static page is allowed.
func (o *OAuthRoutes) authorizeFailure(w http.ResponseWriter, r *http.Request, q url.Values, knownRedirect string, err error) {
	code, msg, unavailable := splitCoded(err)
	if unavailable {
		writeAuthorizePage(w, http.StatusServiceUnavailable, "The authorization server is temporarily unavailable. Try again in a moment.")
		return
	}
	switch code {
	case "OAUTH_INVALID_CLIENT", "OAUTH_CLIENT_NOT_FOUND":
		writeAuthorizePage(w, http.StatusBadRequest, "This application is not registered, so the request cannot continue.")
		return
	case "OAUTH_INVALID_REDIRECT_URI":
		writeAuthorizePage(w, http.StatusBadRequest, "The application sent a redirect address that is not registered, so the request cannot continue.")
		return
	}
	// Any other OAUTH_* failure may be reported to the client: auth-service
	// checks client and redirect_uri first, so reaching here means they matched
	// the registration exactly. Errors without an OAUTH_ code never redirect.
	target := knownRedirect
	if target == "" && strings.HasPrefix(code, "OAUTH_") {
		target = q.Get("redirect_uri")
	}
	if target == "" {
		writeAuthorizePage(w, http.StatusBadRequest, "The authorization request could not be processed.")
		return
	}
	desc := msg
	name := oauthErrorName(code)
	if name == "server_error" {
		desc = "internal error"
	}
	redirectWithError(w, r, target, name, desc, q.Get("state"), o.Issuer)
}

func redirectWithError(w http.ResponseWriter, r *http.Request, target, name, desc, state, issuer string) {
	u, err := url.Parse(target)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.Fragment != "" {
		writeAuthorizePage(w, http.StatusBadRequest, "The authorization request could not be processed.")
		return
	}
	v := u.Query()
	v.Set("error", name)
	if desc != "" {
		v.Set("error_description", desc)
	}
	if state != "" {
		v.Set("state", state)
	}
	if issuer != "" {
		v.Set("iss", issuer) // RFC 9207 mix-up defence
	}
	u.RawQuery = v.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// writeAuthorizePage is the static (non-SPA) error page. The message is a
// fixed string chosen here, never attacker-controlled text.
func writeAuthorizePage(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">" +
		"<title>Authorization error</title></head><body style=\"font-family:system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 1rem\">" +
		"<h1>Authorization error</h1><p>" + html.EscapeString(message) + "</p></body></html>"))
}
