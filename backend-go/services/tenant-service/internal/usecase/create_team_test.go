package usecase

import (
	"context"
	"testing"
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
