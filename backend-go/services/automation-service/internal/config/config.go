// Package config loads automation-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	// WorkflowServiceAddr is the gRPC address of workflow-service, dialed by
	// internal/adapter/grpcclient's real WorkflowStepExecutor implementation
	// — the cross-service call RunNow exists to make, see
	// specs/backend-go/services/automation-service.md §2/§7.
	WorkflowServiceAddr string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("automation-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                base,
		WorkflowServiceAddr: commonconfig.StringEnv("WORKFLOW_SERVICE_ADDR", "localhost:9091"),
	}, nil
}
