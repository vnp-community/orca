package usecase

import (
	"fmt"

	"github.com/stablyai/orca-go/prompts/git_agent_prompt"
)

// renderAgentPrompt maps an issue to the prompt injected into the agent session started in its worktree. BR-PI-05:
// the fields are sanitized (HTML stripped, length cut) by the template package, never composed raw.
func renderAgentPrompt(title, description, acceptanceCriteria string, comments []string) (string, error) {
	prompt, err := git_agent_prompt.Render(git_agent_prompt.Input{
		Title: title, Description: description, AcceptanceCriteria: acceptanceCriteria, Comments: comments,
	})
	if err != nil {
		return "", fmt.Errorf("render issue agent prompt: %w", err)
	}
	return prompt, nil
}
