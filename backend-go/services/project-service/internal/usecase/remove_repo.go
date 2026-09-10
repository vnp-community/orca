package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type RemoveRepoInput struct {
	RepoID string
}

// RemoveRepo deletes a repo from its project's catalog.
//
// Authorization: repo_admin_only — a project owner always passes (see
// requireRepoAccess), or a caller holding an "admin" repo_members grant on
// this specific repo. RemoveRepoInput carries only a repo_id, not a
// project_id, so Execute resolves the repo's owning project via
// RepoRepository.GetRepo before it can check the caller's role against it.
type RemoveRepo struct {
	repo       RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewRemoveRepo(repo RepoRepository, membership MembershipRepository, opa OPAClient) *RemoveRepo {
	return &RemoveRepo{repo: repo, membership: membership, opa: opa}
}

func (uc *RemoveRepo) Execute(ctx context.Context, in RemoveRepoInput) error {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.RepoID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REPO_ID_REQUIRED", "repo_id is required", nil)
	}

	existing, err := uc.repo.GetRepo(ctx, in.RepoID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_REPO_FETCH_FAILED", "failed to fetch repo", err)
	}
	if err := requireRepoAccess(ctx, uc.membership, uc.repo, uc.opa, existing.ProjectID, existing.ID, repoActionAdminOnly); err != nil {
		return err
	}

	err = uc.repo.RemoveRepo(ctx, in.RepoID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_REMOVE_REPO_FAILED", "failed to remove repo", err)
	}
	return nil
}
