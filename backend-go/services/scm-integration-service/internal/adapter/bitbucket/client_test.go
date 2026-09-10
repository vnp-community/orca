package bitbucket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// TestListIssues_RealHTTPCall exercises the real request/response path — a
// GET request with a proper Authorization header, against a real JSON
// response shape modeled on Bitbucket's
// GET /repositories/{workspace}/{repo_slug}/issues endpoint, including its
// "values" pagination envelope. httptest.Server stands in for
// api.bitbucket.org so this test has no network dependency.
func TestListIssues_RealHTTPCall(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"values": []map[string]any{
				{
					"id":    42,
					"title": "something is broken",
					"state": "open",
					"links": map[string]any{
						"html": map[string]any{"href": "https://bitbucket.org/acme/widgets/issues/42"},
					},
				},
			},
			"next": "",
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	issues, err := client.ListIssues(context.Background(), usecase.Credential{Token: "bb_faketoken"}, "acme/widgets", usecase.IssueFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/repositories/acme/widgets/issues" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if gotAuth != "Bearer bb_faketoken" {
		t.Errorf("expected Authorization header to carry the resolved credential, got %q", gotAuth)
	}

	if len(issues) != 1 {
		t.Fatalf("expected the values envelope to be unwrapped into 1 issue, got %d", len(issues))
	}
	if issues[0].ID != "42" || issues[0].Title != "something is broken" || issues[0].State != "open" {
		t.Errorf("unexpected issue: %+v", issues[0])
	}
	if issues[0].Repo != "acme/widgets" {
		t.Errorf("expected repo to be set on the mapped issue, got %q", issues[0].Repo)
	}
	if issues[0].URL != "https://bitbucket.org/acme/widgets/issues/42" {
		t.Errorf("expected links.html.href to map to URL, got %q", issues[0].URL)
	}
}

func TestListIssues_NonOKStatusIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.ListIssues(context.Background(), usecase.Credential{Token: "bad-token"}, "acme/widgets", usecase.IssueFilter{})
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
			"id":    9,
			"title": "add feature",
			"state": "OPEN",
			"links": map[string]any{
				"html": map[string]any{"href": "https://bitbucket.org/acme/widgets/pull-requests/9"},
			},
			"source":      map[string]any{"branch": map[string]any{"name": "feature"}},
			"destination": map[string]any{"branch": map[string]any{"name": "main"}},
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	pr, err := client.CreatePullRequest(context.Background(), usecase.Credential{Token: "bb_faketoken"}, "acme/widgets", usecase.CreatePullRequestInput{
		Title: "add feature", Body: "desc", HeadBranch: "feature", BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/repositories/acme/widgets/pullrequests" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if gotAuth != "Bearer bb_faketoken" {
		t.Errorf("expected Authorization header to carry the resolved credential, got %q", gotAuth)
	}

	source, _ := gotBody["source"].(map[string]any)
	sourceBranch, _ := source["branch"].(map[string]any)
	destination, _ := gotBody["destination"].(map[string]any)
	destinationBranch, _ := destination["branch"].(map[string]any)
	if sourceBranch["name"] != "feature" || destinationBranch["name"] != "main" {
		t.Errorf("unexpected request body: %+v", gotBody)
	}

	if pr.ID != "9" || pr.State != "OPEN" || pr.HeadBranch != "feature" || pr.BaseBranch != "main" {
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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"values": []map[string]any{
				{
					"id":    3,
					"title": "fix bug",
					"state": "OPEN",
					"links": map[string]any{
						"html": map[string]any{"href": "https://bitbucket.org/acme/widgets/pull-requests/3"},
					},
					"source":      map[string]any{"branch": map[string]any{"name": "fix"}},
					"destination": map[string]any{"branch": map[string]any{"name": "main"}},
				},
			},
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	prs, err := client.ListPullRequests(context.Background(), usecase.Credential{Token: "bb_faketoken"}, "acme/widgets")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/repositories/acme/widgets/pullrequests" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if len(prs) != 1 || prs[0].ID != "3" || prs[0].HeadBranch != "fix" {
		t.Errorf("unexpected pull requests: %+v", prs)
	}
}

func TestGetRateLimitStatus_RealHTTPCall(t *testing.T) {
	var gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("X-RateLimit-Limit", "1000")
		w.Header().Set("X-RateLimit-Remaining", "340")
		w.Header().Set("X-RateLimit-Reset", "1700000000")
		_ = json.NewEncoder(w).Encode(map[string]any{"username": "acme-bot"})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	status, err := client.GetRateLimitStatus(context.Background(), usecase.Credential{Token: "bb_faketoken"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/user" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if gotAuth != "Bearer bb_faketoken" {
		t.Errorf("expected Authorization header to carry the resolved credential, got %q", gotAuth)
	}
	if status.Remaining != 340 || status.Limit != 1000 {
		t.Errorf("unexpected rate limit status: %+v", status)
	}
}

// TestGetRateLimitStatus_NoHeadersPresent covers Bitbucket's inconsistent
// header exposure: absent X-RateLimit-* headers must not be an error, just
// zero-valued ("unknown").
func TestGetRateLimitStatus_NoHeadersPresent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"username": "acme-bot"})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	status, err := client.GetRateLimitStatus(context.Background(), usecase.Credential{Token: "bb_faketoken"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Remaining != 0 || status.Limit != 0 || !status.ResetAt.IsZero() {
		t.Errorf("expected zero-valued rate limit status when headers are absent, got %+v", status)
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

// TestCreatePullRequest_DraftReturnsCapabilityUnsupported asserts Draft:
// true returns an error satisfying errors.Is(err,
// domain.ErrCapabilityUnsupported) without making any HTTP call —
// Bitbucket Cloud's PR API has no draft concept (BR-CR-20).
func TestCreatePullRequest_DraftReturnsCapabilityUnsupported(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.CreatePullRequest(context.Background(), usecase.Credential{Token: "tok"}, "a/b", usecase.CreatePullRequestInput{Title: "t", HeadBranch: "h", BaseBranch: "b", Draft: true})
	if !errors.Is(err, domain.ErrCapabilityUnsupported) {
		t.Fatalf("expected an error satisfying errors.Is(err, domain.ErrCapabilityUnsupported), got %v", err)
	}
	if called {
		t.Error("expected zero HTTP calls for a draft request against Bitbucket")
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
