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

	uc := NewIssueServiceToken(users, signer, clock, 15*time.Minute)
	out, err := uc.Execute(context.Background(), IssueServiceTokenInput{UserID: "u1", Audience: "api-gateway"})
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
	uc := NewIssueServiceToken(users, &fakeTokenSigner{}, &fakeClock{now: time.Now()}, 15*time.Minute)

	_, err := uc.Execute(context.Background(), IssueServiceTokenInput{UserID: "nobody", Audience: "api-gateway"})
	if err == nil {
		t.Fatal("expected an error for an unknown user")
	}
}

func TestIssueServiceToken_RequiresUserIDAndAudience(t *testing.T) {
	users := newFakeUserRepository()
	u, _ := domain.NewUser("u1", "t1", "alice@example.com", "Alice", domain.RoleUser, true, time.Now())
	users.seed(u, "irrelevant-hash")
	uc := NewIssueServiceToken(users, &fakeTokenSigner{}, &fakeClock{now: time.Now()}, 15*time.Minute)

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

	uc := NewIssueServiceToken(users, signer, &fakeClock{now: time.Now()}, 15*time.Minute)
	if _, err := uc.Execute(context.Background(), IssueServiceTokenInput{UserID: "u1", Audience: "x"}); err == nil {
		t.Fatal("expected the signer's error to propagate")
	}
}
