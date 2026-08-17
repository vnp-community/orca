// Package stepexecutors implements the five concrete domain.StepExecutor
// strategies — Condition and Webhook run in-process for real; Agent, Shell,
// and Notification are stubs pending infra-fleet-service's relay client
// (see workflow-service.md §2/§4 and this service's README). Also implements
// usecase.StepExecutorRegistry's concrete Registry, wired in
// cmd/server/main.go.
package stepexecutors

import (
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// Registry is a simple in-memory implementation of
// usecase.StepExecutorRegistry — a map, not a database, since the set of
// step types is fixed and small (five) and known entirely at composition
// time (cmd/server/main.go).
type Registry struct {
	executors map[domain.StepType]domain.StepExecutor
}

func NewRegistry() *Registry {
	return &Registry{executors: make(map[domain.StepType]domain.StepExecutor)}
}

// Register wires a StepType to the StepExecutor that runs it. Called once
// per step type from cmd/server/main.go; not safe for concurrent use during
// registration (registration happens entirely during startup, before the
// gRPC server starts accepting requests).
func (r *Registry) Register(stepType domain.StepType, executor domain.StepExecutor) {
	r.executors[stepType] = executor
}

// Resolve implements usecase.StepExecutorRegistry.
func (r *Registry) Resolve(stepType domain.StepType) (domain.StepExecutor, error) {
	executor, ok := r.executors[stepType]
	if !ok {
		return nil, usecase.ErrStepExecutorNotRegistered
	}
	return executor, nil
}
