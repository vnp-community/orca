// Package config loads infra-fleet-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	"os"
	"strconv"
	"time"

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
	// NATSURL is where the transactional-outbox relay connects to publish
	// infra.outbox_events rows — both cmd/server/main.go's SSH-connect audit
	// outbox relay (TASK-AUTH-05-08) and TASK-FLEET-03-06's HealthPublisher
	// (dev_server.health_degraded) publish through it. Mirrors usage-service's
	// identical Config.NATSURL/NATS_URL convention. If NATS is unreachable at
	// startup, outbox rows still get written durably (see cmd/server/main.go),
	// they just queue up unpublished until a future restart.
	NATSURL string
	// FleetPollInterval is BL-FLEET-03's poll cadence — read from
	// FLEET_POLL_INTERVAL_SEC (default 30). An unparseable or non-positive
	// value falls back to the default, fail-safe: a misconfigured interval
	// should not silently disable polling (0) or spin (negative).
	FleetPollInterval time.Duration
	// FleetWebhookURL is BL-FLEET-03's status-change alert target — read
	// from FLEET_WEBHOOK_URL, empty (the default) disables webhook.Alerter
	// entirely (see that package's doc comment).
	FleetWebhookURL string
}

const defaultFleetPollIntervalSec = 30

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("infra-fleet-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:              base,
		ServerDeployment:  os.Getenv("ORCA_SERVER_DEPLOYMENT") == "true",
		NATSURL:           commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		FleetPollInterval: fleetPollIntervalFromEnv(),
		FleetWebhookURL:   os.Getenv("FLEET_WEBHOOK_URL"),
	}, nil
}

func fleetPollIntervalFromEnv() time.Duration {
	sec := defaultFleetPollIntervalSec
	if raw := os.Getenv("FLEET_POLL_INTERVAL_SEC"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			sec = parsed
		}
	}
	return time.Duration(sec) * time.Second
}
