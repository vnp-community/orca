package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestUpdateProject_UpdatesOnlyNonEmptyFields(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{
		ID: "p1", TenantID: "tenant-1", Name: "old-name", Description: "old-desc",
		DefaultBranch: "main", Visibility: domain.VisibilityPrivate,
	}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewUpdateProject(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, UpdateProjectInput{ProjectID: "p1", Name: "new-name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "new-name" {
		t.Errorf("expected Name=new-name, got %q", got.Name)
	}
	// Description/DefaultBranch/Visibility left empty in the input — must be unchanged.
	if got.Description != "old-desc" {
		t.Errorf("expected Description to remain unchanged, got %q", got.Description)
	}
	if got.DefaultBranch != "main" {
		t.Errorf("expected DefaultBranch to remain unchanged, got %q", got.DefaultBranch)
	}
	if got.Visibility != domain.VisibilityPrivate {
		t.Errorf("expected Visibility to remain unchanged, got %q", got.Visibility)
	}
}

func TestUpdateProject_NeverTouchesDevServerID(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewUpdateProject(repo, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, UpdateProjectInput{ProjectID: "p1", Name: "renamed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DevServerID != "dev-1" {
		t.Errorf("expected DevServerID to remain unchanged, got %q", got.DevServerID)
	}
}

func TestUpdateProject_RejectsInvalidVisibility(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewUpdateProject(repo, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, UpdateProjectInput{ProjectID: "p1", Visibility: "bogus"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_INVALID_VISIBILITY")
}

func TestUpdateProject_RequiresProjectID(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewUpdateProject(repo, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, UpdateProjectInput{Name: "renamed"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_ID_REQUIRED")
}

func TestUpdateProject_NotFound(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "missing", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewUpdateProject(repo, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, UpdateProjectInput{ProjectID: "missing", Name: "renamed"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_NOT_FOUND")
}

func TestUpdateProject_RequiresTenantContext(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewUpdateProject(repo, allowAllOPA())

	_, err := uc.Execute(context.Background(), UpdateProjectInput{ProjectID: "p1", Name: "renamed"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestUpdateProject_OwnerAllowedMemberDenied(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewUpdateProject(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	_, err := uc.Execute(ctx, UpdateProjectInput{ProjectID: "p1", Name: "renamed"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repo.projects["p1"].Name != "proj" {
		t.Errorf("expected project to remain unchanged, got %+v", repo.projects["p1"])
	}
}

func TestUpdateProject_GlobalAdminAllowedRegardlessOfProjectRole(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	uc := NewUpdateProject(repo, &fakeOPAClient{decide: func(callerProjectRole, callerGlobalRole, action string) bool { return true }})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "admin-1")
	if _, err := uc.Execute(ctx, UpdateProjectInput{ProjectID: "p1", Name: "renamed"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateProject_FailsClosedOnPolicyEvalError(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewUpdateProject(repo, &fakeOPAClient{err: errors.New("opa unreachable")})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, UpdateProjectInput{ProjectID: "p1", Name: "renamed"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED")
	if repo.projects["p1"].Name != "proj" {
		t.Errorf("expected project to remain unchanged, got %+v", repo.projects["p1"])
	}
}
