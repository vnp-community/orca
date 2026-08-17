// Package config loads scm-integration-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
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
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("scm-integration-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:          base,
		GitHubBaseURL: commonconfig.StringEnv("GITHUB_BASE_URL", "https://api.github.com"),
		GitLabBaseURL: commonconfig.StringEnv("GITLAB_BASE_URL", "https://gitlab.com/api/v4"),
	}, nil
}
