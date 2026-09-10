package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
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
			"html_url": "https://github.com/octocat/hello-world/pull/9",
			"head":     map[string]any{"ref": "feature"},
			"base":     map[string]any{"ref": "main"},
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	pr, err := client.CreatePullRequest(context.Background(), usecase.Credential{Token: "gho_faketoken"}, "octocat/hello-world", usecase.CreatePullRequestInput{
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
	if gotAuth != "Bearer gho_faketoken" {
		t.Errorf("expected Authorization header to carry the resolved credential, got %q", gotAuth)
	}
	if gotBody["head"] != "feature" || gotBody["base"] != "main" {
		t.Errorf("unexpected request body: %+v", gotBody)
	}
	if pr.ID != "9" || pr.State != "open" || pr.HeadBranch != "feature" || pr.BaseBranch != "main" {
		t.Errorf("unexpected pull request: %+v", pr)
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
				"html_url": "https://github.com/octocat/hello-world/pull/3",
				"head":     map[string]any{"ref": "fix"},
				"base":     map[string]any{"ref": "main"},
			},
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	prs, err := client.ListPullRequests(context.Background(), usecase.Credential{Token: "gho_faketoken"}, "octocat/hello-world")
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

func TestGetRateLimitStatus_RealHTTPCall(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resources": map[string]any{
				"core": map[string]any{"limit": 5000, "remaining": 340, "reset": 1700000000},
			},
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	status, err := client.GetRateLimitStatus(context.Background(), usecase.Credential{Token: "gho_faketoken"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/rate_limit" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if status.Remaining != 340 || status.Limit != 5000 {
		t.Errorf("unexpected rate limit status: %+v", status)
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

// TestCreatePullRequest_DraftSerializesIntoPayload asserts Draft: true
// serializes into the "draft" JSON field — BR-CR-20.
func TestCreatePullRequest_DraftSerializesIntoPayload(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"number": 9, "title": "wip", "state": "open",
			"html_url": "https://github.com/octocat/hello-world/pull/9", "draft": true,
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

// TestClient_SubmitReview_BuildsExactPayloadShape asserts the exact
// event/comments[].path/line/body request body GitHub's Reviews API
// expects, for each of the three review types (BUG-PI-04/TASK-PI-04-07).
func TestClient_SubmitReview_BuildsExactPayloadShape(t *testing.T) {
	tests := []struct {
		name      string
		in        domain.ReviewInput
		wantEvent string
	}{
		{
			name:      "comment",
			in:        domain.ReviewInput{Type: domain.ReviewTypeComment, Summary: "looks fine", Comments: []domain.ReviewComment{{Path: "a.go", Line: 10, Body: "nit"}}},
			wantEvent: "COMMENT",
		},
		{
			name:      "approve",
			in:        domain.ReviewInput{Type: domain.ReviewTypeApprove, Summary: "ship it", Comments: []domain.ReviewComment{{Path: "b.go", Line: 20, Body: "great"}}},
			wantEvent: "APPROVE",
		},
		{
			name:      "request_changes",
			in:        domain.ReviewInput{Type: domain.ReviewTypeRequestChanges, Summary: "needs work", Comments: []domain.ReviewComment{{Path: "c.go", Line: 30, Body: "fix this"}}},
			wantEvent: "REQUEST_CHANGES",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string
			var gotBody map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				_ = json.NewDecoder(r.Body).Decode(&gotBody)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id": 99, "state": tt.wantEvent, "user": map[string]any{"login": "octocat"},
					"submitted_at": "2024-01-01T00:00:00Z", "html_url": "https://github.com/o/r/pull/1#review-99",
				})
			}))
			defer server.Close()

			client := New(server.Client(), server.URL)
			review, err := client.SubmitReview(context.Background(), usecase.Credential{Token: "gho_faketoken"}, "o/r", 1, tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotMethod != http.MethodPost {
				t.Errorf("expected POST, got %s", gotMethod)
			}
			if gotPath != "/repos/o/r/pulls/1/reviews" {
				t.Errorf("unexpected request path: %s", gotPath)
			}
			if gotBody["event"] != tt.wantEvent {
				t.Errorf("event = %v, want %v", gotBody["event"], tt.wantEvent)
			}
			if gotBody["body"] != tt.in.Summary {
				t.Errorf("body = %v, want %v", gotBody["body"], tt.in.Summary)
			}
			comments, ok := gotBody["comments"].([]any)
			if !ok || len(comments) != 1 {
				t.Fatalf("expected exactly one comment in the request body, got %v", gotBody["comments"])
			}
			c := comments[0].(map[string]any)
			if c["path"] != tt.in.Comments[0].Path || int32(c["line"].(float64)) != tt.in.Comments[0].Line || c["body"] != tt.in.Comments[0].Body {
				t.Errorf("unexpected comment shape: %+v", c)
			}
			if review.ID != "99" || review.ReviewerID != "octocat" {
				t.Errorf("unexpected review: %+v", review)
			}
		})
	}
}
