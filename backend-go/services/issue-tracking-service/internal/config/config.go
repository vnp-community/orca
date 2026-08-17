// Package config loads issue-tracking-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

// Config embeds commonconfig.Base's DatabaseDSN field but this service
// never uses it — issue-tracking-service owns no database (design doc
// §2/§5: Jira and Linear remain the systems of record). NATSURL is
// required-in-spirit, not required-to-boot: main.go degrades gracefully if
// NATS is unreachable, same as usage-service, but LinkIssue then fails
// closed at call time since publishing is this service's only persisted
// side effect (see internal/usecase/link_issue.go).
type Config struct {
	commonconfig.Base
	NATSURL string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("issue-tracking-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:    base,
		NATSURL: commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}
