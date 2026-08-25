package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

// TestResolveIssueType_ExactMatch checks that a case-exact "Task" among the
// real returned issue types is preferred over anything else, per
// docs/execution-plan.md §3 Phase 1 — resolve against real data instead of
// blindly hardcoding the string.
func TestResolveIssueType_ExactMatch(t *testing.T) {
	types := []jiraIssueTypeMeta{
		{ID: "1", Name: "Bug", Subtask: false},
		{ID: "2", Name: "Task", Subtask: false},
		{ID: "3", Name: "Story", Subtask: false},
	}
	got, err := resolveIssueType(types, "Task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Task" {
		t.Errorf("expected Task, got %q", got)
	}
}

// TestResolveIssueType_CaseInsensitiveMatch checks that a real Jira site
// naming its type "task" (lowercase) or "TASK" still resolves — Jira issue
// type names vary by site, and matching must not be case-sensitive.
func TestResolveIssueType_CaseInsensitiveMatch(t *testing.T) {
	types := []jiraIssueTypeMeta{
		{ID: "1", Name: "task", Subtask: false},
		{ID: "2", Name: "Bug", Subtask: false},
	}
	got, err := resolveIssueType(types, "Task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "task" {
		t.Errorf("expected the real returned name %q to be preserved, got %q", "task", got)
	}
}

// TestResolveIssueType_NoMatchFallsBackToFirstNonSubtask checks the
// sensible fallback when a project has no issue type named "Task" at
// all — a real scenario on Jira sites that renamed or removed it.
func TestResolveIssueType_NoMatchFallsBackToFirstNonSubtask(t *testing.T) {
	types := []jiraIssueTypeMeta{
		{ID: "1", Name: "Subtask", Subtask: true},
		{ID: "2", Name: "Story", Subtask: false},
		{ID: "3", Name: "Bug", Subtask: false},
	}
	got, err := resolveIssueType(types, "Task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Story" {
		t.Errorf("expected the first non-subtask type Story, got %q", got)
	}
}

// TestResolveIssueType_NoIssueTypesReturnsClearError checks that a project
// with zero issue types (a possible, if unusual, real Jira response) fails
// loudly instead of silently falling back to a guessed string.
func TestResolveIssueType_NoIssueTypesReturnsClearError(t *testing.T) {
	_, err := resolveIssueType(nil, "Task")
	if err == nil {
		t.Fatal("expected an error for zero issue types")
	}
	if !strings.Contains(err.Error(), "no issue types available") {
		t.Errorf("expected a clear no-issue-types error, got %v", err)
	}
}

// TestResolveIssueType_AllSubtasksReturnsClearError checks the case where a
// project has issue types but every one is a subtask type — none of them
// can be the target of a bare top-level CreateIssue.
func TestResolveIssueType_AllSubtasksReturnsClearError(t *testing.T) {
	types := []jiraIssueTypeMeta{
		{ID: "1", Name: "Subtask", Subtask: true},
	}
	_, err := resolveIssueType(types, "Task")
	if err == nil {
		t.Fatal("expected an error when every issue type is a subtask type")
	}
	if !strings.Contains(err.Error(), "non-subtask") {
		t.Errorf("expected a clear non-subtask error, got %v", err)
	}
}

// TestListIssueTypes_RealHTTPCall exercises the real request path — a GET
// against /rest/api/3/issue/createmeta/{projectKey}/issuetypes, modeled on
// Jira Cloud's actual (non-deprecated) response shape.
func TestListIssueTypes_RealHTTPCall(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"values": []map[string]any{
				{"id": "10001", "name": "Task", "subtask": false},
				{"id": "10002", "name": "Subtask", "subtask": true},
			},
		})
	}))
	defer server.Close()

	client := New(server.Client())
	cred := usecase.Credential{BaseURL: server.URL, Email: "a@example.com", Token: "tok"}
	types, err := client.listIssueTypes(context.Background(), cred, "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/rest/api/3/issue/createmeta/PROJ/issuetypes" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if gotAuth != "Basic "+basicAuth("a@example.com", "tok") {
		t.Errorf("expected Authorization header to carry the resolved credential, got %q", gotAuth)
	}
	if len(types) != 2 || types[0].Name != "Task" || types[1].Subtask != true {
		t.Errorf("unexpected issue types: %+v", types)
	}
}

// TestCreateIssue_UsesResolvedIssueTypeNotHardcodedString is the
// end-to-end regression test for this fix: CreateIssue must send the real
// issue type Jira returned, not the literal "Task" string, once the
// project's issue types have a different exact-cased "Task" name.
func TestCreateIssue_UsesResolvedIssueTypeNotHardcodedString(t *testing.T) {
	var gotCreateBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/issuetypes"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"values": []map[string]any{
					{"id": "10001", "name": "task", "subtask": false}, // real site names it lowercase
					{"id": "10002", "name": "Bug", "subtask": false},
				},
			})
		case r.URL.Path == "/rest/api/3/issue":
			_ = json.NewDecoder(r.Body).Decode(&gotCreateBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-1"})
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := New(server.Client())
	cred := usecase.Credential{BaseURL: server.URL, Email: "a@example.com", Token: "tok"}
	issue, err := client.CreateIssue(context.Background(), cred, domain.NewIssueInput{ProjectKey: "PROJ", Title: "a title"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.ID != "PROJ-1" {
		t.Errorf("unexpected issue: %+v", issue)
	}

	fields, _ := gotCreateBody["fields"].(map[string]any)
	issueType, _ := fields["issuetype"].(map[string]any)
	if issueType["name"] != "task" {
		t.Errorf("expected CreateIssue to send the resolved real issue type %q, got %v", "task", issueType["name"])
	}
}

// TestCreateIssue_NoIssueTypesReturnsClearError checks that CreateIssue
// itself surfaces resolveIssueType's error rather than falling through to
// POST an issue with no valid issue type.
func TestCreateIssue_NoIssueTypesReturnsClearError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/issuetypes") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"values": []map[string]any{}})
			return
		}
		t.Errorf("unexpected request to %s — CreateIssue must not POST without a resolved issue type", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := New(server.Client())
	cred := usecase.Credential{BaseURL: server.URL, Email: "a@example.com", Token: "tok"}
	_, err := client.CreateIssue(context.Background(), cred, domain.NewIssueInput{ProjectKey: "PROJ", Title: "a title"})
	if err == nil {
		t.Fatal("expected an error when the project has no issue types")
	}
	if !strings.Contains(err.Error(), "no issue types available") {
		t.Errorf("expected a clear no-issue-types error, got %v", err)
	}
}
