package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type RemoveSparsePresetInput struct {
	RepoID   string
	PresetID string
}

// RemoveSparsePreset deletes a sparse-checkout preset — gated by the same
// repo_any_functional_role tier as ListSparsePresets/SaveSparsePreset.
type RemoveSparsePreset struct {
	presets    SparsePresetRepository
	repos      RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewRemoveSparsePreset(presets SparsePresetRepository, repos RepoRepository, membership MembershipRepository, opa OPAClient) *RemoveSparsePreset {
	return &RemoveSparsePreset{presets: presets, repos: repos, membership: membership, opa: opa}
}

func (uc *RemoveSparsePreset) Execute(ctx context.Context, in RemoveSparsePresetInput) error {
	existing, err := uc.repos.GetRepo(ctx, in.RepoID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_REPO_FETCH_FAILED", "failed to fetch repo", err)
	}

	if err := requireRepoAccess(ctx, uc.membership, uc.repos, uc.opa, existing.ProjectID, in.RepoID, repoActionAnyFunctionalRole); err != nil {
		return err
	}

	if err := uc.presets.RemoveSparsePreset(ctx, in.RepoID, in.PresetID); err != nil {
		if errors.Is(err, domain.ErrSparsePresetNotFound) {
			return apperrors.New(apperrors.KindNotFound, "PROJECT_SPARSE_PRESET_NOT_FOUND", "sparse preset does not exist", err)
		}
		return apperrors.New(apperrors.KindInternal, "PROJECT_REMOVE_SPARSE_PRESET_FAILED", "failed to remove sparse preset", err)
	}
	return nil
}
