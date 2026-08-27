package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestRemoveRepo_DeletesRepo(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewRemoveRepo(repo, ownerMembership("p1", "u1"), &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	if err := uc.Execute(ctx, RemoveRepoInput{RepoID: "r1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.repos["r1"]; ok {
		t.Error("expected repo to be deleted")
	}
}

func TestRemoveRepo_NotFound(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewRemoveRepo(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, RemoveRepoInput{RepoID: "missing"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND")
}

func TestRemoveRepo_RequiresRepoID(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewRemoveRepo(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, RemoveRepoInput{RepoID: ""})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_REPO_ID_REQUIRED")
}

func TestRemoveRepo_OwnerAllowedMemberDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewRemoveRepo(repo, membership, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	err := uc.Execute(ctx, RemoveRepoInput{RepoID: "r1"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if _, ok := repo.repos["r1"]; !ok {
		t.Error("expected repo to remain undeleted")
	}
}

func TestRemoveRepo_GlobalAdminAllowedRegardlessOfProjectRole(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	opa := &fakeOPAClient{decide: func(callerProjectRole, callerGlobalRole, action string) bool { return true }}
	uc := NewRemoveRepo(repo, newFakeProjectRepository(), opa)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "admin-1")
	if err := uc.Execute(ctx, RemoveRepoInput{RepoID: "r1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveRepo_FailsClosedOnPolicyEvalError(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewRemoveRepo(repo, ownerMembership("p1", "u1"), &fakeOPAClient{err: errors.New("opa unreachable")})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	err := uc.Execute(ctx, RemoveRepoInput{RepoID: "r1"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED")
	if _, ok := repo.repos["r1"]; !ok {
		t.Error("expected repo to remain undeleted")
	}
}
