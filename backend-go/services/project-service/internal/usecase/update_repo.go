package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// UpdateRepoInput mirrors the gRPC request 1:1 — empty URL/DisplayName
// means "no change," same field-mask convention UpdateProject already
// uses (project.proto's UpdateProjectRequest doc comment). HookSettings
// uses explicit presence (nil pointer) instead, since an empty JSON blob
// is a legitimate value (the user cleared all local scripts) that must be
// distinguishable from "this request doesn't touch hook_settings at all."
type UpdateRepoInput struct {
	RepoID       string
	URL          string
	DisplayName  string
	HookSettings *string
}

// UpdateRepo applies a field-masked edit to a repo's url/display_name.
//
// Authorization: repo_admin_only — a project owner always passes (see
// requireRepoAccess), or a caller holding an "admin" repo_members grant on
// this specific repo. UpdateRepoInput carries only a repo_id, so Execute
// resolves the repo's owning project via RepoRepository.GetRepo before it
// can check the caller's role against it.
type UpdateRepo struct {
	repo       RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewUpdateRepo(repo RepoRepository, membership MembershipRepository, opa OPAClient) *UpdateRepo {
	return &UpdateRepo{repo: repo, membership: membership, opa: opa}
}

func (uc *UpdateRepo) Execute(ctx context.Context, in UpdateRepoInput) (domain.Repo, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.RepoID == "" {
		return domain.Repo{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REPO_ID_REQUIRED", "repo_id is required", nil)
	}

	repo, err := uc.repo.GetRepo(ctx, in.RepoID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return domain.Repo{}, apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindInternal, "PROJECT_REPO_FETCH_FAILED", "failed to fetch repo", err)
	}
	if err := requireRepoAccess(ctx, uc.membership, uc.repo, uc.opa, repo.ProjectID, repo.ID, repoActionAdminOnly); err != nil {
		return domain.Repo{}, err
	}

	if in.URL != "" {
		repo.URL = in.URL
	}
	if in.DisplayName != "" {
		repo.DisplayName = in.DisplayName
	}
	if in.HookSettings != nil {
		repo.HookSettings = *in.HookSettings
	}

	updated, err := uc.repo.Update(ctx, repo)
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindInternal, "PROJECT_UPDATE_REPO_FAILED", "failed to persist repo update", err)
	}
	return updated, nil
}
