package usecase

import (
	"context"
	"testing"

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
	audit := &fakeAuditRepository{entries: []domain.AuditEntry{
		{ID: "e1", TenantID: "t1", ActorID: "admin1", Action: "user.created", Target: "u2"},
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
