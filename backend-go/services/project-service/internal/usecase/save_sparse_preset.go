package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type SaveSparsePresetInput struct {
	RepoID string
	// ID is optional — empty creates a new preset; non-empty updates the
	// existing preset with that id (see Execute's doc comment for the exact
	// fallback behavior when it doesn't match any existing preset).
	ID          string
	Name        string
	Directories []string
}

// SaveSparsePreset upserts a sparse-checkout preset — gated by the same
// repo_any_functional_role tier as ListSparsePresets.
type SaveSparsePreset struct {
	presets    SparsePresetRepository
	repos      RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewSaveSparsePreset(presets SparsePresetRepository, repos RepoRepository, membership MembershipRepository, opa OPAClient) *SaveSparsePreset {
	return &SaveSparsePreset{presets: presets, repos: repos, membership: membership, opa: opa}
}

func (uc *SaveSparsePreset) Execute(ctx context.Context, in SaveSparsePresetInput) (domain.SparsePreset, error) {
	existingRepo, err := uc.repos.GetRepo(ctx, in.RepoID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return domain.SparsePreset{}, apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return domain.SparsePreset{}, apperrors.New(apperrors.KindInternal, "PROJECT_REPO_FETCH_FAILED", "failed to fetch repo", err)
	}

	if err := requireRepoAccess(ctx, uc.membership, uc.repos, uc.opa, existingRepo.ProjectID, in.RepoID, repoActionAnyFunctionalRole); err != nil {
		return domain.SparsePreset{}, err
	}

	name, err := normalizeSparsePresetName(in.Name)
	if err != nil {
		return domain.SparsePreset{}, err
	}
	directories, err := normalizeSparsePresetDirectories(in.Directories)
	if err != nil {
		return domain.SparsePreset{}, err
	}

	now := time.Now()
	id := uuid.NewString()
	createdAt := now
	// Why (matches the legacy reference exactly, see sparse-presets.ts's
	// saveSparsePresetForRepo): a non-empty in.ID that doesn't match any
	// existing preset is NOT an error — it silently falls back to creating
	// a brand-new preset (fresh id, fresh createdAt), same as no id at all.
	if in.ID != "" {
		if existing, err := uc.presets.GetSparsePreset(ctx, in.RepoID, in.ID); err == nil {
			id = existing.ID
			createdAt = existing.CreatedAt
		} else if !errors.Is(err, domain.ErrSparsePresetNotFound) {
			return domain.SparsePreset{}, apperrors.New(apperrors.KindInternal, "PROJECT_SPARSE_PRESET_FETCH_FAILED", "failed to fetch existing preset", err)
		}
	}

	preset := domain.SparsePreset{
		ID:          id,
		RepoID:      in.RepoID,
		Name:        name,
		Directories: directories,
		CreatedAt:   createdAt,
		UpdatedAt:   now,
	}
	saved, err := uc.presets.SaveSparsePreset(ctx, preset)
	if err != nil {
		return domain.SparsePreset{}, apperrors.New(apperrors.KindInternal, "PROJECT_SAVE_SPARSE_PRESET_FAILED", "failed to save sparse preset", err)
	}
	return saved, nil
}
