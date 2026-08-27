package gitlab

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
// response shape modeled on GitLab's GET /projects/{id}/issues endpoint.
// httptest.Server stands in for gitlab.com/api/v4 so this test has no
// network dependency. repo contains a "/" to verify the project path gets
// URL-escaped.
func TestListIssues_RealHTTPCall(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"iid":     42,
				"title":   "something is broken",
				"state":   "opened",
				"web_url": "https://gitlab.com/group/project/-/issues/42",
			},
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	issues, err := client.ListIssues(context.Background(), usecase.Credential{Token: "glpat-faketoken"}, "group/project", usecase.IssueFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/projects/group%2Fproject/issues" {
		t.Errorf("expected repo to be URL-escaped in the request path, got %s", gotPath)
	}
	if gotAuth != "Bearer glpat-faketoken" {
		t.Errorf("expected Authorization header to carry the resolved credential, got %q", gotAuth)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].ID != "42" || issues[0].Title != "something is broken" || issues[0].State != "opened" {
		t.Errorf("unexpected issue: %+v", issues[0])
	}
	if issues[0].Repo != "group/project" {
		t.Errorf("expected repo to be set on the mapped issue, got %q", issues[0].Repo)
	}
}

// TestListIssues_StateFilterForwardedAsQueryParam verifies IssueFilter.State
// is forwarded as GitLab's ?state= query parameter when non-empty.
func TestListIssues_StateFilterForwardedAsQueryParam(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("state")
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.ListIssues(context.Background(), usecase.Credential{Token: "tok"}, "group/project", usecase.IssueFilter{State: "closed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "closed" {
		t.Errorf("expected state=closed to be forwarded, got %q", gotQuery)
	}
}

func TestListIssues_NonOKStatusIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.ListIssues(context.Background(), usecase.Credential{Token: "bad-token"}, "group/project", usecase.IssueFilter{})
	if err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

func TestCreatePullRequest_RealHTTPCall(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"iid":           9,
			"title":         "add feature",
			"state":         "opened",
			"web_url":       "https://gitlab.com/group/project/-/merge_requests/9",
			"source_branch": "feature",
			"target_branch": "main",
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	pr, err := client.CreatePullRequest(context.Background(), usecase.Credential{Token: "glpat-faketoken"}, "group/project", usecase.CreatePullRequestInput{
		Title: "add feature", Body: "desc", HeadBranch: "feature", BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/projects/group%2Fproject/merge_requests" {
		t.Errorf("expected repo to be URL-escaped in the request path, got %s", gotPath)
	}
	if gotAuth != "Bearer glpat-faketoken" {
		t.Errorf("expected Authorization header to carry the resolved credential, got %q", gotAuth)
	}
	if gotBody["source_branch"] != "feature" || gotBody["target_branch"] != "main" {
		t.Errorf("unexpected request body: %+v", gotBody)
	}
	if pr.ID != "9" || pr.State != "opened" || pr.HeadBranch != "feature" || pr.BaseBranch != "main" {
		t.Errorf("unexpected pull request: %+v", pr)
	}
}

func TestCreatePullRequest_NonCreatedStatusIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.CreatePullRequest(context.Background(), usecase.Credential{Token: "tok"}, "group/project", usecase.CreatePullRequestInput{Title: "t", HeadBranch: "h", BaseBranch: "b"})
	if err == nil {
		t.Fatal("expected an error for a non-201 response")
	}
}

func TestListPullRequests_RealHTTPCall(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"iid":           3,
				"title":         "fix bug",
				"state":         "opened",
				"web_url":       "https://gitlab.com/group/project/-/merge_requests/3",
				"source_branch": "fix",
				"target_branch": "main",
			},
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	prs, err := client.ListPullRequests(context.Background(), usecase.Credential{Token: "glpat-faketoken"}, "group/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/projects/group%2Fproject/merge_requests" {
		t.Errorf("expected repo to be URL-escaped in the request path, got %s", gotPath)
	}
	if len(prs) != 1 || prs[0].ID != "3" || prs[0].HeadBranch != "fix" {
		t.Errorf("unexpected pull requests: %+v", prs)
	}
}

func TestListPullRequests_NonOKStatusIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.ListPullRequests(context.Background(), usecase.Credential{Token: "bad-token"}, "group/project")
	if err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

// TestGetRateLimitStatus_RealHTTPCall verifies the RateLimit-* response
// headers on GET /user are parsed correctly.
func TestGetRateLimitStatus_RealHTTPCall(t *testing.T) {
	var gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("RateLimit-Limit", "2000")
		w.Header().Set("RateLimit-Remaining", "1985")
		w.Header().Set("RateLimit-Reset", "1700000000")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "username": "octocat"})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	status, err := client.GetRateLimitStatus(context.Background(), usecase.Credential{Token: "glpat-faketoken"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/user" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if gotAuth != "Bearer glpat-faketoken" {
		t.Errorf("expected Authorization header to carry the resolved credential, got %q", gotAuth)
	}
	if status.Limit != 2000 || status.Remaining != 1985 {
		t.Errorf("unexpected rate limit status: %+v", status)
	}
	if status.ResetAt.Unix() != 1700000000 {
		t.Errorf("unexpected reset time: %v", status.ResetAt)
	}
	if status.Provider != "gitlab" {
		t.Errorf("expected provider to be gitlab, got %q", status.Provider)
	}
}

// TestGetRateLimitStatus_NoHeadersReturnsZeroValueNotError verifies that a
// response with no RateLimit-* headers (some GitLab deployments/plans don't
// send them) doesn't error — it's an absent signal, not a request failure.
func TestGetRateLimitStatus_NoHeadersReturnsZeroValueNotError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "username": "octocat"})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	status, err := client.GetRateLimitStatus(context.Background(), usecase.Credential{Token: "glpat-faketoken"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Limit != 0 || status.Remaining != 0 || !status.ResetAt.IsZero() {
		t.Errorf("expected zero-value rate limit status, got %+v", status)
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
// serializes into the "draft" JSON field — BR-CR-20.
func TestCreatePullRequest_DraftSerializesIntoPayload(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"iid": 9, "title": "wip", "state": "opened",
			"web_url": "https://gitlab.com/a/b/-/merge_requests/9",
			"source_branch": "h", "target_branch": "b", "draft": true,
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
