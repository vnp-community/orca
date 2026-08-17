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
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("ai-provider-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                 base,
		CredentialBrokerAddr: commonconfig.StringEnv("CREDENTIAL_BROKER_ADDR", "credential-broker-service:9090"),
	}, nil
}
