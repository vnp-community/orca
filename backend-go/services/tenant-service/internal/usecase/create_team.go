package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// CreateTeamInput mirrors CreateTeamRequest 1:1. Settings is already parsed
// domain.Settings — the gRPC adapter owns the JSON boundary (settings_json.go),
// matching CreateCompany/CreateDepartment's convention of never letting
// encoding/json leak past that layer.
type CreateTeamInput struct {
	CompanyID string
	Name      string
	Settings  domain.Settings
}

// CreateTeam creates a Team under a Company. CompanyID comes from
// CreateTeamRequest's own bound field, same trust model as CreateDepartment.
type CreateTeam struct {
	companies CompanyRepository
	teams     TeamRepository
}

func NewCreateTeam(companies CompanyRepository, teams TeamRepository) *CreateTeam {
	return &CreateTeam{companies: companies, teams: teams}
}

func (uc *CreateTeam) Execute(ctx context.Context, in CreateTeamInput) (domain.Team, error) {
	exists, err := uc.companies.Exists(ctx, in.CompanyID)
	if err != nil {
		return domain.Team{}, apperrors.New(apperrors.KindInternal, "TENANT_COMPANY_LOOKUP_FAILED", "failed to check company existence", err)
	}
	if !exists {
		return domain.Team{}, apperrors.New(apperrors.KindNotFound, "TENANT_COMPANY_NOT_FOUND", "company does not exist", nil)
	}

	team, err := domain.NewTeam(uuid.NewString(), in.CompanyID, in.Name, in.Settings)
	if err != nil {
		return domain.Team{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_TEAM", err.Error(), err)
	}

	created, err := uc.teams.Create(ctx, team)
	if err != nil {
		return domain.Team{}, apperrors.New(apperrors.KindInternal, "TENANT_CREATE_TEAM_FAILED", "failed to persist team", err)
	}
	return created, nil
}
