package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type RebindDevServerInput struct {
	ProjectID      string
	NewDevServerID string
}

// RebindDevServer closes the TS rebind-guard gap described in
// project-service.md §3/§10: TS's project.update accepts devServerId in its
// patch with no active-execution check, so a live worktree mid-execution can
// be silently rebound to a different host. This usecase runs the synchronous
// saga instead — check workflow-service, check task-service, only then
// write — per the sequence diagram in that section.
type RebindDevServer struct {
	repo            ProjectRepository
	workflowChecker WorkflowExecutionChecker
	taskChecker     TaskExecutionChecker
}

func NewRebindDevServer(repo ProjectRepository, workflowChecker WorkflowExecutionChecker, taskChecker TaskExecutionChecker) *RebindDevServer {
	return &RebindDevServer{repo: repo, workflowChecker: workflowChecker, taskChecker: taskChecker}
}

func (uc *RebindDevServer) Execute(ctx context.Context, in RebindDevServerInput) (domain.Project, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.NewDevServerID == "" {
		return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_DEV_SERVER", "new_dev_server_id is required", nil)
	}

	// Both checks are synchronous gRPC calls per project-service.md §3 (saga
	// pattern) — the caller cannot know a rebind is safe without both
	// answers. A checker error fails closed (treated as "has active
	// executions") per §3/§8's timeout policy, rather than letting a
	// transient outbound failure silently allow the rebind through.
	workflowActive, err := uc.workflowChecker.HasActiveExecutions(ctx, in.ProjectID)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_HAS_ACTIVE_WORKFLOWS", "failed to verify workflow executions, failing closed", err)
	}
	taskActive, err := uc.taskChecker.HasActiveExecutions(ctx, in.ProjectID)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_HAS_ACTIVE_WORKFLOWS", "failed to verify task executions, failing closed", err)
	}
	if workflowActive || taskActive {
		return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_HAS_ACTIVE_WORKFLOWS", "project has active workflow or task executions", nil)
	}

	updated, err := uc.repo.UpdateDevServerID(ctx, tenantID, in.ProjectID, in.NewDevServerID)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_REBIND_FAILED", "failed to update dev server binding", err)
	}
	return updated, nil
}
