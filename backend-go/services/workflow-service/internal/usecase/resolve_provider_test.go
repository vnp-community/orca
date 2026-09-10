package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// fakeAIProviderClient is a minimal test double for AIProviderClient — no
// real network client needed, matching this codebase's existing
// port-per-dependency pattern.
type fakeAIProviderClient struct {
	resolveAccountID string
	resolveErr       error
	resolveCalled    bool

	accountStatus     string
	statusErr         error
	gotStatusAccentID string // captures the accountID GetAccountStatus was called with
}

func (f *fakeAIProviderClient) ResolveForProject(ctx context.Context, userID, projectID string) (string, error) {
	f.resolveCalled = true
	if f.resolveErr != nil {
		return "", f.resolveErr
	}
	return f.resolveAccountID, nil
}

func (f *fakeAIProviderClient) GetAccountStatus(ctx context.Context, accountID string) (string, error) {
	f.gotStatusAccentID = accountID
	if f.statusErr != nil {
		return "", f.statusErr
	}
	return f.accountStatus, nil
}

func TestProviderResolver_ExplicitActivePinWins(t *testing.T) {
	client := &fakeAIProviderClient{accountStatus: "active", resolveAccountID: "should-not-be-used"}
	r := NewProviderResolver(client)

	cfg := domain.AgentStepConfig{Provider: &domain.ProviderPin{AccountID: "acct-pinned", Model: "claude-3-opus"}}
	got, err := r.Resolve(context.Background(), cfg, "proj-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AccountID != "acct-pinned" || got.Model != "claude-3-opus" {
		t.Errorf("expected the pinned account+model, got %+v", got)
	}
	if client.resolveCalled {
		t.Error("expected ResolveForProject to never be called when an active pin is present")
	}
}

func TestProviderResolver_ExplicitInactivePinIsHardError(t *testing.T) {
	client := &fakeAIProviderClient{accountStatus: "revoked", resolveAccountID: "should-not-be-used"}
	r := NewProviderResolver(client)

	cfg := domain.AgentStepConfig{Provider: &domain.ProviderPin{AccountID: "acct-pinned"}}
	_, err := r.Resolve(context.Background(), cfg, "proj-1", "user-1")
	if err == nil {
		t.Fatal("expected a hard error for an inactive pinned account")
	}
	if client.resolveCalled {
		t.Error("expected no silent fallback to the priority chain for an inactive pin")
	}
}

func TestProviderResolver_NoPinFallsBackToChain(t *testing.T) {
	client := &fakeAIProviderClient{resolveAccountID: "acct-chain"}
	r := NewProviderResolver(client)

	got, err := r.Resolve(context.Background(), domain.AgentStepConfig{}, "proj-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AccountID != "acct-chain" {
		t.Errorf("expected acct-chain from the priority chain, got %q", got.AccountID)
	}
	if got.Model != "" {
		t.Errorf("expected empty Model when resolution falls through to the chain, got %q", got.Model)
	}
}

func TestProviderResolver_PinWithEmptyAccountIDFallsBackToChain(t *testing.T) {
	client := &fakeAIProviderClient{resolveAccountID: "acct-chain"}
	r := NewProviderResolver(client)

	cfg := domain.AgentStepConfig{Provider: &domain.ProviderPin{AccountID: "", Model: "ignored"}}
	got, err := r.Resolve(context.Background(), cfg, "proj-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AccountID != "acct-chain" {
		t.Errorf("expected fallback to the chain when Provider.AccountID is empty, got %+v", got)
	}
}

func TestProviderResolver_ChainErrorPropagates(t *testing.T) {
	client := &fakeAIProviderClient{resolveErr: errors.New("ai-provider-service unreachable")}
	r := NewProviderResolver(client)

	_, err := r.Resolve(context.Background(), domain.AgentStepConfig{}, "proj-1", "user-1")
	if err == nil {
		t.Fatal("expected the chain error to propagate")
	}
}

func TestProviderResolver_PinStatusLookupErrorPropagates(t *testing.T) {
	client := &fakeAIProviderClient{statusErr: errors.New("ai-provider-service unreachable")}
	r := NewProviderResolver(client)

	cfg := domain.AgentStepConfig{Provider: &domain.ProviderPin{AccountID: "acct-pinned"}}
	_, err := r.Resolve(context.Background(), cfg, "proj-1", "user-1")
	if err == nil {
		t.Fatal("expected the status lookup error to propagate")
	}
}
