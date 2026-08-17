// Package config loads tenant-service's runtime configuration — env/flag
// parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

// Config is just commonconfig.Base — tenant-service has no NATS/eventbus
// dependency: per tenant-service.md §7, it makes zero outbound synchronous
// service calls (Vault only, for DB credentials — not wired in this
// scaffold either, see README "Known gaps") and publishes no events.
type Config struct {
	commonconfig.Base
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("tenant-service")
	if err != nil {
		return Config{}, err
	}
	return Config{Base: base}, nil
}
