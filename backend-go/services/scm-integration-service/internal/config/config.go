// Package config loads scm-integration-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	"strings"

	commonconfig "github.com/stablyai/orca-go/common/config"
)

// Config holds this service's settings. Unlike usage-service, no
// DatabaseDSN is required at startup: scm-integration-service owns no
// business data (§5) and this scaffold doesn't wire the operational
// rate_limit_cache/webhook_delivery_log tables yet — see README's
// "Known gaps".
type Config struct {
	commonconfig.Base
	// GitHubBaseURL overrides GitHub's REST API root — used for tests and
	// GitHub Enterprise deployments.
	GitHubBaseURL string
	// GitLabBaseURL overrides GitLab's REST API root — used for tests and
	// self-hosted GitLab deployments.
	GitLabBaseURL string
	// CredentialBrokerAddr is credential-broker-service's gRPC target —
	// dialed for real by internal/adapter/credentialbroker as of Epic B
	// (docs/execution-plan.md §8).
	CredentialBrokerAddr string

	// DatabaseCredentialsFile is the path a Vault Agent sidecar renders
	// dynamic Postgres credentials to in production (see
	// common/secrets.DatabaseCredentialsFromFile) — same convention as
	// usage-service's config. Falls back to DATABASE_DSN (via Base) when
	// the file doesn't exist, which is what local dev and this scaffold's
	// testcontainers path use instead. Backs scm.rate_limit_cache as of
	// Phase 3 (docs/execution-plan.md §3) — see internal/adapter/postgres.
	DatabaseCredentialsFile string

	// BitbucketBaseURL/AzureDevOpsBaseURL/GiteaBaseURL mirror
	// GitHubBaseURL/GitLabBaseURL above — overridable for tests and
	// self-hosted deployments (Gitea and Azure DevOps Server especially).
	BitbucketBaseURL   string
	AzureDevOpsBaseURL string
	GiteaBaseURL       string

	// OAuthStateSecret signs/verifies the stateless StartOAuthFlow/
	// CompleteOAuthFlow state token (§9.1) — see
	// internal/adapter/oauthstate's package doc comment. The default below
	// is an insecure, well-known dev value (same local-dev-convenience
	// posture as main.go's insecure gRPC transport credentials) — any real
	// deployment MUST override this via OAUTH_STATE_SECRET, or every minted
	// state token is forgeable.
	OAuthStateSecret string

	// OAuth is this service's per-provider OAuth 2.0 app registration
	// (§9.1) — client_id/client_secret from env, with each provider's real
	// authorize/token URLs and default scope. A provider whose ClientID is
	// empty is simply not registered in cmd/server/main.go's
	// OAuthRegistry — see internal/adapter/providerregistry.OAuthRegistry's
	// doc comment.
	OAuth OAuthProvidersConfig

	// NATSURL is the transactional-outbox relay's NATS JetStream target
	// (SOL-PI-03) — mirrors issue-tracking-service's own NATSURL field.
	NATSURL string
}

// OAuthProviderConfig is one provider's OAuth 2.0 app registration.
type OAuthProviderConfig struct {
	AuthorizeURL string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scope        string
}

// OAuthProvidersConfig holds one OAuthProviderConfig per provider this
// service knows about (§1: GitHub, GitLab, Bitbucket, Azure DevOps, Gitea).
type OAuthProvidersConfig struct {
	GitHub      OAuthProviderConfig
	GitLab      OAuthProviderConfig
	Bitbucket   OAuthProviderConfig
	AzureDevOps OAuthProviderConfig
	Gitea       OAuthProviderConfig
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("scm-integration-service")
	if err != nil {
		return Config{}, err
	}
	giteaBaseURL := commonconfig.StringEnv("GITEA_BASE_URL", "https://gitea.com/api/v1")
	return Config{
		Base:                    base,
		GitHubBaseURL:           commonconfig.StringEnv("GITHUB_BASE_URL", "https://api.github.com"),
		GitLabBaseURL:           commonconfig.StringEnv("GITLAB_BASE_URL", "https://gitlab.com/api/v4"),
		BitbucketBaseURL:        commonconfig.StringEnv("BITBUCKET_BASE_URL", "https://api.bitbucket.org/2.0"),
		AzureDevOpsBaseURL:      commonconfig.StringEnv("AZURE_DEVOPS_BASE_URL", "https://dev.azure.com"),
		GiteaBaseURL:            giteaBaseURL,
		CredentialBrokerAddr:    commonconfig.StringEnv("CREDENTIAL_BROKER_ADDR", "credential-broker-service:9090"),
		DatabaseCredentialsFile: commonconfig.StringEnv("DATABASE_CREDENTIALS_FILE", "/vault/secrets/database-credentials"),
		OAuthStateSecret:        commonconfig.StringEnv("OAUTH_STATE_SECRET", "dev-only-insecure-oauth-state-secret"),
		NATSURL:                 commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		OAuth: OAuthProvidersConfig{
			GitHub: OAuthProviderConfig{
				AuthorizeURL: commonconfig.StringEnv("GITHUB_OAUTH_AUTHORIZE_URL", "https://github.com/login/oauth/authorize"),
				TokenURL:     commonconfig.StringEnv("GITHUB_OAUTH_TOKEN_URL", "https://github.com/login/oauth/access_token"),
				ClientID:     commonconfig.StringEnv("GITHUB_OAUTH_CLIENT_ID", ""),
				ClientSecret: commonconfig.StringEnv("GITHUB_OAUTH_CLIENT_SECRET", ""),
				Scope:        commonconfig.StringEnv("GITHUB_OAUTH_SCOPE", "repo"),
			},
			GitLab: OAuthProviderConfig{
				AuthorizeURL: commonconfig.StringEnv("GITLAB_OAUTH_AUTHORIZE_URL", "https://gitlab.com/oauth/authorize"),
				TokenURL:     commonconfig.StringEnv("GITLAB_OAUTH_TOKEN_URL", "https://gitlab.com/oauth/token"),
				ClientID:     commonconfig.StringEnv("GITLAB_OAUTH_CLIENT_ID", ""),
				ClientSecret: commonconfig.StringEnv("GITLAB_OAUTH_CLIENT_SECRET", ""),
				Scope:        commonconfig.StringEnv("GITLAB_OAUTH_SCOPE", "api"),
			},
			Bitbucket: OAuthProviderConfig{
				AuthorizeURL: commonconfig.StringEnv("BITBUCKET_OAUTH_AUTHORIZE_URL", "https://bitbucket.org/site/oauth2/authorize"),
				TokenURL:     commonconfig.StringEnv("BITBUCKET_OAUTH_TOKEN_URL", "https://bitbucket.org/site/oauth2/access_token"),
				ClientID:     commonconfig.StringEnv("BITBUCKET_OAUTH_CLIENT_ID", ""),
				ClientSecret: commonconfig.StringEnv("BITBUCKET_OAUTH_CLIENT_SECRET", ""),
				Scope:        commonconfig.StringEnv("BITBUCKET_OAUTH_SCOPE", "repository:write pullrequest:write"),
			},
			// Azure DevOps' OAuth app model (app.vssps.visualstudio.com) is
			// distinct from Azure AD app registration — this targets the
			// former, the one Azure DevOps' own "Create an app" flow
			// documents for third-party integrations. Self-hosted Azure
			// DevOps Server deployments use different URLs entirely — hence
			// these being fully env-overridable rather than hardcoded like
			// GitHub/GitLab's public defaults.
			AzureDevOps: OAuthProviderConfig{
				AuthorizeURL: commonconfig.StringEnv("AZURE_DEVOPS_OAUTH_AUTHORIZE_URL", "https://app.vssps.visualstudio.com/oauth2/authorize"),
				TokenURL:     commonconfig.StringEnv("AZURE_DEVOPS_OAUTH_TOKEN_URL", "https://app.vssps.visualstudio.com/oauth2/token"),
				ClientID:     commonconfig.StringEnv("AZURE_DEVOPS_OAUTH_CLIENT_ID", ""),
				ClientSecret: commonconfig.StringEnv("AZURE_DEVOPS_OAUTH_CLIENT_SECRET", ""),
				Scope:        commonconfig.StringEnv("AZURE_DEVOPS_OAUTH_SCOPE", "vso.code_write"),
			},
			// Gitea is typically self-hosted with no single public OAuth
			// app registry — authorize/token URLs default relative to
			// GiteaBaseURL's host (Gitea's own OAuth endpoints live under
			// /login/oauth/... on the instance itself, not under /api/v1).
			Gitea: OAuthProviderConfig{
				AuthorizeURL: commonconfig.StringEnv("GITEA_OAUTH_AUTHORIZE_URL", giteaAuthURL(giteaBaseURL, "authorize")),
				TokenURL:     commonconfig.StringEnv("GITEA_OAUTH_TOKEN_URL", giteaAuthURL(giteaBaseURL, "access_token")),
				ClientID:     commonconfig.StringEnv("GITEA_OAUTH_CLIENT_ID", ""),
				ClientSecret: commonconfig.StringEnv("GITEA_OAUTH_CLIENT_SECRET", ""),
				Scope:        commonconfig.StringEnv("GITEA_OAUTH_SCOPE", "write:repository"),
			},
		},
	}, nil
}

// giteaAuthURL derives Gitea's default oauth endpoint from its configured
// API base URL (strips the /api/v1 REST-API suffix, since Gitea's OAuth
// endpoints live at the instance root under /login/oauth/...) — only used
// as a fallback default; GITEA_OAUTH_AUTHORIZE_URL/GITEA_OAUTH_TOKEN_URL
// override it directly for any instance where this heuristic is wrong.
func giteaAuthURL(baseURL, action string) string {
	root := strings.TrimSuffix(baseURL, "/api/v1")
	return root + "/login/oauth/" + action
}
