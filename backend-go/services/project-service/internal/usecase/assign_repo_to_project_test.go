package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestAssignRepoToProject_OwnerOnBothProjectsSucceeds(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source", URL: "https://old"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members,
		domain.ProjectMember{ProjectID: "source", UserID: "u1", Role: domain.ProjectRoleOwner},
		domain.ProjectMember{ProjectID: "target", UserID: "u1", Role: domain.ProjectRoleOwner},
	)
	uc := NewAssignRepoToProject(repo, membership, &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ProjectID != "target" {
		t.Errorf("got ProjectID %q, want %q", got.ProjectID, "target")
	}
	if repo.repos["r1"].ProjectID != "target" {
		t.Error("expected repo's stored ProjectID to be updated")
	}
}

func TestAssignRepoToProject_AppendsAtNextPositionInTarget(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source"}
	repo.repos["r2"] = domain.Repo{ID: "r2", ProjectID: "target", Position: 0}
	repo.repos["r3"] = domain.Repo{ID: "r3", ProjectID: "target", Position: 1}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members,
		domain.ProjectMember{ProjectID: "source", UserID: "u1", Role: domain.ProjectRoleOwner},
		domain.ProjectMember{ProjectID: "target", UserID: "u1", Role: domain.ProjectRoleOwner},
	)
	uc := NewAssignRepoToProject(repo, membership, &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Position != 2 {
		t.Errorf("got Position %d, want 2", got.Position)
	}
}

func TestAssignRepoToProject_RequiresRepoID(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewAssignRepoToProject(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_REPO_ID_REQUIRED")
}

func TestAssignRepoToProject_RequiresTargetProjectID(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewAssignRepoToProject(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_TARGET_PROJECT_ID_REQUIRED")
}

func TestAssignRepoToProject_RepoNotFound(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewAssignRepoToProject(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "missing", TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND")
}

func TestAssignRepoToProject_AlreadyInTargetProject(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "target"}
	uc := NewAssignRepoToProject(repo, ownerMembership("target", "u1"), &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_REPO_ALREADY_IN_PROJECT")
}

func TestAssignRepoToProject_MemberOfSourceDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members,
		domain.ProjectMember{ProjectID: "source", UserID: "u1", Role: domain.ProjectRoleMember},
		domain.ProjectMember{ProjectID: "target", UserID: "u1", Role: domain.ProjectRoleOwner},
	)
	uc := NewAssignRepoToProject(repo, membership, &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repo.repos["r1"].ProjectID != "source" {
		t.Error("expected repo to remain unchanged")
	}
}

func TestAssignRepoToProject_NotAMemberOfTargetDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source"}
	uc := NewAssignRepoToProject(repo, ownerMembership("source", "u1"), &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repo.repos["r1"].ProjectID != "source" {
		t.Error("expected repo to remain unchanged")
	}
}

func TestAssignRepoToProject_MemberOfTargetDenied(t *testing.T) {
	// Owner on source (passes the repo-admin check), but only a member
	// (not owner) of the target — AddRepo-equivalent bar (owner_only)
	// must still deny.
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members,
		domain.ProjectMember{ProjectID: "source", UserID: "u1", Role: domain.ProjectRoleOwner},
		domain.ProjectMember{ProjectID: "target", UserID: "u1", Role: domain.ProjectRoleMember},
	)
	uc := NewAssignRepoToProject(repo, membership, &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repo.repos["r1"].ProjectID != "source" {
		t.Error("expected repo to remain unchanged")
	}
}
