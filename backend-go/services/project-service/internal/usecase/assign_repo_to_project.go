package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// AssignRepoToProjectInput mirrors the gRPC request 1:1.
type AssignRepoToProjectInput struct {
	RepoID          string
	TargetProjectID string
}

// AssignRepoToProject moves an existing repo (already living in some other
// project) into a different project — distinct from AddRepo, which always
// creates a brand-new repo row. Backs Project Settings' "Repos" tab
// candidate picker (attach an already-known repo to this OrcaProject).
//
// Authorization: repo_admin_only on the repo's CURRENT project (same bar
// as RemoveRepo/UpdateRepo — moving a repo out from under its current
// owner is as sensitive as deleting it) AND project owner on the TARGET
// project (same bar AddRepo uses — attaching a repo to a project is
// equivalent to adding one there).
type AssignRepoToProject struct {
	repo       RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewAssignRepoToProject(repo RepoRepository, membership MembershipRepository, opa OPAClient) *AssignRepoToProject {
	return &AssignRepoToProject{repo: repo, membership: membership, opa: opa}
}

func (uc *AssignRepoToProject) Execute(ctx context.Context, in AssignRepoToProjectInput) (domain.Repo, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.RepoID == "" {
		return domain.Repo{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REPO_ID_REQUIRED", "repo_id is required", nil)
	}
	if in.TargetProjectID == "" {
		return domain.Repo{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_TARGET_PROJECT_ID_REQUIRED", "target_project_id is required", nil)
	}

	repo, err := uc.repo.GetRepo(ctx, in.RepoID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return domain.Repo{}, apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindInternal, "PROJECT_REPO_FETCH_FAILED", "failed to fetch repo", err)
	}
	if repo.ProjectID == in.TargetProjectID {
		return domain.Repo{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REPO_ALREADY_IN_PROJECT", "repo is already in the target project", nil)
	}

	if err := requireRepoAccess(ctx, uc.membership, uc.repo, uc.opa, repo.ProjectID, repo.ID, repoActionAdminOnly); err != nil {
		return domain.Repo{}, err
	}
	if err := requireProjectAccess(ctx, uc.membership, uc.opa, in.TargetProjectID, projectActionOwnerOnly); err != nil {
		return domain.Repo{}, err
	}

	updated, err := uc.repo.ReassignProject(ctx, in.RepoID, in.TargetProjectID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return domain.Repo{}, apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindInternal, "PROJECT_ASSIGN_REPO_FAILED", "failed to move repo to target project", err)
	}
	return updated, nil
}
