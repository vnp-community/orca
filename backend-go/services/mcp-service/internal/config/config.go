// Package config loads mcp-service's runtime configuration — env parsing
// only, no business logic.
package config

import (
	"fmt"
	"os"
	"strconv"

	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	NATSURL string
	// DatabaseCredentialsFile is the Vault-Agent-rendered credentials file;
	// falls back to DATABASE_DSN when absent (see common/secrets).
	DatabaseCredentialsFile string
	// TenantDefaultEnabled is the `enabled` value written when a tenant's
	// settings row is created lazily (D6). It never changes risk defaults,
	// hard-denies, scopes or the kill switch.
	TenantDefaultEnabled bool
	// DefaultMaxTokenDays is the PAT lifetime ceiling for new tenants (T4).
	DefaultMaxTokenDays int
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("mcp-service")
	if err != nil {
		return Config{}, err
	}
	enabled, err := boolEnv("MCP_TENANT_DEFAULT_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	days, err := intEnv("MCP_DEFAULT_MAX_TOKEN_DAYS", 90)
	if err != nil {
		return Config{}, err
	}
	// Mirrors the CHECK on mcp.tenant_settings.max_token_days so a bad env
	// fails at startup instead of on the first tenant's lazy insert.
	if days < 1 || days > 90 {
		return Config{}, fmt.Errorf("config: MCP_DEFAULT_MAX_TOKEN_DAYS=%d out of range 1..90", days)
	}
	return Config{
		Base:                    base,
		NATSURL:                 commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		DatabaseCredentialsFile: commonconfig.StringEnv("DATABASE_CREDENTIALS_FILE", "/vault/secrets/database-credentials"),
		TenantDefaultEnabled:    enabled,
		DefaultMaxTokenDays:     days,
	}, nil
}

func boolEnv(key string, def bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("config: invalid bool for %s=%q: %w", key, v, err)
	}
	return b, nil
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
