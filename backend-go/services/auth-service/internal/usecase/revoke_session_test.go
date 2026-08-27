package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestRevokeSession_DeniedWhenOPADecisionIsFalse(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "member@example.com", "pw", domain.RoleUser)

	opa := &fakeOPAClient{allow: false}
	uc := NewRevokeSession(users, newFakeSessionRepository(), &fakeAuditRepository{}, &fakeClock{now: time.Now()}, opa)
	ctx := withActor(context.Background(), "t1", "u1")
	if err := uc.Execute(ctx, "sometoken"); err == nil {
		t.Fatal("expected an error when OPA denies the actor")
	}
	if !opa.called {
		t.Error("expected OPAClient.Decision to be called")
	}
}

func TestRevokeSession_AllowedWhenOPADecisionIsTrue(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	sessions := newFakeSessionRepository()
	rawToken := "session-token"
	if err := sessions.CreateSession(context.Background(), domain.Session{
		TokenHash: domain.HashSessionToken(rawToken),
		TenantID:  "t1",
		UserID:    "u2",
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("seeding session: %v", err)
	}
	audit := &fakeAuditRepository{}

	opa := &fakeOPAClient{allow: true}
	uc := NewRevokeSession(users, sessions, audit, &fakeClock{now: time.Now()}, opa)
	ctx := tenant.WithUserID(context.Background(), "admin1")
	if err := uc.Execute(ctx, rawToken); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "session.revoked" {
		t.Errorf("expected one session.revoked audit entry, got %+v", audit.entries)
	}
}

func TestRevokeSession_FailsClosedOnOPAEvaluationError(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)

	opa := &fakeOPAClient{decisionErr: context.DeadlineExceeded}
	uc := NewRevokeSession(users, newFakeSessionRepository(), &fakeAuditRepository{}, &fakeClock{now: time.Now()}, opa)
	ctx := tenant.WithUserID(context.Background(), "admin1")
	if err := uc.Execute(ctx, "sometoken"); err == nil {
		t.Fatal("expected an error when OPA evaluation itself fails")
	}
}
