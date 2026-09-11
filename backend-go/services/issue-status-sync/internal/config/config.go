// Package config loads issue-status-sync's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base

	// NATSURL is where this service's eventbus.Consumer subscribes —
	// project-service's/scm-integration-service's own outbox relays are the
	// publishers on the other end.
	NATSURL string

	// DatabaseCredentialsFile is the path a Vault Agent sidecar renders
	// dynamic database credentials to in production (see
	// common/secrets.DatabaseCredentialsFromFile). Falls back to
	// DATABASE_DSN (via Base) when the file doesn't exist, which is what
	// local dev and this service's testcontainers path use instead. Added
	// alongside the mysql/TiDB adapter rollout (BE-DB-SOL-003) to match
	// usage-service's pilot pattern — no new env var, same
	// DATABASE_CREDENTIALS_FILE name/default every other rolled-out
	// service uses.
	DatabaseCredentialsFile string

	IssueTrackingServiceAddr  string
	SCMIntegrationServiceAddr string
	ProjectServiceAddr        string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("issue-status-sync")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                      base,
		NATSURL:                   commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		DatabaseCredentialsFile:   commonconfig.StringEnv("DATABASE_CREDENTIALS_FILE", "/vault/secrets/database-credentials"),
		IssueTrackingServiceAddr:  commonconfig.StringEnv("ISSUE_TRACKING_SERVICE_ADDR", "issue-tracking-service:9090"),
		SCMIntegrationServiceAddr: commonconfig.StringEnv("SCM_INTEGRATION_SERVICE_ADDR", "scm-integration-service:9090"),
		ProjectServiceAddr:        commonconfig.StringEnv("PROJECT_SERVICE_ADDR", "project-service:9090"),
	}, nil
}
