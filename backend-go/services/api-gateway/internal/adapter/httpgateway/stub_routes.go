package httpgateway

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/stablyai/orca-go/services/api-gateway/internal/domain"
)

// mountStubRoutes wires a catch-all 501 handler per RouteStubbed rule in
// registry. Per this service's own build instructions: "don't fake full
// proxying for services whose contracts are still evolving" — every route
// under a stubbed prefix returns the same documented, structured 501 body
// rather than silently 404ing or (worse) pretending to succeed.
func mountStubRoutes(r chi.Router, registry *domain.ServiceRegistry) {
	for _, rule := range registry.Rules() {
		if rule.Status != domain.RouteStubbed {
			continue
		}
		serviceName := rule.ServiceName
		r.Route(rule.PathPrefix, func(sub chi.Router) {
			sub.HandleFunc("/*", stubHandler(serviceName))
			// Also handle the bare prefix itself (e.g. "/v1/tasks" with no
			// trailing segment), not just "/v1/tasks/*".
			sub.HandleFunc("/", stubHandler(serviceName))
		})
	}
}

func stubHandler(serviceName string) http.HandlerFunc {
	message := fmt.Sprintf("this route will proxy to %s once its gRPC contract stabilizes", serviceName)
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSONError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", message)
	}
}
