package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestAddTeamMember_RequiresTenantContext(t *testing.T) {
	uc := NewAddTeamMember(newFakeTeamRepository(), newFakeProfileCache(), nil)
	_, err := uc.Execute(context.Background(), AddTeamMemberInput{TeamID: "team-1", UserID: "user-1", Priority: 1})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestAddTeamMember_TeamFromAnotherCompanyIsNotFound(t *testing.T) {
	teams := newFakeTeamRepository()
	_, _ = teams.Create(context.Background(), mustTeam(t, "team-1", "other-company", "Platform", nil))

	uc := NewAddTeamMember(teams, newFakeProfileCache(), nil)
	ctx := withTenant(context.Background(), "company-1")

	_, err := uc.Execute(ctx, AddTeamMemberInput{TeamID: "team-1", UserID: "user-1", Priority: 1})
	if err == nil {
		t.Fatal("expected a cross-tenant team_id to resolve as not-found, per tenant-service.md §9")
	}
}

func TestAddTeamMember_AddsMemberAndInvalidatesCache(t *testing.T) {
	teams := newFakeTeamRepository()
	_, _ = teams.Create(context.Background(), mustTeam(t, "team-1", "company-1", "Platform", nil))
	cache := newFakeProfileCache()
	cache.byUserID["user-1"] = domain.ResolvedProfile{Settings: domain.Settings{"stale": true}}

	uc := NewAddTeamMember(teams, cache, nil)
	ctx := withTenant(context.Background(), "company-1")

	got, err := uc.Execute(ctx, AddTeamMemberInput{TeamID: "team-1", UserID: "user-1", Priority: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Priority != 7 {
		t.Errorf("expected Priority=7, got %d", got.Priority)
	}

	members := teams.members["team-1"]
	if len(members) != 1 || members[0].UserID != "user-1" {
		t.Fatalf("expected 1 member for team-1, got %+v", members)
	}

	if _, cached := cache.byUserID["user-1"]; cached {
		t.Error("expected AddTeamMember to invalidate the affected user's cached profile")
	}
}

func TestAddTeamMember_BroadcastsInvalidationToOtherReplicas(t *testing.T) {
	teams := newFakeTeamRepository()
	_, _ = teams.Create(context.Background(), mustTeam(t, "team-1", "company-1", "Platform", nil))
	invalidation := newFakeCacheInvalidationPublisher()

	uc := NewAddTeamMember(teams, newFakeProfileCache(), invalidation)
	ctx := withTenant(context.Background(), "company-1")

	if _, err := uc.Execute(ctx, AddTeamMemberInput{TeamID: "team-1", UserID: "user-1", Priority: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(invalidation.calls) != 1 || invalidation.calls[0] != "user-1" {
		t.Errorf("expected exactly one PublishProfileInvalidated call for user-1, got %+v", invalidation.calls)
	}
}
