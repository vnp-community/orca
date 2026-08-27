package usecase

import "context"

// ListTeamsForUser answers the user->teams direction TeamScopeResolver
// (task-service) needs — tenant-service's existing RPC surface only offers
// company->teams (ListTeams) and team->members (ListTeamMembers), which
// would force an N+1 fan-out to answer this from the client side. Reuses
// TeamRepository.ListUserTeamLayers (built for ResolveProfile's team-layer
// fetch) rather than adding a new query — see TASK-TG-03-02's Context.
type ListTeamsForUser struct {
	teams TeamRepository
}

func NewListTeamsForUser(teams TeamRepository) *ListTeamsForUser {
	return &ListTeamsForUser{teams: teams}
}

func (uc *ListTeamsForUser) Execute(ctx context.Context, companyID, userID string) ([]string, error) {
	if userID == "" {
		return nil, nil
	}
	layers, err := uc.teams.ListUserTeamLayers(ctx, companyID, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(layers))
	for _, l := range layers {
		ids = append(ids, l.TeamID)
	}
	return ids, nil
}
