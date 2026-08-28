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
	// WorktreeProvisioner (TASK-TG-04-02) dial git-gateway-service's
	// ReadFile/CreateWorktree RPCs — a genuine scope addition (a new
	// task-service -> git-gateway-service dependency edge, flagged
	// explicitly in both tasks' Context sections).
	GitGatewayServiceAddr string
	// ProjectServiceAddr is ProjectContextResolver's dependency — dials
	// project-service's GetProjectContext (SimpleExecutor's profile-aware
	// env injection, TASK-PRF-04-07/08) and GetProject/ListRepos
	// (TASK-TG-02-04's AIDecompose context bundle) RPCs.
	ProjectServiceAddr string
	// TenantServiceAddr is ProfileResolver's dependency (SimpleExecutor's
	// profile-aware env injection, TASK-PRF-04-07/08) AND TeamScopeResolver's
	// (TASK-TG-03-03, dials tenant-service's ListTeamsForUser RPC,
	// TASK-TG-03-02) — one address, two independent dials.
	TenantServiceAddr string
	// OrchestrationServiceAddr is where ComplexExecutor (TASK-TG-04-04)
	// dials orchestration-service's StartCoordinatorRun RPC — the complex
	// (subtree-dispatch) execution path.
	OrchestrationServiceAddr string
	// NATSURL is where the transactional-outbox relay (TASK-TG-03-07,
	// TASK-PW-04-04) publishes both grant audit events and task.* domain
	// events — mirrors usage-service's identical config field.
	NATSURL string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("task-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                     base,
		OPABundlePath:            commonconfig.StringEnv("OPA_BUNDLE_PATH", "../../policy/orca-authz"),
		InfraFleetServiceAddr:    commonconfig.StringEnv("INFRA_FLEET_SERVICE_ADDR", "infra-fleet-service:9090"),
		AIProviderServiceAddr:    commonconfig.StringEnv("AI_PROVIDER_SERVICE_ADDR", "ai-provider-service:9090"),
		GitGatewayServiceAddr:    commonconfig.StringEnv("GIT_GATEWAY_SERVICE_ADDR", "git-gateway-service:9090"),
		ProjectServiceAddr:       commonconfig.StringEnv("PROJECT_SERVICE_ADDR", "project-service:9090"),
		TenantServiceAddr:        commonconfig.StringEnv("TENANT_SERVICE_ADDR", "tenant-service:9090"),
		OrchestrationServiceAddr: commonconfig.StringEnv("ORCHESTRATION_SERVICE_ADDR", "orchestration-service:9090"),
		NATSURL:                  commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}
