package httpgateway

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/stablyai/orca-go/services/api-gateway/internal/domain"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	automationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/automation/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
	notificationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/notification/v1"
	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
	usagev1 "github.com/stablyai/orca-go/proto/gen/go/orca/usage/v1"
	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// Deps is everything NewRouter needs to build api-gateway's REST edge.
type Deps struct {
	Logger          *slog.Logger
	Registry        *domain.ServiceRegistry
	AuthValidator   *usecase.AuthValidator
	CookieValidator CookieSessionValidator
	RateLimiter     *usecase.RateLimiter
	UsageClient     usagev1.UsageServiceClient
	AuthClient      authv1.AuthServiceClient
	// The remaining downstream clients below back the Phase 5 REST routes
	// (execution-plan.md) — each wired only once its owning service was
	// confirmed mature (real RPCs, not blanket Unimplemented), and marked
	// RouteWired in registry.go accordingly. A nil client here (e.g. in a
	// test harness that doesn't need every downstream) simply skips the
	// corresponding mountXRoutes call — cmd/server/main.go always dials
	// all of them for real, so in production this is never nil. Note this
	// is NOT the same as falling back to the 501 stub: once a prefix is
	// RouteWired, mountStubRoutes no longer registers anything under it,
	// so an unmounted RouteWired prefix 404s rather than 501ing.
	AnnotationClient    annotationv1.AnnotationServiceClient
	TaskClient          taskv1.TaskServiceClient
	GitGatewayClient    gitgatewayv1.GitGatewayServiceClient
	AutomationClient    automationv1.AutomationServiceClient
	InfraFleetClient    infrafleetv1.InfraFleetServiceClient
	NotificationClient  notificationv1.NotificationServiceClient
	TenantClient        tenantv1.TenantServiceClient
	ProjectClient       projectv1.ProjectServiceClient
	IssueTrackingClient issuetrackingv1.IssueTrackingServiceClient
	AIProviderClient    aiproviderv1.AiProviderServiceClient
	OrchestrationClient orchestrationv1.OrchestrationServiceClient
	SCMClient           scmintegrationv1.ScmIntegrationServiceClient
	WorkflowClient      workflowv1.WorkflowServiceClient
	// WSHandler serves the one real WS<->gRPC-stream bridge route
	// (/v1/notifications/stream). Built by internal/adapter/wsbridge and
	// passed in rather than constructed here, since it owns its own
	// long-lived gRPC stream lifecycle per connection (see
	// cmd/server/main.go). Nil is valid — the route is simply not mounted.
	WSHandler http.HandlerFunc
	// WSCompatHandler serves /ws — the legacy channel-based RPC transport
	// (internal/adapter/wscompat) the deployed frontend/ actually speaks.
	// Auth happens INSIDE this handler (session-cookie validation at
	// upgrade time), not via authMiddleware below — see NewRouter's
	// mounting order.
	WSCompatHandler http.HandlerFunc
	// AgentProxyHandler serves /agent (WS) and /api/agent-token (HTTP),
	// raw-proxied to infra-fleet-service — see agent_proxy_routes.go's
	// NewAgentProxyHandler doc comment for why this is a byte-for-byte
	// proxy rather than a gRPC translation. Auth happens INSIDE
	// infra-fleet-service (ORCA_AGENT_API_SECRET bearer check /
	// single-use token slots), same "auth inside the handler, not via
	// authMiddleware" shape as WSCompatHandler above — the Dev Server
	// Agent has no user session cookie to present. Nil is valid — the
	// routes are simply not mounted (see Config.InfraFleetHTTPAddr).
	AgentProxyHandler http.Handler
}

// NewRouter builds api-gateway's chi router. Three route groups, in order:
//  1. Unauthenticated: POST /auth/local (login itself can't require a
//     session — that would be circular), GET /ws (auth handled inside
//     wscompat.Handler, once at upgrade, not per-HTTP-request — matching
//     the old backend's WsSessionRouter design), GET /agent + /api/
//     agent-token (auth handled inside infra-fleet-service, see
//     AgentProxyHandler's doc comment above), and POST /v1/scm/webhooks/
//     {provider} (auth is the provider's own webhook signature, verified
//     inside scm-integration-service — see mountSCMWebhookRoutes).
//  2. Authenticated: everything else (/v1/*, including the real
//     usage-service proxy, the real notification WS bridge, the Phase 5
//     mountXRoutes proxies below, and 501 stubs for the remaining
//     not-yet-mature services), behind authMiddleware + rateLimitMiddleware.
func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()

	if deps.AuthClient != nil {
		mountAuthRoutes(r, deps.AuthClient, deps.CookieValidator)
	}
	mountTraceRoutes(r)
	// mountPushRoutes is unauthenticated by design (see its doc comment) —
	// mounted here, outside the authed group below, never moved inside it.
	if deps.NotificationClient != nil {
		mountPushRoutes(r, deps.NotificationClient)
	}
	// mountSCMWebhookRoutes is unauthenticated by design (see its doc
	// comment): GitHub/GitLab's own servers call this, never carrying an
	// Orca JWT. Authenticity comes from ReceiveWebhook's own signature
	// verification inside scm-integration-service, not authMiddleware.
	if deps.SCMClient != nil {
		mountSCMWebhookRoutes(r, deps.SCMClient)
	}
	if deps.WSCompatHandler != nil {
		r.Get("/ws", deps.WSCompatHandler)
	}
	if deps.AgentProxyHandler != nil {
		r.Get("/agent", deps.AgentProxyHandler.ServeHTTP)
		r.Handle("/api/agent-token", deps.AgentProxyHandler)
	}

	r.Group(func(authed chi.Router) {
		authed.Use(authMiddleware(deps.AuthValidator, deps.CookieValidator))
		authed.Use(rateLimitMiddleware(deps.RateLimiter))

		mountUsageRoutes(authed, deps.UsageClient)

		// Phase 5 REST routes (execution-plan.md) — one mountXRoutes call
		// per backend confirmed mature enough to wire for real; see
		// registry.go's NewDefaultServiceRegistry doc comment for which
		// prefixes are still RouteStubbed and why. Each guarded on its
		// client being non-nil so a partially-configured deployment
		// degrades to that prefix's 501 stub instead of panicking.
		if deps.AuthClient != nil {
			mountAuthAdminRoutes(authed, deps.AuthClient)
			mountAdminRoutes(authed, deps.AuthClient)
		}
		if deps.AnnotationClient != nil {
			mountAnnotationRoutes(authed, deps.AnnotationClient)
		}
		if deps.TaskClient != nil {
			mountTaskRoutes(authed, deps.TaskClient)
		}
		if deps.GitGatewayClient != nil {
			mountGitRoutes(authed, deps.GitGatewayClient)
		}
		if deps.AutomationClient != nil {
			mountAutomationRoutes(authed, deps.AutomationClient)
		}
		if deps.InfraFleetClient != nil {
			mountInfraRoutes(authed, deps.InfraFleetClient)
		}
		if deps.NotificationClient != nil {
			mountNotificationRoutes(authed, deps.NotificationClient)
		}
		if deps.TenantClient != nil {
			mountTenantRoutes(authed, deps.TenantClient)
		}
		if deps.ProjectClient != nil {
			mountProjectRoutes(authed, deps.ProjectClient)
		}
		if deps.IssueTrackingClient != nil {
			mountIssueTrackingRoutes(authed, deps.IssueTrackingClient)
		}
		if deps.AIProviderClient != nil {
			mountAIProviderRoutes(authed, deps.AIProviderClient)
		}
		if deps.OrchestrationClient != nil {
			mountOrchestrationRoutes(authed, deps.OrchestrationClient)
		}
		if deps.SCMClient != nil {
			mountSCMRoutes(authed, deps.SCMClient)
		}
		if deps.WorkflowClient != nil {
			mountWorkflowRoutes(authed, deps.WorkflowClient)
		}

		if deps.WSHandler != nil {
			// A literal path always wins over a "/v1/notifications/*"
			// wildcard in chi's radix-tree router regardless of
			// registration order, so this stays real even though
			// mountNotificationRoutes above also claims
			// "/v1/notifications/*" for the Subscribe/GetVapidPublicKey
			// REST surface.
			authed.Get("/v1/notifications/stream", deps.WSHandler)
		}

		mountStubRoutes(authed, deps.Registry)
	})

	return r
}
