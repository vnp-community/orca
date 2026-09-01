package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestRemoveRepoMember_ProjectOwnerCanRevoke(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	repo.repoMembers = append(repo.repoMembers, domain.RepoMember{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleDeveloper})
	uc := NewRemoveRepoMember(repo, ownerMembership("p1", "owner-1"), &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	if err := uc.Execute(ctx, RemoveRepoMemberInput{RepoID: "r1", UserID: "dev-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.repoMembers) != 0 {
		t.Errorf("expected grant to be removed, got %+v", repo.repoMembers)
	}
}

func TestRemoveRepoMember_PlainMemberDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	repo.repoMembers = append(repo.repoMembers, domain.RepoMember{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleDeveloper})
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewRemoveRepoMember(repo, membership, &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	err := uc.Execute(ctx, RemoveRepoMemberInput{RepoID: "r1", UserID: "dev-1"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if len(repo.repoMembers) != 1 {
		t.Errorf("expected grant to remain, got %+v", repo.repoMembers)
	}
}

func TestRemoveRepoMember_RepoNotFound(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewRemoveRepoMember(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, RemoveRepoMemberInput{RepoID: "missing", UserID: "dev-1"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND")
}

func TestRemoveRepoMember_MembershipNotFound(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewRemoveRepoMember(repo, ownerMembership("p1", "owner-1"), &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	err := uc.Execute(ctx, RemoveRepoMemberInput{RepoID: "r1", UserID: "never-granted"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_MEMBERSHIP_NOT_FOUND")
}
