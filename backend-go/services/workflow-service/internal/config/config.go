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
	// into a dev_server_id — mirrors git-gateway-service's identically-named
	// config field.
	ProjectServiceAddr string
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
		AIProviderServiceAddr: commonconfig.StringEnv("AI_PROVIDER_SERVICE_ADDR", "ai-provider-service:9090"),
		AuthServiceAddr:       commonconfig.StringEnv("AUTH_SERVICE_ADDR", "auth-service:9090"),
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
