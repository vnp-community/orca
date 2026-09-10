package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// ListTeamsForUserInput mirrors ListTeamsForUserRequest 1:1.
type ListTeamsForUserInput struct {
	UserID string
}

// ListTeamsForUser answers the user->teams direction TeamScopeResolver
// (task-service) needs, and is also the RPC devServer.listForUser's handler
// doc comment names as missing (channels_dev_server_access_control.go:
// 279-284, BUG-013) — tenant-service's existing RPC surface only offers
// company->teams (ListTeams) and team->members (ListTeamMembers), which
// would force an N+1 fan-out to answer this from the client side. Thin:
// reuses TeamRepository.ListUserTeamLayers, the same indexed
// (tenant.team_members.user_id) query GetResolvedProfile already runs for
// its team settings-layer — no new repository method (see TASK-TG-03-02's
// Context).
type ListTeamsForUser struct {
	teams TeamRepository
}

func NewListTeamsForUser(teams TeamRepository) *ListTeamsForUser {
	return &ListTeamsForUser{teams: teams}
}

func (uc *ListTeamsForUser) Execute(ctx context.Context, in ListTeamsForUserInput) ([]string, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}

	layers, err := uc.teams.ListUserTeamLayers(ctx, companyID, in.UserID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TENANT_LIST_TEAMS_FOR_USER_FAILED", "failed to list teams for user", err)
	}

	teamIDs := make([]string, 0, len(layers))
	for _, layer := range layers {
		teamIDs = append(teamIDs, layer.TeamID)
	}
	return teamIDs, nil
}
