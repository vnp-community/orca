// Package config loads notification-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	NATSURL string
	// CredentialBrokerAddr is credential-broker-service's gRPC target —
	// dialed for real by internal/adapter/vaultsigner as of Epic B
	// (docs/execution-plan.md §8), which replaced this service's previous
	// direct common/secrets.TransitEncrypt call with a
	// credentialbrokerv1.SignVapidPayload RPC, closing the one documented
	// exception to "no service but credential-broker-service touches
	// Vault directly."
	CredentialBrokerAddr string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("notification-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                 base,
		NATSURL:              commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		CredentialBrokerAddr: commonconfig.StringEnv("CREDENTIAL_BROKER_ADDR", "credential-broker-service:9090"),
	}, nil
}
