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
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("project-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                base,
		WorkflowServiceAddr: commonconfig.StringEnv("WORKFLOW_SERVICE_ADDR", "workflow-service:9090"),
		TaskServiceAddr:     commonconfig.StringEnv("TASK_SERVICE_ADDR", "task-service:9090"),
	}, nil
}
