package httpgateway

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

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
func mountAuthRoutes(mux chi.Router, authClient authv1.AuthServiceClient, cookieValidator CookieSessionValidator) {
	mux.Post("/auth/local", func(w http.ResponseWriter, r *http.Request) {
		var body loginRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
			return
		}

		resp, err := authClient.Login(r.Context(), &authv1.LoginRequest{
			Email:    body.Email,
			Password: body.Password,
		})
		if err != nil {
			// Deliberately generic — do not leak "user not found" vs "wrong
			// password" distinctions to the client, matching standard
			// login-endpoint practice.
			writeJSONError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
			return
		}

		setSessionCookie(w, resp.GetSessionToken())
		writeJSON(w, http.StatusOK, toAuthUserResponse(resp.GetUser()))
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
