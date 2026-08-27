package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// ListTeamMembersInput mirrors ListTeamMembersRequest 1:1.
type ListTeamMembersInput struct {
	TeamID string
}

// ListTeamMembers is a read path, scoped by the request context's tenant
// the same way AddTeamMember is — a team_id from another company must
// resolve as not-found (tenant-service.md §9).
type ListTeamMembers struct {
	teams TeamRepository
}

func NewListTeamMembers(teams TeamRepository) *ListTeamMembers {
	return &ListTeamMembers{teams: teams}
}

func (uc *ListTeamMembers) Execute(ctx context.Context, in ListTeamMembersInput) ([]domain.TeamMember, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}

	_, found, err := uc.teams.Get(ctx, companyID, in.TeamID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TENANT_TEAM_LOOKUP_FAILED", "failed to look up team", err)
	}
	if !found {
		return nil, apperrors.New(apperrors.KindNotFound, "TENANT_TEAM_NOT_FOUND", "team does not exist", nil)
	}

	members, err := uc.teams.ListMembers(ctx, in.TeamID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TENANT_LIST_TEAM_MEMBERS_FAILED", "failed to list team members", err)
	}
	return members, nil
}
