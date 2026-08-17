// Package domain holds workflow-service's entities, value objects, and the
// StepExecutor strategy interface. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework, and (deliberately, see StepExecutor's
// doc comment) no infra-fleet-service relay client either.
package domain

import "context"

// StepType is the discriminator for a workflow step's kind — mirrors
// workflowv1.StepType (proto) one-for-one, see
// specs/backend-go/services/workflow-service.md §4.
type StepType string

const (
	StepTypeUnspecified  StepType = ""
	StepTypeAgent        StepType = "agent"
	StepTypeShell        StepType = "shell"
	StepTypeNotification StepType = "notification"
	StepTypeWebhook      StepType = "webhook"
	StepTypeCondition    StepType = "condition"
)

// Valid reports whether t is one of the five known step types.
func (t StepType) Valid() bool {
	switch t {
	case StepTypeAgent, StepTypeShell, StepTypeNotification, StepTypeWebhook, StepTypeCondition:
		return true
	default:
		return false
	}
}

// ResultStatus is a StepResult's outcome — "completed" and "failed" only;
// no in-progress/partial state, since a StepExecutor call is synchronous
// from this service's point of view (§3.1: ExecuteAdHocStep is
// synchronous, and a real Execute's wave dispatch awaits each step before
// gating the next wave).
type ResultStatus string

const (
	ResultStatusCompleted ResultStatus = "completed"
	ResultStatusFailed    ResultStatus = "failed"
)

// StepResult is the outcome of running one step through a StepExecutor —
// mirrors workflowv1.StepResult (proto) one-for-one.
type StepResult struct {
	Status     ResultStatus
	OutputJSON string
}

// StepExecutor is the domain-level strategy interface each step type
// implements — one Execute per StepType, dispatched by
// usecase.StepExecutorRegistry.Resolve.
//
// Deviation note: specs/backend-go/services/workflow-service.md §4 places
// this interface in usecase/, per
// architecture/03-clean-architecture-guidelines.md's general "interface
// lives with its consumer" rule. It's defined here instead, per this
// service's explicit build instructions: the strategy itself (dispatch by
// StepType to a pure Execute(ctx, config) -> (StepResult, error) contract)
// is a pure domain concept independent of any adapter, even though four of
// five concrete implementations happen to need I/O. The port that *resolves*
// a StepType to a StepExecutor (usecase.StepExecutorRegistry) still lives in
// usecase/, matching the general rule for the one piece that's genuinely
// usecase-shaped.
type StepExecutor interface {
	Execute(ctx context.Context, stepConfigJSON string) (StepResult, error)
}
