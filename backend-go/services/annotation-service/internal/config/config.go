// Package config loads annotation-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

// Config is commonconfig.Base plus OPABundlePath — the one
// service-specific setting annotation-service needs, for the Epic E
// author-only edit/delete check (internal/adapter/opaclient). Per
// annotation-service.md §6, this is otherwise deliberately the simplest
// service in the catalog: no NATS/eventbus (no events to publish), no
// third-party integration.
type Config struct {
	commonconfig.Base
	// OPABundlePath points common/policy.Evaluator at the orca-authz Rego
	// bundle. Defaults to its location relative to this service's module
	// root (../../policy/orca-authz), matching the "cd services/
	// annotation-service && go run ./cmd/server" invocation in README.md.
	OPABundlePath string
	// AuthServiceAddr is UpdateAnnotation/DeleteAnnotation's audit-append
	// dependency (TASK-BE-021/CR-RBAC-005, common/auditclient) — the only
	// other service this one talks to purely to write audit_log rows.
	AuthServiceAddr string
	// DatabaseCredentialsFile is the path a Vault Agent sidecar renders
	// dynamic database credentials to in production (see
	// common/secrets.DatabaseCredentialsFromFile). Falls back to
	// DATABASE_DSN (via Base) when the file doesn't exist, which is what
	// local dev and this service's testcontainers path use instead. Added
	// for CR-DB-002/CR-DB-003's multi-database rollout (BE-DB-SOL-005),
	// replacing the direct cfg.DatabaseDSN read main.go used to do —
	// mirrors usage-service's (the pilot) identical field 1:1.
	DatabaseCredentialsFile string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("annotation-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                    base,
		OPABundlePath:           commonconfig.StringEnv("OPA_BUNDLE_PATH", "/policy/orca-authz"),
		AuthServiceAddr:         commonconfig.StringEnv("AUTH_SERVICE_ADDR", "auth-service:9090"),
		DatabaseCredentialsFile: commonconfig.StringEnv("DATABASE_CREDENTIALS_FILE", "/vault/secrets/database-credentials"),
	}, nil
}
