package wscompat

import (
	"context"
	"errors"
	"testing"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

func TestCredentialsSet_Jira_CallsSetIntegrationCredential(t *testing.T) {
	var gotReq *issuetrackingv1.SetIntegrationCredentialRequest
	fake := &fakeIssueTrackingClient{
		setIntegrationCredentialFunc: func(_ context.Context, in *issuetrackingv1.SetIntegrationCredentialRequest) (*issuetrackingv1.SetIntegrationCredentialResponse, error) {
			gotReq = in
			return &issuetrackingv1.SetIntegrationCredentialResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, fake)

	args := argsJSON(t, map[string]any{"service": "jira", "token": "tok-123", "config": map[string]string{"baseUrl": "https://x.atlassian.net", "email": "a@b.com"}})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "credentials.set", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetTenantId() != "t1" || gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA || gotReq.GetToken() != "tok-123" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	if gotReq.GetConfigJson() == "" {
		t.Error("expected non-empty config_json")
	}
	m, ok := result.(map[string]bool)
	if !ok || !m["success"] {
		t.Fatalf("expected {success:true}, got %+v", result)
	}
}

func TestCredentialsSet_Linear_CallsSetIntegrationCredential(t *testing.T) {
	var gotReq *issuetrackingv1.SetIntegrationCredentialRequest
	fake := &fakeIssueTrackingClient{
		setIntegrationCredentialFunc: func(_ context.Context, in *issuetrackingv1.SetIntegrationCredentialRequest) (*issuetrackingv1.SetIntegrationCredentialResponse, error) {
			gotReq = in
			return &issuetrackingv1.SetIntegrationCredentialResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, fake)

	args := argsJSON(t, map[string]any{"service": "linear", "token": "tok-456"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.set", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR || gotReq.GetToken() != "tok-456" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
}

func TestCredentialsSet_UnknownService_ErrorsWithoutCallingClient(t *testing.T) {
	called := false
	fake := &fakeIssueTrackingClient{
		setIntegrationCredentialFunc: func(context.Context, *issuetrackingv1.SetIntegrationCredentialRequest) (*issuetrackingv1.SetIntegrationCredentialResponse, error) {
			called = true
			return &issuetrackingv1.SetIntegrationCredentialResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, fake)

	args := argsJSON(t, map[string]any{"service": "bitbucket", "token": "x"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.set", args)
	if err == nil {
		t.Fatal("expected CREDENTIALS_UNKNOWN_SERVICE error")
	}
	if called {
		t.Error("expected SetIntegrationCredential to never be called for an unrecognized service")
	}
}

func TestCredentialsRevoke_CallsRevokeAuth(t *testing.T) {
	var gotReq *issuetrackingv1.RevokeAuthRequest
	fake := &fakeIssueTrackingClient{
		revokeAuthFunc: func(_ context.Context, in *issuetrackingv1.RevokeAuthRequest) (*issuetrackingv1.RevokeAuthResponse, error) {
			gotReq = in
			return &issuetrackingv1.RevokeAuthResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, fake)

	args := argsJSON(t, map[string]any{"service": "jira"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.revoke", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA {
		t.Errorf("expected jira provider, got %v", gotReq.GetProvider())
	}
	m, ok := result.(map[string]bool)
	if !ok || !m["success"] {
		t.Fatalf("expected {success:true}, got %+v", result)
	}
}

func TestCredentialsRevoke_UnknownService_Errors(t *testing.T) {
	fake := &fakeIssueTrackingClient{}
	r := NewRegistry()
	registerCredentialsChannels(r, fake)

	args := argsJSON(t, map[string]any{"service": "azure-devops"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.revoke", args)
	if err == nil {
		t.Fatal("expected CREDENTIALS_UNKNOWN_SERVICE error")
	}
}

func TestCredentialsRevoke_PropagatesError(t *testing.T) {
	wantErr := errors.New("issue-tracking-service unavailable")
	fake := &fakeIssueTrackingClient{
		revokeAuthFunc: func(context.Context, *issuetrackingv1.RevokeAuthRequest) (*issuetrackingv1.RevokeAuthResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, fake)

	args := argsJSON(t, map[string]any{"service": "linear"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.revoke", args)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestCredentialsStatus_Configured_DecodesConfigJSON(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		getIntegrationCredentialStatusFunc: func(_ context.Context, in *issuetrackingv1.GetIntegrationCredentialStatusRequest) (*issuetrackingv1.GetIntegrationCredentialStatusResponse, error) {
			if in.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA {
				t.Fatalf("want jira provider, got %v", in.GetProvider())
			}
			return &issuetrackingv1.GetIntegrationCredentialStatusResponse{
				Configured: true, ConfigJson: `{"baseUrl":"https://x.atlassian.net"}`,
			}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, fake)

	args := argsJSON(t, map[string]any{"service": "jira"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.status", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(credentialsStatusView)
	if !ok || !view.Configured || view.Mode != "server" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if view.Config["baseUrl"] != "https://x.atlassian.net" {
		t.Errorf("expected decoded baseUrl, got %+v", view.Config)
	}
}

func TestCredentialsStatus_NotConfigured(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		getIntegrationCredentialStatusFunc: func(context.Context, *issuetrackingv1.GetIntegrationCredentialStatusRequest) (*issuetrackingv1.GetIntegrationCredentialStatusResponse, error) {
			return &issuetrackingv1.GetIntegrationCredentialStatusResponse{Configured: false}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, fake)

	args := argsJSON(t, map[string]any{"service": "linear"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.status", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(credentialsStatusView)
	if !ok || view.Configured || view.Config != nil {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCredentialsStatus_UnknownService_ErrorsWithoutCallingClient(t *testing.T) {
	called := false
	fake := &fakeIssueTrackingClient{
		getIntegrationCredentialStatusFunc: func(context.Context, *issuetrackingv1.GetIntegrationCredentialStatusRequest) (*issuetrackingv1.GetIntegrationCredentialStatusResponse, error) {
			called = true
			return &issuetrackingv1.GetIntegrationCredentialStatusResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, fake)

	args := argsJSON(t, map[string]any{"service": "gitea"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.status", args)
	if err == nil {
		t.Fatal("expected CREDENTIALS_UNKNOWN_SERVICE error")
	}
	if called {
		t.Error("expected GetIntegrationCredentialStatus to never be called for an unrecognized service")
	}
}

func TestCredentialsList_MergesJiraAndLinear(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listIntegrationCredentialsFunc: func(_ context.Context, in *issuetrackingv1.ListIntegrationCredentialsRequest) (*issuetrackingv1.ListIntegrationCredentialsResponse, error) {
			if in.GetTenantId() != "t1" {
				t.Fatalf("expected tenant t1, got %q", in.GetTenantId())
			}
			return &issuetrackingv1.ListIntegrationCredentialsResponse{
				ConfiguredProviders: []issuetrackingv1.IssueProvider{
					issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA,
					issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR,
				},
			}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(credentialsListView)
	if !ok || view.Mode != "server" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(view.Services) != 2 || view.Services[0] != "jira" || view.Services[1] != "linear" {
		t.Errorf("expected [jira linear], got %v", view.Services)
	}
}

func TestCredentialsList_PropagatesError(t *testing.T) {
	wantErr := errors.New("issue-tracking-service unavailable")
	fake := &fakeIssueTrackingClient{
		listIntegrationCredentialsFunc: func(context.Context, *issuetrackingv1.ListIntegrationCredentialsRequest) (*issuetrackingv1.ListIntegrationCredentialsResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.list", nil)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}
