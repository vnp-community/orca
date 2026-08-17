package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestUpdateUserRole_DeniedWhenOPADecisionIsFalse(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "member@example.com", "pw", domain.RoleUser)

	opa := &fakeOPAClient{allow: false}
	uc := NewUpdateUserRole(users, &fakeAuditRepository{}, &fakeClock{now: time.Now()}, opa)
	ctx := withActor(context.Background(), "t1", "u1")
	if _, err := uc.Execute(ctx, "u1", domain.RoleAdmin); err == nil {
		t.Fatal("expected an error when OPA denies the actor")
	}
	if !opa.called {
		t.Error("expected OPAClient.Decision to be called")
	}
}

func TestUpdateUserRole_AllowedWhenOPADecisionIsTrue(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	seedActiveUser(t, users, fakeHasher{}, "u2", "t1", "member@example.com", "pw", domain.RoleUser)
	audit := &fakeAuditRepository{}

	opa := &fakeOPAClient{allow: true}
	uc := NewUpdateUserRole(users, audit, &fakeClock{now: time.Now()}, opa)
	ctx := withActor(context.Background(), "t1", "admin1")
	updated, err := uc.Execute(ctx, "u2", domain.RoleAdmin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Role != domain.RoleAdmin {
		t.Errorf("expected role to be updated to admin, got %v", updated.Role)
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "user.role_updated" {
		t.Errorf("expected one user.role_updated audit entry, got %+v", audit.entries)
	}
}

func TestUpdateUserRole_FailsClosedOnOPAEvaluationError(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)

	opa := &fakeOPAClient{decisionErr: context.DeadlineExceeded}
	uc := NewUpdateUserRole(users, &fakeAuditRepository{}, &fakeClock{now: time.Now()}, opa)
	ctx := withActor(context.Background(), "t1", "admin1")
	if _, err := uc.Execute(ctx, "admin1", domain.RoleUser); err == nil {
		t.Fatal("expected an error when OPA evaluation itself fails")
	}
}
