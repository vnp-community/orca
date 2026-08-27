package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestRebindDevServer_AllowedWhenNoActiveExecutions(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	uc := NewRebindDevServer(repo, &fakeExecutionChecker{active: false}, &fakeExecutionChecker{active: false}, allowAllOPA())

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

	uc := NewRebindDevServer(repo, &fakeExecutionChecker{active: true}, &fakeExecutionChecker{active: false}, allowAllOPA())

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

	uc := NewRebindDevServer(repo, &fakeExecutionChecker{active: false}, &fakeExecutionChecker{active: true}, allowAllOPA())

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

	uc := NewRebindDevServer(repo, &fakeExecutionChecker{err: errors.New("workflow-service unreachable")}, &fakeExecutionChecker{active: false}, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	assertFailedPrecondition(t, err)
}

func TestRebindDevServer_RequiresTenantContext(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, allowAllOPA())

	_, err := uc.Execute(context.Background(), RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestRebindDevServer_RejectsEmptyNewDevServerID(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, allowAllOPA())

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

	uc := NewRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeOPAClient{decide: projectRegoDecide})

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

	uc := NewRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, opa)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "admin-1")
	if _, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRebindDevServer_FailsClosedOnPolicyEvalError(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})

	uc := NewRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeOPAClient{err: errors.New("opa unreachable")})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED")
	if repo.projects["p1"].DevServerID != "dev-1" {
		t.Errorf("expected dev_server_id to remain unchanged, got %q", repo.projects["p1"].DevServerID)
	}
}
