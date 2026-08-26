package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

func TestConnect_WhoamiFailure_NeverPersists(t *testing.T) {
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: &fakeIssueTrackerProvider{
		whoamiErr: errors.New("401 unauthorized"),
	}}
	connections := &fakeConnectionRepository{}
	credentials := &fakeCredentialResolver{}

	uc := NewConnect(registry, credentials, connections)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	_, err := uc.Execute(ctx, ConnectInput{Provider: domain.ProviderJira, SiteURL: "https://x.atlassian.net", Email: "a@b.com", Token: "bad"})
	if err == nil {
		t.Fatal("expected error")
	}
	if connections.upsertCalled {
		t.Fatal("Upsert must not be called when Whoami fails — an invalid token must not create a connected row")
	}
	if credentials.writeCalled {
		t.Fatal("Write must not be called when Whoami fails")
	}
}

func TestConnect_Success_PersistsCredentialThenConnection(t *testing.T) {
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: &fakeIssueTrackerProvider{
		whoamiViewer: domain.Viewer{ID: "acc-1", DisplayName: "Ada", Email: "ada@x.com"},
	}}
	credentials := &fakeCredentialResolver{writeReturnsID: "cred-123"}
	connections := &fakeConnectionRepository{
		upsertReturns: domain.ConnectionStatus{Connected: true, Viewer: domain.Viewer{ID: "acc-1"}},
	}

	uc := NewConnect(registry, credentials, connections)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	status, err := uc.Execute(ctx, ConnectInput{Provider: domain.ProviderJira, SiteURL: "https://x.atlassian.net", Email: "a@b.com", Token: "good"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Connected {
		t.Fatal("expected Connected=true")
	}
	got := connections.upsertCredentialID
	if got != "cred-123" {
		t.Errorf("expected Upsert to receive the credential id Write returned, got %q", got)
	}
}

func TestConnect_RequiresSiteURLForJira(t *testing.T) {
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: &fakeIssueTrackerProvider{}}
	uc := NewConnect(registry, &fakeCredentialResolver{}, &fakeConnectionRepository{})
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	_, err := uc.Execute(ctx, ConnectInput{Provider: domain.ProviderJira, Token: "tok"})
	if err == nil {
		t.Fatal("expected an error when site_url is empty for jira")
	}
}

func TestConnect_LinearDoesNotRequireSiteURL(t *testing.T) {
	registry := &fakeProviderRegistry{provider: domain.ProviderLinear, impl: &fakeIssueTrackerProvider{
		whoamiViewer: domain.Viewer{ID: "u-1"},
	}}
	credentials := &fakeCredentialResolver{writeReturnsID: "cred-1"}
	connections := &fakeConnectionRepository{upsertReturns: domain.ConnectionStatus{Connected: true}}
	uc := NewConnect(registry, credentials, connections)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	_, err := uc.Execute(ctx, ConnectInput{Provider: domain.ProviderLinear, Token: "tok"})
	if err != nil {
		t.Fatalf("unexpected error for a Linear connect with no site_url: %v", err)
	}
}

func TestDisconnect_RequiresTenantContext(t *testing.T) {
	uc := NewDisconnect(&fakeConnectionRepository{})
	err := uc.Execute(context.Background(), DisconnectInput{Provider: domain.ProviderJira})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestGetConnectionStatus_NotConnected_IsNotAnError(t *testing.T) {
	connections := &fakeConnectionRepository{getStatusReturns: domain.ConnectionStatus{Connected: false}}
	uc := NewGetConnectionStatus(connections)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	status, err := uc.Execute(ctx, GetConnectionStatusInput{Provider: domain.ProviderJira})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Connected {
		t.Fatal("expected Connected=false")
	}
}

func TestTestConnection_AuthFailure_ReturnsOKFalseNotError(t *testing.T) {
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: &fakeIssueTrackerProvider{
		whoamiErr: errors.New("bad token"),
	}}
	credentials := &fakeCredentialResolver{cred: Credential{Token: "tok"}}
	uc := NewTestConnection(registry, credentials)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	result, err := uc.Execute(ctx, TestConnectionInput{Provider: domain.ProviderJira})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OK {
		t.Fatal("expected OK=false")
	}
	if result.Error == "" {
		t.Fatal("expected an error message")
	}
}
