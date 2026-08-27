// Package config loads infra-fleet-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	"os"

	commonconfig "github.com/stablyai/orca-go/common/config"
)

// Config embeds commonconfig.Base plus ServerDeployment — the one
// service-specific setting TASK-185 (Terminal/PTY RPCs) adds: whether this
// deployment is running in server mode (multiple users/tenants against
// shared dev servers) vs. the desktop/Electron single-user mode. See
// usecase.SpawnTerminalSession's doc comment: SpawnTerminalSessionRequest's
// empty connection_id ("host-local") is rejected when ServerDeployment is
// true, per the proto's own doc comment on that field.
type Config struct {
	commonconfig.Base
	// ServerDeployment is read from ORCA_SERVER_DEPLOYMENT (default false —
	// local/desktop dev is the common case for running this service
	// standalone). Any value other than exactly "true" is treated as false,
	// fail-safe: an unrecognized value should not silently widen what
	// host-local terminal spawning is allowed.
	ServerDeployment bool
	// NATSURL is where the transactional-outbox relay (TASK-FLEET-03-06's
	// HealthPublisher, dev_server.health_degraded) publishes — mirrors
	// usage-service's identical Config.NATSURL/NATS_URL convention. If NATS
	// is unreachable at startup, outbox rows still get written durably (see
	// cmd/server/main.go), they just queue up unpublished until a future
	// restart.
	NATSURL string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("infra-fleet-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:             base,
		ServerDeployment: os.Getenv("ORCA_SERVER_DEPLOYMENT") == "true",
		NATSURL:          commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}
