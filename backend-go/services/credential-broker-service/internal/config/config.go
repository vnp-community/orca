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
	// DatabaseCredentialsFile is the path a Vault Agent sidecar renders
	// dynamic database credentials to in production (see
	// common/secrets.DatabaseCredentialsFromFile) — added for CR-DB-002/003
	// multi-database rollout, same field/default usage-service's config
	// already has. Falls back to DATABASE_DSN (via Base) when the file
	// doesn't exist, which is what local dev and this scaffold's
	// testcontainers path use instead.
	DatabaseCredentialsFile string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("credential-broker-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                    base,
		DatabaseCredentialsFile: commonconfig.StringEnv("DATABASE_CREDENTIALS_FILE", "/vault/secrets/database-credentials"),
	}, nil
}
