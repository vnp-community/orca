package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestListRepoMembers_ProjectOwnerCanList(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	repo.repoMembers = append(repo.repoMembers, domain.RepoMember{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleDeveloper})
	uc := NewListRepoMembers(repo, ownerMembership("p1", "owner-1"), &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	got, err := uc.Execute(ctx, ListRepoMembersInput{RepoID: "r1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].UserID != "dev-1" {
		t.Errorf("unexpected members: %+v", got)
	}
}

func TestListRepoMembers_GrantedDeveloperCanList(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	repo.repoMembers = append(repo.repoMembers, domain.RepoMember{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleDeveloper})
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "dev-1", Role: domain.ProjectRoleMember})
	uc := NewListRepoMembers(repo, membership, &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "dev-1")
	if _, err := uc.Execute(ctx, ListRepoMembersInput{RepoID: "r1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListRepoMembers_UngrantedMemberDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewListRepoMembers(repo, membership, &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	_, err := uc.Execute(ctx, ListRepoMembersInput{RepoID: "r1"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
}

func TestListRepoMembers_RepoNotFound(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewListRepoMembers(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, ListRepoMembersInput{RepoID: "missing"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND")
}
