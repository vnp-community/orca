// Package config loads a service's runtime configuration from environment
// variables. Every service defines its own typed Config struct and embeds
// Base for the settings every service needs identically — see
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// internal/config/ layer contract: env/flag parsing only, no business logic.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Base holds the settings every service's cmd/server/main.go needs,
// regardless of that service's own domain-specific config.
type Base struct {
	// ServiceName identifies this service in logs/traces/metrics.
	ServiceName string
	// GRPCPort is the port the service's gRPC server listens on.
	GRPCPort int
	// HTTPPort serves /healthz, /readyz, and /metrics (see common/health).
	HTTPPort int
	// DatabaseDSN is populated from a file written by the Vault Agent
	// sidecar (dynamic credentials), not a static env var, in production —
	// see specs/backend-go/architecture/06-secrets-vault-architecture.md.
	// Falling back to an env var here is a local-dev convenience only.
	DatabaseDSN string
	// OTLPEndpoint is where traces/metrics are exported; empty disables export
	// (useful for unit/integration tests).
	OTLPEndpoint string
}

// LoadBase reads the common settings from the environment. serviceName is
// passed explicitly rather than read from an env var so every service's
// main.go is unambiguous about its own identity at a glance.
func LoadBase(serviceName string) (Base, error) {
	grpcPort, err := intEnv("GRPC_PORT", 9090)
	if err != nil {
		return Base{}, err
	}
	httpPort, err := intEnv("HTTP_PORT", 8080)
	if err != nil {
		return Base{}, err
	}
	return Base{
		ServiceName:  serviceName,
		GRPCPort:     grpcPort,
		HTTPPort:     httpPort,
		DatabaseDSN:  os.Getenv("DATABASE_DSN"),
		OTLPEndpoint: os.Getenv("OTLP_ENDPOINT"),
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

// StringEnv reads a string env var, returning def if unset. Exported for
// services' own config.Load() to reuse the same convention.
func StringEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
