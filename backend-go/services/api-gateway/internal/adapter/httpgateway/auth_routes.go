package httpgateway

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

const sessionCookieName = "orca_session"

// authUserResponse mirrors frontend/src/renderer/src/auth/auth-types.ts's
// AuthUser EXACTLY — GET /auth/me and POST /auth/local both return this
// shape directly (not nested under a "user" key — an earlier version of
// this file did nest it, which silently broke loginLocal()'s `return body
// as AuthUser`; found live 2026-08-17, see docs/execution-plan.md §0).
type authUserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	// Role: frontend's type is 'developer'|'lead'|'admin' — auth-service's
	// domain.Role only has User|Admin (no "lead" concept yet), so Admin
	// maps to "admin" and everything else maps to "developer". Revisit if
	// backend-go ever grows a 3-tier role model.
	Role string `json:"role"`
	// Provider is always "none" — SSO is a stub in auth-service (see its
	// README), so every session here is local-password.
	Provider string `json:"provider"`
}

func toAuthUserResponse(u *authv1.User) authUserResponse {
	role := "developer"
	if u.GetRole() == authv1.Role_ROLE_ADMIN {
		role = "admin"
	}
	return authUserResponse{
		ID: u.GetId(), Email: u.GetEmail(), Name: u.GetName(),
		Role: role, Provider: "none",
	}
}

type loginRequestBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// mountAuthRoutes wires the plain-HTTP (non-WS, non-/v1) routes this
// gateway serves directly under /auth/* — matching
// specs/frontend/api/http-endpoints.md's documented contract exactly
// (paths, response shapes) so frontend/'s auth-api-client.ts works
// unmodified. None of these run behind authMiddleware (see router.go) —
// /auth/me and /auth/logout each validate their own cookie inline instead.
func mountAuthRoutes(mux chi.Router, authClient authv1.AuthServiceClient, cookieValidator CookieSessionValidator, loginLimiter *usecase.RateLimiter) {
	mux.Post("/auth/local", func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !loginLimiter.Allow(ip) {
			writeJSONError(w, http.StatusTooManyRequests, "too_many_attempts", "too many login attempts, try again later")
			return
		}

		var body loginRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
			return
		}

		resp, err := authClient.Login(r.Context(), &authv1.LoginRequest{
			Email:     body.Email,
			Password:  body.Password,
			Ip:        ip,
			UserAgent: r.UserAgent(),
		})
		if err != nil {
			st, _ := status.FromError(err)
			switch st.Code() {
			case codes.PermissionDenied:
				writeJSONError(w, http.StatusForbidden, "account_inactive", "account is deactivated")
			default:
				// Deliberately generic for every other failure kind — do not leak
				// "user not found" vs "wrong password" vs "malformed input"
				// distinctions to the client.
				writeJSONError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			}
			return
		}

		setSessionCookie(w, resp.GetSessionToken())
		writeJSON(w, http.StatusOK, toAuthUserResponse(resp.GetUser()))
	})

	// /auth/cli-token issues a bearer JWT instead of the HttpOnly session
	// cookie /auth/local sets — CLI/CI callers can't use a cookie, and
	// orca-cli stores this JWT itself (~/.config/orca/credentials.json,
	// 0600). Deliberately not wrapped in authMiddleware, same as
	// /auth/local — this route establishes identity, it doesn't assume it.
	mux.Post("/auth/cli-token", func(w http.ResponseWriter, r *http.Request) {
		var body loginRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
			return
		}

		loginResp, err := authClient.Login(r.Context(), &authv1.LoginRequest{
			Email: body.Email, Password: body.Password,
		})
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
			return
		}

		tokenResp, err := authClient.IssueServiceToken(r.Context(), &authv1.IssueServiceTokenRequest{
			UserId: loginResp.GetUser().GetId(), Audience: "cli",
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}

		// No cookie set here — deliberate. A CLI/CI caller stores the JWT
		// itself (orca-cli writes it to ~/.config/orca/credentials.json,
		// 0600); a cookie would be silently dropped by any non-browser client.
		writeJSON(w, http.StatusOK, map[string]any{
			"jwt": tokenResp.GetJwt(), "expires_at": tokenResp.GetExpiresAt(),
			"user": toAuthUserResponse(loginResp.GetUser()),
		})
	})

	mux.Get("/auth/me", func(w http.ResponseWriter, r *http.Request) {
		id, err := cookieValidator.ValidateCookie(r.Context(), r)
		if err != nil {
			// fetchCurrentUser() specifically branches on status===401 to
			// mean "not logged in" (returns null, not an error) — every
			// other non-2xx is treated as a hard failure. Must be a real 401.
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// ValidateCookie only resolves tenant/user IDs (wscompat.Identity),
		// not the full profile — one more round trip to get
		// email/name/role for the response body.
		resp, err := authClient.ValidateSession(r.Context(), &authv1.ValidateSessionRequest{SessionToken: cookieValue(r)})
		if err != nil || !resp.GetValid() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = id // identity already reflected in resp.GetUser(); kept for symmetry with other handlers
		writeJSON(w, http.StatusOK, toAuthUserResponse(resp.GetUser()))
	})

	mux.Post("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if token := cookieValue(r); token != "" {
			_, _ = authClient.Logout(r.Context(), &authv1.LogoutRequest{SessionToken: token})
		}
		http.SetCookie(w, &http.Cookie{
			Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1,
			HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
		})
		w.WriteHeader(http.StatusOK)
	})

	// SSO is a stub in auth-service (see its README) — no providers enabled
	// yet. localEnabled: true always, since /auth/local is real.
	mux.Get("/auth/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"providers":    []string{},
			"localEnabled": true,
		})
	})

	// SSO is not implemented anywhere in the target architecture yet
	// (auth-service.md's RPC surface has no OAuth/provider-login concept) —
	// /auth/config already reports providers: [] honestly. This route exists
	// only so a client that somehow reaches it gets the same documented 501
	// the old TS backend returned, instead of a bare 404.
	mux.Get("/auth/sso/{provider}", func(w http.ResponseWriter, r *http.Request) {
		writeJSONError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "SSO login is not yet supported")
	})
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // always on, per api-gateway.md §9 — no NODE_ENV-gated exception, unlike the old TS backend's documented bug
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(24 * time.Hour / time.Second),
	})
}

func cookieValue(r *http.Request) string {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// clientIP prefers the first X-Forwarded-For hop (this gateway may sit
// behind a reverse proxy/load balancer — SSH/tunnel deployments are a
// first-class use case per AGENTS.md, not just direct-connect), falling
// back to r.RemoteAddr. Trusting X-Forwarded-For without a configured
// trusted-proxy allowlist is a known, narrow spoofing surface for the
// per-IP rate limiter only (TASK-AUTH-01-05) — acceptable for a
// brute-force *throttle* (defense in depth, not the sole control) but
// flagged so a future trusted-proxy-list config isn't mistaken for out of
// scope.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
