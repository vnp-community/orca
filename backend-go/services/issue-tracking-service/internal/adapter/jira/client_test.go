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
// TestSearchIssues_MapsProjectIssueTypeAssigneeAndPriority is BUG-016's
// regression guard: before this fix, toRichIssue only mapped Key/Summary/
// Status, leaving Project/IssueType/Assignee/Reporter/Priority/Labels at
// their zero value for every issue — which the gRPC server's toProtoIssue
// then omits from the wire entirely (nil, not an empty object) for any
// zero-value ref, crashing frontend code that reads the required (non-
// optional) issue.project.key field. This is a live-confirmed bug: user
// saw "Couldn't load Jira issues... Cannot read properties of undefined
// (reading 'key')" once CR-TSRC-001's capability fix let a real fetch
// happen for the first time.
func TestSearchIssues_MapsProjectIssueTypeAssigneeAndPriority(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"deploymentType": "Cloud"})
		case "/rest/api/3/search":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issues": []map[string]any{{
					"key": "PROJ-1",
					"fields": map[string]any{
						"summary": "Fix the widget",
						"status":  map[string]any{"name": "In Progress"},
						"project": map[string]any{"id": "10000", "key": "PROJ", "name": "Project"},
						"issuetype": map[string]any{
							"id": "3", "name": "Task", "subtask": false,
						},
						"assignee": map[string]any{
							"accountId": "acc-1", "displayName": "A User", "emailAddress": "a@example.com",
						},
						"priority": map[string]any{"id": "2", "name": "High"},
						"labels":   []string{"backend", "urgent"},
					},
				}},
			})
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := New(server.Client())
	cred := usecase.Credential{BaseURL: server.URL, Email: "a@example.com", Token: "tok"}
	issues, err := client.SearchIssues(context.Background(), cred, "", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	got := issues[0]
	if got.Project != (domain.ProjectRef{ID: "10000", Key: "PROJ", Name: "Project"}) {
		t.Errorf("expected Project mapped from fields.project, got %+v", got.Project)
	}
	if got.IssueType != (domain.IssueTypeRef{ID: "3", Name: "Task", Subtask: false}) {
		t.Errorf("expected IssueType mapped from fields.issuetype, got %+v", got.IssueType)
	}
	if got.Assignee.ID != "acc-1" || got.Assignee.DisplayName != "A User" {
		t.Errorf("expected Assignee mapped from fields.assignee, got %+v", got.Assignee)
	}
	if got.Priority != (domain.PriorityRef{ID: "2", Name: "High"}) {
		t.Errorf("expected Priority mapped from fields.priority, got %+v", got.Priority)
	}
	if len(got.Labels) != 2 || got.Labels[0] != "backend" {
		t.Errorf("expected Labels mapped from fields.labels, got %+v", got.Labels)
	}
}

// TestSearchIssues_UnassignedIssue_AssigneeStaysZeroValue guards the nil-
// pointer path: Jira omits "assignee" entirely (not an empty object) for an
// unassigned issue — this must not panic, and must leave Assignee at its
// zero value (toProtoIssue then correctly omits it from the wire, matching
// existing behavior for every other optional ref).
func TestSearchIssues_UnassignedIssue_AssigneeStaysZeroValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"deploymentType": "Cloud"})
		case "/rest/api/3/search":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issues": []map[string]any{{
					"key": "PROJ-2",
					"fields": map[string]any{
						"summary":  "Unassigned issue",
						"status":   map[string]any{"name": "To Do"},
						"project":  map[string]any{"id": "10000", "key": "PROJ", "name": "Project"},
						"assignee": nil,
					},
				}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := New(server.Client())
	cred := usecase.Credential{BaseURL: server.URL, Email: "a@example.com", Token: "tok"}
	issues, err := client.SearchIssues(context.Background(), cred, "", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issues[0].Assignee != (domain.UserRef{}) {
		t.Errorf("expected zero-value Assignee for an unassigned issue, got %+v", issues[0].Assignee)
	}
}

// TestListProjects_SelfHostedDataCenter_UsesFlatProjectEndpoint is BUG-017's
// regression guard: Jira Server/Data Center below 8.4 has no paginated
// GET /project/search endpoint — that path 404s, misleadingly reporting
// "No project could be found with key 'search'" (confirmed live via curl
// against a real self-hosted instance). v2 must use the older, flat-array
// GET /project endpoint instead.
func TestListProjects_SelfHostedDataCenter_UsesFlatProjectEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"deploymentType": "Data Center"})
		case "/rest/api/2/project":
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			// Flat array — NOT {values:[...]} — this is the real v2 shape.
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "17501", "key": "PROJ", "name": "Project"},
			})
		case "/rest/api/2/project/search":
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"errorMessages": []string{"No project could be found with key 'search'."}})
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := New(server.Client())
	cred := usecase.Credential{BaseURL: server.URL, Token: "my-pat-token"}
	projects, err := client.ListProjects(context.Background(), cred, "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/rest/api/2/project" {
		t.Errorf("expected the flat /project endpoint to be called, got %q", gotPath)
	}
	if len(projects) != 1 || projects[0].Key != "PROJ" || projects[0].WorkspaceID != "ws-1" {
		t.Errorf("unexpected projects: %+v", projects)
	}
}

// TestListProjects_CloudSite_StillUsesPaginatedSearchEndpoint is the
// regression guard for BUG-017's fix not breaking the already-working Cloud
// case (same "additive, not a regression" posture as CR-JIRA-001).
func TestListProjects_CloudSite_StillUsesPaginatedSearchEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"deploymentType": "Cloud"})
		case "/rest/api/3/project/search":
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"values": []map[string]any{{"id": "1", "key": "PROJ", "name": "Project"}},
			})
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := New(server.Client())
	cred := usecase.Credential{BaseURL: server.URL, Email: "a@example.com", Token: "tok"}
	projects, err := client.ListProjects(context.Background(), cred, "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/rest/api/3/project/search" {
		t.Errorf("expected the paginated /project/search endpoint to be called, got %q", gotPath)
	}
	if len(projects) != 1 || projects[0].Key != "PROJ" {
		t.Errorf("unexpected projects: %+v", projects)
	}
}

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
		case r.URL.Path == "/rest/api/2/serverInfo":
			// CR-JIRA-001's version probe — not "Server"/"Data Center", so
			// resolveAPIVersion falls back to "3" (this test's existing
			// Cloud-shaped expectations, unaffected by the probe itself).
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"deploymentType": "Cloud"})
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
		if r.URL.Path == "/rest/api/2/serverInfo" {
			// CR-JIRA-001's version probe — see the sibling test above.
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"deploymentType": "Cloud"})
			return
		}
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

// TestWhoami_SelfHostedDataCenter_UsesV2AndBearerAuth is the regression test
// for CR-JIRA-001/BUG-013: a self-hosted Jira (Server/Data Center) has no
// /rest/api/3/ at all — Whoami must probe deploymentType first and use
// /rest/api/2/ once it detects "Data Center", and must send the token as a
// Bearer PAT (not Basic email:token) when no email is supplied.
func TestWhoami_SelfHostedDataCenter_UsesV2AndBearerAuth(t *testing.T) {
	var gotMyselfPath, gotMyselfAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"deploymentType": "Data Center"})
		case "/rest/api/2/myself":
			gotMyselfPath = r.URL.Path
			gotMyselfAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"accountId": "u1", "displayName": "A User", "emailAddress": "a@example.com",
			})
		default:
			t.Errorf("unexpected request path: %s (expected only the v2 probe/myself endpoints)", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := New(server.Client())
	// No Email: a self-hosted caller authenticating with a Personal Access
	// Token, per authHeaderValue's doc comment.
	cred := usecase.Credential{BaseURL: server.URL, Token: "my-pat-token"}
	viewer, err := client.Whoami(context.Background(), cred)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if viewer.ID != "u1" {
		t.Errorf("unexpected viewer: %+v", viewer)
	}
	if gotMyselfPath != "/rest/api/2/myself" {
		t.Errorf("expected the v2 endpoint (Server/Data Center has no v3), got %q", gotMyselfPath)
	}
	if gotMyselfAuth != "Bearer my-pat-token" {
		t.Errorf("expected Bearer PAT auth when no email is set, got %q", gotMyselfAuth)
	}
}

// TestWhoami_CloudSite_StillUsesV3AndBasicAuth confirms this fix doesn't
// change any existing Cloud caller's behavior (CR-JIRA-001's stated goal:
// additive, not a regression for the already-working case).
func TestWhoami_CloudSite_StillUsesV3AndBasicAuth(t *testing.T) {
	var gotMyselfPath, gotMyselfAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"deploymentType": "Cloud"})
		case "/rest/api/3/myself":
			gotMyselfPath = r.URL.Path
			gotMyselfAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"accountId": "u1", "displayName": "A User", "emailAddress": "a@example.com",
			})
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := New(server.Client())
	cred := usecase.Credential{BaseURL: server.URL, Email: "a@example.com", Token: "tok"}
	if _, err := client.Whoami(context.Background(), cred); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMyselfPath != "/rest/api/3/myself" {
		t.Errorf("expected Cloud to still use v3, got %q", gotMyselfPath)
	}
	if gotMyselfAuth != "Basic "+basicAuth("a@example.com", "tok") {
		t.Errorf("expected Basic email:token auth unchanged for Cloud, got %q", gotMyselfAuth)
	}
}
