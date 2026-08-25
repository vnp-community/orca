# TASK-101: Tests for SOL-015 (`jira.*` connection/credential, issue-CRUD, metadata, wscompat)

**From Solution:** SOL-015
**Priority:** P2
**Service:** `issue-tracking-service`, `api-gateway`
**File:** `services/issue-tracking-service/internal/usecase/connect_test.go`, `create_issue_test.go` (extend), `services/issue-tracking-service/internal/adapter/postgres/connections_test.go`, `services/api-gateway/internal/adapter/wscompat/channels_jira_test.go`
**Depends on:** TASK-096, TASK-097, TASK-098, TASK-099, TASK-100
**Status:** `[partial]` — connect_test.go (Connect/Disconnect/GetConnectionStatus/TestConnection usecase tests), create_issue_test.go extensions, and channels_jira_test.go (11 tests) all written and passing. `internal/adapter/postgres/connections_test.go` (TASK-097) is written and type-checks under `-tags=integration` but was NOT executed — no live Postgres/Docker in this environment.

---

## Context

Implements SOL-015's "Test plan" section concretely, following this
codebase's established test patterns: table-driven usecase tests with hand
-written fakes (see `create_dispatch_context_test.go`'s
`fakes_test.go` pattern in `orchestration-service`), `testcontainers`-backed
repository tests (see `issue-tracking-service/internal/adapter/postgres/repository_test.go`),
and `channels_test.go`'s fake-gRPC-client + `r.Dispatch(...)` pattern for
`wscompat`.

## Changes to make

### 1. `internal/usecase/connect_test.go` (new)

```go
package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

// fakeProvider/fakeCredentialResolver/fakeConnectionRepository/fakeProviderRegistry
// mirror this package's existing fakes_test.go pattern (see
// orchestration-service/internal/usecase/fakes_test.go for the sibling
// convention) — implement only the methods each test exercises, panic
// (via t.Fatal in the func body) on an unexpected call.

func TestConnect_WhoamiFailure_NeverPersists(t *testing.T) {
	registry := &fakeProviderRegistry{provider: &fakeProvider{
		whoamiErr: errors.New("401 unauthorized"),
	}}
	connections := &fakeConnectionRepository{}
	credentials := &fakeCredentialResolver{}

	uc := usecase.NewConnect(registry, credentials, connections)
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "tenant-1"), "user-1")

	_, err := uc.Execute(ctx, usecase.ConnectInput{Provider: domain.ProviderJira, SiteURL: "https://x.atlassian.net", Email: "a@b.com", Token: "bad"})
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
	registry := &fakeProviderRegistry{provider: &fakeProvider{
		whoamiViewer: domain.Viewer{ID: "acc-1", DisplayName: "Ada", Email: "ada@x.com"},
	}}
	credentials := &fakeCredentialResolver{writeReturnsID: "cred-123"}
	connections := &fakeConnectionRepository{
		upsertReturns: domain.ConnectionStatus{Connected: true, Viewer: domain.Viewer{ID: "acc-1"}},
	}

	uc := usecase.NewConnect(registry, credentials, connections)
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "tenant-1"), "user-1")

	status, err := uc.Execute(ctx, usecase.ConnectInput{Provider: domain.ProviderJira, SiteURL: "https://x.atlassian.net", Email: "a@b.com", Token: "good"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Connected {
		t.Fatal("expected Connected=true")
	}
	if credentials.writtenCredentialID := connections.upsertCredentialID; credentials.writtenCredentialID != "cred-123" {
		t.Errorf("expected Upsert to receive the credential id Write returned, got %q", credentials.writtenCredentialID)
	}
}
```

Adjust the second test's final assertion syntax to valid Go (the sketch
above has a placeholder `:=` inside an `if` for illustration — write it as
two statements: `got := connections.upsertCredentialID; if got != "cred-123" { ... }`).

### 2. `internal/usecase/create_issue_test.go` — extend for the new signature

Update every existing table case to construct `CreateIssueInput` with the
new fields (`WorkspaceID`, etc. — zero values are fine for cases not
exercising them) and every `fakeCredentialResolver.Resolve` stub to accept
`(ctx, tenantID, userID, provider, workspaceID)`. Add:

```go
func TestCreateIssue_CredentialNotFound_ShortCircuitsBeforeProviderCall(t *testing.T) {
	registry := &fakeProviderRegistry{provider: &fakeProvider{}}
	credentials := &fakeCredentialResolver{resolveErr: usecase.ErrConnectionNotFound}

	uc := usecase.NewCreateIssue(registry, credentials)
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "tenant-1"), "user-1")

	_, err := uc.Execute(ctx, usecase.CreateIssueInput{Provider: domain.ProviderJira, Title: "t", ProjectKey: "PROJ"})
	if err == nil {
		t.Fatal("expected error")
	}
	if registry.provider.(*fakeProvider).createIssueCalled {
		t.Fatal("CreateIssue must not reach the provider when credential resolution fails")
	}
}

func TestCreateIssue_CustomFieldsPassThroughVerbatim(t *testing.T) {
	registry := &fakeProviderRegistry{provider: &fakeProvider{}}
	credentials := &fakeCredentialResolver{}
	uc := usecase.NewCreateIssue(registry, credentials)
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "tenant-1"), "user-1")

	const custom = `{"customfield_10010":"value"}`
	_, err := uc.Execute(ctx, usecase.CreateIssueInput{
		Provider: domain.ProviderJira, Title: "t", ProjectKey: "PROJ", CustomFieldsJSON: custom,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := registry.provider.(*fakeProvider).lastCreateIssueInput.CustomFieldsJSON
	if got != custom {
		t.Errorf("custom fields not passed through verbatim: got %q want %q", got, custom)
	}
}
```

### 3. `internal/adapter/postgres/connections_test.go` (new, `testcontainers`)

Follow `repository_test.go`'s existing `testcontainers`-Postgres setup
exactly (same `//go:build integration`-style guard if that file uses one —
check `repository_test.go`'s build tag before writing this file and match
it).

```go
func TestConnectionsRepository_MultiSiteUpsert_AddsRowDoesNotOverwrite(t *testing.T) {
	repo := newTestRepository(t) // reuse repository_test.go's test-pool helper

	ctx := context.Background()
	ws1 := domain.Workspace{ID: "https://a.atlassian.net", Name: "Site A"}
	ws2 := domain.Workspace{ID: "https://b.atlassian.net", Name: "Site B"}
	viewer := domain.Viewer{ID: "acc-1", DisplayName: "Ada"}

	if _, err := repo.Upsert(ctx, "tenant-1", "user-1", domain.ProviderJira, ws1, viewer, "cred-1"); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if _, err := repo.Upsert(ctx, "tenant-1", "user-1", domain.ProviderJira, ws2, viewer, "cred-2"); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	status, err := repo.GetStatus(ctx, "tenant-1", "user-1", domain.ProviderJira)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if len(status.Workspaces) != 2 {
		t.Fatalf("want 2 connected workspaces, got %d", len(status.Workspaces))
	}
}

func TestConnectionsRepository_SelectWorkspace_MovesIsSelected(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()
	ws1 := domain.Workspace{ID: "site-a"}
	ws2 := domain.Workspace{ID: "site-b"}
	viewer := domain.Viewer{ID: "acc-1"}
	_, _ = repo.Upsert(ctx, "tenant-1", "user-1", domain.ProviderJira, ws1, viewer, "cred-1")
	_, _ = repo.Upsert(ctx, "tenant-1", "user-1", domain.ProviderJira, ws2, viewer, "cred-2")

	status, err := repo.SelectWorkspace(ctx, "tenant-1", "user-1", domain.ProviderJira, "site-b")
	if err != nil {
		t.Fatalf("select workspace: %v", err)
	}
	if status.SelectedWorkspaceID != "site-b" {
		t.Errorf("want selected site-b, got %q", status.SelectedWorkspaceID)
	}
}
```

### 4. `services/api-gateway/internal/adapter/wscompat/channels_jira_test.go` (new)

Mirrors `channels_test.go`'s `TestDevServerListChannel_Success`/
`_PropagatesError` shape — a `fakeIssueTrackingClient` test double, one test
per channel at minimum for the connection group and `createIssue` (the
regression-prone view-translation path), plus the contract test SOL-015's
test plan calls for:

```go
package wscompat

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

type fakeIssueTrackingClient struct {
	issuetrackingv1.IssueTrackingServiceClient

	getConnectionStatusFunc func(ctx context.Context, in *issuetrackingv1.GetConnectionStatusRequest) (*issuetrackingv1.ConnectionStatus, error)
	createIssueFunc         func(ctx context.Context, in *issuetrackingv1.CreateIssueRequest) (*issuetrackingv1.CreateIssueResponse, error)
}

func (f *fakeIssueTrackingClient) GetConnectionStatus(ctx context.Context, in *issuetrackingv1.GetConnectionStatusRequest, _ ...grpc.CallOption) (*issuetrackingv1.ConnectionStatus, error) {
	return f.getConnectionStatusFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) CreateIssue(ctx context.Context, in *issuetrackingv1.CreateIssueRequest, _ ...grpc.CallOption) (*issuetrackingv1.CreateIssueResponse, error) {
	return f.createIssueFunc(ctx, in)
}

func TestJiraStatusChannel_Success(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		getConnectionStatusFunc: func(ctx context.Context, in *issuetrackingv1.GetConnectionStatusRequest) (*issuetrackingv1.ConnectionStatus, error) {
			if in.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA {
				t.Fatalf("want jira provider, got %v", in.GetProvider())
			}
			return &issuetrackingv1.ConnectionStatus{Connected: true, ViewerId: "acc-1"}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "jira.status", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(jiraConnectionStatusView)
	if !ok || !view.Connected {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJiraStatusChannel_PropagatesError(t *testing.T) {
	wantErr := errors.New("issue-tracking-service unavailable")
	fake := &fakeIssueTrackingClient{
		getConnectionStatusFunc: func(ctx context.Context, in *issuetrackingv1.GetConnectionStatusRequest) (*issuetrackingv1.ConnectionStatus, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.status", nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want %v, got %v", wantErr, err)
	}
}

func TestJiraCreateIssueChannel_MapsRequestFieldsAndViewShape(t *testing.T) {
	var gotReq *issuetrackingv1.CreateIssueRequest
	fake := &fakeIssueTrackingClient{
		createIssueFunc: func(ctx context.Context, in *issuetrackingv1.CreateIssueRequest) (*issuetrackingv1.CreateIssueResponse, error) {
			gotReq = in
			return &issuetrackingv1.CreateIssueResponse{Issue: &issuetrackingv1.Issue{Id: "1", Key: "PROJ-1", Title: in.GetTitle()}}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	args := argsJSON(t, map[string]any{"projectKey": "PROJ", "title": "New bug", "issueType": "Bug", "siteId": "https://x.atlassian.net"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "jira.createIssue", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetProjectKey() != "PROJ" || gotReq.GetIssueTypeId() != "Bug" || gotReq.GetWorkspaceId() != "https://x.atlassian.net" {
		t.Fatalf("request fields not mapped correctly: %+v", gotReq)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != true {
		t.Fatalf("unexpected result shape: %+v", result)
	}
}
```

Add one test per remaining channel following this shape (at minimum: a
success-path field-mapping assertion for `listIssues`/`getIssue`/
`updateIssue`/`addIssueComment`/`issueComments`/`listProjects`/
`listCreateFields`, and a `_PropagatesError` test per mutation channel).

`argsJSON`/`fakeInfraFleetClient`'s helpers already exist in
`channels_test.go` (same package) — reuse `argsJSON`, do not redefine it.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/issue-tracking-service/internal/usecase/... -run 'TestConnect|TestCreateIssue' -count=1 -v
go test ./services/issue-tracking-service/internal/adapter/postgres/... -count=1 -v -tags=integration
go test ./services/api-gateway/internal/adapter/wscompat/... -run 'TestJira' -count=1 -v
```

(Drop `-tags=integration` if `repository_test.go` does not use that build
tag — match whatever convention that existing file already follows.)
