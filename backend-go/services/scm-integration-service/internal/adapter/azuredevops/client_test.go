package azuredevops

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// TestListIssues_ReturnsErrCapabilityUnsupported asserts the deliberate
// behavioral distinction from every other provider adapter in this
// scaffold: Azure DevOps has no native issue-tracking concept, only work
// items — a different, heavily-typed system out of scope for this pass — so
// ListIssues returns the typed ErrCapabilityUnsupported sentinel rather than
// ErrNotImplemented ("not built yet") or a faked Work Items call. No HTTP
// request should be made at all.
func TestListIssues_ReturnsErrCapabilityUnsupported(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.ListIssues(context.Background(), usecase.Credential{Token: "fake-token"}, "org/project/repo", usecase.IssueFilter{})
	if !errors.Is(err, ErrCapabilityUnsupported) {
		t.Fatalf("expected ErrCapabilityUnsupported, got %v", err)
	}
	if called {
		t.Error("ListIssues should not make any HTTP request — it's unsupported by design, not by omission")
	}
}

// TestCreatePullRequest_RealHTTPCall exercises the real request/response
// path — a POST request with a proper Authorization header and api-version
// query param, against a real JSON response shape modeled on Azure DevOps'
// POST {org}/{project}/_apis/git/repositories/{repositoryId}/pullrequests
// endpoint. httptest.Server stands in for dev.azure.com so this test has no
// network dependency.
func TestCreatePullRequest_RealHTTPCall(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotAPIVersion string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAPIVersion = r.URL.Query().Get("api-version")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"pullRequestId": 9,
			"title":         "add feature",
			"status":        "active",
			"sourceRefName": "refs/heads/feature",
			"targetRefName": "refs/heads/main",
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	pr, err := client.CreatePullRequest(context.Background(), usecase.Credential{Token: "ado_faketoken"}, "acme/widgets/repo1", usecase.CreatePullRequestInput{
		Title: "add feature", Body: "desc", HeadBranch: "feature", BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/acme/widgets/_apis/git/repositories/repo1/pullrequests" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if gotAuth != "Bearer ado_faketoken" {
		t.Errorf("expected Authorization header to carry the resolved credential, got %q", gotAuth)
	}
	if gotAPIVersion != "7.1" {
		t.Errorf("expected api-version=7.1 query param, got %q", gotAPIVersion)
	}
	if gotBody["sourceRefName"] != "refs/heads/feature" || gotBody["targetRefName"] != "refs/heads/main" {
		t.Errorf("expected refs/heads/ prefix on branch names in request body, got: %+v", gotBody)
	}

	if pr.ID != "9" || pr.State != "active" || pr.HeadBranch != "feature" || pr.BaseBranch != "main" {
		t.Errorf("unexpected pull request: %+v", pr)
	}
	if pr.URL != "https://dev.azure.com/acme/widgets/_git/repo1/pullrequest/9" {
		t.Errorf("expected a constructed pull request URL, got %q", pr.URL)
	}
	if pr.Repo != "acme/widgets/repo1" {
		t.Errorf("expected repo to be set on the mapped pull request, got %q", pr.Repo)
	}
}

func TestCreatePullRequest_NonCreatedStatusIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.CreatePullRequest(context.Background(), usecase.Credential{Token: "tok"}, "org/project/repo", usecase.CreatePullRequestInput{Title: "t", HeadBranch: "h", BaseBranch: "b"})
	if err == nil {
		t.Fatal("expected an error for a non-201 response")
	}
}

// TestCreatePullRequest_InvalidRepoIsAnError covers the "repo must split
// into exactly 3 parts" guard — a malformed repo string must return an
// error, not panic on a bad index.
func TestCreatePullRequest_InvalidRepoIsAnError(t *testing.T) {
	client := New(http.DefaultClient, "https://dev.azure.com")
	_, err := client.CreatePullRequest(context.Background(), usecase.Credential{Token: "tok"}, "not-enough-parts", usecase.CreatePullRequestInput{Title: "t", HeadBranch: "h", BaseBranch: "b"})
	if err == nil {
		t.Fatal("expected an error for a repo string that doesn't split into org/project/repositoryId")
	}
}

// TestListPullRequests_RealHTTPCall asserts the {value, count} list envelope
// Azure DevOps wraps list responses in — unlike GitHub's bare array.
func TestListPullRequests_RealHTTPCall(t *testing.T) {
	var gotPath, gotAPIVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIVersion = r.URL.Query().Get("api-version")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{
				{
					"pullRequestId": 3,
					"title":         "fix bug",
					"status":        "active",
					"sourceRefName": "refs/heads/fix",
					"targetRefName": "refs/heads/main",
				},
			},
			"count": 1,
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	prs, err := client.ListPullRequests(context.Background(), usecase.Credential{Token: "ado_faketoken"}, "acme/widgets/repo1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/acme/widgets/_apis/git/repositories/repo1/pullrequests" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if gotAPIVersion != "7.1" {
		t.Errorf("expected api-version=7.1 query param, got %q", gotAPIVersion)
	}
	if len(prs) != 1 || prs[0].ID != "3" || prs[0].HeadBranch != "fix" || prs[0].BaseBranch != "main" {
		t.Errorf("unexpected pull requests: %+v", prs)
	}
}

func TestListPullRequests_NonOKStatusIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.ListPullRequests(context.Background(), usecase.Credential{Token: "bad-token"}, "org/project/repo")
	if err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

func TestListPullRequests_InvalidRepoIsAnError(t *testing.T) {
	client := New(http.DefaultClient, "https://dev.azure.com")
	_, err := client.ListPullRequests(context.Background(), usecase.Credential{Token: "tok"}, "org/project")
	if err == nil {
		t.Fatal("expected an error for a repo string that doesn't split into org/project/repositoryId")
	}
}

func TestGetRateLimitStatus_RealHTTPCall(t *testing.T) {
	var gotPath, gotAuth, gotAPIVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAPIVersion = r.URL.Query().Get("api-version")
		w.Header().Set("X-RateLimit-Limit", "1000")
		w.Header().Set("X-RateLimit-Remaining", "340")
		w.Header().Set("X-RateLimit-Reset", "1700000000")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": []map[string]any{}, "count": 0})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	status, err := client.GetRateLimitStatus(context.Background(), usecase.Credential{Token: "ado_faketoken"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/_apis/projects" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if gotAuth != "Bearer ado_faketoken" {
		t.Errorf("expected Authorization header to carry the resolved credential, got %q", gotAuth)
	}
	if gotAPIVersion != "7.1" {
		t.Errorf("expected api-version=7.1 query param, got %q", gotAPIVersion)
	}
	if status.Remaining != 340 || status.Limit != 1000 {
		t.Errorf("unexpected rate limit status: %+v", status)
	}
}

// TestGetRateLimitStatus_NoHeadersPresent covers the TFS/Azure DevOps
// convention of only sometimes exposing X-RateLimit-* headers: their
// absence must not be an error, just a zero-valued ("unknown") status.
func TestGetRateLimitStatus_NoHeadersPresent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"value": []map[string]any{}, "count": 0})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	status, err := client.GetRateLimitStatus(context.Background(), usecase.Credential{Token: "ado_faketoken"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Remaining != 0 || status.Limit != 0 || !status.ResetAt.IsZero() {
		t.Errorf("expected zero-valued rate limit status when headers are absent, got %+v", status)
	}
	if status.Provider != "azure_devops" {
		t.Errorf("expected Provider to be set even on a zero-valued status, got %q", status.Provider)
	}
}

func TestGetRateLimitStatus_NonOKStatusIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.GetRateLimitStatus(context.Background(), usecase.Credential{Token: "bad-token"})
	if err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

// TestCreatePullRequest_DraftSerializesIntoPayload asserts Draft: true
// serializes into the "isDraft" JSON field — BR-CR-20.
func TestCreatePullRequest_DraftSerializesIntoPayload(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"pullRequestId": 9, "title": "wip", "status": "active",
			"sourceRefName": "refs/heads/h", "targetRefName": "refs/heads/b", "isDraft": true,
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	pr, err := client.CreatePullRequest(context.Background(), usecase.Credential{Token: "tok"}, "org/project/repo", usecase.CreatePullRequestInput{Title: "t", HeadBranch: "h", BaseBranch: "b", Draft: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if draft, ok := gotBody["isDraft"].(bool); !ok || !draft {
		t.Errorf("expected isDraft:true in the request payload, got %v", gotBody["isDraft"])
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
	content, found, err := client.GetRepoFileContent(context.Background(), usecase.Credential{Token: "tok"}, "org/project/repo", "CODEOWNERS", "main")
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
	_, found, err := client.GetRepoFileContent(context.Background(), usecase.Credential{Token: "tok"}, "org/project/repo", "CODEOWNERS", "main")
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
	_, _, err := client.GetRepoFileContent(context.Background(), usecase.Credential{Token: "tok"}, "org/project/repo", "CODEOWNERS", "main")
	if err == nil {
		t.Fatal("expected an error for a non-200/404 response")
	}
}
