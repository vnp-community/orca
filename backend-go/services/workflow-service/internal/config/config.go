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
	// TenantServiceAddr is ProfileResolver's dependency — a NEW dial, this
	// service never called tenant-service before this task (closes the
	// prose/graph gap tenant-service.md §7 already documented).
	TenantServiceAddr string
	// ProjectServiceAddr is ProjectContextResolver's dependency
	// (GetProjectContext, TASK-PRF-04-01/02) — also new.
	ProjectServiceAddr string
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
		TenantServiceAddr:     commonconfig.StringEnv("TENANT_SERVICE_ADDR", "tenant-service:9090"),
		ProjectServiceAddr:    commonconfig.StringEnv("PROJECT_SERVICE_ADDR", "project-service:9090"),
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
