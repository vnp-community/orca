package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestListTenantMemberDirectory_ReturnsMinimalProjectionForAnyAuthenticatedActor(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "member@example.com", "pw", domain.RoleUser)
	seedActiveUser(t, users, fakeHasher{}, "u2", "t1", "other@example.com", "pw", domain.RoleUser)

	uc := NewListTenantMemberDirectory(users)
	// Why a plain non-admin member, not seedActiveUser("admin1", ..., domain.RoleAdmin):
	// this is the whole point of this usecase — no admin check, unlike ListUsers.
	ctx := withActor(context.Background(), "t1", "u1")

	entries, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error for a non-admin actor: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.ID == "" || e.Name == "" || e.Email == "" {
			t.Errorf("expected id/name/email all populated, got %+v", e)
		}
	}
}

func TestListTenantMemberDirectory_ScopedToTheActorsOwnTenant(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "member@t1.example.com", "pw", domain.RoleUser)
	seedActiveUser(t, users, fakeHasher{}, "u2", "t2", "member@t2.example.com", "pw", domain.RoleUser)

	uc := NewListTenantMemberDirectory(users)
	ctx := withActor(context.Background(), "t1", "u1")

	entries, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "u1" {
		t.Fatalf("expected only t1's own member, got %+v", entries)
	}
}

func TestListTenantMemberDirectory_RequiresAnAuthenticatedActor(t *testing.T) {
	users := newFakeUserRepository()
	uc := NewListTenantMemberDirectory(users)

	if _, err := uc.Execute(context.Background()); err == nil {
		t.Fatal("expected an error when the context has no tenant/actor")
	}
}
