// Package config loads task-service's runtime configuration — env/flag
// parsing only, no business logic, per architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	// OPABundlePath points ResolvePermission's OPA client
	// (internal/adapter/opaclient, via common/policy.Evaluator) at the
	// orca-authz Rego bundle on disk. Defaults to the bundle's location
	// relative to this service's module root when run the way this
	// service's README's "Running locally" section runs it (`cd
	// services/task-service && go run ./cmd/server`); override for
	// container images that lay the bundle out elsewhere.
	OPABundlePath string

	// InfraFleetServiceAddr is where SimpleExecutor/AIDecompose's real
	// (TASK-224) grpcclient adapters dial infra-fleet-service's
	// ResolveConnection and Relay RPCs.
	InfraFleetServiceAddr string
	// AIProviderServiceAddr is where AIDecompose's AIProviderContextResolver
	// dials ai-provider-service's ResolveProvider RPC.
	AIProviderServiceAddr string
	// GitGatewayServiceAddr is where TechStackDetector (TASK-TG-02-03) and
	// WorktreeProvisioner (TASK-TG-04-02/SOL-TG-04) dial git-gateway-service's
	// ReadFile/CreateWorktree RPCs — a genuine scope addition (a new
	// task-service -> git-gateway-service dependency edge, flagged
	// explicitly in both tasks' Context sections).
	GitGatewayServiceAddr string
	// ProjectServiceAddr is ProjectContextResolver's dependency — dials
	// project-service's GetProjectContext (SimpleExecutor's profile-aware
	// env injection, TASK-PRF-04-07/08), GetProject/ListRepos
	// (TASK-TG-02-04's AIDecompose context bundle) RPCs, and
	// WorktreeProvisioner's task->repo_id resolution (see that adapter's doc
	// comment for why this lookup is needed at all).
	ProjectServiceAddr string
	// TenantServiceAddr is ProfileResolver's dependency (SimpleExecutor's
	// profile-aware env injection, TASK-PRF-04-07/08) AND TeamScopeResolver's
	// (TASK-TG-03-03, dials tenant-service's ListTeamsForUser RPC,
	// TASK-TG-03-02) — one address, two independent dials.
	TenantServiceAddr string
	// OrchestrationServiceAddr is where ComplexExecutor (TASK-TG-04-04/
	// BE-SOL-002 integration addendum) dials orchestration-service's
	// StartCoordinatorRun RPC — the complex (subtree-dispatch, Engine 2)
	// execution path.
	OrchestrationServiceAddr string
	// WorkflowServiceAddr is where WorkflowExecutor (TASK-FT-002-03) dials
	// workflow-service's Execute RPC for Engine 3 dispatch.
	WorkflowServiceAddr string
	// NATSURL is where the transactional-outbox relay (TASK-TG-03-07,
	// TASK-PW-04-04) publishes both grant audit events and task.* domain
	// events, and where the execution-status mirror consumer
	// (BE-SOL-003/TASK-FT-003-05) subscribes
	// orca.orchestration.task.statuschanged / orca.workflow.step.completed —
	// mirrors usage-service's identical config field.
	NATSURL string
	// AuthServiceAddr is ResolvePermission's audit-append dependency
	// (TASK-BE-020/CR-RBAC-005, common/auditclient) — the only other service
	// this one talks to purely to write audit_log rows.
	AuthServiceAddr string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("task-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base: base,
		// Matches this service's deploy/Dockerfile `COPY --from=build
		// /src/policy/orca-authz /policy/orca-authz` and dev
		// docker-compose.yml's identical bind-mount — NOT a relative
		// go-run-from-module-root path, since the container never lays the
		// bundle out there.
		OPABundlePath:            commonconfig.StringEnv("OPA_BUNDLE_PATH", "/policy/orca-authz"),
		InfraFleetServiceAddr:    commonconfig.StringEnv("INFRA_FLEET_SERVICE_ADDR", "infra-fleet-service:9090"),
		AIProviderServiceAddr:    commonconfig.StringEnv("AI_PROVIDER_SERVICE_ADDR", "ai-provider-service:9090"),
		GitGatewayServiceAddr:    commonconfig.StringEnv("GIT_GATEWAY_SERVICE_ADDR", "git-gateway-service:9090"),
		ProjectServiceAddr:       commonconfig.StringEnv("PROJECT_SERVICE_ADDR", "project-service:9090"),
		TenantServiceAddr:        commonconfig.StringEnv("TENANT_SERVICE_ADDR", "tenant-service:9090"),
		OrchestrationServiceAddr: commonconfig.StringEnv("ORCHESTRATION_SERVICE_ADDR", "orchestration-service:9090"),
		WorkflowServiceAddr:      commonconfig.StringEnv("WORKFLOW_SERVICE_ADDR", "workflow-service:9090"),
		NATSURL:                  commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		AuthServiceAddr:          commonconfig.StringEnv("AUTH_SERVICE_ADDR", "auth-service:9090"),
	}, nil
}
