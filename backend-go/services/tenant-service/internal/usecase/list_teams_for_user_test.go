package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestListTeamsForUser_ReturnsTeamIDsForAMultiTeamUser(t *testing.T) {
	teams := newFakeTeamRepository()
	teamA := mustTeam(t, "team-a", "company-1", "A", domain.Settings{})
	teamB := mustTeam(t, "team-b", "company-1", "B", domain.Settings{})
	_, _ = teams.Create(context.Background(), teamA)
	_, _ = teams.Create(context.Background(), teamB)
	_ = teams.AddMember(context.Background(), domain.TeamMember{TeamID: teamA.ID, UserID: "user-1", Priority: 1})
	_ = teams.AddMember(context.Background(), domain.TeamMember{TeamID: teamB.ID, UserID: "user-1", Priority: 2})

	uc := NewListTeamsForUser(teams)
	got, err := uc.Execute(context.Background(), "company-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 team IDs, got %+v", got)
	}
	want := map[string]bool{"team-a": true, "team-b": true}
	for _, id := range got {
		if !want[id] {
			t.Errorf("unexpected team id %q", id)
		}
	}
}

func TestListTeamsForUser_EmptyUserID_ReturnsNilNoError(t *testing.T) {
	uc := NewListTeamsForUser(newFakeTeamRepository())
	got, err := uc.Execute(context.Background(), "company-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for an empty user_id, got %+v", got)
	}
}

func TestListTeamsForUser_NoTeams_ReturnsEmpty(t *testing.T) {
	uc := NewListTeamsForUser(newFakeTeamRepository())
	got, err := uc.Execute(context.Background(), "company-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no teams, got %+v", got)
	}
}

func TestListTeamsForUser_RepositoryFailurePropagates(t *testing.T) {
	teams := newFakeTeamRepository()
	teams.layersErr = errors.New("db unavailable")
	uc := NewListTeamsForUser(teams)

	if _, err := uc.Execute(context.Background(), "company-1", "user-1"); err == nil {
		t.Fatal("expected an error to propagate from the repository")
	}
}
