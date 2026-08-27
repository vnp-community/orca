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
