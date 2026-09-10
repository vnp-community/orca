package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestListSessions_ScopesToActorTenantNotRequestTenant(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "actor-tenant", "admin@example.com", "pw", domain.RoleAdmin)

	opa := &fakeOPAClient{allow: true}
	uc := NewListSessions(users, sessions, opa)
	ctx := withActor(context.Background(), "actor-tenant", "admin1")

	// The request carries a different tenant_id than the actor's own — this
	// must be ignored; ListForTenant must be called with the actor's
	// tenant, never a caller-supplied one (07-security-architecture.md
	// multi-tenancy isolation layer 2). ListSessionsInput has no TenantID
	// field, so there's nothing to pass here — that's the point.
	if _, err := uc.Execute(ctx, ListSessionsInput{PageSize: 50}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessions.lastTenantID != "actor-tenant" {
		t.Errorf("expected ListForTenant to be called with the actor's tenant, got %q", sessions.lastTenantID)
	}
}

func TestListSessions_DeniedForNonAdminActor(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "member@example.com", "pw", domain.RoleUser)

	opa := &fakeOPAClient{allow: false}
	uc := NewListSessions(users, sessions, opa)
	ctx := withActor(context.Background(), "t1", "u1")

	_, err := uc.Execute(ctx, ListSessionsInput{})
	if err == nil {
		t.Fatal("expected an error for a non-admin actor")
	}
	if sessions.lastTenantID != "" {
		t.Error("expected ListForTenant never to be called when the actor is denied")
	}
}

func TestListSessions_ReturnsSessionsWithUserEmail(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	sessions.userEmails = map[string]string{"u2": "member@example.com"}
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)

	now := time.Now()
	session, err := domain.NewSession(domain.HashSessionToken("raw-token"), "u2", "t1", now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building session: %v", err)
	}
	_ = sessions.CreateSession(context.Background(), session)

	opa := &fakeOPAClient{allow: true}
	uc := NewListSessions(users, sessions, opa)
	ctx := withActor(context.Background(), "t1", "admin1")

	out, err := uc.Execute(ctx, ListSessionsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(out.Sessions))
	}
	if out.Sessions[0].UserEmail != "member@example.com" {
		t.Errorf("expected the joined user email, got %q", out.Sessions[0].UserEmail)
	}
}
