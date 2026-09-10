package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type GenerateAgentPromptInput struct {
	TaskID string
}

// GenerateAgentPrompt produces a Task.PromptTemplate string for one task —
// same ai.complete relay primitive as AIDecompose (AICompleter), a
// different prompt template (single-task instructions, not a subtask
// breakdown). Writes the result to Task.PromptTemplate, which
// SimpleExecutor.buildExecutePrompt (TASK-TG-005-03) later reads.
type GenerateAgentPrompt struct {
	tasks    TaskRepository
	resolver ProjectExecutionResolver
	relay    AICompleter
}

func NewGenerateAgentPrompt(tasks TaskRepository, resolver ProjectExecutionResolver, relay AICompleter) *GenerateAgentPrompt {
	return &GenerateAgentPrompt{tasks: tasks, resolver: resolver, relay: relay}
}

func (uc *GenerateAgentPrompt) Execute(ctx context.Context, in GenerateAgentPromptInput) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	task, err := uc.tasks.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return "", apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	connectionID, _, connected, err := uc.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil || !connected {
		return "", apperrors.New(apperrors.KindFailedPrecondition, "TASK_GENERATE_PROMPT_NO_CONNECTION", "task's project has no connected dev server for AI relay", err)
	}
	prompt := buildAgentPromptRequest(task) // distinct template from buildDecomposePrompt/buildExecutePrompt
	result, err := uc.relay.Complete(ctx, connectionID, prompt)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_GENERATE_PROMPT_FAILED", "failed to generate agent prompt via AI relay", err)
	}
	task.PromptTemplate = result
	// Update persists every column unconditionally (see
	// adapter/postgres.Repository.Update's doc comment) — task was just
	// loaded above, so every other field is written back unchanged, the
	// same "load, mutate one field, Update the whole struct" convention
	// UpdateTask's usecase already follows.
	if err := uc.tasks.Update(ctx, tenantID, task); err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_GENERATE_PROMPT_SAVE_FAILED", "failed to persist generated prompt template", err)
	}
	return result, nil
}

// buildAgentPromptRequest assembles the ai.complete prompt asking for a
// single-task agent prompt template — distinct from buildDecomposePrompt
// (which asks for a subtask breakdown) even though both relay through the
// same AICompleter primitive.
func buildAgentPromptRequest(task domain.Task) string {
	var b strings.Builder
	b.WriteString("Write a clear, actionable agent prompt an autonomous coding agent could follow to complete this task end to end. ")
	b.WriteString("Respond with ONLY the prompt text itself, no preamble or explanation.\n\n")
	b.WriteString("Task: ")
	b.WriteString(task.Title)
	if task.Description != "" {
		b.WriteString("\nDescription: ")
		b.WriteString(task.Description)
	}
	if len(task.AIContext) > 0 {
		b.WriteString("\nContext: ")
		b.WriteString(string(task.AIContext))
	}
	return b.String()
}
