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
	// AIProviderServiceAddr is where SwitchAgentAccount's AIProviderResolver
	// client dials ai-provider-service.ResolveProvider — TASK-AG-04-03, this
	// service's first outbound call to ai-provider-service.
	AIProviderServiceAddr string
	// NATSURL is where TASK-AG-05-05's AgentStatusPublisher (direct publish
	// for statusChanged) and its rateLimited outbox relay connect — same env
	// var convention as tenant-service/usage-service's NATSURL.
	NATSURL string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("infra-fleet-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                  base,
		ServerDeployment:      os.Getenv("ORCA_SERVER_DEPLOYMENT") == "true",
		AIProviderServiceAddr: commonconfig.StringEnv("AI_PROVIDER_SERVICE_ADDR", "ai-provider-service:9090"),
		NATSURL:               commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}
