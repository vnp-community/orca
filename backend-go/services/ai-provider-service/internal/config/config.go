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
	// (TASK-AIP-01-07).
	NATSURL string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("ai-provider-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                  base,
		CredentialBrokerAddr:  commonconfig.StringEnv("CREDENTIAL_BROKER_ADDR", "credential-broker-service:9090"),
		InfraFleetServiceAddr: commonconfig.StringEnv("INFRA_FLEET_SERVICE_ADDR", "infra-fleet-service:9090"),
		NATSURL:               commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}
