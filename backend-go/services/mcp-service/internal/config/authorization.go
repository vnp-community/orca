package config

import (
	"errors"
	"fmt"
	"time"

	commonconfig "github.com/stablyai/orca-go/common/config"
	"github.com/stablyai/orca-go/common/oauthmetadata"
)

// AuthorizationConfig configures the consent/grant half of OAuth
// (BE-MCP-SOL-005). It is off unless MCP_PUBLIC_BASE_URL (or PUBLIC_BASE_URL)
// is set, because the RFC 9207 `iss` value must come from configuration and
// never from a request. Kept apart from Config/Load so enabling it changes
// nothing about how the rest of the service boots.
type AuthorizationConfig struct {
	Enabled          bool
	Issuer           string // public base URL, e.g. https://orca.example.com
	AuthServiceAddr  string
	InternalToken    string // shared secret auth-service requires on internal OAuth RPCs
	ConsentTTL       time.Duration
	ReconcileEvery   time.Duration
	ReconcileBatch   int
	AuthCallDeadline time.Duration
}

func LoadAuthorization() (AuthorizationConfig, error) {
	issuer := commonconfig.StringEnv("MCP_PUBLIC_BASE_URL", commonconfig.StringEnv("PUBLIC_BASE_URL", ""))
	if issuer == "" {
		return AuthorizationConfig{}, nil
	}
	ttl, err := durationEnv("MCP_CONSENT_TTL", 10*time.Minute)
	if err != nil {
		return AuthorizationConfig{}, err
	}
	every, err := durationEnv("MCP_GRANT_RECONCILE_INTERVAL", 15*time.Second)
	if err != nil {
		return AuthorizationConfig{}, err
	}
	cfg := AuthorizationConfig{
		Enabled: true, Issuer: issuer,
		AuthServiceAddr:  commonconfig.StringEnv("AUTH_SERVICE_ADDR", "auth-service:9090"),
		InternalToken:    commonconfig.StringEnv("OAUTH_INTERNAL_CALLER_TOKEN", ""),
		ConsentTTL:       ttl,
		ReconcileEvery:   every,
		ReconcileBatch:   50,
		AuthCallDeadline: 5 * time.Second,
	}
	return cfg, cfg.Validate()
}

func (c AuthorizationConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if err := oauthmetadata.ValidateBaseURL(c.Issuer); err != nil {
		return fmt.Errorf("config: MCP_PUBLIC_BASE_URL: %w", err)
	}
	if c.InternalToken == "" {
		return errors.New("config: OAUTH_INTERNAL_CALLER_TOKEN is required when OAuth consent is enabled")
	}
	if c.ConsentTTL < time.Minute || c.ConsentTTL > time.Hour {
		return fmt.Errorf("config: MCP_CONSENT_TTL must be between 1m and 1h, got %s", c.ConsentTTL)
	}
	if c.ReconcileEvery <= 0 {
		return errors.New("config: MCP_GRANT_RECONCILE_INTERVAL must be positive")
	}
	return nil
}

func durationEnv(key string, def time.Duration) (time.Duration, error) {
	s := commonconfig.StringEnv(key, "")
	if s == "" {
		return def, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("config: invalid duration for %s=%q: %w", key, s, err)
	}
	return d, nil
}
