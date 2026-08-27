// Package config loads auth-service's runtime configuration — env/flag
// parsing only, no business logic, per architecture/03-clean-architecture-guidelines.md.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	// BcryptCost is the bcrypt work factor CreateUser hashes new
	// passwords with. Floored at adapter/bcrypt.MinCost (12) regardless of
	// what's configured — see auth-service.md §9.
	BcryptCost int
	// SessionTTL is how long a session created by Login stays valid.
	SessionTTL time.Duration

	// ServiceTokenTTL is how long a JWT minted by IssueServiceToken stays
	// valid before "exp" — see internal/usecase/issue_service_token.go.
	ServiceTokenTTL time.Duration

	// DeviceAccessTokenTTL is how long the access JWT CompleteDevicePairing
	// mints stays valid before "exp" — see
	// internal/usecase/complete_device_pairing.go.
	DeviceAccessTokenTTL time.Duration

	// ServerAddress is api-gateway's public base URL, echoed back in
	// InitiateDevicePairingResponse so the mobile client knows where to
	// dial CompleteDevicePairing — see SOL-MB-01's "server-mode adaptation"
	// rationale (this desktop-originated pairing flow has no LAN-discovery
	// step to fall back on).
	ServerAddress string

	// OPABundlePath points requireAdminActor's OPA client
	// (internal/adapter/opaclient, via common/policy.Evaluator) at the
	// orca-authz Rego bundle on disk. Defaults to the bundle's location
	// relative to this service's module root when run the way this
	// service's README's "Running locally" section runs it (`cd
	// services/auth-service && go run ./cmd/server`); override for
	// container images that lay the bundle out elsewhere. Same env var
	// name as task-service/annotation-service's own OPABundlePath, for
	// consistency across Epic E's consuming services.
	OPABundlePath string

	// Bootstrap* configure the one-time first-admin creation
	// (internal/usecase/bootstrap.go) — no-op unless BootstrapTenantID and
	// BootstrapAdminEmail are both set. BootstrapAdminPassword empty =>
	// auto-generate and log once at startup, mirroring the old TS
	// backend's ORCA_ADMIN_EMAIL/ORCA_ADMIN_PASSWORD behavior.
	BootstrapTenantID      string
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("auth-service")
	if err != nil {
		return Config{}, err
	}

	bcryptCost, err := intEnv("BCRYPT_COST", 12)
	if err != nil {
		return Config{}, err
	}

	sessionTTL, err := durationEnv("SESSION_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	serviceTokenTTL, err := durationEnv("SERVICE_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}

	// DefaultDeviceAccessTokenTTL mirrors SessionTTL's default — a paired
	// mobile device is a long-lived client, not a short-lived
	// service-to-service call (unlike ServiceTokenTTL's 15m default).
	deviceAccessTokenTTL, err := durationEnv("DEVICE_ACCESS_TOKEN_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Base:                   base,
		BcryptCost:             bcryptCost,
		SessionTTL:             sessionTTL,
		ServiceTokenTTL:        serviceTokenTTL,
		DeviceAccessTokenTTL:   deviceAccessTokenTTL,
		ServerAddress:          commonconfig.StringEnv("SERVER_ADDRESS", ""),
		OPABundlePath:          commonconfig.StringEnv("OPA_BUNDLE_PATH", "../../policy/orca-authz"),
		BootstrapTenantID:      os.Getenv("BOOTSTRAP_TENANT_ID"),
		BootstrapAdminEmail:    os.Getenv("BOOTSTRAP_ADMIN_EMAIL"),
		BootstrapAdminPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
	}, nil
}

func intEnv(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid int for %s=%q: %w", key, v, err)
	}
	return n, nil
}

func durationEnv(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid duration for %s=%q: %w", key, v, err)
	}
	return d, nil
}
