package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type DeleteProjectInput struct {
	ProjectID string
}

// DeleteProject hard-deletes a project. Reuses RebindDevServer's
// active-execution guard: deleting a project out from under a running
// workflow/task is at least as risky as rebinding its dev server (the
// worktree the execution is using disappears from project-service's
// bookkeeping entirely, not just moved to a different host) — so this
// usecase requires the same synchronous WorkflowExecutionChecker/
// TaskExecutionChecker saga as RebindDevServer, fails closed the same way,
// before the repository DELETE runs. See project-service.md §3's rebind
// saga and this service's README "DeleteProject guard" note.
//
// Cascade: project_members/repos/worktrees are removed via ON DELETE
// CASCADE FKs (migrations/0002/0003/0004) — child rows have no independent
// meaning once the owning project is gone, so a single DELETE here is
// sufficient; no explicit multi-table transaction is needed in this
// usecase or the repository.
type DeleteProject struct {
	repo            ProjectRepository
	workflowChecker WorkflowExecutionChecker
	taskChecker     TaskExecutionChecker
	opa             OPAClient
}

func NewDeleteProject(repo ProjectRepository, workflowChecker WorkflowExecutionChecker, taskChecker TaskExecutionChecker, opa OPAClient) *DeleteProject {
	return &DeleteProject{repo: repo, workflowChecker: workflowChecker, taskChecker: taskChecker, opa: opa}
}

// Execute requires the caller's project role to be owner, or global admin —
// project-service.md §9's owner-only tier, checked before the
// active-execution guard so an unauthorized caller can't use this RPC to
// probe whether a project has running executions.
func (uc *DeleteProject) Execute(ctx context.Context, in DeleteProjectInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.ProjectID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "PROJECT_ID_REQUIRED", "project_id is required", nil)
	}
	if err := requireProjectAccess(ctx, uc.repo, uc.opa, in.ProjectID, projectActionOwnerOnly); err != nil {
		return err
	}

	// Same fail-closed policy as RebindDevServer — a checker error is
	// treated as "has active executions" rather than letting a transient
	// outbound failure silently allow the delete through.
	workflowActive, err := uc.workflowChecker.HasActiveExecutions(ctx, in.ProjectID)
	if err != nil {
		return apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_HAS_ACTIVE_WORKFLOWS", "failed to verify workflow executions, failing closed", err)
	}
	taskActive, err := uc.taskChecker.HasActiveExecutions(ctx, in.ProjectID)
	if err != nil {
		return apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_HAS_ACTIVE_WORKFLOWS", "failed to verify task executions, failing closed", err)
	}
	if workflowActive || taskActive {
		return apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_HAS_ACTIVE_WORKFLOWS", "project has active workflow or task executions", nil)
	}

	err = uc.repo.DeleteProject(ctx, tenantID, in.ProjectID)
	if errors.Is(err, domain.ErrProjectNotFound) {
		return apperrors.New(apperrors.KindNotFound, "PROJECT_NOT_FOUND", "project not found", err)
	}
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_DELETE_FAILED", "failed to delete project", err)
	}
	return nil
}
