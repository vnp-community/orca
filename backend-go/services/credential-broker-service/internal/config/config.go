// Package config loads credential-broker-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
//
// No Vault-specific fields are added here: common/secrets.NewClient() reads
// VAULT_ADDR/VAULT_TOKEN directly from the environment itself (see that
// package's doc comment) — this service's own config has nothing to add on
// top of what common/secrets already covers.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("credential-broker-service")
	if err != nil {
		return Config{}, err
	}
	return Config{Base: base}, nil
}
