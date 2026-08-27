package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

func TestWriteCredential_RequiresTenantContext(t *testing.T) {
	uc := NewWriteCredential(newFakeAccountRepository(), &fakeCredentialBroker{})
	_, err := uc.Execute(context.Background(), WriteCredentialForAccountInput{AccountID: "acc-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestWriteCredential_RequiresAccountID(t *testing.T) {
	uc := NewWriteCredential(newFakeAccountRepository(), &fakeCredentialBroker{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, WriteCredentialForAccountInput{})
	if err == nil {
		t.Fatal("expected an error for missing account_id")
	}
}

func TestWriteCredential_UsesAccountUserIDAsOwnerID(t *testing.T) {
	repo := newFakeAccountRepository()
	repo.accounts["acc-1"] = domain.ProviderAccount{ID: "acc-1", TenantID: "tenant-1", UserID: "user-1"}
	repo.getReturns = &domain.ProviderAccount{ID: "acc-1", TenantID: "tenant-1", UserID: "user-1"}
	broker := &fakeCredentialBroker{nextRefID: "cred-ref-new"}
	uc := NewWriteCredential(repo, broker)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, WriteCredentialForAccountInput{AccountID: "acc-1", EncryptedBlob: []byte("ct"), IV: []byte("iv")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if broker.lastWriteOwnerID != "user-1" {
		t.Errorf("expected owner_id to be the account's UserID, got %q", broker.lastWriteOwnerID)
	}
}

func TestWriteCredential_FallsBackToProjectIDThenServiceName(t *testing.T) {
	repo := newFakeAccountRepository()
	repo.accounts["acc-1"] = domain.ProviderAccount{ID: "acc-1", TenantID: "tenant-1", ProjectID: "proj-1"}
	repo.getReturns = &domain.ProviderAccount{ID: "acc-1", TenantID: "tenant-1", ProjectID: "proj-1"}
	broker := &fakeCredentialBroker{}
	uc := NewWriteCredential(repo, broker)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, WriteCredentialForAccountInput{AccountID: "acc-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if broker.lastWriteOwnerID != "proj-1" {
		t.Errorf("expected owner_id to fall back to ProjectID, got %q", broker.lastWriteOwnerID)
	}

	repo.getReturns = &domain.ProviderAccount{ID: "acc-1", TenantID: "tenant-1"}
	if _, err := uc.Execute(ctx, WriteCredentialForAccountInput{AccountID: "acc-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if broker.lastWriteOwnerID != "ai-provider-service" {
		t.Errorf("expected owner_id to fall back to the service name when neither UserID nor ProjectID is set, got %q", broker.lastWriteOwnerID)
	}
}

func TestWriteCredential_SetsStatusPendingWithNewCredentialRef(t *testing.T) {
	repo := newFakeAccountRepository()
	repo.accounts["acc-1"] = domain.ProviderAccount{ID: "acc-1", TenantID: "tenant-1", UserID: "user-1", Status: domain.AccountStatusActive}
	repo.getReturns = &domain.ProviderAccount{ID: "acc-1", TenantID: "tenant-1", UserID: "user-1"}
	broker := &fakeCredentialBroker{nextRefID: "cred-ref-rotated"}
	uc := NewWriteCredential(repo, broker)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, WriteCredentialForAccountInput{AccountID: "acc-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.AccountStatusPending {
		t.Errorf("expected status to become pending until push-confirmed, got %q", got.Status)
	}
	if got.CredentialRef != "cred-ref-rotated" {
		t.Errorf("expected credential_ref to be the broker's new ref, got %q", got.CredentialRef)
	}
}

func TestWriteCredential_PropagatesBrokerFailure(t *testing.T) {
	repo := newFakeAccountRepository()
	repo.getReturns = &domain.ProviderAccount{ID: "acc-1", TenantID: "tenant-1", UserID: "user-1"}
	broker := &fakeCredentialBroker{writeErr: errBoom}
	uc := NewWriteCredential(repo, broker)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, WriteCredentialForAccountInput{AccountID: "acc-1"})
	if err == nil {
		t.Fatal("expected broker failure to propagate")
	}
}

func TestWriteCredential_PropagatesAccountLookupFailure(t *testing.T) {
	repo := newFakeAccountRepository() // no getReturns, empty accounts map -> ErrAccountNotFound
	uc := NewWriteCredential(repo, &fakeCredentialBroker{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, WriteCredentialForAccountInput{AccountID: "missing"})
	if err == nil {
		t.Fatal("expected account lookup failure to propagate")
	}
}
