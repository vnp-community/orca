// Package config loads issue-status-sync's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base

	// NATSURL is where this service's eventbus.Consumer subscribes —
	// project-service's/scm-integration-service's own outbox relays are the
	// publishers on the other end.
	NATSURL string

	IssueTrackingServiceAddr  string
	SCMIntegrationServiceAddr string
	ProjectServiceAddr        string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("issue-status-sync")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                      base,
		NATSURL:                   commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		IssueTrackingServiceAddr:  commonconfig.StringEnv("ISSUE_TRACKING_SERVICE_ADDR", "issue-tracking-service:9090"),
		SCMIntegrationServiceAddr: commonconfig.StringEnv("SCM_INTEGRATION_SERVICE_ADDR", "scm-integration-service:9090"),
		ProjectServiceAddr:        commonconfig.StringEnv("PROJECT_SERVICE_ADDR", "project-service:9090"),
	}, nil
}
