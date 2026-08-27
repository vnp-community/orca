package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestRemoveTeamMember_RequiresTenantContext(t *testing.T) {
	uc := NewRemoveTeamMember(newFakeTeamRepository(), newFakeProfileCache(), nil)
	err := uc.Execute(context.Background(), RemoveTeamMemberInput{TeamID: "team-1", UserID: "user-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestRemoveTeamMember_TeamFromAnotherCompanyIsNotFound(t *testing.T) {
	teams := newFakeTeamRepository()
	_, _ = teams.Create(context.Background(), mustTeam(t, "team-1", "other-company", "Platform", nil))

	uc := NewRemoveTeamMember(teams, newFakeProfileCache(), nil)
	ctx := withTenant(context.Background(), "company-1")

	err := uc.Execute(ctx, RemoveTeamMemberInput{TeamID: "team-1", UserID: "user-1"})
	if err == nil {
		t.Fatal("expected a cross-tenant team_id to resolve as not-found, per tenant-service.md §9")
	}
}

func TestRemoveTeamMember_RemovesExistingMember_InvalidatesCache(t *testing.T) {
	teams := newFakeTeamRepository()
	_, _ = teams.Create(context.Background(), mustTeam(t, "team-1", "company-1", "Platform", nil))
	member, err := domain.NewTeamMember("team-1", "user-1", 3)
	if err != nil {
		t.Fatalf("building team member: %v", err)
	}
	_ = teams.AddMember(context.Background(), member)

	cache := newFakeProfileCache()
	cache.byUserID["user-1"] = domain.ResolvedProfile{}
	publisher := newFakeCacheInvalidationPublisher()

	uc := NewRemoveTeamMember(teams, cache, publisher)
	ctx := withTenant(context.Background(), "company-1")

	if err := uc.Execute(ctx, RemoveTeamMemberInput{TeamID: "team-1", UserID: "user-1"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	members, err := teams.ListMembers(context.Background(), "team-1")
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("expected member to be removed, got %+v", members)
	}
	if _, ok := cache.byUserID["user-1"]; ok {
		t.Fatal("expected cache entry for user-1 to be invalidated")
	}
	if len(publisher.calls) != 1 || publisher.calls[0] != "user-1" {
		t.Fatalf("expected cross-replica invalidation broadcast for user-1, got %v", publisher.calls)
	}
}

func TestRemoveTeamMember_NonMember_IsIdempotentNoOp(t *testing.T) {
	teams := newFakeTeamRepository()
	_, _ = teams.Create(context.Background(), mustTeam(t, "team-1", "company-1", "Platform", nil))

	uc := NewRemoveTeamMember(teams, newFakeProfileCache(), nil)
	ctx := withTenant(context.Background(), "company-1")

	if err := uc.Execute(ctx, RemoveTeamMemberInput{TeamID: "team-1", UserID: "not-a-member"}); err != nil {
		t.Fatalf("expected no error for a no-op removal, got: %v", err)
	}
}
