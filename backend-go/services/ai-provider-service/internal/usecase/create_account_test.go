package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

func TestCreateAccount_RequiresTenantContext(t *testing.T) {
	uc := NewCreateAccount(newFakeAccountRepository(), &fakeCredentialBroker{}, func() string { return "acc-1" }, nil)
	_, err := uc.Execute(context.Background(), CreateAccountInput{ProviderType: domain.ProviderTypeAnthropic})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateAccount_CreatesPendingAccountWithBrokerCredentialRef(t *testing.T) {
	repo := newFakeAccountRepository()
	broker := &fakeCredentialBroker{nextRefID: "cred-ref-xyz"}
	fixedNow := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	uc := NewCreateAccount(repo, broker, func() string { return "acc-1" }, func() time.Time { return fixedNow })

	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	got, err := uc.Execute(ctx, CreateAccountInput{ProviderType: domain.ProviderTypeOpenAI})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Status != domain.AccountStatusPending {
		t.Errorf("expected new account status to be pending until ciphertext push confirms, got %q", got.Status)
	}
	if got.CredentialRef != "cred-ref-xyz" {
		t.Errorf("expected credential_ref from broker stub, got %q", got.CredentialRef)
	}
	if got.Scope != domain.ScopeServer {
		t.Errorf("expected default scope to be server when unset, got %q", got.Scope)
	}
	if _, ok := repo.accounts["acc-1"]; !ok {
		t.Error("expected account to be persisted via repository")
	}
}

func TestCreateAccount_RejectsMismatchedTenantID(t *testing.T) {
	uc := NewCreateAccount(newFakeAccountRepository(), &fakeCredentialBroker{}, func() string { return "acc-1" }, nil)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, CreateAccountInput{TenantID: "tenant-2", ProviderType: domain.ProviderTypeAnthropic})
	if err == nil {
		t.Fatal("expected an error when request tenant_id disagrees with authenticated tenant")
	}
}

func TestCreateAccount_PropagatesBrokerFailure(t *testing.T) {
	repo := newFakeAccountRepository()
	broker := &fakeCredentialBroker{writeErr: errBoom}
	uc := NewCreateAccount(repo, broker, func() string { return "acc-1" }, nil)

	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, CreateAccountInput{ProviderType: domain.ProviderTypeAnthropic})
	if err == nil {
		t.Fatal("expected broker failure to propagate")
	}
	if len(repo.accounts) != 0 {
		t.Error("expected no account to be persisted when the broker call fails")
	}
}

func TestCreateAccount_RejectsInvalidProviderType(t *testing.T) {
	uc := NewCreateAccount(newFakeAccountRepository(), &fakeCredentialBroker{}, func() string { return "acc-1" }, nil)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, CreateAccountInput{ProviderType: domain.ProviderType("bogus")})
	if err == nil {
		t.Fatal("expected an error for an invalid provider type")
	}
}
