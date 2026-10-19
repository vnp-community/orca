package usecase

import (
	"fmt"

	"github.com/stablyai/orca-go/prompts/task_agent_prompt"
	"github.com/stablyai/orca-go/prompts/task_decompose"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// renderDecomposePrompt maps a task and its project context to the decompose prompt (backend-go/prompts/task_decompose).
func renderDecomposePrompt(task domain.Task, providerCtx, projectName, repoURL string, stack domain.TechStack, existingTitles []string, velocity []domain.Task) (string, error) {
	in := task_decompose.Input{
		Title: task.Title, Description: task.Description, AIContext: task.AIContext, ProviderContext: providerCtx,
		ProjectName: projectName, RepoURL: repoURL, Languages: stack.Languages, Frameworks: stack.Frameworks, ExistingTitles: existingTitles,
	}
	for _, v := range velocity {
		in.VelocityTitles = append(in.VelocityTitles, v.Title)
	}
	prompt, err := task_decompose.Render(in)
	if err != nil {
		return "", fmt.Errorf("render decompose prompt: %w", err)
	}
	return prompt, nil
}

// renderPromptGenerationPrompt maps a task to the prompt that asks a model for the task's agent prompt.
func renderPromptGenerationPrompt(task domain.Task, providerCtx string) (string, error) {
	prompt, err := task_agent_prompt.Render(task_agent_prompt.Input{
		Title: task.Title, Description: task.Description, AIContext: task.AIContext, ProviderContext: providerCtx,
	})
	if err != nil {
		return "", fmt.Errorf("render agent prompt request: %w", err)
	}
	return prompt, nil
}
