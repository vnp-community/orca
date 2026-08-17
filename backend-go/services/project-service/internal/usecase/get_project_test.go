package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestGetProject_OwnerAllowed(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	opa := &fakeOPAClient{decide: projectRegoDecide}
	uc := NewGetProject(repo, opa)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, GetProjectInput{ID: "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "p1" {
		t.Errorf("expected project p1, got %+v", got)
	}
	if len(opa.calls) != 1 || opa.calls[0].CallerProjectRole != "owner" || opa.calls[0].Action != projectActionAnyMember {
		t.Errorf("expected one owner/any_member Decision call, got %+v", opa.calls)
	}
}

func TestGetProject_MemberAllowed(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleMember})
	uc := NewGetProject(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	if _, err := uc.Execute(ctx, GetProjectInput{ID: "p1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetProject_NonMemberDenied(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	uc := NewGetProject(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, GetProjectInput{ID: "p1"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
}

func TestGetProject_GlobalAdminAllowedRegardlessOfProjectRole(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	// No membership row at all — the fake's decide func stands in for OPA
	// resolving a global-admin claim (see project.rego's admin-override
	// branch) and authorizes regardless of caller_project_role.
	opa := &fakeOPAClient{decide: func(callerProjectRole, callerGlobalRole, action string) bool { return true }}
	uc := NewGetProject(repo, opa)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	if _, err := uc.Execute(ctx, GetProjectInput{ID: "p1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetProject_FailsClosedOnPolicyEvalError(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewGetProject(repo, &fakeOPAClient{err: errors.New("opa unreachable")})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, GetProjectInput{ID: "p1"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED")
}

func TestGetProject_FailsClosedOnMembershipLookupError(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	repo.getMembershipErr = errors.New("db unreachable")
	uc := NewGetProject(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, GetProjectInput{ID: "p1"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_MEMBERSHIP_LOOKUP_FAILED")
}

func TestGetProject_NotFound(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "missing", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewGetProject(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, GetProjectInput{ID: "missing"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_NOT_FOUND")
}

func TestGetProject_RequiresTenantContext(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewGetProject(repo, allowAllOPA())

	_, err := uc.Execute(context.Background(), GetProjectInput{ID: "p1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestGetProject_RequiresUserContext(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj"}
	uc := NewGetProject(repo, allowAllOPA())

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, GetProjectInput{ID: "p1"})
	assertAppError(t, err, apperrors.KindUnauthenticated, "PROJECT_NO_USER")
}
