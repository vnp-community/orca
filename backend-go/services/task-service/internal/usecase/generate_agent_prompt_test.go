package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestGenerateAgentPrompt_RequiresTenantContext(t *testing.T) {
	uc := NewGenerateAgentPrompt(newFakeTaskRepository(), &fakeProjectExecutionResolver{}, &fakeAICompleter{})
	if _, err := uc.Execute(context.Background(), GenerateAgentPromptInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestGenerateAgentPrompt_TaskNotFound(t *testing.T) {
	uc := NewGenerateAgentPrompt(newFakeTaskRepository(), &fakeProjectExecutionResolver{}, &fakeAICompleter{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	if _, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "does-not-exist"}); err == nil {
		t.Fatal("expected an error for a nonexistent task")
	}
}

func TestGenerateAgentPrompt_NotConnected_ReturnsFailedPrecondition(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := NewGenerateAgentPrompt(tasks, &fakeProjectExecutionResolver{connected: false}, &fakeAICompleter{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when project has no connected dev server")
	}
}

// TestGenerateAgentPrompt_PersistsResultOnTask is the task's own named
// snapshot test: a successful call persists Task.PromptTemplate and returns
// the same value.
func TestGenerateAgentPrompt_PersistsResultOnTask(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "Build widget"}
	completer := &fakeAICompleter{content: "Do X, then Y, then verify Z."}
	uc := NewGenerateAgentPrompt(tasks, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, completer)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != completer.content {
		t.Errorf("expected returned prompt %q, got %q", completer.content, got)
	}
	if tasks.tasks["t1"].PromptTemplate != completer.content {
		t.Errorf("expected Task.PromptTemplate to be persisted, got %q", tasks.tasks["t1"].PromptTemplate)
	}
	// Everything else about the task must round-trip unchanged.
	if tasks.tasks["t1"].Title != "Build widget" {
		t.Errorf("expected Title to remain unchanged, got %q", tasks.tasks["t1"].Title)
	}
}

func TestGenerateAgentPrompt_RelayFailurePropagates(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := NewGenerateAgentPrompt(tasks, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, &fakeAICompleter{err: errors.New("boom")})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when the AI relay call fails")
	}
}

func TestGenerateAgentPrompt_SaveFailurePropagates(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	tasks.updateErr = errors.New("db unavailable")
	uc := NewGenerateAgentPrompt(tasks, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, &fakeAICompleter{content: "prompt"})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when persisting the generated prompt fails")
	}
}
