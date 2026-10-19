package usecase

import (
	"fmt"

	"github.com/stablyai/orca-go/prompts/git_pull_request"
)

// renderPullRequestPrompt maps the diff to the ai.complete prompt that asks for the PR title and description.
func renderPullRequestPrompt(diff string) (string, error) {
	prompt, err := git_pull_request.Render(git_pull_request.Input{Diff: diff})
	if err != nil {
		return "", fmt.Errorf("render pull request prompt: %w", err)
	}
	return prompt, nil
}
