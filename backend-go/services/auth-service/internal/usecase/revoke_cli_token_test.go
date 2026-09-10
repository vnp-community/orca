package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func seedIssuedToken(t *testing.T, repo *fakeServiceTokenRepository, jti, userID string) {
	t.Helper()
	now := time.Now()
	tok := domain.IssuedServiceToken{
		JTI: jti, UserID: userID, Audience: "orca-cli", IssuedAt: now, ExpiresAt: now.Add(15 * time.Minute),
	}
	if err := repo.RecordIssuedToken(context.Background(), tok); err != nil {
		t.Fatalf("seeding issued token: %v", err)
	}
}

// TestRevokeCliToken_TokenRejectedAfterRevoke: mint -> revoke -> the
// repository's IsRevoked reports true for that jti (the property
// api-gateway's AuthValidator revocation check relies on, TestAuthValidator_*
// in the api-gateway package covers the HTTP-request-level rejection).
func TestRevokeCliToken_TokenRejectedAfterRevoke(t *testing.T) {
	repo := newFakeServiceTokenRepository()
	users := newFakeUserRepository()
	u, _ := domain.NewUser("u1", "t1", "alice@example.com", "Alice", domain.RoleUser, true, time.Now())
	users.seed(u, "irrelevant-hash")
	seedIssuedToken(t, repo, "jti-1", "u1")

	uc := NewRevokeCliToken(users, repo, &fakeAuditRepository{}, &fakeClock{now: time.Now()})
	if err := uc.Execute(context.Background(), RevokeCliTokenInput{JTI: "jti-1", UserID: "u1", CallerUserID: "u1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	revoked, err := repo.IsRevoked(context.Background(), "jti-1")
	if err != nil {
		t.Fatalf("IsRevoked: %v", err)
	}
	if !revoked {
		t.Fatal("expected jti-1 to be revoked after Execute")
	}
}

func TestRevokeCliToken_CannotRevokeAnotherUsersToken(t *testing.T) {
	repo := newFakeServiceTokenRepository()
	users := newFakeUserRepository()
	seedIssuedToken(t, repo, "jti-b", "user-b")

	uc := NewRevokeCliToken(users, repo, &fakeAuditRepository{}, &fakeClock{now: time.Now()})
	err := uc.Execute(context.Background(), RevokeCliTokenInput{JTI: "jti-b", UserID: "user-a", CallerUserID: "user-a"})
	if err == nil {
		t.Fatal("expected an error when UserID does not own the jti")
	}

	revoked, _ := repo.IsRevoked(context.Background(), "jti-b")
	if revoked {
		t.Fatal("user-b's token must not be revoked by user-a's request")
	}
}

func TestRevokeCliToken_RejectsCallerActingForAnotherUser(t *testing.T) {
	repo := newFakeServiceTokenRepository()
	users := newFakeUserRepository()
	seedIssuedToken(t, repo, "jti-1", "u1")

	uc := NewRevokeCliToken(users, repo, &fakeAuditRepository{}, &fakeClock{now: time.Now()})
	err := uc.Execute(context.Background(), RevokeCliTokenInput{JTI: "jti-1", UserID: "u1", CallerUserID: "someone-else"})
	if err == nil {
		t.Fatal("expected an error when CallerUserID != UserID")
	}
}

func TestRevokeCliToken_MissingCallerIdentityRejected(t *testing.T) {
	repo := newFakeServiceTokenRepository()
	users := newFakeUserRepository()
	seedIssuedToken(t, repo, "jti-1", "u1")

	uc := NewRevokeCliToken(users, repo, &fakeAuditRepository{}, &fakeClock{now: time.Now()})
	err := uc.Execute(context.Background(), RevokeCliTokenInput{JTI: "jti-1", UserID: "u1", CallerUserID: ""})
	if err == nil {
		t.Fatal("expected an error when CallerUserID is empty")
	}
}

// TestRevokeCliToken_RecordsAuditEntry — CR-CLI-002/TASK-BE-CLI-006.
func TestRevokeCliToken_RecordsAuditEntry(t *testing.T) {
	repo := newFakeServiceTokenRepository()
	users := newFakeUserRepository()
	u, _ := domain.NewUser("u1", "t1", "alice@example.com", "Alice", domain.RoleUser, true, time.Now())
	users.seed(u, "irrelevant-hash")
	seedIssuedToken(t, repo, "jti-1", "u1")
	audit := &fakeAuditRepository{}

	uc := NewRevokeCliToken(users, repo, audit, &fakeClock{now: time.Now()})
	if err := uc.Execute(context.Background(), RevokeCliTokenInput{JTI: "jti-1", UserID: "u1", CallerUserID: "u1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(audit.entries) != 1 {
		t.Fatalf("got %d audit entries, want 1", len(audit.entries))
	}
	entry := audit.entries[0]
	if entry.Action != "cli_token.revoked" {
		t.Errorf("Action = %q, want %q", entry.Action, "cli_token.revoked")
	}
	if entry.ActorID != "u1" {
		t.Errorf("ActorID = %q, want %q", entry.ActorID, "u1")
	}
	if entry.TenantID != "t1" {
		t.Errorf("TenantID = %q, want %q", entry.TenantID, "t1")
	}
	if entry.TargetID != "jti-1" {
		t.Errorf("TargetID = %q, want %q", entry.TargetID, "jti-1")
	}
	if entry.TargetType != "service_token" {
		t.Errorf("TargetType = %q, want %q", entry.TargetType, "service_token")
	}
}

func TestRevokeCliToken_UnknownJTIReturnsNotFound(t *testing.T) {
	repo := newFakeServiceTokenRepository()
	users := newFakeUserRepository()

	uc := NewRevokeCliToken(users, repo, &fakeAuditRepository{}, &fakeClock{now: time.Now()})
	err := uc.Execute(context.Background(), RevokeCliTokenInput{JTI: "no-such-jti", UserID: "u1", CallerUserID: "u1"})
	if err == nil {
		t.Fatal("expected an error for an unknown jti")
	}
}
