package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// newCreateAccountUC wires a CreateAccount usecase against fakes with a
// reachable-by-default InfraFleetClient, so tests that don't care about the
// test-before-save gate itself don't each need to set relayResult.
func newCreateAccountUC(repo *fakeAccountRepository, broker *fakeCredentialBroker, infra *fakeInfraFleetClient, newID func() string, now func() time.Time) *CreateAccount {
	if infra.relayResult == nil && infra.relayErr == nil {
		infra.relayResult = map[string]any{"success": true, "message": "reachable"}
	}
	return NewCreateAccount(repo, broker, infra, newID, now)
}

func TestCreateAccount_RequiresTenantContext(t *testing.T) {
	uc := newCreateAccountUC(newFakeAccountRepository(), &fakeCredentialBroker{}, &fakeInfraFleetClient{}, func() string { return "acc-1" }, nil)
	_, err := uc.Execute(context.Background(), CreateAccountInput{ProviderType: domain.ProviderTypeAnthropic, DevServerID: "dev-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateAccount_CreatesPendingAccountWithBrokerCredentialRef(t *testing.T) {
	repo := newFakeAccountRepository()
	broker := &fakeCredentialBroker{nextRefID: "cred-ref-xyz"}
	infra := &fakeInfraFleetClient{}
	fixedNow := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	uc := newCreateAccountUC(repo, broker, infra, func() string { return "acc-1" }, func() time.Time { return fixedNow })

	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	got, err := uc.Execute(ctx, CreateAccountInput{ProviderType: domain.ProviderTypeOpenAI, DevServerID: "dev-1"})
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
	uc := newCreateAccountUC(newFakeAccountRepository(), &fakeCredentialBroker{}, &fakeInfraFleetClient{}, func() string { return "acc-1" }, nil)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, CreateAccountInput{TenantID: "tenant-2", ProviderType: domain.ProviderTypeAnthropic, DevServerID: "dev-1"})
	if err == nil {
		t.Fatal("expected an error when request tenant_id disagrees with authenticated tenant")
	}
}

func TestCreateAccount_PropagatesBrokerFailure(t *testing.T) {
	repo := newFakeAccountRepository()
	broker := &fakeCredentialBroker{writeErr: errBoom}
	uc := newCreateAccountUC(repo, broker, &fakeInfraFleetClient{}, func() string { return "acc-1" }, nil)

	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, CreateAccountInput{ProviderType: domain.ProviderTypeAnthropic, DevServerID: "dev-1"})
	if err == nil {
		t.Fatal("expected broker failure to propagate")
	}
	if len(repo.accounts) != 0 {
		t.Error("expected no account to be persisted when the broker call fails")
	}
}

func TestCreateAccount_RejectsInvalidProviderType(t *testing.T) {
	uc := newCreateAccountUC(newFakeAccountRepository(), &fakeCredentialBroker{}, &fakeInfraFleetClient{}, func() string { return "acc-1" }, nil)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, CreateAccountInput{ProviderType: domain.ProviderType("bogus"), DevServerID: "dev-1"})
	if err == nil {
		t.Fatal("expected an error for an invalid provider type")
	}
}

func TestCreateAccount_RequiresDevServerID(t *testing.T) {
	uc := newCreateAccountUC(newFakeAccountRepository(), &fakeCredentialBroker{}, &fakeInfraFleetClient{}, func() string { return "acc-1" }, nil)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, CreateAccountInput{ProviderType: domain.ProviderTypeAnthropic})
	if err == nil {
		t.Fatal("expected an error when dev_server_id is missing")
	}
}

func TestCreateAccount_LabelUniquenessPerDevServerProvider(t *testing.T) {
	repo := newFakeAccountRepository()
	broker := &fakeCredentialBroker{}
	infra := &fakeInfraFleetClient{}
	uc := newCreateAccountUC(repo, broker, infra, sequentialIDs("acc"), nil)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, CreateAccountInput{ProviderType: domain.ProviderTypeAnthropic, DevServerID: "dev-1", Label: "my-key"})
	if err != nil {
		t.Fatalf("unexpected error on first create: %v", err)
	}

	// Same label, same dev server, same provider -> AlreadyExists.
	_, err = uc.Execute(ctx, CreateAccountInput{ProviderType: domain.ProviderTypeAnthropic, DevServerID: "dev-1", Label: "my-key"})
	if err == nil {
		t.Fatal("expected AlreadyExists error for duplicate label on same dev server/provider")
	}

	// Different provider -> succeeds.
	_, err = uc.Execute(ctx, CreateAccountInput{ProviderType: domain.ProviderTypeOpenAI, DevServerID: "dev-1", Label: "my-key"})
	if err != nil {
		t.Fatalf("expected success for same label under a different provider, got %v", err)
	}
}

func TestCreateAccount_TestConnectionGate(t *testing.T) {
	repo := newFakeAccountRepository()
	broker := &fakeCredentialBroker{nextRefID: "cred-ref-bad"}
	infra := &fakeInfraFleetClient{relayResult: map[string]any{"success": false, "message": "invalid key"}}
	uc := NewCreateAccount(repo, broker, infra, func() string { return "acc-1" }, nil)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, CreateAccountInput{ProviderType: domain.ProviderTypeAnthropic, DevServerID: "dev-1"})
	if err == nil {
		t.Fatal("expected the test-before-save gate to fail the create")
	}
	if broker.revokeCallCount != 1 || broker.lastRevokedRef != "cred-ref-bad" {
		t.Errorf("expected RevokeCredential to be called with the ref just written, got count=%d ref=%q", broker.revokeCallCount, broker.lastRevokedRef)
	}
	if len(repo.accounts) != 0 {
		t.Error("expected repo.Create to never be called when the connection test fails")
	}
}

func TestCreateAccount_TestConnectionGate_RevokeFailureSurfaced(t *testing.T) {
	repo := newFakeAccountRepository()
	broker := &fakeCredentialBroker{nextRefID: "cred-ref-bad", revokeErr: errBoom}
	infra := &fakeInfraFleetClient{relayResult: map[string]any{"success": false, "message": "invalid key"}}
	uc := NewCreateAccount(repo, broker, infra, func() string { return "acc-1" }, nil)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, CreateAccountInput{ProviderType: domain.ProviderTypeAnthropic, DevServerID: "dev-1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	// testErr is nil here (the relay call itself succeeded; only
	// result.Success was false), so the combined message only has the
	// revoke failure to report — still enough to prove both failure paths
	// were reached (RevokeCredential was actually called and its error
	// surfaced) rather than the create silently swallowing it.
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected error message to include the revoke failure, got: %v", err)
	}
	if broker.revokeCallCount != 1 {
		t.Errorf("expected RevokeCredential to have been attempted once, got %d", broker.revokeCallCount)
	}
}

func sequentialIDs(prefix string) func() string {
	n := 0
	return func() string {
		n++
		return prefix + "-" + string(rune('0'+n))
	}
}
