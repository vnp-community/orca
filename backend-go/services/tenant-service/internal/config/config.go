// Package config loads tenant-service's runtime configuration — env/flag
// parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

// Config carries commonconfig.Base plus NATSURL. tenant-service still makes
// zero outbound synchronous service calls (Vault only, for DB credentials —
// not wired in this scaffold either, see README "Known gaps") — NATSURL is
// only for the best-effort cross-replica profile-cache invalidation
// broadcast (docs/execution-plan.md Epic F). NATS is required-in-spirit,
// not required-to-boot: main.go degrades gracefully to today's
// TTL-bounded-only staleness if it's unreachable, since tenant-service sits
// on the critical path for every other service's tenant resolution and
// must not crash-loop over an optional dependency.
type Config struct {
	commonconfig.Base
	NATSURL string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("tenant-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:    base,
		NATSURL: commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}
