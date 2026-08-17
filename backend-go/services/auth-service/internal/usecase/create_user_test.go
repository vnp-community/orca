package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func withActor(ctx context.Context, tenantID, userID string) context.Context {
	ctx = tenant.WithTenantID(ctx, tenantID)
	return tenant.WithUserID(ctx, userID)
}

func TestCreateUser_RequiresAdminActor(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "member@example.com", "pw", domain.RoleUser)

	uc := NewCreateUser(users, &fakeAuditRepository{}, fakeHasher{}, &fakeClock{now: time.Now()})
	ctx := withActor(context.Background(), "t1", "u1")
	_, err := uc.Execute(ctx, CreateUserInput{Email: "new@example.com", Name: "New", TenantID: "t1", Role: domain.RoleUser})
	if err == nil {
		t.Fatal("expected an error when the actor is not an admin")
	}
}

func TestCreateUser_AdminCanCreateUser(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	audit := &fakeAuditRepository{}

	uc := NewCreateUser(users, audit, fakeHasher{}, &fakeClock{now: time.Now()})
	ctx := withActor(context.Background(), "t1", "admin1")
	created, err := uc.Execute(ctx, CreateUserInput{Email: "new@example.com", Name: "New", TenantID: "t1", Role: domain.RoleUser})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Email != "new@example.com" || created.TenantID != "t1" {
		t.Errorf("unexpected created user: %+v", created)
	}
	if _, _, err := users.GetUserByEmail(ctx, "new@example.com"); err != nil {
		t.Errorf("expected user to be persisted: %v", err)
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "user.created" {
		t.Errorf("expected one user.created audit entry, got %+v", audit.entries)
	}
}

func TestCreateUser_DuplicateEmailFails(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	seedActiveUser(t, users, fakeHasher{}, "u2", "t1", "existing@example.com", "pw", domain.RoleUser)

	uc := NewCreateUser(users, &fakeAuditRepository{}, fakeHasher{}, &fakeClock{now: time.Now()})
	ctx := withActor(context.Background(), "t1", "admin1")
	_, err := uc.Execute(ctx, CreateUserInput{Email: "existing@example.com", Name: "Dup", TenantID: "t1", Role: domain.RoleUser})
	if err == nil {
		t.Fatal("expected an error for a duplicate email")
	}
}

func TestCreateUser_RequiresAuthenticatedActor(t *testing.T) {
	uc := NewCreateUser(newFakeUserRepository(), &fakeAuditRepository{}, fakeHasher{}, &fakeClock{now: time.Now()})
	_, err := uc.Execute(context.Background(), CreateUserInput{Email: "new@example.com", TenantID: "t1"})
	if err == nil {
		t.Fatal("expected an error when no actor is in context")
	}
}
