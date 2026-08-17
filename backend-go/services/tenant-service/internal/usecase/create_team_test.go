package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestCreateTeam_RequiresExistingCompany(t *testing.T) {
	uc := NewCreateTeam(newFakeCompanyRepository(), newFakeTeamRepository())
	_, err := uc.Execute(context.Background(), CreateTeamInput{CompanyID: "missing", Name: "Platform"})
	if err == nil {
		t.Fatal("expected an error when the company doesn't exist")
	}
}

func TestCreateTeam_PersistsUnderCompany(t *testing.T) {
	companies := newFakeCompanyRepository()
	_, _ = companies.Create(context.Background(), mustCompany(t, "company-1", "Acme", nil))
	teams := newFakeTeamRepository()

	uc := NewCreateTeam(companies, teams)
	got, err := uc.Execute(context.Background(), CreateTeamInput{CompanyID: "company-1", Name: "Platform"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CompanyID != "company-1" || got.Name != "Platform" {
		t.Errorf("unexpected team: %+v", got)
	}
}

func TestCreateTeam_PersistsSettings(t *testing.T) {
	companies := newFakeCompanyRepository()
	_, _ = companies.Create(context.Background(), mustCompany(t, "company-1", "Acme", nil))
	teams := newFakeTeamRepository()

	uc := NewCreateTeam(companies, teams)
	settings := domain.Settings{"shell": map[string]any{"pathAdditions": []any{"/opt/team-tools"}}}
	got, err := uc.Execute(context.Background(), CreateTeamInput{CompanyID: "company-1", Name: "Platform", Settings: settings})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Settings) == 0 {
		t.Fatalf("expected team settings to be persisted, got empty: %+v", got)
	}

	stored, found, err := teams.Get(context.Background(), "company-1", got.ID)
	if err != nil || !found {
		t.Fatalf("expected to find the created team, found=%v err=%v", found, err)
	}
	if len(stored.Settings) == 0 {
		t.Errorf("expected repository-stored team to carry settings, got empty: %+v", stored)
	}
}
