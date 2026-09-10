// Package config loads issue-tracking-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

// Config's DatabaseDSN (from commonconfig.Base) is now required-to-boot,
// as of Epic G (docs/execution-plan.md) — issue-tracking-service gained a
// minimal, outbox-only database. It still owns no queryable copy of issue
// data itself (design doc §2/§5: Jira and Linear remain the systems of
// record) — see internal/adapter/postgres's package doc comment. NATSURL
// is required-in-spirit, not required-to-boot for the OUTBOX RELAY
// specifically: main.go degrades gracefully if NATS is unreachable at
// startup (outbox rows still get written durably, they just queue up
// unpublished until a future restart — see cmd/server/main.go), same
// posture as every other NATS-consuming service here. LinkIssue itself now
// only fails closed on a database problem, not a NATS one — see
// internal/usecase/link_issue.go.
type Config struct {
	commonconfig.Base
	NATSURL string
	// CredentialBrokerAddr is credential-broker-service's gRPC target —
	// dialed for real by internal/adapter/credential as of Epic B
	// (docs/execution-plan.md §8).
	CredentialBrokerAddr string

	// DatabaseCredentialsFile is the path a Vault Agent sidecar renders
	// dynamic Postgres credentials to in production (see
	// common/secrets.DatabaseCredentialsFromFile) — same convention as
	// usage-service's config. Falls back to DATABASE_DSN (via Base) when
	// the file doesn't exist, which is what local dev and this scaffold's
	// testcontainers path use instead.
	DatabaseCredentialsFile string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("issue-tracking-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                    base,
		NATSURL:                 commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		CredentialBrokerAddr:    commonconfig.StringEnv("CREDENTIAL_BROKER_ADDR", "credential-broker-service:9090"),
		DatabaseCredentialsFile: commonconfig.StringEnv("DATABASE_CREDENTIALS_FILE", "/vault/secrets/database-credentials"),
	}, nil
}
