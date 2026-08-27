// Package config loads workflow-service's runtime configuration — env/flag
// parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	"strings"

	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	// WebhookAllowlistHosts additionally restricts the Webhook step
	// executor's targets to these exact hostnames — empty by default (see
	// internal/adapter/stepexecutors.WebhookExecutor's doc comment and this
	// service's README: the private/loopback/link-local IP block applies
	// regardless of whether this list is configured).
	WebhookAllowlistHosts []string
	// InfraFleetServiceAddr is where internal/adapter/infrafleetclient dials
	// infra-fleet-service's ResolveConnection and Relay RPCs to run the
	// Agent/Shell/Notification step executors on the execution plane —
	// mirrors git-gateway-service's identically-named config field.
	InfraFleetServiceAddr string
	// ProjectServiceAddr is where internal/adapter/serverresolver dials
	// project-service's GetProject RPC to resolve a "project:<id>" Target
	// into a dev_server_id (mirrors git-gateway-service's identically-named
	// config field), doubles as ProjectContextResolver's dependency
	// (GetProjectContext, TASK-PRF-04-01/02), and is what the
	// STEP_TYPE_CLEANUP_WORKTREES executor
	// (internal/usecase.CleanupWorktreesStepExecutor, BL-AT-04) dials to
	// list candidate worktrees.
	ProjectServiceAddr string
	// GitGatewayServiceAddr/AutomationServiceAddr are where the
	// STEP_TYPE_CLEANUP_WORKTREES executor dials to delete worktrees and
	// write the audit report — new outbound dependency edges this service
	// didn't have before BL-AT-04.
	GitGatewayServiceAddr string
	AutomationServiceAddr string
	// TenantServiceAddr is ProfileResolver's dependency — a NEW dial, this
	// service never called tenant-service before this task (closes the
	// prose/graph gap tenant-service.md §7 already documented).
	TenantServiceAddr string
	// AIProviderServiceAddr is where internal/adapter/providerresolver dials
	// ai-provider-service's ResolveProvider/ListAccounts RPCs to pick which
	// account an Agent step uses — mirrors git-gateway-service's
	// identically-named config field.
	AIProviderServiceAddr string
	// AuthServiceAddr is where internal/adapter/opachecker dials
	// auth-service's ListUsers RPC to answer "is this user an admin" —
	// see usecase.OPAChecker's doc comment (BUG-WF-03's publish-approval
	// gate).
	AuthServiceAddr string
	// NATSURL is where the outbox relay (SOL-PW-04, TASK-PW-04-06)
	// publishes workflow.* domain events — same env var name/default as
	// usage-service's identical field.
	NATSURL string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("workflow-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                  base,
		WebhookAllowlistHosts: splitCSV(commonconfig.StringEnv("WEBHOOK_ALLOWLIST_HOSTS", "")),
		InfraFleetServiceAddr: commonconfig.StringEnv("INFRA_FLEET_SERVICE_ADDR", "infra-fleet-service:9090"),
		ProjectServiceAddr:    commonconfig.StringEnv("PROJECT_SERVICE_ADDR", "project-service:9090"),
		GitGatewayServiceAddr: commonconfig.StringEnv("GIT_GATEWAY_SERVICE_ADDR", "git-gateway-service:9090"),
		AutomationServiceAddr: commonconfig.StringEnv("AUTOMATION_SERVICE_ADDR", "automation-service:9090"),
		TenantServiceAddr:     commonconfig.StringEnv("TENANT_SERVICE_ADDR", "tenant-service:9090"),
		AIProviderServiceAddr: commonconfig.StringEnv("AI_PROVIDER_SERVICE_ADDR", "ai-provider-service:9090"),
		AuthServiceAddr:       commonconfig.StringEnv("AUTH_SERVICE_ADDR", "auth-service:9090"),
		NATSURL:               commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
