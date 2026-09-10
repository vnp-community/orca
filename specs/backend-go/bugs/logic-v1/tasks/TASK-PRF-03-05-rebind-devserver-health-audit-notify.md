# TASK-PRF-03-05: Add health check, audit emission, and member notification to `RebindDevServer`

**From Solution:** SOL-PRF-03
**Priority:** P1
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/usecase/rebind_dev_server.go`
**Depends on:** TASK-PRF-03-01, TASK-PRF-03-02, TASK-PRF-03-03
**Status:** `[x]` DONE — health check + audit + member notify wired, cheapest-check-first ordering; new test cases pass

---

## Context

`RebindDevServer.Execute` already validates the new dev server exists and
checks active workflow/task executions, but never checks the new server is
actually online, and never emits an audit event or notifies project members
on a successful rebind — both required by BL-PRF-03's rebind flow ("Confirm
-> UPDATE ... -> Notify ... -> audit_log(...)").

## Changes to make

In `backend-go/services/project-service/internal/usecase/rebind_dev_server.go`:

```go
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
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go vet ./services/project-service/internal/usecase/rebind_dev_server.go
```

Add test cases per SOL-PRF-03's Test plan: not-found/unreachable new dev
server cases precede the existing active-execution guard — assert
`workflowChecker`/`taskChecker` are never called when the new dev server
fails existence/health (cheapest check first). Assert `AuditPublisher`/
`MemberNotifier` are both called exactly once on success, with correct
old/new dev server ids and the full member-id list from a fake
`ListMembers`. Assert a `nil` `audit`/`notifier` doesn't panic. Full
build/test lands with TASK-PRF-03-07.
