package usecase

import "time"

const (
	DefaultOAuthAccessTokenTTL  = 10 * time.Minute
	MaxOAuthAccessTokenTTL      = 15 * time.Minute // CR-MCP-005: access tokens live at most 15 minutes
	DefaultOAuthRefreshTokenTTL = 30 * 24 * time.Hour
	DefaultOAuthAuthCodeTTL     = 60 * time.Second
	DefaultOAuthDCRMaxClients   = 1000
)

// OAuthConfig is the authorization server's runtime configuration.
type OAuthConfig struct {
	// ResourceURL is the only audience ever issued; the `resource` parameter
	// must equal it exactly (RFC 8707).
	ResourceURL     string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	AuthCodeTTL     time.Duration
	DCREnabled      bool
	DCRMaxClients   int
}

// WithDefaults fills zero values and clamps the access-token lifetime to the
// 15 minute ceiling so a misconfiguration can't mint long-lived bearer tokens.
func (c OAuthConfig) WithDefaults() OAuthConfig {
	if c.AccessTokenTTL <= 0 {
		c.AccessTokenTTL = DefaultOAuthAccessTokenTTL
	}
	if c.AccessTokenTTL > MaxOAuthAccessTokenTTL {
		c.AccessTokenTTL = MaxOAuthAccessTokenTTL
	}
	if c.RefreshTokenTTL <= 0 {
		c.RefreshTokenTTL = DefaultOAuthRefreshTokenTTL
	}
	if c.AuthCodeTTL <= 0 {
		c.AuthCodeTTL = DefaultOAuthAuthCodeTTL
	}
	if c.DCRMaxClients <= 0 {
		c.DCRMaxClients = DefaultOAuthDCRMaxClients
	}
	return c
}
