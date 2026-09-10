package usecase

import (
	"context"
	"errors"
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

// waitForTouchCount polls sessions.touchCallCount() for up to 1s — the
// touch happens on a background goroutine (touchBestEffort), so a
// synchronous read right after Execute returns would race it.
func waitForTouchCount(t *testing.T, sessions *fakeSessionRepository, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if sessions.touchCallCount() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("expected touchCallCount() == %d within 1s, got %d", want, sessions.touchCallCount())
}

// assertNoTouchWithin asserts touchCallCount() stays at want for a short
// grace period — used to check a touch was NOT attempted (within the
// debounce window), which can't be proven by polling for a positive count.
func assertNoTouchWithin(t *testing.T, sessions *fakeSessionRepository, want int, grace time.Duration) {
	t.Helper()
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if got := sessions.touchCallCount(); got != want {
			t.Fatalf("expected touchCallCount() to stay at %d, got %d", want, got)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestValidateSession_TouchesLastSeenWhenNeverTouched(t *testing.T) {
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
	waitForTouchCount(t, sessions, 1)
}

func TestValidateSession_DoesNotTouchWithinDebounceWindow(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	clock := &fakeClock{now: time.Now()}

	u := seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "alice@example.com", "pw", domain.RoleUser)
	rawToken := "raw-session-token"
	session, err := domain.NewSession(domain.HashSessionToken(rawToken), u.ID, u.TenantID, clock.now.Add(-time.Hour), clock.now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building session: %v", err)
	}
	lastSeen := clock.now.Add(-10 * time.Second) // touched 10s ago, well within the 60s debounce
	session.LastSeenAt = &lastSeen
	_ = sessions.CreateSession(context.Background(), session)

	uc := NewValidateSession(sessions, users, clock)
	out, err := uc.Execute(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Valid {
		t.Fatal("expected a valid session")
	}
	assertNoTouchWithin(t, sessions, 0, 50*time.Millisecond)
}

func TestValidateSession_TouchesAfterDebounceWindow(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	clock := &fakeClock{now: time.Now()}

	u := seedActiveUser(t, users, fakeHasher{}, "u1", "t1", "alice@example.com", "pw", domain.RoleUser)
	rawToken := "raw-session-token"
	session, err := domain.NewSession(domain.HashSessionToken(rawToken), u.ID, u.TenantID, clock.now.Add(-2*time.Hour), clock.now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building session: %v", err)
	}
	lastSeen := clock.now.Add(-90 * time.Second) // touched 90s ago, past the 60s debounce
	session.LastSeenAt = &lastSeen
	_ = sessions.CreateSession(context.Background(), session)

	uc := NewValidateSession(sessions, users, clock)
	out, err := uc.Execute(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Valid {
		t.Fatal("expected a valid session")
	}
	waitForTouchCount(t, sessions, 1)
}

func TestValidateSession_TouchFailureDoesNotAffectOutput(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	sessions.touchErr = errors.New("boom")
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
	if !out.Valid || out.User.ID != "u1" {
		t.Errorf("expected a valid session for u1 regardless of a TouchLastSeen failure, got %+v", out)
	}
}
