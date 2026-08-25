package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestDeactivateUser_ThenReactivateUser_RoundTripsIsActive(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	seedActiveUser(t, users, fakeHasher{}, "u2", "t1", "member@example.com", "pw", domain.RoleUser)

	opa := &fakeOPAClient{allow: true}
	audit := &fakeAuditRepository{}
	clock := &fakeClock{now: time.Now()}
	ctx := withActor(context.Background(), "t1", "admin1")

	deactivate := NewDeactivateUser(users, audit, clock, opa)
	deactivated, err := deactivate.Execute(ctx, "u2")
	if err != nil {
		t.Fatalf("DeactivateUser: unexpected error: %v", err)
	}
	if deactivated.IsActive {
		t.Fatal("expected user to be inactive after DeactivateUser")
	}
	stored, err := users.GetUserByID(ctx, "u2")
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if stored.IsActive {
		t.Fatal("expected persisted user to be inactive")
	}

	reactivate := NewReactivateUser(users, audit, clock, opa)
	reactivated, err := reactivate.Execute(ctx, "u2")
	if err != nil {
		t.Fatalf("ReactivateUser: unexpected error: %v", err)
	}
	if !reactivated.IsActive {
		t.Fatal("expected user to be active after ReactivateUser")
	}

	if len(audit.entries) != 2 {
		t.Fatalf("expected 2 audit entries, got %d: %+v", len(audit.entries), audit.entries)
	}
	if audit.entries[0].Action != "user.deactivated" || audit.entries[1].Action != "user.reactivated" {
		t.Errorf("unexpected audit actions: %+v", audit.entries)
	}
}

func TestDeactivateUser_DeniedWhenOPADecisionIsFalse(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "member@example.com", "pw", domain.RoleUser)

	opa := &fakeOPAClient{allow: false}
	uc := NewDeactivateUser(users, &fakeAuditRepository{}, &fakeClock{now: time.Now()}, opa)
	ctx := withActor(context.Background(), "t1", "u1")
	if _, err := uc.Execute(ctx, "u1"); err == nil {
		t.Fatal("expected an error when OPA denies the actor")
	}
}

func TestDeactivateUser_NotFoundUserFails(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)

	uc := NewDeactivateUser(users, &fakeAuditRepository{}, &fakeClock{now: time.Now()}, &fakeOPAClient{allow: true})
	ctx := withActor(context.Background(), "t1", "admin1")
	if _, err := uc.Execute(ctx, "does-not-exist"); err == nil {
		t.Fatal("expected an error for a nonexistent user")
	}
}

func TestForceRevokeAllSessionsForUser_RevokesOnlyThatUsersUnrevokedSessions(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	sessions := newFakeSessionRepository()
	now := time.Now()
	_ = sessions.CreateSession(context.Background(), domain.Session{TokenHash: "h1", UserID: "u2", TenantID: "t1", CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
	_ = sessions.CreateSession(context.Background(), domain.Session{TokenHash: "h2", UserID: "u2", TenantID: "t1", CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
	_ = sessions.CreateSession(context.Background(), domain.Session{TokenHash: "h3", UserID: "other-user", TenantID: "t1", CreatedAt: now, ExpiresAt: now.Add(time.Hour)})

	uc := NewForceRevokeAllSessionsForUser(users, sessions, &fakeAuditRepository{}, &fakeClock{now: now}, &fakeOPAClient{allow: true})
	ctx := withActor(context.Background(), "t1", "admin1")
	revoked, err := uc.Execute(ctx, "u2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if revoked != 2 {
		t.Fatalf("revoked = %d, want 2", revoked)
	}
	other, err := sessions.GetSessionByTokenHash(ctx, "h3")
	if err != nil {
		t.Fatalf("GetSessionByTokenHash(h3): %v", err)
	}
	if other.RevokedAt != nil {
		t.Fatal("expected other user's session to remain unrevoked")
	}
}

func TestListSessionsForUser_ReturnsOnlyThatUsersSessions(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	sessions := newFakeSessionRepository()
	now := time.Now()
	_ = sessions.CreateSession(context.Background(), domain.Session{TokenHash: "h1", UserID: "u2", TenantID: "t1", CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
	_ = sessions.CreateSession(context.Background(), domain.Session{TokenHash: "h2", UserID: "other-user", TenantID: "t1", CreatedAt: now, ExpiresAt: now.Add(time.Hour)})

	uc := NewListSessionsForUser(users, sessions, &fakeOPAClient{allow: true})
	ctx := withActor(context.Background(), "t1", "admin1")
	got, err := uc.Execute(ctx, "u2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].TokenHash != "h1" {
		t.Fatalf("unexpected sessions: %+v", got)
	}
}
