package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// ExecuteInput mirrors ExecuteRequest. ProjectID is accepted (matching the
// proto) but not persisted — this scaffold's executions table is narrowed
// to (id, template_id, tenant_id, status, root_trace_id, paused_at); see
// README "Known gaps" for extending the schema with project_id once a
// consumer needs it.
type ExecuteInput struct {
	TemplateID  string
	ProjectID   string
	RootTraceID string
	RequestID   string
}

// Execute resolves a template, validates its DAG, and records a new
// execution — the part of workflow-service.md §7's dependency diagram this
// scaffold implements. It deliberately stops there: real Kahn's-algorithm
// wave computation and dispatch to StepExecutors is NOT implemented (see
// README "Known gaps" — this is a scaffold limitation, flagged, not a
// silently-dropped requirement). The execution is persisted in
// StatusRunning and never progresses further on its own.
type Execute struct {
	templates  TemplateRepository
	executions ExecutionRepository
}

func NewExecute(templates TemplateRepository, executions ExecutionRepository) *Execute {
	return &Execute{templates: templates, executions: executions}
}

func (uc *Execute) Execute(ctx context.Context, in ExecuteInput) (domain.WorkflowExecution, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}

	tmpl, err := uc.templates.GetTemplate(ctx, tenantID, in.TemplateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return domain.WorkflowExecution{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "workflow template not found", err)
		}
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_FETCH_FAILED", "failed to fetch workflow template", err)
	}

	dag, err := domain.ParseDAG(tmpl.DAGJSON)
	if err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_DAG", err.Error(), err)
	}
	if err := dag.Validate(); err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_DAG_VALIDATION_FAILED", err.Error(), err)
	}

	rootTraceID := in.RootTraceID
	if rootTraceID == "" {
		// No caller-supplied trace to resume against — mint a fresh one so
		// this execution is still resumable after a restart (§8's hard
		// requirement applies to every execution, not just ones the caller
		// happened to pass a root_trace_id for).
		rootTraceID = uuid.NewString()
	}

	exec, err := domain.NewWorkflowExecution(uuid.NewString(), tenantID, tmpl.ID, rootTraceID)
	if err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_EXECUTION", err.Error(), err)
	}

	if err := uc.executions.CreateExecution(ctx, exec); err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_EXECUTION_SAVE_FAILED", "failed to persist workflow execution", err)
	}

	return exec, nil
}
