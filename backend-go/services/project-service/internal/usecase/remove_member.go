package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type RemoveMemberInput struct {
	ProjectID string
	UserID    string
}

// RemoveMember requires owner (or global admin) — same tier as AddMember,
// project-service.md §9 — and enforces the "≥1 owner" invariant before
// mutating.
type RemoveMember struct {
	repo ProjectRepository
	opa  OPAClient
}

func NewRemoveMember(repo ProjectRepository, opa OPAClient) *RemoveMember {
	return &RemoveMember{repo: repo, opa: opa}
}

func (uc *RemoveMember) Execute(ctx context.Context, in RemoveMemberInput) error {
	if err := requireProjectAccess(ctx, uc.repo, uc.opa, in.ProjectID, projectActionOwnerOnly); err != nil {
		return err
	}

	target, err := uc.repo.GetMembership(ctx, in.ProjectID, in.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return apperrors.New(apperrors.KindNotFound, "PROJECT_MEMBERSHIP_NOT_FOUND", "membership does not exist", err)
		}
		return apperrors.New(apperrors.KindInternal, "PROJECT_MEMBERSHIP_LOOKUP_FAILED", "failed to look up membership", err)
	}

	owners, err := uc.repo.CountOwners(ctx, in.ProjectID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_COUNT_OWNERS_FAILED", "failed to count project owners", err)
	}
	if err := domain.AssertNotLastOwnerRemoval(owners, target.Role == domain.ProjectRoleOwner, ""); err != nil {
		return apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_WOULD_BE_OWNERLESS", err.Error(), err)
	}

	if err := uc.repo.RemoveMember(ctx, in.ProjectID, in.UserID); err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_REMOVE_MEMBER_FAILED", "failed to remove project member", err)
	}
	return nil
}
