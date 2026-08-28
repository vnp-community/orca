// Package config loads automation-service's runtime configuration —
// env/flag parsing only, no business logic, per
// architecture/03-clean-architecture-guidelines.md.
package config

import (
	"fmt"
	"os"
	"time"

	commonconfig "github.com/stablyai/orca-go/common/config"
)

// defaultSchedulerInterval matches BL-AT-02's main-flow-specified 30-second
// scheduler tick (BUG-AT-02) — was previously time.Minute, matching an
// earlier "~1 minute" guidance since superseded.
const defaultSchedulerInterval = 30 * time.Second

// defaultSchedulerBatchSize caps how many due automations a single tick
// claims — an unbounded claim could hold the claim transaction (see
// internal/adapter/postgres's ClaimDue) open across an arbitrarily large
// number of workflow-service dispatch calls.
const defaultSchedulerBatchSize = 50

type Config struct {
	commonconfig.Base
	// WorkflowServiceAddr is the gRPC address of workflow-service, dialed by
	// internal/adapter/grpcclient's real WorkflowStepExecutor implementation
	// — the cross-service call RunNow exists to make, see
	// specs/backend-go/services/automation-service.md §2/§7.
	WorkflowServiceAddr string
	// SchedulerInterval is how often internal/adapter/scheduler's ticker
	// loop checks for due automations.
	SchedulerInterval time.Duration
	// SchedulerBatchSize is the max due automations claimed per tick.
	SchedulerBatchSize int32
	// NATSURL is the JetStream connection string — internal/adapter/eventbus
	// uses it both to publish orca.automation.run.completed (via the
	// transactional-outbox relay) and to consume the 5 event-trigger
	// subjects (TASK-AT-03-05).
	NATSURL string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("automation-service")
	if err != nil {
		return Config{}, err
	}
	interval, err := durationEnv("SCHEDULER_INTERVAL", defaultSchedulerInterval)
	if err != nil {
		return Config{}, err
	}
	batchSize, err := int32Env("SCHEDULER_BATCH_SIZE", defaultSchedulerBatchSize)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                base,
		WorkflowServiceAddr: commonconfig.StringEnv("WORKFLOW_SERVICE_ADDR", "localhost:9091"),
		SchedulerInterval:   interval,
		SchedulerBatchSize:  batchSize,
		NATSURL:             commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}

// durationEnv and int32Env mirror common/config's unexported intEnv —
// kept local rather than promoted to common/config since this is currently
// the only service that needs a duration/int32 env var; promote later if a
// second service needs the same helpers.
func durationEnv(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid duration for %s=%q: %w", key, v, err)
	}
	return d, nil
}

func int32Env(key string, def int32) (int32, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	var n int32
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return 0, fmt.Errorf("config: invalid int for %s=%q: %w", key, v, err)
	}
	return n, nil
}
