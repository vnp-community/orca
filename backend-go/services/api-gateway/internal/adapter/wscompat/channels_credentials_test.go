package wscompat

import (
	"context"
	"errors"
	"testing"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// noopScmFake is a *fakeScmIntegrationClient with no func fields set — safe
// to pass into registerCredentialsChannels for tests that only exercise the
// jira/linear (issue-tracking-service) path, which never calls into scm on
// set/revoke/status (only credentials.list fans out to both unconditionally).
func noopScmFake() *fakeScmIntegrationClient {
	return &fakeScmIntegrationClient{
		listIntegrationCredentialsFunc: func(context.Context, *scmintegrationv1.ListIntegrationCredentialsRequest) (*scmintegrationv1.ListIntegrationCredentialsResponse, error) {
			return &scmintegrationv1.ListIntegrationCredentialsResponse{}, nil
		},
	}
}

// noopIssueFake mirrors noopScmFake for tests exercising only the scm path.
func noopIssueFake() *fakeIssueTrackingClient {
	return &fakeIssueTrackingClient{
		listIntegrationCredentialsFunc: func(context.Context, *issuetrackingv1.ListIntegrationCredentialsRequest) (*issuetrackingv1.ListIntegrationCredentialsResponse, error) {
			return &issuetrackingv1.ListIntegrationCredentialsResponse{}, nil
		},
	}
}

func TestCredentialsSet_Jira_CallsSetIntegrationCredential(t *testing.T) {
	var gotReq *issuetrackingv1.SetIntegrationCredentialRequest
	issueFake := &fakeIssueTrackingClient{
		setIntegrationCredentialFunc: func(_ context.Context, in *issuetrackingv1.SetIntegrationCredentialRequest) (*issuetrackingv1.SetIntegrationCredentialResponse, error) {
			gotReq = in
			return &issuetrackingv1.SetIntegrationCredentialResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, noopScmFake(), issueFake)

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
	issueFake := &fakeIssueTrackingClient{
		setIntegrationCredentialFunc: func(_ context.Context, in *issuetrackingv1.SetIntegrationCredentialRequest) (*issuetrackingv1.SetIntegrationCredentialResponse, error) {
			gotReq = in
			return &issuetrackingv1.SetIntegrationCredentialResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, noopScmFake(), issueFake)

	args := argsJSON(t, map[string]any{"service": "linear", "token": "tok-456"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.set", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR || gotReq.GetToken() != "tok-456" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
}

func TestCredentialsSet_Bitbucket_CallsScmSetIntegrationCredential(t *testing.T) {
	var gotReq *scmintegrationv1.SetIntegrationCredentialRequest
	scmFake := &fakeScmIntegrationClient{
		setIntegrationCredentialFunc: func(_ context.Context, in *scmintegrationv1.SetIntegrationCredentialRequest) (*scmintegrationv1.SetIntegrationCredentialResponse, error) {
			gotReq = in
			return &scmintegrationv1.SetIntegrationCredentialResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, scmFake, noopIssueFake())

	args := argsJSON(t, map[string]any{"service": "bitbucket", "token": "tok-bb", "config": map[string]string{"workspace": "acme"}})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "credentials.set", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetTenantId() != "t1" || gotReq.GetProvider() != scmintegrationv1.ScmProvider_SCM_PROVIDER_BITBUCKET || gotReq.GetToken() != "tok-bb" {
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

func TestCredentialsSet_AzureDevOps_CallsScmSetIntegrationCredential(t *testing.T) {
	var gotReq *scmintegrationv1.SetIntegrationCredentialRequest
	scmFake := &fakeScmIntegrationClient{
		setIntegrationCredentialFunc: func(_ context.Context, in *scmintegrationv1.SetIntegrationCredentialRequest) (*scmintegrationv1.SetIntegrationCredentialResponse, error) {
			gotReq = in
			return &scmintegrationv1.SetIntegrationCredentialResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, scmFake, noopIssueFake())

	args := argsJSON(t, map[string]any{"service": "azure-devops", "token": "tok-ado"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.set", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetProvider() != scmintegrationv1.ScmProvider_SCM_PROVIDER_AZURE_DEVOPS || gotReq.GetToken() != "tok-ado" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
}

func TestCredentialsSet_Gitea_CallsScmSetIntegrationCredential(t *testing.T) {
	var gotReq *scmintegrationv1.SetIntegrationCredentialRequest
	scmFake := &fakeScmIntegrationClient{
		setIntegrationCredentialFunc: func(_ context.Context, in *scmintegrationv1.SetIntegrationCredentialRequest) (*scmintegrationv1.SetIntegrationCredentialResponse, error) {
			gotReq = in
			return &scmintegrationv1.SetIntegrationCredentialResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, scmFake, noopIssueFake())

	args := argsJSON(t, map[string]any{"service": "gitea", "token": "tok-gitea"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.set", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetProvider() != scmintegrationv1.ScmProvider_SCM_PROVIDER_GITEA || gotReq.GetToken() != "tok-gitea" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
}

func TestCredentialsSet_UnknownService_ErrorsWithoutCallingClient(t *testing.T) {
	scmCalled, issueCalled := false, false
	scmFake := &fakeScmIntegrationClient{
		setIntegrationCredentialFunc: func(context.Context, *scmintegrationv1.SetIntegrationCredentialRequest) (*scmintegrationv1.SetIntegrationCredentialResponse, error) {
			scmCalled = true
			return &scmintegrationv1.SetIntegrationCredentialResponse{}, nil
		},
	}
	issueFake := &fakeIssueTrackingClient{
		setIntegrationCredentialFunc: func(context.Context, *issuetrackingv1.SetIntegrationCredentialRequest) (*issuetrackingv1.SetIntegrationCredentialResponse, error) {
			issueCalled = true
			return &issuetrackingv1.SetIntegrationCredentialResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, scmFake, issueFake)

	// "github"/"gitlab" are real scmintegrationv1.ScmProvider values but NOT
	// in scmCredentialProviders — credentials.* deliberately only fans out
	// bitbucket/azure-devops/gitea to scm-integration-service (SOL-007);
	// github/gitlab credentials aren't managed through this channel group.
	args := argsJSON(t, map[string]any{"service": "github", "token": "x"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.set", args)
	if err == nil {
		t.Fatal("expected CREDENTIALS_UNKNOWN_SERVICE error")
	}
	if scmCalled || issueCalled {
		t.Error("expected neither backend to be called for an unrecognized service")
	}
}

func TestCredentialsRevoke_CallsRevokeAuth(t *testing.T) {
	var gotReq *issuetrackingv1.RevokeAuthRequest
	issueFake := &fakeIssueTrackingClient{
		revokeAuthFunc: func(_ context.Context, in *issuetrackingv1.RevokeAuthRequest) (*issuetrackingv1.RevokeAuthResponse, error) {
			gotReq = in
			return &issuetrackingv1.RevokeAuthResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, noopScmFake(), issueFake)

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

func TestCredentialsRevoke_Bitbucket_CallsScmRevokeAuth(t *testing.T) {
	var gotReq *scmintegrationv1.RevokeAuthRequest
	scmFake := &fakeScmIntegrationClient{
		revokeAuthFunc: func(_ context.Context, in *scmintegrationv1.RevokeAuthRequest) (*scmintegrationv1.RevokeAuthResponse, error) {
			gotReq = in
			return &scmintegrationv1.RevokeAuthResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, scmFake, noopIssueFake())

	args := argsJSON(t, map[string]any{"service": "bitbucket"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.revoke", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetProvider() != scmintegrationv1.ScmProvider_SCM_PROVIDER_BITBUCKET {
		t.Errorf("expected bitbucket provider, got %v", gotReq.GetProvider())
	}
	m, ok := result.(map[string]bool)
	if !ok || !m["success"] {
		t.Fatalf("expected {success:true}, got %+v", result)
	}
}

func TestCredentialsRevoke_UnknownService_Errors(t *testing.T) {
	r := NewRegistry()
	registerCredentialsChannels(r, noopScmFake(), noopIssueFake())

	args := argsJSON(t, map[string]any{"service": "gitlab"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.revoke", args)
	if err == nil {
		t.Fatal("expected CREDENTIALS_UNKNOWN_SERVICE error")
	}
}

func TestCredentialsRevoke_PropagatesError(t *testing.T) {
	wantErr := errors.New("issue-tracking-service unavailable")
	issueFake := &fakeIssueTrackingClient{
		revokeAuthFunc: func(context.Context, *issuetrackingv1.RevokeAuthRequest) (*issuetrackingv1.RevokeAuthResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, noopScmFake(), issueFake)

	args := argsJSON(t, map[string]any{"service": "linear"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.revoke", args)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestCredentialsStatus_Configured_DecodesConfigJSON(t *testing.T) {
	issueFake := &fakeIssueTrackingClient{
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
	registerCredentialsChannels(r, noopScmFake(), issueFake)

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
	issueFake := &fakeIssueTrackingClient{
		getIntegrationCredentialStatusFunc: func(context.Context, *issuetrackingv1.GetIntegrationCredentialStatusRequest) (*issuetrackingv1.GetIntegrationCredentialStatusResponse, error) {
			return &issuetrackingv1.GetIntegrationCredentialStatusResponse{Configured: false}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, noopScmFake(), issueFake)

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

func TestCredentialsStatus_AzureDevOps_QueriesScm(t *testing.T) {
	scmFake := &fakeScmIntegrationClient{
		getIntegrationCredentialStatusFunc: func(_ context.Context, in *scmintegrationv1.GetIntegrationCredentialStatusRequest) (*scmintegrationv1.GetIntegrationCredentialStatusResponse, error) {
			if in.GetProvider() != scmintegrationv1.ScmProvider_SCM_PROVIDER_AZURE_DEVOPS {
				t.Fatalf("want azure-devops provider, got %v", in.GetProvider())
			}
			return &scmintegrationv1.GetIntegrationCredentialStatusResponse{Configured: true, ConfigJson: `{"org":"acme"}`}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, scmFake, noopIssueFake())

	args := argsJSON(t, map[string]any{"service": "azure-devops"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.status", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(credentialsStatusView)
	if !ok || !view.Configured || view.Mode != "server" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if view.Config["org"] != "acme" {
		t.Errorf("expected decoded org, got %+v", view.Config)
	}
}

func TestCredentialsStatus_UnknownService_ErrorsWithoutCallingClient(t *testing.T) {
	scmCalled, issueCalled := false, false
	scmFake := &fakeScmIntegrationClient{
		getIntegrationCredentialStatusFunc: func(context.Context, *scmintegrationv1.GetIntegrationCredentialStatusRequest) (*scmintegrationv1.GetIntegrationCredentialStatusResponse, error) {
			scmCalled = true
			return &scmintegrationv1.GetIntegrationCredentialStatusResponse{}, nil
		},
	}
	issueFake := &fakeIssueTrackingClient{
		getIntegrationCredentialStatusFunc: func(context.Context, *issuetrackingv1.GetIntegrationCredentialStatusRequest) (*issuetrackingv1.GetIntegrationCredentialStatusResponse, error) {
			issueCalled = true
			return &issuetrackingv1.GetIntegrationCredentialStatusResponse{}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, scmFake, issueFake)

	args := argsJSON(t, map[string]any{"service": "gitlab"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.status", args)
	if err == nil {
		t.Fatal("expected CREDENTIALS_UNKNOWN_SERVICE error")
	}
	if scmCalled || issueCalled {
		t.Error("expected neither backend to be called for an unrecognized service")
	}
}

func TestCredentialsList_MergesJiraAndLinear(t *testing.T) {
	issueFake := &fakeIssueTrackingClient{
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
	registerCredentialsChannels(r, noopScmFake(), issueFake)

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

func TestCredentialsList_MergesAcrossBothServices(t *testing.T) {
	scmFake := &fakeScmIntegrationClient{
		listIntegrationCredentialsFunc: func(_ context.Context, in *scmintegrationv1.ListIntegrationCredentialsRequest) (*scmintegrationv1.ListIntegrationCredentialsResponse, error) {
			if in.GetTenantId() != "t1" {
				t.Fatalf("expected tenant t1, got %q", in.GetTenantId())
			}
			return &scmintegrationv1.ListIntegrationCredentialsResponse{
				ConfiguredProviders: []scmintegrationv1.ScmProvider{
					scmintegrationv1.ScmProvider_SCM_PROVIDER_BITBUCKET,
					scmintegrationv1.ScmProvider_SCM_PROVIDER_GITEA,
				},
			}, nil
		},
	}
	issueFake := &fakeIssueTrackingClient{
		listIntegrationCredentialsFunc: func(context.Context, *issuetrackingv1.ListIntegrationCredentialsRequest) (*issuetrackingv1.ListIntegrationCredentialsResponse, error) {
			return &issuetrackingv1.ListIntegrationCredentialsResponse{
				ConfiguredProviders: []issuetrackingv1.IssueProvider{issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA},
			}, nil
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, scmFake, issueFake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(credentialsListView)
	if !ok || view.Mode != "server" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(view.Services) != 3 {
		t.Fatalf("expected 3 configured services, got %v", view.Services)
	}
	want := map[string]bool{"bitbucket": true, "gitea": true, "jira": true}
	for _, s := range view.Services {
		if !want[s] {
			t.Errorf("unexpected service %q in %v", s, view.Services)
		}
		delete(want, s)
	}
	if len(want) != 0 {
		t.Errorf("missing expected services: %v", want)
	}
}

func TestCredentialsList_PropagatesScmError(t *testing.T) {
	wantErr := errors.New("scm-integration-service unavailable")
	scmFake := &fakeScmIntegrationClient{
		listIntegrationCredentialsFunc: func(context.Context, *scmintegrationv1.ListIntegrationCredentialsRequest) (*scmintegrationv1.ListIntegrationCredentialsResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, scmFake, noopIssueFake())

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.list", nil)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestCredentialsList_PropagatesError(t *testing.T) {
	wantErr := errors.New("issue-tracking-service unavailable")
	issueFake := &fakeIssueTrackingClient{
		listIntegrationCredentialsFunc: func(context.Context, *issuetrackingv1.ListIntegrationCredentialsRequest) (*issuetrackingv1.ListIntegrationCredentialsResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerCredentialsChannels(r, noopScmFake(), issueFake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "credentials.list", nil)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}
