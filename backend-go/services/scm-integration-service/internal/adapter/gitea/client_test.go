package gitea

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// TestListIssues_RealHTTPCall exercises the real request/response path — a
// GET request with a proper Authorization header, against a real JSON
// response shape modeled on Gitea's GET /repos/{owner}/{repo}/issues
// endpoint. httptest.Server stands in for a self-hosted Gitea instance so
// this test has no network dependency.
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
				"html_url": "https://gitea.example.com/octocat/hello-world/issues/42",
			},
			{
				// Gitea's issues endpoint also returns pull requests;
				// this one must be filtered out.
				"number":       7,
				"title":        "a pull request, not an issue",
				"state":        "open",
				"html_url":     "https://gitea.example.com/octocat/hello-world/pulls/7",
				"pull_request": map[string]any{"url": "https://gitea.example.com/api/v1/repos/octocat/hello-world/pulls/7"},
			},
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	issues, err := client.ListIssues(context.Background(), usecase.Credential{Token: "gta_faketoken"}, "octocat/hello-world", usecase.IssueFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/repos/octocat/hello-world/issues" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if gotAuth != "Bearer gta_faketoken" {
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

func TestCreatePullRequest_RealHTTPCall(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"number":   9,
			"title":    "add feature",
			"state":    "open",
			"html_url": "https://gitea.example.com/octocat/hello-world/pulls/9",
			"head":     map[string]any{"ref": "feature"},
			"base":     map[string]any{"ref": "main"},
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	pr, err := client.CreatePullRequest(context.Background(), usecase.Credential{Token: "gta_faketoken"}, "octocat/hello-world", usecase.CreatePullRequestInput{
		Title: "add feature", Body: "desc", HeadBranch: "feature", BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/repos/octocat/hello-world/pulls" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if gotAuth != "Bearer gta_faketoken" {
		t.Errorf("expected Authorization header to carry the resolved credential, got %q", gotAuth)
	}
	if gotBody["head"] != "feature" || gotBody["base"] != "main" {
		t.Errorf("unexpected request body: %+v", gotBody)
	}
	if pr.ID != "9" || pr.State != "open" || pr.HeadBranch != "feature" || pr.BaseBranch != "main" {
		t.Errorf("unexpected pull request: %+v", pr)
	}
}

func TestCreatePullRequest_NonCreatedStatusIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.CreatePullRequest(context.Background(), usecase.Credential{Token: "tok"}, "a/b", usecase.CreatePullRequestInput{Title: "t", HeadBranch: "h", BaseBranch: "b"})
	if err == nil {
		t.Fatal("expected an error for a non-201 response")
	}
}

func TestListPullRequests_RealHTTPCall(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"number":   3,
				"title":    "fix bug",
				"state":    "open",
				"html_url": "https://gitea.example.com/octocat/hello-world/pulls/3",
				"head":     map[string]any{"ref": "fix"},
				"base":     map[string]any{"ref": "main"},
			},
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	prs, err := client.ListPullRequests(context.Background(), usecase.Credential{Token: "gta_faketoken"}, "octocat/hello-world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/repos/octocat/hello-world/pulls" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if len(prs) != 1 || prs[0].ID != "3" || prs[0].HeadBranch != "fix" {
		t.Errorf("unexpected pull requests: %+v", prs)
	}
}

func TestListPullRequests_NonOKStatusIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.ListPullRequests(context.Background(), usecase.Credential{Token: "tok"}, "a/b")
	if err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

// TestGetRateLimitStatus_ReturnsErrCapabilityUnsupported verifies this
// adapter never makes an HTTP call for GetRateLimitStatus — it always
// reports the capability as unsupported, per its doc comment.
func TestGetRateLimitStatus_ReturnsErrCapabilityUnsupported(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.GetRateLimitStatus(context.Background(), usecase.Credential{Token: "gta_faketoken"})
	if !errors.Is(err, ErrCapabilityUnsupported) {
		t.Fatalf("expected ErrCapabilityUnsupported, got %v", err)
	}
	if called {
		t.Error("expected GetRateLimitStatus not to make an HTTP call")
	}
}

// TestCreatePullRequest_DraftSerializesIntoPayload asserts Draft: true
// serializes into the "draft" JSON field — BR-CR-20 (requires Gitea >=
// 1.16, not verifiable here beyond the payload shape).
func TestCreatePullRequest_DraftSerializesIntoPayload(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"number": 9, "title": "wip", "state": "open",
			"html_url": "https://gitea.example.com/a/b/pulls/9", "draft": true,
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	pr, err := client.CreatePullRequest(context.Background(), usecase.Credential{Token: "tok"}, "a/b", usecase.CreatePullRequestInput{Title: "t", HeadBranch: "h", BaseBranch: "b", Draft: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if draft, ok := gotBody["draft"].(bool); !ok || !draft {
		t.Errorf("expected draft:true in the request payload, got %v", gotBody["draft"])
	}
	if !pr.Draft {
		t.Error("expected the echoed PullRequest.Draft to be true")
	}
}

func TestGetRepoFileContent_Found(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("* @alice\n"))
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	content, found, err := client.GetRepoFileContent(context.Background(), usecase.Credential{Token: "tok"}, "a/b", "CODEOWNERS", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found || content != "* @alice\n" {
		t.Errorf("expected found=true content=%q, got found=%v content=%q", "* @alice\n", found, content)
	}
}

func TestGetRepoFileContent_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, found, err := client.GetRepoFileContent(context.Background(), usecase.Credential{Token: "tok"}, "a/b", "CODEOWNERS", "main")
	if err != nil {
		t.Fatalf("expected no error on 404, got %v", err)
	}
	if found {
		t.Error("expected found=false on 404")
	}
}

func TestGetRepoFileContent_NonOKNonNotFoundIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, _, err := client.GetRepoFileContent(context.Background(), usecase.Credential{Token: "tok"}, "a/b", "CODEOWNERS", "main")
	if err == nil {
		t.Fatal("expected an error for a non-200/404 response")
	}
}
