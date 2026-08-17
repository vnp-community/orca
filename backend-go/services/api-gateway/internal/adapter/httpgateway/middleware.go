package httpgateway

import (
	"context"
	"net/http"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

type contextKey struct{ name string }

var identityContextKey = &contextKey{"api-gateway-identity"}

func withIdentity(ctx context.Context, id usecase.Identity) context.Context {
	return context.WithValue(ctx, identityContextKey, id)
}

// identityFromContext returns the Identity a prior authMiddleware run
// resolved for this request. Exported-in-package for the route handlers
// (usage_routes.go) and the WS bridge handler to read.
func identityFromContext(ctx context.Context) (usecase.Identity, bool) {
	id, ok := ctx.Value(identityContextKey).(usecase.Identity)
	return id, ok
}

// authMiddleware runs AuthValidator ahead of every route this router
// serves, per api-gateway.md §9: "session/JWT validation... before a
// request reaches any internal service". See usecase.AuthValidator's doc
// comment for the PLACEHOLDER warning this currently depends on.
func authMiddleware(v *usecase.AuthValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, err := v.Validate(r)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "UNAUTHENTICATED", err.Error())
				return
			}
			next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), identity)))
		})
	}
}

// rateLimitMiddleware runs after authMiddleware so it can key on the
// resolved tenant ID — per-tenant token-bucket decisioning
// (internal/usecase/rate_limit.go), per api-gateway.md §9's "rate
// limiting... ahead of routing".
func rateLimitMiddleware(rl *usecase.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, _ := identityFromContext(r.Context())
			if !rl.Allow(identity.TenantID) {
				writeJSONError(w, http.StatusTooManyRequests, "RATE_LIMITED", "rate limit exceeded for this tenant")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
