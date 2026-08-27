package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type UpdateMemberRoleInput struct {
	ProjectID string
	UserID    string
	Role      domain.ProjectRole
}

// UpdateMemberRole requires owner (or global admin), same tier as
// RemoveMember, and enforces the "≥1 owner" invariant against the NEW role
// before mutating (a demotion away from owner is the only way this can
// trip; a promotion or a non-owner's role change never can).
type UpdateMemberRole struct {
	repo ProjectRepository
	opa  OPAClient
}

func NewUpdateMemberRole(repo ProjectRepository, opa OPAClient) *UpdateMemberRole {
	return &UpdateMemberRole{repo: repo, opa: opa}
}

func (uc *UpdateMemberRole) Execute(ctx context.Context, in UpdateMemberRoleInput) (domain.ProjectMember, error) {
	if err := requireProjectAccess(ctx, uc.repo, uc.opa, in.ProjectID, projectActionOwnerOnly); err != nil {
		return domain.ProjectMember{}, err
	}
	if !in.Role.Valid() {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_ROLE", "invalid project role", nil)
	}

	target, err := uc.repo.GetMembership(ctx, in.ProjectID, in.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return domain.ProjectMember{}, apperrors.New(apperrors.KindNotFound, "PROJECT_MEMBERSHIP_NOT_FOUND", "membership does not exist", err)
		}
		return domain.ProjectMember{}, apperrors.New(apperrors.KindInternal, "PROJECT_MEMBERSHIP_LOOKUP_FAILED", "failed to look up membership", err)
	}

	owners, err := uc.repo.CountOwners(ctx, in.ProjectID)
	if err != nil {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindInternal, "PROJECT_COUNT_OWNERS_FAILED", "failed to count project owners", err)
	}
	if err := domain.AssertNotLastOwnerRemoval(owners, target.Role == domain.ProjectRoleOwner, in.Role); err != nil {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_WOULD_BE_OWNERLESS", err.Error(), err)
	}

	member, err := uc.repo.UpdateMemberRole(ctx, in.ProjectID, in.UserID, in.Role)
	if err != nil {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindInternal, "PROJECT_UPDATE_MEMBER_ROLE_FAILED", "failed to update member role", err)
	}
	return member, nil
}
