package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

func TestRotateKey_RotatesCredentialRefAndSetsStatusRotating(t *testing.T) {
	repo := newFakeAccountRepository()
	now := time.Now()
	acc, err := domain.NewProviderAccount("acc-1", "tenant-1", domain.ProviderTypeAnthropic, domain.AccountStatusActive,
		"cred-ref-old", domain.ScopeServer, "", "", "", nil, now, now)
	if err != nil {
		t.Fatalf("building account: %v", err)
	}
	_ = repo.Create(context.Background(), acc)

	broker := &fakeCredentialBroker{}
	uc := NewRotateKey(repo, broker)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, RotateKeyInput{AccountID: "acc-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.AccountStatusRotating {
		t.Errorf("expected status rotating, got %q", got.Status)
	}
	if got.CredentialRef != "cred-ref-old-rotated" {
		t.Errorf("expected credential_ref to be updated to the broker's new ref, got %q", got.CredentialRef)
	}
}

func TestRotateKey_AccountNotFound(t *testing.T) {
	uc := NewRotateKey(newFakeAccountRepository(), &fakeCredentialBroker{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, RotateKeyInput{AccountID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected an error for a missing account")
	}
}

func TestRotateKey_PropagatesBrokerFailure(t *testing.T) {
	repo := newFakeAccountRepository()
	now := time.Now()
	acc, _ := domain.NewProviderAccount("acc-1", "tenant-1", domain.ProviderTypeAnthropic, domain.AccountStatusActive,
		"cred-ref-old", domain.ScopeServer, "", "", "", nil, now, now)
	_ = repo.Create(context.Background(), acc)

	broker := &fakeCredentialBroker{rotateErr: errBoom}
	uc := NewRotateKey(repo, broker)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, RotateKeyInput{AccountID: "acc-1"})
	if err == nil {
		t.Fatal("expected broker rotation failure to propagate")
	}
}
