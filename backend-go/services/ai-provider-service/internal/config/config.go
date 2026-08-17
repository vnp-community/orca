// Package config loads ai-provider-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	// CredentialBrokerAddr is credential-broker-service's gRPC target.
	// Unused by internal/adapter/grpcclient's current stub (see that
	// package's doc comment) — read here so main.go's composition root
	// already threads it through, ready for the real dial once that
	// service exists.
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
