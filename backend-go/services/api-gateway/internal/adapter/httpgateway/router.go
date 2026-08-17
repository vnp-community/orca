package httpgateway

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/stablyai/orca-go/services/api-gateway/internal/domain"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	usagev1 "github.com/stablyai/orca-go/proto/gen/go/orca/usage/v1"
)

// Deps is everything NewRouter needs to build api-gateway's REST edge.
type Deps struct {
	Logger        *slog.Logger
	Registry      *domain.ServiceRegistry
	AuthValidator *usecase.AuthValidator
	RateLimiter   *usecase.RateLimiter
	UsageClient   usagev1.UsageServiceClient
	// WSHandler serves the one real WS<->gRPC-stream bridge route
	// (/v1/notifications/stream). Built by internal/adapter/wsbridge and
	// passed in rather than constructed here, since it owns its own
	// long-lived gRPC stream lifecycle per connection (see
	// cmd/server/main.go). Nil is valid — the route is simply not mounted.
	WSHandler http.HandlerFunc
}

// NewRouter builds api-gateway's chi router: the auth + rate-limit
// middleware every route runs behind (api-gateway.md §9), the real
// usage-service reverse-proxy routes, the real notification WS bridge
// route, and 501 stubs for every other downstream service's REST surface
// (domain.NewDefaultServiceRegistry).
func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(authMiddleware(deps.AuthValidator))
	r.Use(rateLimitMiddleware(deps.RateLimiter))

	mountUsageRoutes(r, deps.UsageClient)

	if deps.WSHandler != nil {
		// A literal path always wins over a "/v1/notifications/*" wildcard
		// in chi's radix-tree router regardless of registration order, so
		// this stays real even though mountStubRoutes below also claims
		// "/v1/notifications/*" for notification-service's REST surface.
		r.Get("/v1/notifications/stream", deps.WSHandler)
	}

	mountStubRoutes(r, deps.Registry)

	return r
}
