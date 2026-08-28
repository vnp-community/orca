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
	// (internal/usecase/bootstrap.go) — no-op unless BootstrapAdminEmail is
	// set. BootstrapAdminPassword empty => auto-generate and log once at
	// startup, mirroring the old TS backend's
	// ORCA_ADMIN_EMAIL/ORCA_ADMIN_PASSWORD behavior. BootstrapCompanyName
	// is optional (empty => bootstrap.go derives one from the admin
	// email's domain) — there is no BootstrapTenantID: tenant-service
	// originates the tenant id itself (see bootstrap.go's doc comment,
	// specs/backend-go/bugs/missing-v2/BUG-002/SOL-002).
	BootstrapCompanyName   string
	BootstrapAdminEmail    string
	BootstrapAdminPassword string

	// TenantServiceAddr is where bootstrap.go's TenantProvisioner dials to
	// originate a tenant for the first admin — only used when
	// BootstrapAdminEmail is set (see cmd/server/main.go).
	TenantServiceAddr string
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

	return Config{
		Base:                   base,
		BcryptCost:             bcryptCost,
		SessionTTL:             sessionTTL,
		ServiceTokenTTL:        serviceTokenTTL,
		OPABundlePath:          commonconfig.StringEnv("OPA_BUNDLE_PATH", "/policy/orca-authz"),
		BootstrapCompanyName:   os.Getenv("BOOTSTRAP_COMPANY_NAME"),
		BootstrapAdminEmail:    os.Getenv("BOOTSTRAP_ADMIN_EMAIL"),
		BootstrapAdminPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
		TenantServiceAddr:      commonconfig.StringEnv("TENANT_SERVICE_ADDR", "tenant-service:9090"),
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
