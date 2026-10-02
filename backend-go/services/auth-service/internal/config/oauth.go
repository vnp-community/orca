package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	commonconfig "github.com/stablyai/orca-go/common/config"
)

// OAuthConfig configures the OAuth 2.1 authorization server (BE-MCP-SOL-005).
// The server is off unless OAUTH_RESOURCE_URL is set; kept separate from
// Config/Load so enabling it never changes how the rest of the service boots.
type OAuthConfig struct {
	Enabled         bool
	ResourceURL     string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	AuthCodeTTL     time.Duration
	DCREnabled      bool
	DCRMaxClients   int
	// InternalCallerToken guards the mcp-service-only RPCs.
	InternalCallerToken string
}

const maxOAuthAccessTokenTTL = 15 * time.Minute

// LoadOAuth reads the OAUTH_* environment. When enabled it refuses to start
// with settings that would weaken the server (long-lived access tokens, a
// non-https resource, or no internal-caller secret).
func LoadOAuth() (OAuthConfig, error) {
	resource := strings.TrimSpace(commonconfig.StringEnv("OAUTH_RESOURCE_URL", ""))
	if resource == "" {
		return OAuthConfig{}, nil
	}
	access, err := durationEnv("OAUTH_ACCESS_TOKEN_TTL", 10*time.Minute)
	if err != nil {
		return OAuthConfig{}, err
	}
	refresh, err := durationEnv("OAUTH_REFRESH_TOKEN_TTL", 30*24*time.Hour)
	if err != nil {
		return OAuthConfig{}, err
	}
	code, err := durationEnv("OAUTH_AUTH_CODE_TTL", 60*time.Second)
	if err != nil {
		return OAuthConfig{}, err
	}
	maxClients, err := intEnv("OAUTH_DCR_MAX_CLIENTS", 1000)
	if err != nil {
		return OAuthConfig{}, err
	}
	cfg := OAuthConfig{
		Enabled: true, ResourceURL: resource, AccessTokenTTL: access, RefreshTokenTTL: refresh, AuthCodeTTL: code,
		DCREnabled: boolEnv("OAUTH_DCR_ENABLED", true), DCRMaxClients: maxClients,
		InternalCallerToken: commonconfig.StringEnv("OAUTH_INTERNAL_CALLER_TOKEN", ""),
	}
	return cfg, cfg.Validate()
}

func (c OAuthConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	u, err := url.Parse(c.ResourceURL)
	if err != nil || u.Host == "" || u.Fragment != "" || u.RawQuery != "" || u.User != nil {
		return fmt.Errorf("config: OAUTH_RESOURCE_URL %q is not a valid absolute URL", c.ResourceURL)
	}
	loopback := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return errors.New("config: OAUTH_RESOURCE_URL must be https (http only for loopback)")
	}
	if c.AccessTokenTTL <= 0 || c.AccessTokenTTL > maxOAuthAccessTokenTTL {
		return fmt.Errorf("config: OAUTH_ACCESS_TOKEN_TTL must be in (0, 15m], got %s", c.AccessTokenTTL)
	}
	if c.RefreshTokenTTL <= 0 || c.AuthCodeTTL <= 0 || c.AuthCodeTTL > 10*time.Minute {
		return errors.New("config: OAUTH_REFRESH_TOKEN_TTL must be > 0 and OAUTH_AUTH_CODE_TTL in (0, 10m]")
	}
	if c.InternalCallerToken == "" {
		return errors.New("config: OAUTH_INTERNAL_CALLER_TOKEN is required when the OAuth server is enabled")
	}
	return nil
}
