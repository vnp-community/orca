package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestListTeamMembers_RequiresTenantContext(t *testing.T) {
	uc := NewListTeamMembers(newFakeTeamRepository())
	_, err := uc.Execute(context.Background(), ListTeamMembersInput{TeamID: "team-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListTeamMembers_TeamFromAnotherCompanyIsNotFound(t *testing.T) {
	teams := newFakeTeamRepository()
	_, _ = teams.Create(context.Background(), mustTeam(t, "team-1", "other-company", "Platform", nil))

	uc := NewListTeamMembers(teams)
	ctx := withTenant(context.Background(), "company-1")

	_, err := uc.Execute(ctx, ListTeamMembersInput{TeamID: "team-1"})
	if err == nil {
		t.Fatal("expected a cross-tenant team_id to resolve as not-found")
	}
}

func TestListTeamMembers_ReturnsMembers(t *testing.T) {
	teams := newFakeTeamRepository()
	_, _ = teams.Create(context.Background(), mustTeam(t, "team-1", "company-1", "Platform", nil))
	_ = teams.AddMember(context.Background(), domain.TeamMember{TeamID: "team-1", UserID: "user-1", Priority: 1})
	_ = teams.AddMember(context.Background(), domain.TeamMember{TeamID: "team-1", UserID: "user-2", Priority: 2})

	uc := NewListTeamMembers(teams)
	ctx := withTenant(context.Background(), "company-1")

	got, err := uc.Execute(ctx, ListTeamMembersInput{TeamID: "team-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 members, got %d", len(got))
	}
}
