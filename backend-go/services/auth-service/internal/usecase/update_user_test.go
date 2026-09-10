package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestUpdateUser_PartialUpdateOnlyChangesSetFields(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	seedActiveUser(t, users, fakeHasher{}, "u2", "t1", "member@example.com", "pw", domain.RoleUser)

	audit := &fakeAuditRepository{}
	clock := &fakeClock{now: time.Now()}
	uc := NewUpdateUser(users, audit, clock, &fakeOPAClient{allow: true})
	ctx := withActor(context.Background(), "t1", "admin1")

	newEmail := "member-new@example.com"
	updated, err := uc.Execute(ctx, UpdateUserInput{UserID: "u2", Email: &newEmail})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Email != newEmail {
		t.Errorf("expected email to be updated, got %q", updated.Email)
	}
	if updated.Name != "Test User" {
		t.Errorf("expected name to be left unchanged, got %q", updated.Name)
	}
	if updated.Role != domain.RoleUser {
		t.Errorf("expected role to be left unchanged, got %q", updated.Role)
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "user.updated" {
		t.Errorf("expected one user.updated audit entry, got %+v", audit.entries)
	}
}

func TestUpdateUser_InvalidEmailFailsBeforeAnyWrite(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	seedActiveUser(t, users, fakeHasher{}, "u2", "t1", "member@example.com", "pw", domain.RoleUser)

	uc := NewUpdateUser(users, &fakeAuditRepository{}, &fakeClock{now: time.Now()}, &fakeOPAClient{allow: true})
	ctx := withActor(context.Background(), "t1", "admin1")

	badEmail := "not-an-email"
	_, err := uc.Execute(ctx, UpdateUserInput{UserID: "u2", Email: &badEmail})
	if err == nil {
		t.Fatal("expected an error for an invalid email")
	}
	stored, getErr := users.GetUserByID(ctx, "u2")
	if getErr != nil {
		t.Fatalf("unexpected error re-reading user: %v", getErr)
	}
	if stored.Email == badEmail {
		t.Error("expected the invalid email never to have been written")
	}
}

func TestUpdateUser_InvalidRoleFailsBeforeAnyWrite(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	seedActiveUser(t, users, fakeHasher{}, "u2", "t1", "member@example.com", "pw", domain.RoleUser)

	uc := NewUpdateUser(users, &fakeAuditRepository{}, &fakeClock{now: time.Now()}, &fakeOPAClient{allow: true})
	ctx := withActor(context.Background(), "t1", "admin1")

	badRole := domain.Role("not-a-role")
	_, err := uc.Execute(ctx, UpdateUserInput{UserID: "u2", Role: &badRole})
	if err == nil {
		t.Fatal("expected an error for an invalid role")
	}
}

func TestUpdateUser_DeniedForNonAdminActor(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "member@example.com", "pw", domain.RoleUser)

	uc := NewUpdateUser(users, &fakeAuditRepository{}, &fakeClock{now: time.Now()}, &fakeOPAClient{allow: false})
	ctx := withActor(context.Background(), "t1", "u1")

	newName := "New Name"
	_, err := uc.Execute(ctx, UpdateUserInput{UserID: "u1", Name: &newName})
	if err == nil {
		t.Fatal("expected an error for a non-admin actor")
	}
}
