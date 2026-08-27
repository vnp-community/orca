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
	opa             OPAClient
	devServers      DevServerLister        // NEW
	health          DevServerHealthChecker // NEW
	audit           AuditPublisher         // NEW
	notifier        MemberNotifier         // NEW
}

func NewRebindDevServer(repo ProjectRepository, workflowChecker WorkflowExecutionChecker, taskChecker TaskExecutionChecker, opa OPAClient, devServers DevServerLister, health DevServerHealthChecker, audit AuditPublisher, notifier MemberNotifier) *RebindDevServer {
	return &RebindDevServer{repo: repo, workflowChecker: workflowChecker, taskChecker: taskChecker, opa: opa, devServers: devServers, health: health, audit: audit, notifier: notifier}
}

// Execute requires the caller's project role to be owner, or global admin —
// project-service.md §9's owner-only tier, and this RPC's own sequence
// diagram ("PS->>PS: assertAccess — owner/admin only (OPA)").
func (uc *RebindDevServer) Execute(ctx context.Context, in RebindDevServerInput) (domain.Project, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.NewDevServerID == "" {
		return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_DEV_SERVER", "new_dev_server_id is required", nil)
	}
	if err := requireProjectAccess(ctx, uc.repo, uc.opa, in.ProjectID, projectActionOwnerOnly); err != nil {
		return domain.Project{}, err
	}

	// NEW — existence + health, cheapest checks first, before the
	// (unchanged) active-execution guard below.
	exists, err := uc.devServers.Exists(ctx, tenantID, in.NewDevServerID)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_DEV_SERVER_LOOKUP_FAILED", "failed to validate new dev server", err)
	}
	if !exists {
		return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_DEV_SERVER_NOT_FOUND", "new dev server does not exist", nil)
	}
	reachable, err := uc.health.IsReachable(ctx, tenantID, in.NewDevServerID)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_HEALTH_CHECK_FAILED", "failed to verify new dev server is online, failing closed", err)
	}
	if !reachable {
		return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_UNREACHABLE", "new dev server is not online", nil)
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

	before, err := uc.repo.Get(ctx, tenantID, in.ProjectID) // NEW — for the old dev server id in the notify payload
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_LOOKUP_FAILED", "failed to look up project before rebind", err)
	}
	updated, err := uc.repo.UpdateDevServerID(ctx, tenantID, in.ProjectID, in.NewDevServerID)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_REBIND_FAILED", "failed to update dev server binding", err)
	}

	// NEW — both best-effort, after the authoritative write succeeds.
	actorID, _ := tenant.UserID(ctx)
	if uc.audit != nil {
		_ = uc.audit.PublishAuditEvent(ctx, tenantID, actorID, "project.devserver.changed", in.ProjectID)
	}
	if uc.notifier != nil {
		members, err := uc.repo.ListMembers(ctx, in.ProjectID)
		if err == nil {
			ids := make([]string, 0, len(members))
			for _, m := range members {
				ids = append(ids, m.UserID)
			}
			_ = uc.notifier.NotifyDevServerChanged(ctx, tenantID, ids, in.ProjectID, before.DevServerID, in.NewDevServerID)
		}
	}
	return updated, nil
}
