// Package config loads tenant-service's runtime configuration — env/flag
// parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

// Config carries commonconfig.Base plus NATSURL. NATSURL is only for the
// best-effort cross-replica profile-cache invalidation broadcast
// (docs/execution-plan.md Epic F). NATS is required-in-spirit, not
// required-to-boot: main.go degrades gracefully to today's
// TTL-bounded-only staleness if it's unreachable, since tenant-service sits
// on the critical path for every other service's tenant resolution and
// must not crash-loop over an optional dependency.
//
// ScmIntegrationServiceAddr is tenant-service's FIRST outbound synchronous
// service dependency (TASK-013, SOL-005): internal/adapter/scmstarcheck.GrpcAdapter
// dials this to back starNag.starOrca/agentValueMoment's real "is this repo
// starred" check, once ScmIntegrationService.StarRepository existed
// (SOL-012/TASK-034/035). Named the same as git-gateway-service's own
// field for the same downstream service.
type Config struct {
	commonconfig.Base
	NATSURL string
	// OPABundlePath points requireCompanyAdmin/requireDepartmentAccess's OPA
	// client at policy/orca-authz — same convention as project-service's own
	// OPABundlePath, for identical override behavior in every service.
	OPABundlePath             string
	ScmIntegrationServiceAddr string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("tenant-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                      base,
		NATSURL:                   commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		OPABundlePath:             commonconfig.StringEnv("OPA_BUNDLE_PATH", "../../policy/orca-authz"),
		ScmIntegrationServiceAddr: commonconfig.StringEnv("SCM_INTEGRATION_SERVICE_ADDR", "scm-integration-service:9090"),
	}, nil
}
