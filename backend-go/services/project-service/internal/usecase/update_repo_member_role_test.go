package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestUpdateRepoMemberRole_ProjectOwnerCanPromote(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	repo.repoMembers = append(repo.repoMembers, domain.RepoMember{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleDeveloper})
	uc := NewUpdateRepoMemberRole(repo, ownerMembership("p1", "owner-1"), &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	member, err := uc.Execute(ctx, UpdateRepoMemberRoleInput{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleLead})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if member.Role != domain.RepoRoleLead {
		t.Errorf("expected lead, got %v", member.Role)
	}
}

func TestUpdateRepoMemberRole_PlainMemberDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	repo.repoMembers = append(repo.repoMembers, domain.RepoMember{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleDeveloper})
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewUpdateRepoMemberRole(repo, membership, &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	_, err := uc.Execute(ctx, UpdateRepoMemberRoleInput{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleLead})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
}

func TestUpdateRepoMemberRole_RejectsInvalidRole(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewUpdateRepoMemberRole(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, UpdateRepoMemberRoleInput{RepoID: "r1", UserID: "dev-1", Role: "bogus"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_INVALID_REPO_ROLE")
}

func TestUpdateRepoMemberRole_RepoNotFound(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewUpdateRepoMemberRole(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, UpdateRepoMemberRoleInput{RepoID: "missing", UserID: "dev-1", Role: domain.RepoRoleLead})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND")
}

func TestUpdateRepoMemberRole_MembershipNotFound(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewUpdateRepoMemberRole(repo, ownerMembership("p1", "owner-1"), &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	_, err := uc.Execute(ctx, UpdateRepoMemberRoleInput{RepoID: "r1", UserID: "never-granted", Role: domain.RepoRoleLead})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_MEMBERSHIP_NOT_FOUND")
}
