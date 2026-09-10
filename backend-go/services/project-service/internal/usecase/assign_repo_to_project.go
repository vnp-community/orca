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
// Authorization: project OWNER on BOTH the repo's CURRENT project and the
// TARGET project — deliberately NOT repo_admin_only on the source side
// (unlike RemoveRepo/UpdateRepo). A repo_members functional-role grant can
// outlive the caller's actual project membership (nothing revokes it when
// membership lapses or the repo moves) — accepting that grant alone as
// sufficient to move a repo OUT of a project would let a holder of an old,
// forgotten "admin" grant exfiltrate the repo into a project they own,
// with no real relationship to the source project at all. Project
// ownership can't be similarly stale (RemoveMember/tenant offboarding
// revokes it directly), so both sides require it.
//
// Both authorization checks run BEFORE the "already in target project"
// equality check below — otherwise an unauthenticated-for-either-project
// caller could use PROJECT_REPO_ALREADY_IN_PROJECT vs
// PROJECT_NOT_AUTHORIZED to probe "does repo X belong to project Y" for
// free.
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

	if err := requireProjectAccess(ctx, uc.membership, uc.opa, repo.ProjectID, projectActionOwnerOnly); err != nil {
		return domain.Repo{}, err
	}
	if err := requireProjectAccess(ctx, uc.membership, uc.opa, in.TargetProjectID, projectActionOwnerOnly); err != nil {
		return domain.Repo{}, err
	}

	if repo.ProjectID == in.TargetProjectID {
		return domain.Repo{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REPO_ALREADY_IN_PROJECT", "repo is already in the target project", nil)
	}

	updated, err := uc.repo.ReassignProject(ctx, in.RepoID, repo.ProjectID, in.TargetProjectID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return domain.Repo{}, apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if errors.Is(err, domain.ErrRepoProjectChanged) {
		return domain.Repo{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_REPO_PROJECT_CHANGED", "the repo's project changed concurrently — retry", err)
	}
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindInternal, "PROJECT_ASSIGN_REPO_FAILED", "failed to move repo to target project", err)
	}
	return updated, nil
}
