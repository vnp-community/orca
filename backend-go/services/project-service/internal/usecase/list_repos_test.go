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

func TestListRepos_EmptyProjectID_ReturnsAllTenantRepos(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "https://github.com/org/one"}
	repo.repos["r2"] = domain.Repo{ID: "r2", ProjectID: "p2", URL: "https://github.com/org/two"}
	// No membership rows and no OPA decision recorded — an empty ProjectID
	// must never reach requireProjectAccess (there is no single project to
	// check membership against).
	membership := newFakeProjectRepository()
	opa := &fakeOPAClient{decide: projectRegoDecide}
	uc := NewListRepos(repo, membership, opa)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "stranger-1")
	got, err := uc.Execute(ctx, ListReposInput{ProjectID: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected both repos across projects, got %+v", got)
	}
	if len(opa.calls) != 0 {
		t.Errorf("expected no OPA decision for the tenant-wide path, got %+v", opa.calls)
	}
}

func TestListRepos_EmptyProjectID_StillRequiresTenantContext(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewListRepos(repo, newFakeProjectRepository(), allowAllOPA())

	_, err := uc.Execute(context.Background(), ListReposInput{ProjectID: ""})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListRepos_EmptyProjectID_PropagatesRepositoryError(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.listForTenantErr = errors.New("db unavailable")
	uc := NewListRepos(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, ListReposInput{ProjectID: ""})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_LIST_REPOS_FAILED")
}

// TestListRepos_OwnerSeesEveryRepoRegardlessOfRepoMembersGrants asserts the
// repo_members visibility filter's owner bypass: repo_members is opt-in, so
// an owner's own project must never hide a repo from them just because they
// hold no repo_members row for it.
func TestListRepos_OwnerSeesEveryRepoRegardlessOfRepoMembersGrants(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "https://github.com/org/one"}
	repo.repos["r2"] = domain.Repo{ID: "r2", ProjectID: "p1", URL: "https://github.com/org/two"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "u1", Role: domain.ProjectRoleOwner})
	uc := NewListRepos(repo, membership, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, ListReposInput{ProjectID: "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected owner to see both repos with no repo_members rows at all, got %+v", got)
	}
}

// TestListRepos_NonOwnerMemberFilteredToRepoMembersGrants is this feature's
// core behavior: "a developer sees/acts on repo X, a lead manages repo Y" —
// a non-owner project member sees only the repos they hold an explicit
// repo_members grant on, not every repo in the project.
func TestListRepos_NonOwnerMemberFilteredToRepoMembersGrants(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "https://github.com/org/one"}
	repo.repos["r2"] = domain.Repo{ID: "r2", ProjectID: "p1", URL: "https://github.com/org/two"}
	repo.repoMembers = append(repo.repoMembers, domain.RepoMember{RepoID: "r1", UserID: "member-1", Role: domain.RepoRoleDeveloper})
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewListRepos(repo, membership, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	got, err := uc.Execute(ctx, ListReposInput{ProjectID: "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "r1" {
		t.Errorf("expected only r1 (the repo member-1 has a grant on), got %+v", got)
	}
}

// TestListRepos_NonOwnerMemberWithNoRepoMembersGrantsSeesNoRepos covers the
// opposite edge: a member with zero repo_members rows anywhere in the
// project sees an empty list, not every repo (that would defeat the
// visibility filter entirely).
func TestListRepos_NonOwnerMemberWithNoRepoMembersGrantsSeesNoRepos(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "https://github.com/org/one"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewListRepos(repo, membership, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	got, err := uc.Execute(ctx, ListReposInput{ProjectID: "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no repos for a member with no repo_members grants, got %+v", got)
	}
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
