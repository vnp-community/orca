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
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("workflow-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                  base,
		WebhookAllowlistHosts: splitCSV(commonconfig.StringEnv("WEBHOOK_ALLOWLIST_HOSTS", "")),
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
