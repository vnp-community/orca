package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

func TestTestConnection_RequiresTenantContext(t *testing.T) {
	uc := NewTestConnection(newFakeAccountRepository(), &fakeInfraFleetClient{})
	_, err := uc.Execute(context.Background(), TestConnectionInput{AccountID: "acc-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestTestConnection_RequiresDevServerBound(t *testing.T) {
	repo := newFakeAccountRepository()
	repo.getReturns = &domain.ProviderAccount{ID: "acc-1", TenantID: "tenant-1"} // no DevServerID
	uc := NewTestConnection(repo, &fakeInfraFleetClient{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, TestConnectionInput{AccountID: "acc-1"})
	if err == nil {
		t.Fatal("expected an error when the account has no dev server bound yet")
	}
}

func TestTestConnection_RelaysToAccountsDevServerWithCredentialRefAndProviderType(t *testing.T) {
	repo := newFakeAccountRepository()
	repo.getReturns = &domain.ProviderAccount{
		ID: "acc-1", TenantID: "tenant-1", DevServerID: "ds-1",
		CredentialRef: "cred-ref-1", ProviderType: domain.ProviderTypeAnthropic,
	}
	infra := &fakeInfraFleetClient{relayResult: map[string]any{"success": true, "message": "ok"}}
	uc := NewTestConnection(repo, infra)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, TestConnectionInput{AccountID: "acc-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if infra.lastDevServerID != "ds-1" {
		t.Errorf("expected relay to target the account's dev server, got %q", infra.lastDevServerID)
	}
	if infra.lastMethod != "ai.testProviderConnection" {
		t.Errorf("expected relay method ai.testProviderConnection, got %q", infra.lastMethod)
	}
	if infra.lastParams["credentialRef"] != "cred-ref-1" || infra.lastParams["providerType"] != "anthropic" {
		t.Errorf("expected credentialRef/providerType params, got %+v", infra.lastParams)
	}
	if !got.Success || got.Message != "ok" {
		t.Errorf("expected the agent's result to pass through, got %+v", got)
	}
}

func TestTestConnection_DefensivelyParsesMissingResultFields(t *testing.T) {
	repo := newFakeAccountRepository()
	repo.getReturns = &domain.ProviderAccount{ID: "acc-1", TenantID: "tenant-1", DevServerID: "ds-1"}
	infra := &fakeInfraFleetClient{relayResult: map[string]any{}} // agent method doesn't exist yet — no fields present
	uc := NewTestConnection(repo, infra)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, TestConnectionInput{AccountID: "acc-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Success || got.Message != "" {
		t.Errorf("expected zero-value result for a response missing both fields, got %+v", got)
	}
}

func TestTestConnection_PropagatesRelayFailure(t *testing.T) {
	repo := newFakeAccountRepository()
	repo.getReturns = &domain.ProviderAccount{ID: "acc-1", TenantID: "tenant-1", DevServerID: "ds-1"}
	infra := &fakeInfraFleetClient{relayErr: errBoom}
	uc := NewTestConnection(repo, infra)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, TestConnectionInput{AccountID: "acc-1"})
	if err == nil {
		t.Fatal("expected relay failure to propagate")
	}
}

func TestTestConnection_PropagatesAccountLookupFailure(t *testing.T) {
	repo := newFakeAccountRepository() // no getReturns, empty accounts map -> ErrAccountNotFound
	uc := NewTestConnection(repo, &fakeInfraFleetClient{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, TestConnectionInput{AccountID: "missing"})
	if err == nil {
		t.Fatal("expected account lookup failure to propagate")
	}
}
