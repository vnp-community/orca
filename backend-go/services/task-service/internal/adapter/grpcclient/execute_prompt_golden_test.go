package grpcclient

import (
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestPromptGolden_buildExecutePrompt(t *testing.T) {
	root := domain.Task{ID: "task-1", Title: "Add the cache layer", Description: "Cache the landing page for five minutes.", AIContext: "The service is written in Go."}
	parent := &domain.Task{ID: "task-parent", Title: "Speed up the landing page", Description: "Epic: the page must load in under 200 ms."}
	deps := []domain.Task{
		{ID: "task-dep-a", Title: "Measure the baseline", Description: "Record the current load time.", LastExecutionOutput: "baseline: 4012 ms"},
		{ID: "task-dep-b", Title: "Choose the cache", Description: "Pick a cache store.", LastExecutionOutput: "chose: in-process LRU"},
	}
	with := func(mod func(*domain.Task)) domain.Task {
		task := root
		if mod != nil {
			mod(&task)
		}
		return task
	}
	exec := func(task domain.Task, parent *domain.Task, deps []domain.Task) string {
		t.Helper()
		got, err := renderExecutePrompt(task, parent, deps)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	rendered := map[string]string{
		"root_without_template":           exec(root, nil, nil),
		"title_only":                      exec(domain.Task{ID: "t", Title: "Just a title"}, nil, nil),
		"with_prompt_template":            exec(with(func(k *domain.Task) { k.PromptTemplate = "You are implementing a cache.\nFollow the project rules." }), nil, nil),
		"without_description_and_context": exec(domain.Task{ID: "t", Title: "Only title", PromptTemplate: "Template line"}, nil, nil),
		"with_parent":                     exec(root, parent, nil),
		"with_completed_dependencies":     exec(root, nil, deps),
		"with_parent_and_dependencies":    exec(root, parent, deps),
		"outputs_wildcard_and_stdout_tokens": exec(with(func(k *domain.Task) {
			k.PromptTemplate = "Baseline was {{outputs.task-dep-a.*}}.\nChosen store: {{outputs.task-dep-b.stdout}}."
		}), nil, deps),
		"outputs_unknown_task_left_verbatim": exec(with(func(k *domain.Task) {
			k.PromptTemplate = "Unknown: {{outputs.task-missing.*}} and {{outputs.task-missing.stdout}}."
		}), nil, deps),
		"outputs_token_without_dependencies":    exec(with(func(k *domain.Task) { k.PromptTemplate = "Not resolved: {{outputs.task-dep-a.*}}" }), nil, nil),
		"outputs_token_in_description_resolved": exec(with(func(k *domain.Task) { k.Description = "Use {{outputs.task-dep-a.stdout}} as the target." }), parent, deps),
		"all_sections": exec(with(func(k *domain.Task) {
			k.PromptTemplate = "Context first: {{outputs.task-dep-b.*}}"
		}), parent, deps),
	}
	checkPromptGoldens(t, "buildExecutePrompt", rendered)
}
