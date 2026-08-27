package usecase

import (
	"context"
	"testing"
)

func TestListTeams_RequiresTenantContext(t *testing.T) {
	uc := NewListTeams(newFakeTeamRepository())
	if _, err := uc.Execute(context.Background()); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListTeams_ScopesByCompany(t *testing.T) {
	teams := newFakeTeamRepository()
	_, _ = teams.Create(context.Background(), mustTeam(t, "team-1", "company-a", "Platform", nil))
	_, _ = teams.Create(context.Background(), mustTeam(t, "team-2", "company-a", "Growth", nil))
	_, _ = teams.Create(context.Background(), mustTeam(t, "team-3", "company-b", "Other", nil))

	uc := NewListTeams(teams)
	ctx := withTenant(context.Background(), "company-a")

	got, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 teams for company-a, got %d", len(got))
	}
	for _, tm := range got {
		if tm.CompanyID != "company-a" {
			t.Fatalf("cross-company leak: got team %+v while scoped to company-a", tm)
		}
	}
}
