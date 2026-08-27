package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func ownerMembership(projectID, userID string) *fakeProjectRepository {
	m := newFakeProjectRepository()
	m.members = append(m.members, domain.ProjectMember{ProjectID: projectID, UserID: userID, Role: domain.ProjectRoleOwner})
	return m
}

func TestAddRepo_AppendsAtNextPosition(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewAddRepo(repo, ownerMembership("p1", "u1"), &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	first, err := uc.Execute(ctx, AddRepoInput{ProjectID: "p1", URL: "https://github.com/org/one"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first.Position != 0 {
		t.Errorf("expected first repo at position 0, got %d", first.Position)
	}

	second, err := uc.Execute(ctx, AddRepoInput{ProjectID: "p1", URL: "https://github.com/org/two"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if second.Position != 1 {
		t.Errorf("expected second repo at position 1, got %d", second.Position)
	}
}

func TestAddRepo_RejectsEmptyURL(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewAddRepo(repo, ownerMembership("p1", "u1"), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AddRepoInput{ProjectID: "p1", URL: ""})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_REPO_INVALID")
}

func TestAddRepo_RequiresTenantContext(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewAddRepo(repo, newFakeProjectRepository(), allowAllOPA())

	_, err := uc.Execute(context.Background(), AddRepoInput{ProjectID: "p1", URL: "https://github.com/org/one"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestAddRepo_OwnerAllowedMemberDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewAddRepo(repo, membership, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	_, err := uc.Execute(ctx, AddRepoInput{ProjectID: "p1", URL: "https://github.com/org/one"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if len(repo.repos) != 0 {
		t.Errorf("expected no repo added, got %+v", repo.repos)
	}
}

func TestAddRepo_GlobalAdminAllowedRegardlessOfProjectRole(t *testing.T) {
	repo := newFakeRepoRepository()
	opa := &fakeOPAClient{decide: func(callerProjectRole, callerGlobalRole, action string) bool { return true }}
	uc := NewAddRepo(repo, newFakeProjectRepository(), opa)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "admin-1")
	if _, err := uc.Execute(ctx, AddRepoInput{ProjectID: "p1", URL: "https://github.com/org/one"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddRepo_FailsClosedOnPolicyEvalError(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewAddRepo(repo, ownerMembership("p1", "u1"), &fakeOPAClient{err: errors.New("opa unreachable")})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AddRepoInput{ProjectID: "p1", URL: "https://github.com/org/one"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED")
	if len(repo.repos) != 0 {
		t.Errorf("expected no repo added, got %+v", repo.repos)
	}
}
