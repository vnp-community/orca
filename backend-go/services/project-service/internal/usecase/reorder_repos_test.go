package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestReorderRepos_RewritesPositions(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1", Position: 0}
	repo.repos["r2"] = domain.Repo{ID: "r2", ProjectID: "p1", URL: "u2", Position: 1}
	uc := NewReorderRepos(repo, ownerMembership("p1", "u1"), &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, ReorderReposInput{ProjectID: "p1", RepoIDsInOrder: []string{"r2", "r1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.repos["r2"].Position != 0 {
		t.Errorf("expected r2 at position 0, got %d", repo.repos["r2"].Position)
	}
	if repo.repos["r1"].Position != 1 {
		t.Errorf("expected r1 at position 1, got %d", repo.repos["r1"].Position)
	}
}

func TestReorderRepos_RejectsMissingRepo(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	repo.repos["r2"] = domain.Repo{ID: "r2", ProjectID: "p1", URL: "u2"}
	uc := NewReorderRepos(repo, ownerMembership("p1", "u1"), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	// Only r1 listed — r2 missing from the reorder list.
	err := uc.Execute(ctx, ReorderReposInput{ProjectID: "p1", RepoIDsInOrder: []string{"r1"}})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_REORDER_REPOS_MISMATCH")
}

func TestReorderRepos_RejectsUnknownRepo(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewReorderRepos(repo, ownerMembership("p1", "u1"), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, ReorderReposInput{ProjectID: "p1", RepoIDsInOrder: []string{"r1", "unknown"}})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_REORDER_REPOS_MISMATCH")
}

func TestReorderRepos_RejectsDuplicateInList(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewReorderRepos(repo, ownerMembership("p1", "u1"), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, ReorderReposInput{ProjectID: "p1", RepoIDsInOrder: []string{"r1", "r1"}})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_REORDER_REPOS_MISMATCH")
}

func TestReorderRepos_OwnerAllowedMemberDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewReorderRepos(repo, membership, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	err := uc.Execute(ctx, ReorderReposInput{ProjectID: "p1", RepoIDsInOrder: []string{"r1"}})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repo.repos["r1"].Position != 0 {
		t.Errorf("expected position unchanged, got %d", repo.repos["r1"].Position)
	}
}

func TestReorderRepos_FailsClosedOnPolicyEvalError(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewReorderRepos(repo, ownerMembership("p1", "u1"), &fakeOPAClient{err: errors.New("opa unreachable")})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, ReorderReposInput{ProjectID: "p1", RepoIDsInOrder: []string{"r1"}})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED")
}
