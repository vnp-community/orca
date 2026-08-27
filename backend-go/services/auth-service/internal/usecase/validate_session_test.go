package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestValidateSession_ValidTokenReturnsUser(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	clock := &fakeClock{now: time.Now()}

	u := seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "alice@example.com", "pw", domain.RoleUser)
	rawToken := "raw-session-token"
	session, err := domain.NewSession(domain.HashSessionToken(rawToken), u.ID, u.TenantID, clock.now, clock.now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building session: %v", err)
	}
	_ = sessions.CreateSession(context.Background(), session)

	uc := NewValidateSession(sessions, users, clock)
	out, err := uc.Execute(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Valid {
		t.Fatal("expected a valid session")
	}
	if out.User.ID != "u1" {
		t.Errorf("expected user u1, got %s", out.User.ID)
	}
}

func TestValidateSession_UnknownTokenIsInvalidNotError(t *testing.T) {
	uc := NewValidateSession(newFakeSessionRepository(), newFakeUserRepository(), &fakeClock{now: time.Now()})
	out, err := uc.Execute(context.Background(), "does-not-exist")
	if err != nil {
		t.Fatalf("expected no error for an unknown token, got %v", err)
	}
	if out.Valid {
		t.Fatal("expected an unknown token to be invalid")
	}
}

func TestValidateSession_ExpiredTokenIsInvalid(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	createdAt := time.Now().Add(-2 * time.Hour)
	u := seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "alice@example.com", "pw", domain.RoleUser)
	rawToken := "raw-session-token"
	session, _ := domain.NewSession(domain.HashSessionToken(rawToken), u.ID, u.TenantID, createdAt, createdAt.Add(time.Hour))
	_ = sessions.CreateSession(context.Background(), session)

	uc := NewValidateSession(sessions, users, &fakeClock{now: time.Now()}) // now is well past expiry
	out, err := uc.Execute(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Valid {
		t.Fatal("expected an expired session to be invalid")
	}
}

func TestValidateSession_RevokedTokenIsInvalid(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	clock := &fakeClock{now: time.Now()}
	u := seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "alice@example.com", "pw", domain.RoleUser)
	rawToken := "raw-session-token"
	session, _ := domain.NewSession(domain.HashSessionToken(rawToken), u.ID, u.TenantID, clock.now, clock.now.Add(time.Hour))
	_ = sessions.CreateSession(context.Background(), session)
	_ = sessions.RevokeSession(context.Background(), domain.HashSessionToken(rawToken), clock.now)

	uc := NewValidateSession(sessions, users, clock)
	out, err := uc.Execute(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Valid {
		t.Fatal("expected a revoked session to be invalid")
	}
}

func TestValidateSession_EmptyTokenIsInvalid(t *testing.T) {
	uc := NewValidateSession(newFakeSessionRepository(), newFakeUserRepository(), &fakeClock{now: time.Now()})
	out, err := uc.Execute(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Valid {
		t.Fatal("expected an empty token to be invalid")
	}
}
