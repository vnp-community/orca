package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestListWorktrees_ReturnsOnlyProjectWorktrees(t *testing.T) {
	repo := newFakeWorktreeRepository()
	repo.worktrees["w1"] = domain.Worktree{ID: "w1", ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "main"}
	repo.worktrees["w2"] = domain.Worktree{ID: "w2", ProjectID: "p2", RepoID: "r2", Path: "/srv/w2", Branch: "main"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewListWorktrees(repo, membership, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, ListWorktreesInput{ProjectID: "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "w1" {
		t.Errorf("expected only w1, got %+v", got)
	}
}

func TestListWorktrees_RequiresTenantContext(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewListWorktrees(repo, newFakeProjectRepository(), allowAllOPA())

	_, err := uc.Execute(context.Background(), ListWorktreesInput{ProjectID: "p1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListWorktrees_MemberAllowedNonMemberDenied(t *testing.T) {
	repo := newFakeWorktreeRepository()
	repo.worktrees["w1"] = domain.Worktree{ID: "w1", ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "main"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewListWorktrees(repo, membership, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	if _, err := uc.Execute(ctx, ListWorktreesInput{ProjectID: "p1"}); err != nil {
		t.Fatalf("unexpected error for member: %v", err)
	}

	ctx = withTenantAndUser(context.Background(), "tenant-1", "stranger-1")
	_, err := uc.Execute(ctx, ListWorktreesInput{ProjectID: "p1"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
}

func TestListWorktrees_FailsClosedOnPolicyEvalError(t *testing.T) {
	repo := newFakeWorktreeRepository()
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewListWorktrees(repo, membership, &fakeOPAClient{err: errors.New("opa unreachable")})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, ListWorktreesInput{ProjectID: "p1"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED")
}
