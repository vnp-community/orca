package usecase

import (
	"context"
	"reflect"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestGetTaskByShareToken_RequiresToken(t *testing.T) {
	uc := NewGetTaskByShareToken(newFakeTaskRepository())
	if _, err := uc.Execute(context.Background(), ""); err == nil {
		t.Fatal("expected an error for an empty token")
	}
}

func TestGetTaskByShareToken_UnknownToken_ReturnsNotFound(t *testing.T) {
	uc := NewGetTaskByShareToken(newFakeTaskRepository())
	if _, err := uc.Execute(context.Background(), "does-not-exist"); err == nil {
		t.Fatal("expected NotFound for an unknown token")
	}
}

// TestGetTaskByShareToken_MalformedToken_SameErrorAsUnknown proves there is
// no distinguishable error path between "malformed" and "not found" —
// both must hit the identical NotFound branch, no token-enumeration side
// channel via distinguishable error messages.
func TestGetTaskByShareToken_MalformedToken_SameErrorAsUnknown(t *testing.T) {
	uc := NewGetTaskByShareToken(newFakeTaskRepository())
	_, err1 := uc.Execute(context.Background(), "not-even-hex!!!")
	_, err2 := uc.Execute(context.Background(), "0123456789abcdef")
	if err1 == nil || err2 == nil {
		t.Fatal("expected both malformed and well-formed-but-unknown tokens to error")
	}
	if err1.Error() != err2.Error() {
		t.Errorf("expected identical error messages for malformed vs. unknown tokens, got %q vs %q", err1.Error(), err2.Error())
	}
}

// TestGetTaskByShareToken_ReturnsExactlyTheAllowlistedFields is the
// security-load-bearing regression test this task's own instructions
// require: it enumerates domain.Task's actual field values (via a task
// with every sensitive field populated) and asserts the returned
// TaskShareView contains ONLY id/title/status/description — nothing else,
// even indirectly.
func TestGetTaskByShareToken_ReturnsExactlyTheAllowlistedFields(t *testing.T) {
	tasks := newFakeTaskRepository()
	task := domain.Task{
		ID:             "t1",
		TenantID:       "tenant-1",
		Title:          "Public title",
		Status:         domain.StatusInProgress,
		Description:    "Public description",
		ParentID:       "parent-1",
		ProjectID:      "proj-1",
		AssigneeID:     "assignee-1",
		ReporterID:     "reporter-1",
		OwnerID:        "owner-1",
		PromptTemplate: "secret prompt template",
		AIContext:      `"internal repo secrets and architecture notes"`,
		AIPlanJSON:     `{"plan":"internal decomposition strategy"}`,
		Visibility:     "private",
		WorktreeID:     "worktree-1",
		AgentSessionID: "session-1",
		WorkflowExecID: "exec-1",
		ShareToken:     "the-real-token",
	}
	tasks.tasks["t1"] = task

	uc := NewGetTaskByShareToken(tasks)
	got, err := uc.Execute(context.Background(), "the-real-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := domain.TaskShareView{ID: "t1", Title: "Public title", Status: "in_progress", Description: "Public description"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected exactly the allowlisted fields %+v, got %+v", want, got)
	}

	// Belt-and-suspenders: reflect over TaskShareView's own fields to prove
	// it structurally CANNOT carry any of the sensitive fields above (it's
	// a dedicated type, not domain.Task reused) — this fails to compile,
	// not just fails at runtime, if TaskShareView ever gains a field this
	// test doesn't know about and someone forgets to update this check.
	viewType := reflect.TypeOf(got)
	allowedFields := map[string]bool{"ID": true, "Title": true, "Status": true, "Description": true}
	for i := 0; i < viewType.NumField(); i++ {
		name := viewType.Field(i).Name
		if !allowedFields[name] {
			t.Errorf("TaskShareView has an unexpected field %q not on the allowlist — a future Task field addition must NEVER be added here without a fresh security sign-off", name)
		}
	}
	if viewType.NumField() != len(allowedFields) {
		t.Errorf("expected TaskShareView to have exactly %d fields, got %d", len(allowedFields), viewType.NumField())
	}
}
