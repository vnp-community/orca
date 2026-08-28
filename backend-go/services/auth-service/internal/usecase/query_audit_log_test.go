package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestQueryAuditLog_DeniedWhenOPADecisionIsFalse(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "member@example.com", "pw", domain.RoleUser)

	opa := &fakeOPAClient{allow: false}
	uc := NewQueryAuditLog(users, &fakeAuditRepository{}, opa)
	ctx := withActor(context.Background(), "t1", "u1")
	if _, err := uc.Execute(ctx, QueryAuditLogInput{TenantID: "t1"}); err == nil {
		t.Fatal("expected an error when OPA denies the actor")
	}
	if !opa.called {
		t.Error("expected OPAClient.Decision to be called")
	}
}

func TestQueryAuditLog_AllowedWhenOPADecisionIsTrue(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	now := time.Now()
	audit := &fakeAuditRepository{entries: []domain.AuditEntry{
		{ID: "e1", TenantID: "t1", ActorID: "admin1", Action: "user.created", TargetType: "user", TargetID: "u2", OccurredAt: now},
	}}

	opa := &fakeOPAClient{allow: true}
	uc := NewQueryAuditLog(users, audit, opa)
	ctx := withActor(context.Background(), "t1", "admin1")
	out, err := uc.Execute(ctx, QueryAuditLogInput{TenantID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Entries) != 1 {
		t.Errorf("expected 1 audit entry, got %d", len(out.Entries))
	}
}

func TestQueryAuditLog_ForwardsExtendedFilters(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)

	now := time.Now()
	audit := &fakeAuditRepository{entries: []domain.AuditEntry{
		{ID: "e1", TenantID: "t1", ActorID: "admin1", Action: "user.created", TargetType: "user", TargetID: "u2", OccurredAt: now},
		{ID: "e2", TenantID: "t1", ActorID: "admin1", Action: "user.deactivated", TargetType: "user", TargetID: "u3", OccurredAt: now},
		{ID: "e3", TenantID: "t1", ActorID: "other-admin", Action: "user.created", TargetType: "user", TargetID: "u4", OccurredAt: now},
	}}

	opa := &fakeOPAClient{allow: true}
	uc := NewQueryAuditLog(users, audit, opa)
	ctx := withActor(context.Background(), "t1", "admin1")

	out, err := uc.Execute(ctx, QueryAuditLogInput{TenantID: "t1", Action: "user.created", ActorID: "admin1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Entries) != 1 || out.Entries[0].ID != "e1" {
		t.Errorf("expected only e1 to match action+actor_id filters, got %+v", out.Entries)
	}

	// `to` in the past excludes every entry (all seeded at `now`).
	out, err = uc.Execute(ctx, QueryAuditLogInput{TenantID: "t1", To: now.Add(-time.Hour)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Entries) != 0 {
		t.Errorf("expected no entries past the `to` bound, got %+v", out.Entries)
	}
}

func TestQueryAuditLog_FailsClosedOnOPAEvaluationError(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)

	opa := &fakeOPAClient{decisionErr: context.DeadlineExceeded}
	uc := NewQueryAuditLog(users, &fakeAuditRepository{}, opa)
	ctx := withActor(context.Background(), "t1", "admin1")
	if _, err := uc.Execute(ctx, QueryAuditLogInput{TenantID: "t1"}); err == nil {
		t.Fatal("expected an error when OPA evaluation itself fails")
	}
}
