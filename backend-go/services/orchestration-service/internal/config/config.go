// Package config loads orchestration-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	commonconfig "github.com/stablyai/orca-go/common/config"
)

type Config struct {
	commonconfig.Base
	// InfraFleetServiceAddr is where WorkerDispatcher dials
	// infra-fleet-service's Relay RPC (TASK-TASKV1-005-08).
	InfraFleetServiceAddr string
	// TaskServiceAddr is where TaskServiceReporter dials task-service's
	// ReportTaskExecutionResult RPC (TASK-TASKV1-005-08).
	TaskServiceAddr string
	// NATSURL is where the outbox relay (BE-SOL-003/TASK-FT-003-01)
	// publishes orchestration.outbox_events rows — same field name/default
	// as usage-service's identically-purposed config field. Absent before
	// this task: this service had no eventbus package at all.
	NATSURL string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("orchestration-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                  base,
		InfraFleetServiceAddr: commonconfig.StringEnv("INFRA_FLEET_SERVICE_ADDR", "infra-fleet-service:9090"),
		TaskServiceAddr:       commonconfig.StringEnv("TASK_SERVICE_ADDR", "task-service:9090"),
		NATSURL:               commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}
