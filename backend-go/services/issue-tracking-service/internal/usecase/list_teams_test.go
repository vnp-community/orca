package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

func TestListTeams_AlwaysResolvesLinearProvider(t *testing.T) {
	var gotProvider domain.Provider
	registry := &fakeProviderRegistry{
		resolveFunc: func(p domain.Provider) (IssueTrackerProvider, error) {
			gotProvider = p
			return &fakeIssueTrackerProvider{listTeamsReturn: []domain.Team{{ID: "team-1", Name: "Engineering", Key: "ENG"}}}, nil
		},
	}
	credentials := &fakeCredentialResolver{}
	uc := NewListTeams(registry, credentials)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	teams, err := uc.Execute(ctx, ListTeamsInput{WorkspaceID: "ws-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotProvider != domain.ProviderLinear {
		t.Errorf("want ProviderLinear, got %v — ListTeams must never resolve Jira", gotProvider)
	}
	if len(teams) != 1 || teams[0].Key != "ENG" {
		t.Errorf("unexpected teams: %+v", teams)
	}
}

func TestGetCustomView_NotFound_ReturnsError(t *testing.T) {
	registry := &fakeProviderRegistry{provider: domain.ProviderLinear, impl: &fakeIssueTrackerProvider{getCustomViewErr: ErrConnectionNotFound}}
	credentials := &fakeCredentialResolver{}
	uc := NewGetCustomView(registry, credentials)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	_, err := uc.Execute(ctx, GetCustomViewInput{ViewID: "missing", Model: "issue"})
	if err == nil {
		t.Fatal("expected error")
	}
}
