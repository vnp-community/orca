package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// TestListIssues_RealHTTPCall exercises the real request/response path — a
// GET request with a proper Authorization header, against a real JSON
// response shape modeled on GitHub's
// GET /repos/{owner}/{repo}/issues endpoint. httptest.Server stands in for
// api.github.com so this test has no network dependency.
func TestListIssues_RealHTTPCall(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"number":   42,
				"title":    "something is broken",
				"state":    "open",
				"html_url": "https://github.com/octocat/hello-world/issues/42",
			},
			{
				// GitHub's issues endpoint also returns pull requests;
				// this one must be filtered out.
				"number":       7,
				"title":        "a pull request, not an issue",
				"state":        "open",
				"html_url":     "https://github.com/octocat/hello-world/pull/7",
				"pull_request": map[string]any{"url": "https://api.github.com/repos/octocat/hello-world/pulls/7"},
			},
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	issues, err := client.ListIssues(context.Background(), usecase.Credential{Token: "gho_faketoken"}, "octocat/hello-world", usecase.IssueFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/repos/octocat/hello-world/issues" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if gotAuth != "Bearer gho_faketoken" {
		t.Errorf("expected Authorization header to carry the resolved credential, got %q", gotAuth)
	}

	if len(issues) != 1 {
		t.Fatalf("expected the pull-request entry to be filtered out, got %d issues", len(issues))
	}
	if issues[0].ID != "42" || issues[0].Title != "something is broken" || issues[0].State != "open" {
		t.Errorf("unexpected issue: %+v", issues[0])
	}
	if issues[0].Repo != "octocat/hello-world" {
		t.Errorf("expected repo to be set on the mapped issue, got %q", issues[0].Repo)
	}
}

func TestListIssues_NonOKStatusIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.ListIssues(context.Background(), usecase.Credential{Token: "bad-token"}, "octocat/hello-world", usecase.IssueFilter{})
	if err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

func TestUnimplementedMethods_ReturnErrNotImplemented(t *testing.T) {
	client := New(nil, "")
	ctx := context.Background()
	cred := usecase.Credential{Token: "tok"}

	if _, err := client.CreatePullRequest(ctx, cred, "a/b", usecase.CreatePullRequestInput{}); err != ErrNotImplemented {
		t.Errorf("expected ErrNotImplemented, got %v", err)
	}
	if _, err := client.ListPullRequests(ctx, cred, "a/b"); err != ErrNotImplemented {
		t.Errorf("expected ErrNotImplemented, got %v", err)
	}
	if _, err := client.GetRateLimitStatus(ctx, cred); err != ErrNotImplemented {
		t.Errorf("expected ErrNotImplemented, got %v", err)
	}
}
