package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// newTestRebindDevServer builds a RebindDevServer with fresh fakes for its
// four new (TASK-PRF-03-05) dependencies. devServers/health default to
// "exists and reachable" so every pre-existing test (written before those
// checks existed) keeps passing without change.
func newTestRebindDevServer(repo *fakeProjectRepository, workflowChecker, taskChecker executionChecker, opa OPAClient) (*RebindDevServer, *fakeDevServerLister, *fakeDevServerHealthChecker, *fakeProjectAuditPublisher, *fakeMemberNotifier) {
	devServers := &fakeDevServerLister{exists: true}
	health := &fakeDevServerHealthChecker{reachable: true}
	audit := &fakeProjectAuditPublisher{}
	notifier := &fakeMemberNotifier{}
	return NewRebindDevServer(repo, workflowChecker, taskChecker, opa, devServers, health, audit, notifier), devServers, health, audit, notifier
}

// executionChecker is the shared interface WorkflowExecutionChecker/
// TaskExecutionChecker both satisfy — lets newTestRebindDevServer take
// either fake without importing the concrete usecase port types twice.
type executionChecker interface {
	HasActiveExecutions(ctx context.Context, projectID string) (bool, error)
}

func TestRebindDevServer_AllowedWhenNoActiveExecutions(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	uc, _, _, _, _ := newTestRebindDevServer(repo, &fakeExecutionChecker{active: false}, &fakeExecutionChecker{active: false}, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DevServerID != "dev-2" {
		t.Errorf("expected DevServerID=dev-2, got %q", got.DevServerID)
	}
}

func TestRebindDevServer_RejectedWhenWorkflowHasActiveExecutions(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	uc, _, _, _, _ := newTestRebindDevServer(repo, &fakeExecutionChecker{active: true}, &fakeExecutionChecker{active: false}, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	assertFailedPrecondition(t, err)

	if repo.projects["p1"].DevServerID != "dev-1" {
		t.Errorf("expected dev_server_id to remain unchanged, got %q", repo.projects["p1"].DevServerID)
	}
}

func TestRebindDevServer_RejectedWhenTaskHasActiveExecutions(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	uc, _, _, _, _ := newTestRebindDevServer(repo, &fakeExecutionChecker{active: false}, &fakeExecutionChecker{active: true}, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	assertFailedPrecondition(t, err)

	if repo.projects["p1"].DevServerID != "dev-1" {
		t.Errorf("expected dev_server_id to remain unchanged, got %q", repo.projects["p1"].DevServerID)
	}
}

func TestRebindDevServer_FailsClosedOnCheckerError(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	uc, _, _, _, _ := newTestRebindDevServer(repo, &fakeExecutionChecker{err: errors.New("workflow-service unreachable")}, &fakeExecutionChecker{active: false}, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	assertFailedPrecondition(t, err)
}

func TestRebindDevServer_RequiresTenantContext(t *testing.T) {
	repo := newFakeProjectRepository()
	uc, _, _, _, _ := newTestRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, allowAllOPA())

	_, err := uc.Execute(context.Background(), RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestRebindDevServer_RejectsEmptyNewDevServerID(t *testing.T) {
	repo := newFakeProjectRepository()
	uc, _, _, _, _ := newTestRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: ""})
	if err == nil {
		t.Fatal("expected an error for empty new_dev_server_id")
	}
}

func TestRebindDevServer_OwnerAllowedMemberDenied(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})

	uc, _, _, _, _ := newTestRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repo.projects["p1"].DevServerID != "dev-1" {
		t.Errorf("expected dev_server_id to remain unchanged, got %q", repo.projects["p1"].DevServerID)
	}
}

func TestRebindDevServer_GlobalAdminAllowedRegardlessOfProjectRole(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	opa := &fakeOPAClient{decide: func(callerProjectRole, callerGlobalRole, action string) bool { return true }}

	uc, _, _, _, _ := newTestRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, opa)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "admin-1")
	if _, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRebindDevServer_FailsClosedOnPolicyEvalError(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	uc, _, _, _, _ := newTestRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeOPAClient{err: errors.New("opa unreachable")})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED")
	if repo.projects["p1"].DevServerID != "dev-1" {
		t.Errorf("expected dev_server_id to remain unchanged, got %q", repo.projects["p1"].DevServerID)
	}
}

func TestRebindDevServer_NewDevServerNotFound_PrecedesActiveExecutionGuard(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	workflowChecker := &fakeExecutionChecker{}
	taskChecker := &fakeExecutionChecker{}
	uc, devServers, _, _, _ := newTestRebindDevServer(repo, workflowChecker, taskChecker, allowAllOPA())
	devServers.exists = false

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_DEV_SERVER_NOT_FOUND")
}

func TestRebindDevServer_NewDevServerUnreachable_PrecedesActiveExecutionGuard(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	uc, _, health, _, _ := newTestRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, allowAllOPA())
	health.reachable = false

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	assertAppError(t, err, apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_UNREACHABLE")
}

func TestRebindDevServer_PublishesAuditAndNotifiesMembersOnSuccess(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	repo.members = append(repo.members,
		domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner},
		domain.ProjectMember{ProjectID: "p1", UserID: "u2", Role: domain.ProjectRoleMember},
	)

	uc, _, _, audit, notifier := newTestRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	if _, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(audit.calls) != 1 || audit.calls[0].action != "project.devserver.changed" {
		t.Errorf("expected exactly 1 audit event, got %+v", audit.calls)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected exactly 1 notify call, got %+v", notifier.calls)
	}
	call := notifier.calls[0]
	if call.oldDevServer != "dev-1" || call.newDev != "dev-2" {
		t.Errorf("expected old=dev-1 new=dev-2, got old=%q new=%q", call.oldDevServer, call.newDev)
	}
	if len(call.userIDs) != 2 {
		t.Errorf("expected both members notified, got %v", call.userIDs)
	}
}

func TestRebindDevServer_NilAuditAndNotifierDoNotPanic(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	uc := NewRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, allowAllOPA(), &fakeDevServerLister{exists: true}, &fakeDevServerHealthChecker{reachable: true}, nil, nil)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	if _, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
