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
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("notification-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:    base,
		NATSURL: commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}
