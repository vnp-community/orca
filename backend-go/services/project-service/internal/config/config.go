// Package config loads project-service's runtime configuration — env/flag
// parsing only, no business logic, per architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	// WorkflowServiceAddr and TaskServiceAddr are wired now even though
	// internal/adapter/grpcclient's checkers are currently STUBs (see that
	// package) — the config field shouldn't need to change when the real RPC
	// call is wired in.
	WorkflowServiceAddr string
	TaskServiceAddr     string
	// InfraFleetServiceAddr is ScanNested/ImportNested's/SetupExistingFolder's
	// DevServerRelay (and CreateHostSetup's DevServerLister) dependency —
	// also dialed for DevServerHealthChecker/ProfileResolver.DevServerTags/
	// GetProjectContext's DevServerHostnameResolver (TASK-PRF-03/04).
	InfraFleetServiceAddr string
	// TenantServiceAddr is ProfileResolver's dependency (ListProjects's
	// fleet.allowedServerTags visibility filter) — a NEW dial, this service
	// never called tenant-service before this task.
	TenantServiceAddr string
	// NATSURL backs adapter/eventbus.Publisher (AuditPublisher/
	// MemberNotifier for RebindDevServer) — best-effort, non-fatal if
	// unreachable at startup, same posture as tenant-service's own NATSURL.
	// Also doubles as the transactional-outbox relay's NATS JetStream
	// target (SOL-PI-03) — mirrors issue-tracking-service's own NATSURL field.
	NATSURL string
	// OPABundlePath points requireProjectAccess's OPA client
	// (internal/adapter/opaclient, via common/policy.Evaluator) at the
	// orca-authz Rego bundle directory. Defaults to the bundle's location
	// relative to this service's cmd/server working directory in local dev
	// (`go run ./cmd/server` from services/project-service) — same relative
	// path/env var name as auth-service/annotation-service/task-service's
	// own OPABundlePath, for identical override behavior in every
	// deployment environment.
	OPABundlePath string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("project-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                  base,
		WorkflowServiceAddr:   commonconfig.StringEnv("WORKFLOW_SERVICE_ADDR", "workflow-service:9090"),
		TaskServiceAddr:       commonconfig.StringEnv("TASK_SERVICE_ADDR", "task-service:9090"),
		InfraFleetServiceAddr: commonconfig.StringEnv("INFRA_FLEET_SERVICE_ADDR", "infra-fleet-service:9090"),
		TenantServiceAddr:     commonconfig.StringEnv("TENANT_SERVICE_ADDR", "tenant-service:9090"),
		NATSURL:               commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		OPABundlePath:         commonconfig.StringEnv("OPA_BUNDLE_PATH", "../../policy/orca-authz"),
	}, nil
}
