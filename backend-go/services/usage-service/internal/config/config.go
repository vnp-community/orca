// Package config loads usage-service's runtime configuration — env/flag
// parsing only, no business logic, per architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	NATSURL string
	// DatabaseCredentialsFile is the path a Vault Agent sidecar renders
	// dynamic Postgres credentials to in production (see
	// common/secrets.DatabaseCredentialsFromFile). Falls back to DATABASE_DSN
	// (via Base) when the file doesn't exist, which is what local dev and
	// this scaffold's testcontainers path use instead.
	DatabaseCredentialsFile string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("usage-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                    base,
		NATSURL:                 commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		DatabaseCredentialsFile: commonconfig.StringEnv("DATABASE_CREDENTIALS_FILE", "/vault/secrets/database-credentials"),
	}, nil
}
