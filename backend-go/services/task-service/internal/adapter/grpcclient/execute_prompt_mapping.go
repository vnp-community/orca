package grpcclient

import (
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/prompts/task_execution"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// renderExecutePrompt maps a task to the execute prompt, then resolves `{{outputs.<taskId>.*}}` in the rendered text.
// The substitution is data from earlier runs, not wording, so it stays here and runs after the template.
func renderExecutePrompt(task domain.Task, parent *domain.Task, completedDeps []domain.Task) (string, error) {
	in := task_execution.Input{
		Task: task_execution.Task{Title: task.Title, Description: task.Description, AIContext: task.AIContext, PromptTemplate: task.PromptTemplate},
	}
	if parent != nil {
		in.Parent = &task_execution.Parent{Title: parent.Title, Description: parent.Description}
	}
	for _, d := range completedDeps {
		in.CompletedDeps = append(in.CompletedDeps, task_execution.Dependency{Title: d.Title, Description: d.Description})
	}
	prompt, err := task_execution.Render(in)
	if err != nil {
		return "", fmt.Errorf("render execute prompt: %w", err)
	}
	return interpolateOutputs(prompt, completedDeps), nil
}

// renderDefaultExecutePrompt is the prompt of a run that came without one. A contract run also gets the way to report
// back: the result block the service's parser reads.
func renderDefaultExecutePrompt(task domain.Task, parent *domain.Task, completedDeps []domain.Task, opts *contractRunOptions, repoRoot string) (string, error) {
	prompt, err := renderExecutePrompt(task, parent, completedDeps)
	if err != nil {
		return "", err
	}
	if opts == nil {
		return prompt, nil
	}
	suffix, err := domain.ContractPromptSuffix(opts.nonce, opts.requirePassingCheck, opts.useKnowledgeHub && repoRoot != "")
	if err != nil {
		return "", fmt.Errorf("render result block instruction: %w", err)
	}
	return prompt + suffix, nil
}

// interpolateOutputs resolves `{{outputs.<taskId>.*}}` and `{{outputs.<taskId>.stdout}}` against completedDeps'
// LastExecutionOutput (stdout is the only field captured, so both tokens give the same value). A token naming a task
// that is not a finished dependency is left as written, so a broken reference is visible in the prompt.
func interpolateOutputs(prompt string, completedDeps []domain.Task) string {
	if len(completedDeps) == 0 {
		return prompt
	}
	for _, d := range completedDeps {
		prompt = strings.ReplaceAll(prompt, fmt.Sprintf("{{outputs.%s.stdout}}", d.ID), d.LastExecutionOutput)
		prompt = strings.ReplaceAll(prompt, fmt.Sprintf("{{outputs.%s.*}}", d.ID), d.LastExecutionOutput)
	}
	return prompt
}
