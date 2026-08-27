package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestGenerateAgentPrompt_RequiresTenantContext(t *testing.T) {
	uc := NewGenerateAgentPrompt(newFakeTaskRepository(), &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{}, &fakeAICompleter{})
	if _, err := uc.Execute(context.Background(), GenerateAgentPromptInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestGenerateAgentPrompt_TaskNotFound(t *testing.T) {
	uc := NewGenerateAgentPrompt(newFakeTaskRepository(), &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{}, &fakeAICompleter{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "does-not-exist"}); err == nil {
		t.Fatal("expected an error for a nonexistent task")
	}
}

func TestGenerateAgentPrompt_NotConnected_ReturnsFailedPrecondition(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := NewGenerateAgentPrompt(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connected: false}, &fakeAICompleter{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "t1"})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition, got %v", err)
	}
}

// TestGenerateAgentPrompt_SaveFalse_NeverPersists is the core Save=false
// contract: preview only, UpdatePromptTemplate must never be called.
func TestGenerateAgentPrompt_SaveFalse_NeverPersists(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "Build widget"}
	completer := &fakeAICompleter{content: "Do the thing carefully."}
	uc := NewGenerateAgentPrompt(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, completer)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "t1", Save: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != completer.content {
		t.Errorf("expected the generated text to be returned verbatim, got %q", got)
	}
	if tasks.tasks["t1"].PromptTemplate != "" {
		t.Errorf("expected Save=false to NOT persist, but PromptTemplate=%q", tasks.tasks["t1"].PromptTemplate)
	}
}

// TestGenerateAgentPrompt_SaveTrue_PersistsToPromptTemplate is Save=true's
// mirror case.
func TestGenerateAgentPrompt_SaveTrue_PersistsToPromptTemplate(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "Build widget"}
	completer := &fakeAICompleter{content: "Do the thing carefully."}
	uc := NewGenerateAgentPrompt(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, completer)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "t1", Save: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tasks.tasks["t1"].PromptTemplate; got != completer.content {
		t.Errorf("expected PromptTemplate=%q, got %q", completer.content, got)
	}
}

func TestGenerateAgentPrompt_RelayFailurePropagates(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := NewGenerateAgentPrompt(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, &fakeAICompleter{err: errors.New("boom")})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when the AI relay call fails")
	}
}
