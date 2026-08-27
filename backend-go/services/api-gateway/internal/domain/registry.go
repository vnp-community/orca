// Package domain holds api-gateway's minimal value objects. Per
// specs/backend-go/services/api-gateway.md §4/§6, this service carries no
// business invariants — RoutingRule/ServiceRegistry are closer to static
// config than a domain model, and WSSession is ephemeral connection state,
// not a persisted entity.
package domain

// RouteStatus describes whether a route is really wired end-to-end (a live
// gRPC client call to the owning service) or a documented 501 stub because
// the owning service's contract hasn't stabilized yet (design doc §10,
// "dual-routing" — here expressed as "wired vs stub" rather than
// "TS vs Go", since this scaffold has no TS dispatcher to fall back to).
type RouteStatus int

const (
	// RouteStubbed means the route returns 501 Not Implemented with an
	// explanatory body; no gRPC call is made.
	RouteStubbed RouteStatus = iota
	// RouteWired means the route really proxies to the owning service's
	// gRPC API.
	RouteWired
)

// RoutingRule maps one logical route prefix (e.g. "/v1/tasks") to the proto
// package / owning service that will eventually (or already does) serve it.
// See specs/backend-go/services/api-gateway.md §4 — "which proto package
// (orca.<service>.v1) and upstream address a route or WS endpoint maps to".
type RoutingRule struct {
	// PathPrefix is the REST path prefix this rule owns, e.g. "/v1/usage".
	PathPrefix string
	// ServiceName is the owning service's name, e.g. "usage-service".
	ServiceName string
	// ProtoPackage is the owning service's proto package, e.g. "orca.usage.v1".
	ProtoPackage string
	// Status reports whether this route is really wired or still a stub.
	Status RouteStatus
}

// ServiceRegistry is the static, in-memory source of truth for "which
// service owns this route" — a Go map/struct per the design doc's framing
// (§4: "Mostly static config... not a rich domain model"), not a
// database-backed lookup. It also carries the resolved network address for
// services api-gateway actually dials (usage-service, notification-service
// in this scaffold; see internal/config).
type ServiceRegistry struct {
	rules []RoutingRule
}

// NewServiceRegistry builds a registry from the given rules, in the order
// they should be matched (longest/most-specific prefix first is the
// caller's responsibility — see NewDefaultServiceRegistry for this
// service's actual rule set).
func NewServiceRegistry(rules []RoutingRule) *ServiceRegistry {
	return &ServiceRegistry{rules: append([]RoutingRule(nil), rules...)}
}

// Rules returns the registry's routing rules, in match order.
func (r *ServiceRegistry) Rules() []RoutingRule {
	return append([]RoutingRule(nil), r.rules...)
}

// NewDefaultServiceRegistry returns api-gateway's real routing table: one
// rule per downstream service's REST prefix, per specs/backend-go/services/api-gateway.md
// §7's list of all 16 services this gateway has an edge to.
//
// RouteWired means a real mountXRoutes function
// (internal/adapter/httpgateway) proxies that prefix to the owning
// service's gRPC API. Everything else stays RouteStubbed and returns 501,
// per this doc's own instructions not to fake proxying ahead of a
// service's contract being ready — see execution-plan.md Phase 5 for which
// backends were confirmed mature enough to wire, and why the remainder
// (scm-integration-service, workflow-service) are still stubbed.
//
// notification-service's REST prefix covers Subscribe/GetVapidPublicKey
// only — its StreamNotifications RPC is the separate WS<->gRPC-stream
// bridge at /v1/notifications/stream (internal/adapter/wsbridge), a
// distinct route mounted directly, not derived from this table.
//
// credential-broker-service has no direct rule: per §7, it's "reached only
// indirectly via infra-fleet-service's credential path" — no client calls
// it through this gateway directly.
func NewDefaultServiceRegistry() *ServiceRegistry {
	return NewServiceRegistry([]RoutingRule{
		{PathPrefix: "/v1/usage", ServiceName: "usage-service", ProtoPackage: "orca.usage.v1", Status: RouteWired},

		{PathPrefix: "/v1/auth", ServiceName: "auth-service", ProtoPackage: "orca.auth.v1", Status: RouteWired},
		{PathPrefix: "/v1/tenants", ServiceName: "tenant-service", ProtoPackage: "orca.tenant.v1", Status: RouteWired},
		{PathPrefix: "/v1/projects", ServiceName: "project-service", ProtoPackage: "orca.project.v1", Status: RouteWired},
		{PathPrefix: "/v1/infra", ServiceName: "infra-fleet-service", ProtoPackage: "orca.infrafleet.v1", Status: RouteWired},
		{PathPrefix: "/v1/git", ServiceName: "git-gateway-service", ProtoPackage: "orca.gitgateway.v1", Status: RouteWired},
		{PathPrefix: "/v1/scm", ServiceName: "scm-integration-service", ProtoPackage: "orca.scmintegration.v1", Status: RouteWired},
		{PathPrefix: "/v1/issues", ServiceName: "issue-tracking-service", ProtoPackage: "orca.issuetracking.v1", Status: RouteWired},
		{PathPrefix: "/v1/ai-providers", ServiceName: "ai-provider-service", ProtoPackage: "orca.aiprovider.v1", Status: RouteWired},
		{PathPrefix: "/v1/workflows", ServiceName: "workflow-service", ProtoPackage: "orca.workflow.v1", Status: RouteWired},
		{PathPrefix: "/v1/tasks", ServiceName: "task-service", ProtoPackage: "orca.task.v1", Status: RouteWired},
		{PathPrefix: "/v1/orchestration", ServiceName: "orchestration-service", ProtoPackage: "orca.orchestration.v1", Status: RouteWired},
		{PathPrefix: "/v1/automations", ServiceName: "automation-service", ProtoPackage: "orca.automation.v1", Status: RouteWired},
		{PathPrefix: "/v1/annotations", ServiceName: "annotation-service", ProtoPackage: "orca.annotation.v1", Status: RouteWired},
		{PathPrefix: "/v1/notifications", ServiceName: "notification-service", ProtoPackage: "orca.notification.v1", Status: RouteWired},
	})
}
