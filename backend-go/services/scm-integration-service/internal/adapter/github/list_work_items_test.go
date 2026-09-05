package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// TestListWorkItems_CombinesIssuesAndPullRequests exercises the real
// request/response path against two httptest-served endpoints (issues +
// pulls), mirroring TestListIssues_RealHTTPCall's approach.
func TestListWorkItems_CombinesIssuesAndPullRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/octocat/hello-world/issues":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"number": 1, "title": "issue one", "state": "open",
					"html_url":   "https://github.com/octocat/hello-world/issues/1",
					"updated_at": "2026-01-02T00:00:00Z", "user": map[string]any{"login": "alice"},
					"labels": []map[string]any{{"name": "bug"}},
				},
				{
					// filtered out — issues endpoint also returns PRs.
					"number": 2, "title": "a pr", "state": "open",
					"html_url":     "https://github.com/octocat/hello-world/pull/2",
					"pull_request": map[string]any{},
				},
			})
		case "/repos/octocat/hello-world/pulls":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"number": 3, "title": "pr three", "state": "open", "draft": false,
					"html_url":   "https://github.com/octocat/hello-world/pull/3",
					"updated_at": "2026-01-03T00:00:00Z", "user": map[string]any{"login": "bob"},
				},
				{
					"number": 4, "title": "pr four merged", "state": "closed",
					"html_url":   "https://github.com/octocat/hello-world/pull/4",
					"updated_at": "2026-01-01T00:00:00Z", "merged_at": "2026-01-01T00:00:00Z",
					"user": map[string]any{"login": "carol"},
				},
			})
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	items, err := client.ListWorkItems(context.Background(), usecase.Credential{Token: "gho_faketoken"}, "octocat/hello-world", usecase.WorkItemFilter{Scope: "all", State: "open", Limit: 24})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 1 issue + 2 PRs (the pull_request-flagged issue row filtered out), got %d: %+v", len(items), items)
	}

	byID := map[string]bool{}
	for _, item := range items {
		byID[item.ID] = true
	}
	for _, want := range []string{"issue:1", "pr:3", "pr:4"} {
		if !byID[want] {
			t.Errorf("expected item %q in result, got %+v", want, items)
		}
	}

	for _, item := range items {
		if item.ID == "issue:1" {
			if item.Author != "alice" || len(item.Labels) != 1 || item.Labels[0] != "bug" {
				t.Errorf("issue:1 mapped incorrectly: %+v", item)
			}
		}
		if item.ID == "pr:4" && item.State != "merged" {
			t.Errorf("pr:4 should map to state=merged (has merged_at), got %q", item.State)
		}
		if item.ID == "pr:3" && item.State != "open" {
			t.Errorf("pr:3 should map to state=open, got %q", item.State)
		}
	}
}

func TestListWorkItems_IssueScopeSkipsPullRequestsEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/octocat/hello-world/pulls" {
			t.Fatal("scope=issue should never hit the pulls endpoint")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	items, err := client.ListWorkItems(context.Background(), usecase.Credential{Token: "t"}, "octocat/hello-world", usecase.WorkItemFilter{Scope: "issue", State: "open", Limit: 24})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected no items, got %+v", items)
	}
}

func TestListWorkItems_PullRequestFailureFailsTheWholeCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/octocat/hello-world/issues":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case "/repos/octocat/hello-world/pulls":
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.ListWorkItems(context.Background(), usecase.Credential{Token: "t"}, "octocat/hello-world", usecase.WorkItemFilter{Scope: "all", State: "open", Limit: 24})
	if err == nil {
		t.Fatal("expected a PR-side failure to fail the whole call (asymmetric with issues, see ListWorkItems' doc comment)")
	}
}

func TestListWorkItems_IssuesFailureIsSwallowedSoPullRequestsStillReturn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/octocat/hello-world/issues":
			w.WriteHeader(http.StatusGone) // e.g. issues disabled on this repo
		case "/repos/octocat/hello-world/pulls":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"number": 3, "title": "pr three", "state": "open", "html_url": "https://github.com/o/r/pull/3", "updated_at": "2026-01-03T00:00:00Z"},
			})
		}
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	items, err := client.ListWorkItems(context.Background(), usecase.Credential{Token: "t"}, "octocat/hello-world", usecase.WorkItemFilter{Scope: "all", State: "open", Limit: 24})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].ID != "pr:3" {
		t.Fatalf("expected the PR to still return despite the issues-side failure, got %+v", items)
	}
}
