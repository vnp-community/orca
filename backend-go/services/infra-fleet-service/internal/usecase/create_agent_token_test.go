package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestCreateAgentToken_RequiresTenantContext(t *testing.T) {
	uc := NewCreateAgentToken(newFakeAgentTokenRepository(), &fakeDevServerRepository{}, &fakeCredentialBrokerClient{})
	_, _, err := uc.Execute(context.Background(), "ds-1", "my-token")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateAgentToken_RequiresName(t *testing.T) {
	uc := NewCreateAgentToken(newFakeAgentTokenRepository(), &fakeDevServerRepository{}, &fakeCredentialBrokerClient{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, _, err := uc.Execute(ctx, "ds-1", "")
	if err != domain.ErrEmptyAgentTokenName {
		t.Fatalf("expected ErrEmptyAgentTokenName, got %v", err)
	}
}

func TestCreateAgentToken_DirectWebSocket_StoresHashOnly(t *testing.T) {
	repo := newFakeAgentTokenRepository()
	devServers := &fakeDevServerRepository{byID: map[string]domain.DevServer{
		"ds-1": {ID: "ds-1", TenantID: "tenant-1", Mode: domain.ConnectionModeDirectWebSocket},
	}}
	broker := &fakeCredentialBrokerClient{}
	uc := NewCreateAgentToken(repo, devServers, broker)

	ctx := withTenant(context.Background(), "tenant-1")
	plaintext, tok, err := uc.Execute(ctx, "ds-1", "my-laptop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plaintext == "" {
		t.Fatal("expected a non-empty plaintext token")
	}
	if tok.TokenHash == "" {
		t.Error("expected TokenHash to be set for a direct-websocket dev server")
	}
	if tok.CredentialRefID != "" {
		t.Errorf("expected CredentialRefID to be empty for direct-websocket, got %q", tok.CredentialRefID)
	}
	if broker.writeCalls != 0 {
		t.Errorf("expected WriteCredential not to be called for direct-websocket, got %d calls", broker.writeCalls)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("expected 1 inserted token, got %d", len(repo.inserted))
	}
}

func TestCreateAgentToken_RelayWebSocket_WritesCredentialAndStoresRefOnly(t *testing.T) {
	repo := newFakeAgentTokenRepository()
	devServers := &fakeDevServerRepository{byID: map[string]domain.DevServer{
		"ds-2": {ID: "ds-2", TenantID: "tenant-1", Mode: domain.ConnectionModeRelayWebSocket},
	}}
	broker := &fakeCredentialBrokerClient{writtenRefID: "cred-ref-42"}
	uc := NewCreateAgentToken(repo, devServers, broker)

	ctx := withTenant(context.Background(), "tenant-1")
	plaintext, tok, err := uc.Execute(ctx, "ds-2", "relay-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plaintext == "" {
		t.Fatal("expected a non-empty plaintext token")
	}
	if broker.writeCalls != 1 {
		t.Fatalf("expected WriteCredential to be called exactly once, got %d", broker.writeCalls)
	}
	if string(broker.lastEnvelope) != plaintext {
		t.Errorf("expected the plaintext to be written to the credential broker verbatim")
	}
	if tok.CredentialRefID != "cred-ref-42" {
		t.Errorf("CredentialRefID = %q, want cred-ref-42", tok.CredentialRefID)
	}
	if tok.TokenHash != "" {
		t.Errorf("expected TokenHash to be empty for relay-websocket, got %q", tok.TokenHash)
	}
}

func TestCreateAgentToken_RelaySSH_Rejected(t *testing.T) {
	repo := newFakeAgentTokenRepository()
	devServers := &fakeDevServerRepository{byID: map[string]domain.DevServer{
		"ds-3": {ID: "ds-3", TenantID: "tenant-1", Mode: domain.ConnectionModeRelaySSH},
	}}
	uc := NewCreateAgentToken(repo, devServers, &fakeCredentialBrokerClient{})

	ctx := withTenant(context.Background(), "tenant-1")
	_, _, err := uc.Execute(ctx, "ds-3", "should-fail")
	if err != domain.ErrInvalidConnectionMode {
		t.Fatalf("expected ErrInvalidConnectionMode, got %v", err)
	}
}

func TestCreateAgentToken_EleventhTokenRejected(t *testing.T) {
	repo := newFakeAgentTokenRepository()
	devServers := &fakeDevServerRepository{byID: map[string]domain.DevServer{
		"ds-4": {ID: "ds-4", TenantID: "tenant-1", Mode: domain.ConnectionModeDirectWebSocket},
	}}
	uc := NewCreateAgentToken(repo, devServers, &fakeCredentialBrokerClient{})
	ctx := withTenant(context.Background(), "tenant-1")

	for i := 0; i < domain.MaxActiveAgentTokensPerDevServer; i++ {
		if _, _, err := uc.Execute(ctx, "ds-4", "token"); err != nil {
			t.Fatalf("token %d: unexpected error: %v", i, err)
		}
	}
	_, _, err := uc.Execute(ctx, "ds-4", "eleventh")
	if err != domain.ErrAgentTokenLimitReached {
		t.Fatalf("expected ErrAgentTokenLimitReached on the 11th token, got %v", err)
	}
}
