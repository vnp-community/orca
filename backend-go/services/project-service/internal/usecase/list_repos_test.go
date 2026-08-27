package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestListRepos_ReturnsOnlyProjectRepos(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "https://github.com/org/one"}
	repo.repos["r2"] = domain.Repo{ID: "r2", ProjectID: "p2", URL: "https://github.com/org/two"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewListRepos(repo, membership, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, ListReposInput{ProjectID: "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "r1" {
		t.Errorf("expected only r1, got %+v", got)
	}
}

func TestListRepos_RequiresTenantContext(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewListRepos(repo, newFakeProjectRepository(), allowAllOPA())

	_, err := uc.Execute(context.Background(), ListReposInput{ProjectID: "p1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListRepos_MemberAllowedNonMemberDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewListRepos(repo, membership, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	if _, err := uc.Execute(ctx, ListReposInput{ProjectID: "p1"}); err != nil {
		t.Fatalf("unexpected error for member: %v", err)
	}

	ctx = withTenantAndUser(context.Background(), "tenant-1", "stranger-1")
	_, err := uc.Execute(ctx, ListReposInput{ProjectID: "p1"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
}

func TestListRepos_FailsClosedOnPolicyEvalError(t *testing.T) {
	repo := newFakeRepoRepository()
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewListRepos(repo, membership, &fakeOPAClient{err: errors.New("opa unreachable")})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, ListReposInput{ProjectID: "p1"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED")
}
