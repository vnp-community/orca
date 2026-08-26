# TASK-107: Tests for SOL-016 (`linear.*` team/custom-view usecases, GraphQL adapter, wscompat)

**From Solution:** SOL-016
**Priority:** P2
**Service:** `issue-tracking-service`, `api-gateway`
**File:** `services/issue-tracking-service/internal/usecase/list_teams_test.go`, `get_custom_view_test.go`, `services/issue-tracking-service/internal/adapter/linear/client_test.go` (extend), `services/api-gateway/internal/adapter/wscompat/channels_linear_test.go`
**Depends on:** TASK-102, TASK-103, TASK-104, TASK-105, TASK-106
**Status:** `[partial]` — usecase/adapter/wscompat tests written and passing (30 new wscompat tests total across the group). Postgres integration test halves written, compile under `-tags=integration`, but not executed — no Docker/Postgres in this environment. Worktree `agent-a412325f0d1276bb5`, committed as `c29ca9e6a`.

---

## Context

Implements SOL-016's "Test plan" section: the 5 Linear-only usecases
(`ListTeams`/`ListTeamLabels`/`ListTeamMembers`/`GetCustomView`/
`ListWorkflowStates`), the GraphQL request/response mapping for the new
adapter queries, and `wscompat`'s `linear.listIssues` envelope-shape
regression guard plus the "no false unification" cross-provider guard.

## Changes to make

### 1. `internal/usecase/list_teams_test.go`, `get_custom_view_test.go` (new)

Same fakes-based pattern as TASK-101's `connect_test.go`:

```go
// list_teams_test.go
package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

func TestListTeams_AlwaysResolvesLinearProvider(t *testing.T) {
	var gotProvider domain.Provider
	registry := &fakeProviderRegistry{
		resolveFunc: func(p domain.Provider) (usecase.IssueTrackerProvider, error) {
			gotProvider = p
			return &fakeProvider{listTeamsReturns: []domain.Team{{ID: "team-1", Name: "Engineering", Key: "ENG"}}}, nil
		},
	}
	credentials := &fakeCredentialResolver{}
	uc := usecase.NewListTeams(registry, credentials)
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "tenant-1"), "user-1")

	teams, err := uc.Execute(ctx, usecase.ListTeamsInput{WorkspaceID: "ws-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotProvider != domain.ProviderLinear {
		t.Errorf("want ProviderLinear, got %v — ListTeams must never resolve Jira, even if a caller-supplied Provider field existed", gotProvider)
	}
	if len(teams) != 1 || teams[0].Key != "ENG" {
		t.Errorf("unexpected teams: %+v", teams)
	}
}

func TestGetCustomView_NotFound_ReturnsError(t *testing.T) {
	registry := &fakeProviderRegistry{provider: &fakeProvider{getCustomViewErr: usecase.ErrConnectionNotFound}}
	credentials := &fakeCredentialResolver{}
	uc := usecase.NewGetCustomView(registry, credentials)
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "tenant-1"), "user-1")

	_, err := uc.Execute(ctx, usecase.GetCustomViewInput{ViewID: "missing", Model: "issue"})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

(Extend this package's shared `fakeProvider`/`fakeProviderRegistry`
test-double, added in TASK-101, with `listTeamsReturns`/
`getCustomViewErr` fields and a `resolveFunc` override hook if not already
present — do not redefine the fakes in a second file.)

### 2. `internal/adapter/linear/client_test.go` — extend

Mirror this file's existing `ListIssues`/`CreateIssue` mocked-HTTP-transport
test shape (a `httptest.Server` or a custom `http.RoundTripper` stubbing
Linear's GraphQL endpoint) for the new queries:

```go
func TestClient_ListTeams_ParsesGraphQLResponse(t *testing.T) {
	srv := newFakeLinearServer(t, map[string]any{
		"data": map[string]any{
			"teams": map[string]any{
				"nodes": []map[string]any{{"id": "team-1", "name": "Engineering", "key": "ENG"}},
			},
		},
	})
	defer srv.Close()
	c := New(srv.Client()) // adjust to however client_test.go already redirects the endpoint const

	teams, err := c.ListTeams(context.Background(), usecase.Credential{Token: "tok"}, "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(teams) != 1 || teams[0].Key != "ENG" {
		t.Fatalf("unexpected teams: %+v", teams)
	}
}

func TestClient_GetCustomView_ParsesGraphQLResponse(t *testing.T) {
	srv := newFakeLinearServer(t, map[string]any{
		"data": map[string]any{
			"customView": map[string]any{"id": "view-1", "name": "My View", "team": map[string]any{"id": "team-1"}},
		},
	})
	defer srv.Close()
	c := New(srv.Client())

	view, err := c.GetCustomView(context.Background(), usecase.Credential{Token: "tok"}, "view-1", "issue")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.ID != "view-1" || view.TeamID != "team-1" {
		t.Fatalf("unexpected view: %+v", view)
	}
}
```

Add one more test each for `ListTeamLabels`/`ListTeamMembers`/
`ListWorkflowStates` following the same shape. Match whatever HTTP-stubbing
helper (`newFakeLinearServer` or equivalent) `client_test.go` already
defines — do not introduce a second one.

### 3. `services/api-gateway/internal/adapter/wscompat/channels_linear_test.go` (new)

```go
package wscompat

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

func TestLinearListIssuesChannel_ReturnsItemsHasMoreEnvelope(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listIssuesFunc: func(ctx context.Context, in *issuetrackingv1.ListIssuesRequest) (*issuetrackingv1.ListIssuesResponse, error) {
			return &issuetrackingv1.ListIssuesResponse{Issues: []*issuetrackingv1.Issue{{Id: "1", Key: "ENG-1", Title: "Bug"}}}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "linear.listIssues", argsJSON(t, map[string]any{"teamKey": "ENG"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	envelope, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("want map envelope, got %T", result)
	}
	if _, hasItems := envelope["items"]; !hasItems {
		t.Fatal("linear.listIssues must return an {items, hasMore} envelope, not a bare array — regression guard vs jira.listIssues's bare-array shape")
	}
	if envelope["hasMore"] != false {
		t.Errorf("want hasMore=false, got %v", envelope["hasMore"])
	}
}

func TestJiraListIssuesChannel_ReturnsBareArray_NotEnvelope(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listIssuesFunc: func(ctx context.Context, in *issuetrackingv1.ListIssuesRequest) (*issuetrackingv1.ListIssuesResponse, error) {
			return &issuetrackingv1.ListIssuesResponse{Issues: []*issuetrackingv1.Issue{{Id: "1", Key: "PROJ-1", Title: "Bug"}}}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "jira.listIssues", argsJSON(t, map[string]any{"projectKey": "PROJ"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.([]jiraIssueView); !ok {
		t.Fatalf("jira.listIssues must return a bare []jiraIssueView, not an envelope — got %T", result)
	}
}

func TestLinearListTeamsChannel_NeverReturnsJiraProjectRows(t *testing.T) {
	// Regression guard for SOL-016's "no false unification" design: a fake
	// client that would answer ListProjects with Jira-shaped data must never
	// be reachable from linear.listTeams — asserted by construction, since
	// registerLinearChannels's linear.listTeams handler only ever calls
	// client.ListTeams (see TASK-106's grep-based Verify step for the
	// static-code-shape guard; this test exercises the runtime behavior).
	fake := &fakeIssueTrackingClient{
		listTeamsFunc: func(ctx context.Context, in *issuetrackingv1.ListTeamsRequest) (*issuetrackingv1.ListTeamsResponse, error) {
			return &issuetrackingv1.ListTeamsResponse{Teams: []*issuetrackingv1.Team{{Id: "team-1", Name: "Engineering", Key: "ENG"}}}, nil
		},
		listProjectsFunc: func(ctx context.Context, in *issuetrackingv1.ListProjectsRequest) (*issuetrackingv1.ListProjectsResponse, error) {
			t.Fatal("linear.listTeams must never call ListProjects")
			return nil, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "linear.listTeams", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]linearTeamView)
	if !ok || len(views) != 1 || views[0].Key != "ENG" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
```

Extend `fakeIssueTrackingClient` (TASK-101, same package) with
`listIssuesFunc`/`listTeamsFunc`/`listProjectsFunc` fields and their method
overrides, following the existing `getConnectionStatusFunc`/
`createIssueFunc` pattern — do not create a second fake client type.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/issue-tracking-service/internal/usecase/... -run 'TestListTeams|TestGetCustomView' -count=1 -v
go test ./services/issue-tracking-service/internal/adapter/linear/... -count=1 -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run 'TestLinear|TestJiraListIssuesChannel' -count=1 -v
```
