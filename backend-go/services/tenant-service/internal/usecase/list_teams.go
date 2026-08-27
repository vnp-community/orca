package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// ListTeams lists every team in the caller's company — the missing read
// half of team.* CRUD (create/get exist; list never did). See
// tenant-service.md §3.
type ListTeams struct {
	teams TeamRepository
}

func NewListTeams(teams TeamRepository) *ListTeams { return &ListTeams{teams: teams} }

func (uc *ListTeams) Execute(ctx context.Context) ([]domain.Team, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}
	teams, err := uc.teams.ListByCompany(ctx, companyID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TENANT_LIST_TEAMS_FAILED", "failed to list teams", err)
	}
	return teams, nil
}
