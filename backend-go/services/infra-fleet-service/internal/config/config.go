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
	// DatabaseCredentialsFile is the path a Vault Agent sidecar renders
	// dynamic Postgres credentials to in production (see
	// common/secrets.DatabaseCredentialsFromFile). Falls back to DATABASE_DSN
	// (via Base) when the file doesn't exist, which is what local dev and
	// this scaffold's testcontainers path use instead.
	DatabaseCredentialsFile string
	// NATSURL is where the transactional-outbox relay connects to publish
	// infra.outbox_events rows — both cmd/server/main.go's SSH-connect audit
	// outbox relay (TASK-AUTH-05-08) and TASK-FLEET-03-06's HealthPublisher
	// (dev_server.health_degraded) publish through it, as well as
	// TASK-AG-05-05's AgentStatusPublisher (direct publish for
	// statusChanged) and its rateLimited outbox relay, TASK-MB-02-01's
	// agent-lifecycle event publisher, and usecase.PollFleetHealth's
	// orca.infrafleet.dev_server.disconnected publish. Mirrors
	// usage-service's identical Config.NATSURL/NATS_URL convention. If NATS
	// is unreachable at startup, outbox rows still get written durably (see
	// cmd/server/main.go), they just queue up unpublished until a future
	// restart — and mobile push notifications degrade to "none", never a
	// fatal error.
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
	// CredentialBrokerAddr is credential-broker-service's gRPC target —
	// dialed for relay-websocket agent token write/resolve (SOL-AWS-01).
	CredentialBrokerAddr string
	// AIProviderServiceAddr is where SwitchAgentAccount's AIProviderResolver
	// client dials ai-provider-service.ResolveProvider — TASK-AG-04-03, this
	// service's first outbound call to ai-provider-service.
	AIProviderServiceAddr string
	// EphemeralVmSshMode selects which usecase.EphemeralVmSshProvisioner
	// implementation cmd/server/main.go wires for `ssh`-type ephemeral VM
	// recipe results (TASK-BE-EVM-012, BE-SOL-EVM-004 §5a): "agent-outbound"
	// (Hướng A, TASK-BE-EVM-014) or "backend-relay-deploy" (Hướng B,
	// TASK-BE-EVM-013). Any value other than exactly "agent-outbound" is
	// treated as "backend-relay-deploy" — fail-safe default, matching
	// ServerDeployment's "unrecognized value doesn't silently widen
	// behavior" convention just above, and because backend-relay-deploy
	// reuses the already-shipped sshrelay/sshconn pipeline (no new agent
	// capability required).
	EphemeralVmSshMode string
	// AuthServiceAddr is EstablishConnection's audit-append dependency
	// (TASK-BE-022/CR-RBAC-005, common/auditclient) — the only other service
	// this one talks to purely to write audit_log rows.
	AuthServiceAddr string
}

const defaultFleetPollIntervalSec = 30

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("infra-fleet-service")
	if err != nil {
		return Config{}, err
	}
	sshMode := os.Getenv("EPHEMERAL_VM_SSH_MODE")
	if sshMode != "agent-outbound" {
		sshMode = "backend-relay-deploy"
	}

	return Config{
		Base:                    base,
		ServerDeployment:        os.Getenv("ORCA_SERVER_DEPLOYMENT") == "true",
		DatabaseCredentialsFile: commonconfig.StringEnv("DATABASE_CREDENTIALS_FILE", "/vault/secrets/database-credentials"),
		NATSURL:                 commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		FleetPollInterval:       fleetPollIntervalFromEnv(),
		FleetWebhookURL:         os.Getenv("FLEET_WEBHOOK_URL"),
		CredentialBrokerAddr:    commonconfig.StringEnv("CREDENTIAL_BROKER_ADDR", "credential-broker-service:9090"),
		AIProviderServiceAddr:   commonconfig.StringEnv("AI_PROVIDER_SERVICE_ADDR", "ai-provider-service:9090"),
		EphemeralVmSshMode:      sshMode,
		AuthServiceAddr:         commonconfig.StringEnv("AUTH_SERVICE_ADDR", "auth-service:9090"),
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
