// Package config loads task-service's runtime configuration — env/flag
// parsing only, no business logic, per architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("task-service")
	if err != nil {
		return Config{}, err
	}
	return Config{Base: base}, nil
}
