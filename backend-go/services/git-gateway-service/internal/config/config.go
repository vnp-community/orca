// Package config loads git-gateway-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
//
// No database config here — per git-gateway-service.md §5, this service
// owns no data, so unlike usage-service's config there is no DATABASE_DSN
// use (commonconfig.Base still carries the field, but this service's Load
// never reads or requires it). What this service does need instead:
// downstream service addresses to eventually dial (project-service,
// infra-fleet-service) — present here as env-configurable fields even
// though the adapters that would use them (internal/adapter/grpcclient) are
// stubs in this scaffold, so wiring a real client later is a one-line
// change, not a new config surface.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base

	// InfraFleetServiceAddr is where a real internal/adapter/grpcclient
	// implementation would dial infra-fleet-service's ResolveConnection and
	// relay RPCs (git-gateway-service.md §7). Unused by this scaffold's stub
	// adapters; present so wiring a real client doesn't need a new env var.
	InfraFleetServiceAddr string

	// ProjectServiceAddr is where a real worktree_id -> repo path resolver
	// would dial project-service (git-gateway-service.md §7 step 1). Now a
	// real outbound dependency (TASK-194): grpcclient.ProjectClient dials
	// this for the CreateWorktree/RemoveWorktree saga's bookkeeping calls.
	ProjectServiceAddr string

	// AIProviderServiceAddr is where DiscoverCommitMessageModels
	// (TASK-211) dials ai-provider-service's ResolveProvider RPC directly —
	// distinct from infra-fleet-service's Relay RPC used for the actual
	// ai.complete execution-plane call.
	AIProviderServiceAddr string
	// SCMIntegrationServiceAddr is where scmclient.Client dials
	// scm-integration-service for ResolvePrBase/ResolveMrBase's PR/MR
	// base-branch lookups (SOL-031 / TASK-193) — a new outbound dependency
	// edge this service didn't have before this batch.
	SCMIntegrationServiceAddr string
	// IssueTrackingServiceAddr is where IssueTrackingSourceClient dials
	// issue-tracking-service's GetIssue RPC — CreateWorktreeFromIssue's
	// tracker_issue half of its oneof issue_source (SOL-PI-02), a new
	// outbound dependency edge (git --> issue) this service didn't have
	// before this batch.
	IssueTrackingServiceAddr string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("git-gateway-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                      base,
		InfraFleetServiceAddr:     commonconfig.StringEnv("INFRA_FLEET_SERVICE_ADDR", "infra-fleet-service:9090"),
		ProjectServiceAddr:        commonconfig.StringEnv("PROJECT_SERVICE_ADDR", "project-service:9090"),
		AIProviderServiceAddr:     commonconfig.StringEnv("AI_PROVIDER_SERVICE_ADDR", "ai-provider-service:9090"),
		SCMIntegrationServiceAddr: commonconfig.StringEnv("SCM_INTEGRATION_SERVICE_ADDR", "scm-integration-service:9090"),
		IssueTrackingServiceAddr:  commonconfig.StringEnv("ISSUE_TRACKING_SERVICE_ADDR", "issue-tracking-service:9090"),
	}, nil
}
