package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func seedActiveUser(t *testing.T, users *fakeUserRepository, hasher PasswordHasher, id, tenantID, email, password string, role domain.Role) domain.User {
	t.Helper()
	u, err := domain.NewUser(id, tenantID, email, "Test User", role, true, time.Now())
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("hashing password: %v", err)
	}
	users.seed(u, hash)
	return u
}

func TestLogin_SucceedsAndCreatesSession(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}

	seedActiveUser(t, users, hasher, "u1", "t1", "alice@example.com", "correct-password", domain.RoleUser)

	uc := NewLogin(users, sessions, audit, hasher, clock, time.Hour)
	out, err := uc.Execute(context.Background(), LoginInput{Email: "alice@example.com", Password: "correct-password"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.SessionToken == "" {
		t.Fatal("expected a non-empty session token")
	}
	if out.User.ID != "u1" {
		t.Errorf("expected user u1, got %s", out.User.ID)
	}

	stored, err := sessions.GetSessionByTokenHash(context.Background(), domain.HashSessionToken(out.SessionToken))
	if err != nil {
		t.Fatalf("expected session to be stored: %v", err)
	}
	if stored.UserID != "u1" {
		t.Errorf("expected stored session for u1, got %s", stored.UserID)
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "user.login" {
		t.Errorf("expected one user.login audit entry, got %+v", audit.entries)
	}
}

func TestLogin_WrongPasswordFails(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}

	seedActiveUser(t, users, hasher, "u1", "t1", "alice@example.com", "correct-password", domain.RoleUser)

	uc := NewLogin(users, sessions, audit, hasher, clock, time.Hour)
	_, err := uc.Execute(context.Background(), LoginInput{Email: "alice@example.com", Password: "wrong-password"})
	if err == nil {
		t.Fatal("expected an error for a wrong password")
	}
	if len(sessions.byHash) != 0 {
		t.Error("expected no session to be created on failed login")
	}
}

func TestLogin_UnknownEmailFails(t *testing.T) {
	users := newFakeUserRepository()
	uc := NewLogin(users, newFakeSessionRepository(), &fakeAuditRepository{}, fakeHasher{}, &fakeClock{now: time.Now()}, time.Hour)

	_, err := uc.Execute(context.Background(), LoginInput{Email: "nobody@example.com", Password: "whatever"})
	if err == nil {
		t.Fatal("expected an error for an unknown email")
	}
}

func TestLogin_DeactivatedAccountFails(t *testing.T) {
	users := newFakeUserRepository()
	hasher := fakeHasher{}
	u, err := domain.NewUser("u1", "t1", "alice@example.com", "Alice", domain.RoleUser, false, time.Now())
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	hash, _ := hasher.Hash("correct-password")
	users.seed(u, hash)

	uc := NewLogin(users, newFakeSessionRepository(), &fakeAuditRepository{}, hasher, &fakeClock{now: time.Now()}, time.Hour)
	_, err = uc.Execute(context.Background(), LoginInput{Email: "alice@example.com", Password: "correct-password"})
	if err == nil {
		t.Fatal("expected an error for a deactivated account")
	}
}

func TestLogin_RequiresEmailAndPassword(t *testing.T) {
	uc := NewLogin(newFakeUserRepository(), newFakeSessionRepository(), &fakeAuditRepository{}, fakeHasher{}, &fakeClock{now: time.Now()}, time.Hour)

	if _, err := uc.Execute(context.Background(), LoginInput{Email: "", Password: "x"}); err == nil {
		t.Error("expected an error for empty email")
	}
	if _, err := uc.Execute(context.Background(), LoginInput{Email: "a@example.com", Password: ""}); err == nil {
		t.Error("expected an error for empty password")
	}
}
