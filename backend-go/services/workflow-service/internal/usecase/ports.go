// Package usecase holds workflow-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per
// the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// TemplateRepository is the persistence port for workflow templates.
// Implemented by internal/adapter/postgres against workflow-service's own
// database — see architecture/05-data-architecture.md's
// database-per-service rule.
type TemplateRepository interface {
	CreateTemplate(ctx context.Context, tmpl domain.WorkflowTemplate) error
	// GetTemplate returns domain.ErrTemplateNotFound (wrapped) if no
	// matching row exists for tenantID/id.
	GetTemplate(ctx context.Context, tenantID, id string) (domain.WorkflowTemplate, error)
}

// ExecutionRepository is the persistence port for workflow executions.
type ExecutionRepository interface {
	CreateExecution(ctx context.Context, exec domain.WorkflowExecution) error
	// GetExecution returns domain.ErrExecutionNotFound (wrapped) if no
	// matching row exists for tenantID/id.
	GetExecution(ctx context.Context, tenantID, id string) (domain.WorkflowExecution, error)
	// UpdateExecution persists an execution's mutable fields (status,
	// paused_at) — called after Pause/Resume transitions.
	UpdateExecution(ctx context.Context, exec domain.WorkflowExecution) error
}

// ErrStepExecutorNotRegistered is returned by StepExecutorRegistry.Resolve
// when no StepExecutor is wired for the requested StepType — a composition
// root bug (cmd/server/main.go didn't register all five types), not a
// normal runtime condition.
var ErrStepExecutorNotRegistered = errors.New("usecase: no step executor registered for this step type")

// StepExecutorRegistry resolves a StepType to the concrete StepExecutor
// that runs it. Implemented by internal/adapter/stepexecutors and wired in
// cmd/server/main.go with all five step types (Condition/Webhook real,
// Agent/Shell/Notification stubbed pending infra-fleet-service — see
// workflow-service.md §4 and this service's README).
type StepExecutorRegistry interface {
	Resolve(stepType domain.StepType) (domain.StepExecutor, error)
}
