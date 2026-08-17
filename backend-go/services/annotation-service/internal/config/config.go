// Package config loads annotation-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

// Config is just commonconfig.Base — annotation-service has no
// service-specific settings. Per annotation-service.md §6, this is
// deliberately the simplest service in the catalog: no NATS/eventbus (no
// events to publish), no third-party integration.
type Config struct {
	commonconfig.Base
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("annotation-service")
	if err != nil {
		return Config{}, err
	}
	return Config{Base: base}, nil
}
