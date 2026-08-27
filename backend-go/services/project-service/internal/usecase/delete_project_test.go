package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestDeleteProject_AllowedWhenNoActiveExecutions(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewDeleteProject(repo, &fakeExecutionChecker{active: false}, &fakeExecutionChecker{active: false}, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	if err := uc.Execute(ctx, DeleteProjectInput{ProjectID: "p1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.projects["p1"]; ok {
		t.Error("expected project to be deleted")
	}
}

func TestDeleteProject_RejectedWhenWorkflowHasActiveExecutions(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewDeleteProject(repo, &fakeExecutionChecker{active: true}, &fakeExecutionChecker{active: false}, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, DeleteProjectInput{ProjectID: "p1"})
	assertFailedPrecondition(t, err)

	if _, ok := repo.projects["p1"]; !ok {
		t.Error("expected project to remain undeleted")
	}
}

func TestDeleteProject_RejectedWhenTaskHasActiveExecutions(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewDeleteProject(repo, &fakeExecutionChecker{active: false}, &fakeExecutionChecker{active: true}, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, DeleteProjectInput{ProjectID: "p1"})
	assertFailedPrecondition(t, err)
}

func TestDeleteProject_FailsClosedOnCheckerError(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewDeleteProject(repo, &fakeExecutionChecker{err: errors.New("workflow-service unreachable")}, &fakeExecutionChecker{active: false}, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, DeleteProjectInput{ProjectID: "p1"})
	assertFailedPrecondition(t, err)
}

func TestDeleteProject_NotFound(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "missing", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewDeleteProject(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, DeleteProjectInput{ProjectID: "missing"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_NOT_FOUND")
}

func TestDeleteProject_RequiresTenantContext(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewDeleteProject(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, allowAllOPA())

	err := uc.Execute(context.Background(), DeleteProjectInput{ProjectID: "p1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestDeleteProject_OwnerAllowedMemberDenied(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewDeleteProject(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	err := uc.Execute(ctx, DeleteProjectInput{ProjectID: "p1"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if _, ok := repo.projects["p1"]; !ok {
		t.Error("expected project to remain undeleted")
	}
}

func TestDeleteProject_GlobalAdminAllowedRegardlessOfProjectRole(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	opa := &fakeOPAClient{decide: func(callerProjectRole, callerGlobalRole, action string) bool { return true }}
	uc := NewDeleteProject(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, opa)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "admin-1")
	if err := uc.Execute(ctx, DeleteProjectInput{ProjectID: "p1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteProject_FailsClosedOnPolicyEvalError(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewDeleteProject(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{}, &fakeOPAClient{err: errors.New("opa unreachable")})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, DeleteProjectInput{ProjectID: "p1"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED")
	if _, ok := repo.projects["p1"]; !ok {
		t.Error("expected project to remain undeleted")
	}
}
