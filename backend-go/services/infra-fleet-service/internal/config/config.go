// Package config loads infra-fleet-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

// Config is deliberately just commonconfig.Base today — unlike
// usage-service, this scaffold doesn't publish NATS events yet (see this
// service's README "Known gaps": connection.established/connection.lost
// event publishing per the design doc's §7 is not wired), so there is no
// service-specific setting to add on top of Base yet.
type Config struct {
	commonconfig.Base
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("infra-fleet-service")
	if err != nil {
		return Config{}, err
	}
	return Config{Base: base}, nil
}
