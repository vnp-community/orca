package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/prompts/render"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestRenderDecomposePrompt_TaskWithoutTitleIsAnError(t *testing.T) {
	_, err := renderDecomposePrompt(domain.Task{Description: "d"}, "", "", "", domain.TechStack{}, nil, nil)
	if !errors.Is(err, render.ErrMissingField) {
		t.Fatalf("got %v, want ErrMissingField", err)
	}
}

func TestRenderPromptGenerationPrompt_TaskWithoutTitleIsAnError(t *testing.T) {
	_, err := renderPromptGenerationPrompt(domain.Task{Description: "d"}, "")
	if !errors.Is(err, render.ErrMissingField) {
		t.Fatalf("got %v, want ErrMissingField", err)
	}
}

// A prompt that cannot be built must stop the call before the AI relay is paid for, with a code of its own.
func TestAIDecompose_PromptBuildFailure_DoesNotCallTheRelay(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	completer := &fakeAICompleter{content: `{"subtasks":[]}`}
	uc := newAIDecomposeForTest(tasks, &fakeEdgeRepository{}, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, completer)

	_, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), AIDecomposeInput{TaskID: "t1"})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "TASK_AI_DECOMPOSE_PROMPT_BUILD_FAILED" || completer.gotConn != "" {
		t.Fatalf("got %v (relay called with %q), want TASK_AI_DECOMPOSE_PROMPT_BUILD_FAILED and no relay call", err, completer.gotConn)
	}
}

func TestGenerateAgentPrompt_PromptBuildFailure_DoesNotCallTheRelay(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	completer := &fakeAICompleter{content: "generated"}
	uc := NewGenerateAgentPrompt(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, completer)

	_, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), GenerateAgentPromptInput{TaskID: "t1"})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "TASK_GENERATE_PROMPT_BUILD_FAILED" || completer.gotConn != "" {
		t.Fatalf("got %v (relay called with %q), want TASK_GENERATE_PROMPT_BUILD_FAILED and no relay call", err, completer.gotConn)
	}
}
