package jira

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

// fakeJira serves one issue on Jira Cloud (API v3) and records what was written.
type fakeJira struct {
	mu            sync.Mutex
	statusName    string
	statusKey     string // statusCategory.key
	transitions   []map[string]any
	transitionErr int // when non-zero, POST /transitions answers with this status
	requests      []string
	postedBody    map[string]any
}

func (f *fakeJira) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/serverInfo":
			_ = json.NewEncoder(w).Encode(map[string]any{"deploymentType": "Cloud"})
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/ENG-1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"key": "ENG-1",
				"fields": map[string]any{
					"summary": "Fix login",
					"status":  map[string]any{"name": f.statusName, "statusCategory": map[string]any{"key": f.statusKey}},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/ENG-1/transitions":
			_ = json.NewEncoder(w).Encode(map[string]any{"transitions": f.transitions})
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/issue/ENG-1/transitions":
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &f.postedBody)
			if f.transitionErr != 0 {
				w.WriteHeader(f.transitionErr)
				_, _ = w.Write([]byte(`{"errorMessages":["Field 'resolution' is required"]}`))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/3/issue/ENG-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func (f *fakeJira) wrote(prefix string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.requests {
		if strings.HasPrefix(r, prefix) {
			return true
		}
	}
	return false
}

func transition(id, name, to string) map[string]any {
	return map[string]any{"id": id, "name": name, "to": map[string]any{"id": "s" + id, "name": to, "statusCategory": map[string]any{"key": "indeterminate"}}}
}

func newTransitionClient(t *testing.T, f *fakeJira) (*Client, usecase.Credential) {
	t.Helper()
	server := httptest.NewServer(f.handler(t))
	t.Cleanup(server.Close)
	return New(server.Client()), usecase.Credential{BaseURL: server.URL, Email: "a@example.com", Token: "tok"}
}

func TestUpdateIssue_WorkflowState_PostsTheMatchingTransition(t *testing.T) {
	f := &fakeJira{statusName: "To Do", statusKey: "new", transitions: []map[string]any{
		transition("11", "Back to backlog", "Backlog"),
		transition("21", "Start progress", "In Progress"),
	}}
	client, cred := newTransitionClient(t, f)

	if _, err := client.UpdateIssue(context.Background(), cred, domain.IssueUpdate{IssueID: "ENG-1", WorkflowStateID: "in progress"}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	tr, _ := f.postedBody["transition"].(map[string]any)
	if tr["id"] != "21" {
		t.Fatalf("want transition 21 (destination In Progress, matched case-insensitively), body=%v", f.postedBody)
	}
	if f.wrote("PUT ") {
		t.Error("a status-only update must not PUT an empty fields object")
	}
}

func TestUpdateIssue_WorkflowState_AcceptsATransitionID(t *testing.T) {
	f := &fakeJira{statusName: "To Do", statusKey: "new", transitions: []map[string]any{transition("21", "Start progress", "In Progress")}}
	client, cred := newTransitionClient(t, f)

	if _, err := client.UpdateIssue(context.Background(), cred, domain.IssueUpdate{IssueID: "ENG-1", WorkflowStateID: "21"}); err != nil {
		t.Fatal(err)
	}
	if tr, _ := f.postedBody["transition"].(map[string]any); tr["id"] != "21" {
		t.Errorf("want transition 21, got %v", f.postedBody)
	}
}

func TestUpdateIssue_WorkflowState_AlreadyThereIsANoOp(t *testing.T) {
	f := &fakeJira{statusName: "In Progress", statusKey: "indeterminate", transitions: []map[string]any{transition("21", "Start progress", "In Progress")}}
	client, cred := newTransitionClient(t, f)

	if _, err := client.UpdateIssue(context.Background(), cred, domain.IssueUpdate{IssueID: "ENG-1", WorkflowStateID: "In Progress"}); err != nil {
		t.Fatal(err)
	}
	if f.wrote("POST ") {
		t.Error("an issue already in the target status must not be transitioned again")
	}
}

// The core regression: before, an unreachable target "succeeded" without doing anything.
func TestUpdateIssue_WorkflowState_UnreachableTargetIsAnError(t *testing.T) {
	f := &fakeJira{statusName: "Done", statusKey: "done", transitions: []map[string]any{transition("31", "Reopen", "To Do")}}
	client, cred := newTransitionClient(t, f)

	_, err := client.UpdateIssue(context.Background(), cred, domain.IssueUpdate{IssueID: "ENG-1", WorkflowStateID: "In Progress"})
	if !errors.Is(err, ErrTransitionUnavailable) {
		t.Fatalf("want ErrTransitionUnavailable, got %v", err)
	}
	if f.wrote("POST ") {
		t.Error("nothing may be posted when no transition matches")
	}
}

func TestUpdateIssue_WorkflowState_JiraRejectionIsNotSwallowed(t *testing.T) {
	f := &fakeJira{statusName: "To Do", statusKey: "new", transitionErr: http.StatusBadRequest, transitions: []map[string]any{transition("21", "Start progress", "In Progress")}}
	client, cred := newTransitionClient(t, f)

	_, err := client.UpdateIssue(context.Background(), cred, domain.IssueUpdate{IssueID: "ENG-1", WorkflowStateID: "In Progress"})
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("a rejected transition must surface as an error, got %v", err)
	}
}

func TestUpdateIssue_TitleAndStatus_PutsFieldsThenTransitions(t *testing.T) {
	f := &fakeJira{statusName: "To Do", statusKey: "new", transitions: []map[string]any{transition("21", "Start progress", "In Progress")}}
	client, cred := newTransitionClient(t, f)

	if _, err := client.UpdateIssue(context.Background(), cred, domain.IssueUpdate{IssueID: "ENG-1", Title: "New title", WorkflowStateID: "In Progress"}); err != nil {
		t.Fatal(err)
	}
	put, post := -1, -1
	for i, r := range f.requests {
		if r == "PUT /rest/api/3/issue/ENG-1" {
			put = i
		}
		if r == "POST /rest/api/3/issue/ENG-1/transitions" {
			post = i
		}
	}
	if put < 0 || post < 0 || put > post {
		t.Errorf("want PUT then POST, got %v", f.requests)
	}
}

func TestUpdateIssue_TitleOnly_NeverListsTransitions(t *testing.T) {
	f := &fakeJira{statusName: "To Do", statusKey: "new"}
	client, cred := newTransitionClient(t, f)

	if _, err := client.UpdateIssue(context.Background(), cred, domain.IssueUpdate{IssueID: "ENG-1", Title: "New title"}); err != nil {
		t.Fatal(err)
	}
	if f.wrote("GET /rest/api/3/issue/ENG-1/transitions") || f.wrote("POST ") {
		t.Errorf("a field-only update must not touch transitions, got %v", f.requests)
	}
}

func TestGetIssue_MapsStatusCategoryToDomainVocabulary(t *testing.T) {
	cases := map[string]string{"new": "todo", "indeterminate": "in_progress", "done": "done", "": "", "weird": ""}
	for key, want := range cases {
		f := &fakeJira{statusName: "Whatever", statusKey: key}
		client, cred := newTransitionClient(t, f)

		issue, err := client.GetIssue(context.Background(), cred, "ENG-1")
		if err != nil {
			t.Fatal(err)
		}
		if issue.WorkflowState.Category != want {
			t.Errorf("statusCategory %q -> %q, want %q", key, issue.WorkflowState.Category, want)
		}
	}
}

func TestPickTransition_PrefersIDThenDestinationThenName(t *testing.T) {
	ts := []domain.Transition{
		{ID: "5", Name: "In Progress", To: domain.WorkflowState{Name: "Elsewhere"}}, // name collides with the target below
		{ID: "6", Name: "Go", To: domain.WorkflowState{Name: "In Progress"}},
	}
	if got, _ := pickTransition(ts, "In Progress"); got.ID != "6" {
		t.Errorf("destination status must win over a transition's own name, got %s", got.ID)
	}
	if got, ok := pickTransition(ts, "5"); !ok || got.ID != "5" {
		t.Errorf("an exact id must win, got %v %v", got, ok)
	}
	if _, ok := pickTransition(ts, "Nowhere"); ok {
		t.Error("an unknown target must not match anything")
	}
}
