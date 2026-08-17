package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestListUsers_DeniedWhenOPADecisionIsFalse(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "member@example.com", "pw", domain.RoleUser)

	opa := &fakeOPAClient{allow: false}
	uc := NewListUsers(users, opa)
	ctx := withActor(context.Background(), "t1", "u1")
	if _, err := uc.Execute(ctx, ListUsersInput{TenantID: "t1"}); err == nil {
		t.Fatal("expected an error when OPA denies the actor")
	}
	if !opa.called {
		t.Error("expected OPAClient.Decision to be called")
	}
}

func TestListUsers_AllowedWhenOPADecisionIsTrue(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	seedActiveUser(t, users, fakeHasher{}, "u2", "t1", "member@example.com", "pw", domain.RoleUser)

	opa := &fakeOPAClient{allow: true}
	uc := NewListUsers(users, opa)
	ctx := withActor(context.Background(), "t1", "admin1")
	out, err := uc.Execute(ctx, ListUsersInput{TenantID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(out.Users))
	}
}

func TestListUsers_FailsClosedOnOPAEvaluationError(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)

	opa := &fakeOPAClient{decisionErr: context.DeadlineExceeded}
	uc := NewListUsers(users, opa)
	ctx := withActor(context.Background(), "t1", "admin1")
	if _, err := uc.Execute(ctx, ListUsersInput{TenantID: "t1"}); err == nil {
		t.Fatal("expected an error when OPA evaluation itself fails")
	}
}
