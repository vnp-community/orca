package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ListSparsePresetsInput struct {
	RepoID string
}

// ListSparsePresets returns every sparse-checkout preset saved for a repo —
// gated by repo_any_functional_role, the same visibility tier ListRepos
// uses (an owner always bypasses; a non-owner project member needs a
// repo_members grant on this specific repo).
type ListSparsePresets struct {
	presets    SparsePresetRepository
	repos      RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewListSparsePresets(presets SparsePresetRepository, repos RepoRepository, membership MembershipRepository, opa OPAClient) *ListSparsePresets {
	return &ListSparsePresets{presets: presets, repos: repos, membership: membership, opa: opa}
}

func (uc *ListSparsePresets) Execute(ctx context.Context, in ListSparsePresetsInput) ([]domain.SparsePreset, error) {
	existing, err := uc.repos.GetRepo(ctx, in.RepoID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return nil, apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_REPO_FETCH_FAILED", "failed to fetch repo", err)
	}

	if err := requireRepoAccess(ctx, uc.membership, uc.repos, uc.opa, existing.ProjectID, in.RepoID, repoActionAnyFunctionalRole); err != nil {
		return nil, err
	}

	presets, err := uc.presets.ListSparsePresets(ctx, in.RepoID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_SPARSE_PRESETS_FAILED", "failed to list sparse presets", err)
	}
	return presets, nil
}
