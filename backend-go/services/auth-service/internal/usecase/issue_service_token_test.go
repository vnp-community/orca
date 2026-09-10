package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

var errSignFailed = errors.New("fake: sign failed")

func TestIssueServiceToken_SucceedsForExistingUser(t *testing.T) {
	users := newFakeUserRepository()
	signer := &fakeTokenSigner{}
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

	u, err := domain.NewUser("u1", "t1", "alice@example.com", "Alice", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	users.seed(u, "irrelevant-hash")

	uc := NewIssueServiceToken(users, newFakeServiceTokenRepository(), &fakeAuditRepository{}, signer, clock, 15*time.Minute)
	out, err := uc.Execute(context.Background(), IssueServiceTokenInput{UserID: "u1", Audience: "api-gateway", CallerUserID: "u1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.JWT == "" {
		t.Fatal("expected a non-empty JWT")
	}
	wantExpiry := clock.now.Add(15 * time.Minute)
	if !out.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("expected expiry %v, got %v", wantExpiry, out.ExpiresAt)
	}

	if signer.lastCall.Subject != "u1" {
		t.Errorf("expected sub=u1, got %s", signer.lastCall.Subject)
	}
	if signer.lastCall.TenantID != "t1" {
		t.Errorf("expected tenant_id=t1, got %s", signer.lastCall.TenantID)
	}
	if len(signer.lastCall.Audience) != 1 || signer.lastCall.Audience[0] != "api-gateway" {
		t.Errorf("expected aud=[api-gateway], got %v", signer.lastCall.Audience)
	}
	if signer.lastCall.ID == "" {
		t.Error("expected a non-empty jti")
	}
}

func TestIssueServiceToken_UnknownUserFails(t *testing.T) {
	users := newFakeUserRepository()
	uc := NewIssueServiceToken(users, newFakeServiceTokenRepository(), &fakeAuditRepository{}, &fakeTokenSigner{}, &fakeClock{now: time.Now()}, 15*time.Minute)

	_, err := uc.Execute(context.Background(), IssueServiceTokenInput{UserID: "nobody", Audience: "api-gateway", CallerUserID: "nobody"})
	if err == nil {
		t.Fatal("expected an error for an unknown user")
	}
}

func TestIssueServiceToken_RequiresUserIDAndAudience(t *testing.T) {
	users := newFakeUserRepository()
	u, _ := domain.NewUser("u1", "t1", "alice@example.com", "Alice", domain.RoleUser, true, time.Now())
	users.seed(u, "irrelevant-hash")
	uc := NewIssueServiceToken(users, newFakeServiceTokenRepository(), &fakeAuditRepository{}, &fakeTokenSigner{}, &fakeClock{now: time.Now()}, 15*time.Minute)

	if _, err := uc.Execute(context.Background(), IssueServiceTokenInput{UserID: "", Audience: "x"}); err == nil {
		t.Error("expected an error for empty user_id")
	}
	if _, err := uc.Execute(context.Background(), IssueServiceTokenInput{UserID: "u1", Audience: ""}); err == nil {
		t.Error("expected an error for empty audience")
	}
}

func TestIssueServiceToken_SignerFailurePropagates(t *testing.T) {
	users := newFakeUserRepository()
	u, _ := domain.NewUser("u1", "t1", "alice@example.com", "Alice", domain.RoleUser, true, time.Now())
	users.seed(u, "irrelevant-hash")
	signer := &fakeTokenSigner{signErr: errSignFailed}

	uc := NewIssueServiceToken(users, newFakeServiceTokenRepository(), &fakeAuditRepository{}, signer, &fakeClock{now: time.Now()}, 15*time.Minute)
	if _, err := uc.Execute(context.Background(), IssueServiceTokenInput{UserID: "u1", Audience: "x", CallerUserID: "u1"}); err == nil {
		t.Fatal("expected the signer's error to propagate")
	}
}

// TestIssueServiceToken_RejectsCallerMintingForOtherUser is the most
// important security test in this task set (CR-CLI-002/TASK-BE-CLI-004,
// decision 2026-09-09: self-mint-only, no admin-override in v1) — a caller
// whose identity differs from the requested UserID must be rejected with
// PermissionDenied and no JWT signed.
func TestIssueServiceToken_RejectsCallerMintingForOtherUser(t *testing.T) {
	users := newFakeUserRepository()
	u, _ := domain.NewUser("u1", "t1", "alice@example.com", "Alice", domain.RoleUser, true, time.Now())
	users.seed(u, "irrelevant-hash")
	signer := &fakeTokenSigner{}

	uc := NewIssueServiceToken(users, newFakeServiceTokenRepository(), &fakeAuditRepository{}, signer, &fakeClock{now: time.Now()}, 15*time.Minute)
	_, err := uc.Execute(context.Background(), IssueServiceTokenInput{UserID: "u1", Audience: "api-gateway", CallerUserID: "someone-else"})
	if err == nil {
		t.Fatal("expected an error when CallerUserID != UserID")
	}
	if signer.lastCall.Subject != "" {
		t.Errorf("signer must never be called when minting for another user, but got sub=%q", signer.lastCall.Subject)
	}
}

// TestIssueServiceToken_AllowsSelfMint is the regression guard: the CLI
// route (TASK-BE-CLI-003) must keep working when CallerUserID == UserID.
func TestIssueServiceToken_AllowsSelfMint(t *testing.T) {
	users := newFakeUserRepository()
	u, _ := domain.NewUser("u1", "t1", "alice@example.com", "Alice", domain.RoleUser, true, time.Now())
	users.seed(u, "irrelevant-hash")

	uc := NewIssueServiceToken(users, newFakeServiceTokenRepository(), &fakeAuditRepository{}, &fakeTokenSigner{}, &fakeClock{now: time.Now()}, 15*time.Minute)
	out, err := uc.Execute(context.Background(), IssueServiceTokenInput{UserID: "u1", Audience: "api-gateway", CallerUserID: "u1"})
	if err != nil {
		t.Fatalf("unexpected error for a self-mint request: %v", err)
	}
	if out.JWT == "" {
		t.Fatal("expected a non-empty JWT for a valid self-mint request")
	}
}

// TestIssueServiceToken_RecordsAuditEntry — CR-CLI-002/TASK-BE-CLI-006.
func TestIssueServiceToken_RecordsAuditEntry(t *testing.T) {
	users := newFakeUserRepository()
	u, _ := domain.NewUser("u1", "t1", "alice@example.com", "Alice", domain.RoleUser, true, time.Now())
	users.seed(u, "irrelevant-hash")
	audit := &fakeAuditRepository{}

	uc := NewIssueServiceToken(users, newFakeServiceTokenRepository(), audit, &fakeTokenSigner{}, &fakeClock{now: time.Now()}, 15*time.Minute)
	if _, err := uc.Execute(context.Background(), IssueServiceTokenInput{UserID: "u1", Audience: "api-gateway", CallerUserID: "u1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(audit.entries) != 1 {
		t.Fatalf("got %d audit entries, want 1", len(audit.entries))
	}
	entry := audit.entries[0]
	if entry.Action != "cli_token.issued" {
		t.Errorf("Action = %q, want %q", entry.Action, "cli_token.issued")
	}
	if entry.ActorID != "u1" {
		t.Errorf("ActorID = %q, want %q", entry.ActorID, "u1")
	}
	if entry.TenantID != "t1" {
		t.Errorf("TenantID = %q, want %q", entry.TenantID, "t1")
	}
	if entry.Target == "" {
		t.Error("TargetID (jti) must not be empty")
	}
}

// TestIssueServiceToken_MissingCallerIdentityRejected is a fail-closed
// guard: an empty CallerUserID (e.g. a bug in the propagation interceptor)
// must reject with a clear internal error, never silently succeed.
func TestIssueServiceToken_MissingCallerIdentityRejected(t *testing.T) {
	users := newFakeUserRepository()
	u, _ := domain.NewUser("u1", "t1", "alice@example.com", "Alice", domain.RoleUser, true, time.Now())
	users.seed(u, "irrelevant-hash")
	signer := &fakeTokenSigner{}

	uc := NewIssueServiceToken(users, newFakeServiceTokenRepository(), &fakeAuditRepository{}, signer, &fakeClock{now: time.Now()}, 15*time.Minute)
	_, err := uc.Execute(context.Background(), IssueServiceTokenInput{UserID: "u1", Audience: "api-gateway", CallerUserID: ""})
	if err == nil {
		t.Fatal("expected an error when CallerUserID is empty")
	}
	if signer.lastCall.Subject != "" {
		t.Errorf("signer must never be called when caller identity is missing, but got sub=%q", signer.lastCall.Subject)
	}
}
