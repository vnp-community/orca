package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// AddTeamMemberInput mirrors AddTeamMemberRequest 1:1.
type AddTeamMemberInput struct {
	TeamID   string
	UserID   string
	Priority int32
}

// AddTeamMember upserts one TeamMember row (priority — tenant.proto's
// AddTeamMemberRequest doesn't carry a role field in this reduced surface;
// see README "Known gaps"). AddTeamMemberRequest has no company_id bound
// field, so the scoping company comes from the request context, same as
// SetUserDepartment.
type AddTeamMember struct {
	teams TeamRepository
	cache ProfileCache
}

func NewAddTeamMember(teams TeamRepository, cache ProfileCache) *AddTeamMember {
	return &AddTeamMember{teams: teams, cache: cache}
}

func (uc *AddTeamMember) Execute(ctx context.Context, in AddTeamMemberInput) (domain.TeamMember, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.TeamMember{}, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}

	_, found, err := uc.teams.Get(ctx, companyID, in.TeamID)
	if err != nil {
		return domain.TeamMember{}, apperrors.New(apperrors.KindInternal, "TENANT_TEAM_LOOKUP_FAILED", "failed to look up team", err)
	}
	if !found {
		return domain.TeamMember{}, apperrors.New(apperrors.KindNotFound, "TENANT_TEAM_NOT_FOUND", "team does not exist", nil)
	}

	member, err := domain.NewTeamMember(in.TeamID, in.UserID, in.Priority)
	if err != nil {
		return domain.TeamMember{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_TEAM_MEMBER", err.Error(), err)
	}
	if err := uc.teams.AddMember(ctx, member); err != nil {
		return domain.TeamMember{}, apperrors.New(apperrors.KindInternal, "TENANT_ADD_TEAM_MEMBER_FAILED", "failed to persist team member", err)
	}

	// Invalidation correctness (tenant-service.md §8): a team-membership
	// change affects exactly the added member's team layer.
	if uc.cache != nil {
		uc.cache.Invalidate(ctx, in.UserID)
	}
	return member, nil
}
