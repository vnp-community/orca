// Package config loads ai-provider-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	// CredentialBrokerAddr is credential-broker-service's gRPC target —
	// dialed for real by internal/adapter/grpcclient as of Epic B
	// (docs/execution-plan.md §8).
	CredentialBrokerAddr string
	// InfraFleetServiceAddr is infra-fleet-service's gRPC target — dialed
	// by TestConnection's InfraFleetClient (TASK-028).
	InfraFleetServiceAddr string
	// NATSURL is the JetStream endpoint the outbox relay publishes to —
	// same convention as usage-service's cmd/server/main.go wiring
	// (TASK-AIP-01-07) — and also the endpoint used to publish
	// orca.aiprovider.account.rate_limited (TASK-MB-02-04, SOL-MB-02).
	NATSURL string
	// DatabaseCredentialsFile is the path a Vault Agent sidecar renders
	// dynamic database credentials to in production (see
	// common/secrets.DatabaseCredentialsFromFile) — added for CR-DB-002/003
	// multi-database rollout (TASK-BE-DB-014), same field/default
	// usage-service's config already has. Falls back to DATABASE_DSN (via
	// Base) when the file doesn't exist, which is what local dev and this
	// scaffold's testcontainers path use instead.
	DatabaseCredentialsFile string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("ai-provider-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                    base,
		CredentialBrokerAddr:    commonconfig.StringEnv("CREDENTIAL_BROKER_ADDR", "credential-broker-service:9090"),
		InfraFleetServiceAddr:   commonconfig.StringEnv("INFRA_FLEET_SERVICE_ADDR", "infra-fleet-service:9090"),
		NATSURL:                 commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		DatabaseCredentialsFile: commonconfig.StringEnv("DATABASE_CREDENTIALS_FILE", "/vault/secrets/database-credentials"),
	}, nil
}
