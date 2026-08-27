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
	// ProjectServiceAddr/GitGatewayServiceAddr/AutomationServiceAddr are
	// where the STEP_TYPE_CLEANUP_WORKTREES executor
	// (internal/usecase.CleanupWorktreesStepExecutor, BL-AT-04) dials to
	// list candidate worktrees, delete them, and write the audit report —
	// three new outbound dependency edges this service didn't have before.
	ProjectServiceAddr    string
	GitGatewayServiceAddr string
	AutomationServiceAddr string
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
