package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func withActor(ctx context.Context, tenantID, userID string) context.Context {
	ctx = tenant.WithTenantID(ctx, tenantID)
	return tenant.WithUserID(ctx, userID)
}

func TestCreateUser_DeniedWhenOPADecisionIsFalse(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "member@example.com", "pw", domain.RoleUser)

	opa := &fakeOPAClient{allow: false}
	uc := NewCreateUser(users, &fakeAuditRepository{}, fakeHasher{}, &fakeClock{now: time.Now()}, opa)
	ctx := withActor(context.Background(), "t1", "u1")
	_, err := uc.Execute(ctx, CreateUserInput{Email: "new@example.com", Name: "New", TenantID: "t1", Password: "initial-pw", Role: domain.RoleUser})
	if err == nil {
		t.Fatal("expected an error when OPA denies the actor")
	}
	if !opa.called {
		t.Error("expected OPAClient.Decision to be called")
	}
}

func TestCreateUser_AllowedWhenOPADecisionIsTrue(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	audit := &fakeAuditRepository{}

	opa := &fakeOPAClient{allow: true}
	hasher := fakeHasher{}
	uc := NewCreateUser(users, audit, hasher, &fakeClock{now: time.Now()}, opa)
	ctx := withActor(context.Background(), "t1", "admin1")
	created, err := uc.Execute(ctx, CreateUserInput{Email: "new@example.com", Name: "New", TenantID: "t1", Password: "admin-chosen-pw", Role: domain.RoleUser})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Email != "new@example.com" || created.TenantID != "t1" {
		t.Errorf("unexpected created user: %+v", created)
	}
	_, storedHash, err := users.GetUserByEmail(ctx, "new@example.com")
	if err != nil {
		t.Errorf("expected user to be persisted: %v", err)
	}
	// The exact admin-supplied plaintext must verify against the stored
	// hash — the prior behavior generated-and-discarded a random password,
	// leaving the account permanently unusable.
	if err := hasher.Compare(storedHash, "admin-chosen-pw"); err != nil {
		t.Errorf("expected the admin-supplied password to verify against the stored hash: %v", err)
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "user.created" {
		t.Errorf("expected one user.created audit entry, got %+v", audit.entries)
	}
	if opa.lastActor.ID != "admin1" || opa.lastActor.Role != domain.RoleAdmin {
		t.Errorf("expected OPA to be queried with the resolved admin actor, got %+v", opa.lastActor)
	}
}

func TestCreateUser_WeakPasswordFailsBeforeAnyWrite(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)

	uc := NewCreateUser(users, &fakeAuditRepository{}, fakeHasher{}, &fakeClock{now: time.Now()}, &fakeOPAClient{allow: true})
	ctx := withActor(context.Background(), "t1", "admin1")
	_, err := uc.Execute(ctx, CreateUserInput{Email: "new@example.com", Name: "New", TenantID: "t1", Password: "short", Role: domain.RoleUser})
	if err == nil {
		t.Fatal("expected an error for a too-short password")
	}
	if _, _, err := users.GetUserByEmail(ctx, "new@example.com"); err == nil {
		t.Error("expected no user to have been created for a weak password")
	}
}

func TestCreateUser_FailsClosedOnOPAEvaluationError(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)

	opa := &fakeOPAClient{decisionErr: errors.New("bundle unavailable")}
	uc := NewCreateUser(users, &fakeAuditRepository{}, fakeHasher{}, &fakeClock{now: time.Now()}, opa)
	ctx := withActor(context.Background(), "t1", "admin1")
	_, err := uc.Execute(ctx, CreateUserInput{Email: "new@example.com", Name: "New", TenantID: "t1", Role: domain.RoleUser})
	if err == nil {
		t.Fatal("expected an error when OPA evaluation itself fails")
	}
}

func TestCreateUser_DuplicateEmailFails(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	seedActiveUser(t, users, fakeHasher{}, "u2", "t1", "existing@example.com", "pw", domain.RoleUser)

	uc := NewCreateUser(users, &fakeAuditRepository{}, fakeHasher{}, &fakeClock{now: time.Now()}, &fakeOPAClient{allow: true})
	ctx := withActor(context.Background(), "t1", "admin1")
	_, err := uc.Execute(ctx, CreateUserInput{Email: "existing@example.com", Name: "Dup", TenantID: "t1", Password: "initial-pw", Role: domain.RoleUser})
	if err == nil {
		t.Fatal("expected an error for a duplicate email")
	}
}

func TestCreateUser_RequiresAuthenticatedActor(t *testing.T) {
	uc := NewCreateUser(newFakeUserRepository(), &fakeAuditRepository{}, fakeHasher{}, &fakeClock{now: time.Now()}, &fakeOPAClient{allow: true})
	_, err := uc.Execute(context.Background(), CreateUserInput{Email: "new@example.com", TenantID: "t1"})
	if err == nil {
		t.Fatal("expected an error when no actor is in context")
	}
}
