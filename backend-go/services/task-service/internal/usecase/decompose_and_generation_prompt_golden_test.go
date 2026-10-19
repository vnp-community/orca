package usecase

import (
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestPromptGolden_buildDecomposePrompt(t *testing.T) {
	full := domain.Task{ID: "task-1", Title: "Add the cache layer", Description: "Cache the landing page for five minutes.", AIContext: "The service is written in Go."}
	stack := domain.TechStack{Languages: []string{"Go", "TypeScript"}, Frameworks: []string{"gRPC", "React"}}
	velocity := []domain.Task{{Title: "Add rate limiting"}, {Title: "Fix the login redirect"}}
	decompose := func(task domain.Task, providerCtx, projectName, repoURL string, stack domain.TechStack, existing []string, velocity []domain.Task) string {
		t.Helper()
		got, err := renderDecomposePrompt(task, providerCtx, projectName, repoURL, stack, existing, velocity)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	rendered := map[string]string{
		"title_only":                  decompose(domain.Task{Title: "Just a title"}, "", "", "", domain.TechStack{}, nil, nil),
		"task_detail_only":            decompose(full, "", "", "", domain.TechStack{}, nil, nil),
		"with_project":                decompose(full, "", "orca", "https://example.com/orca.git", domain.TechStack{}, nil, nil),
		"with_tech_stack":             decompose(full, "", "", "", stack, nil, nil),
		"with_existing_subtasks":      decompose(full, "", "", "", domain.TechStack{}, []string{"Write the cache", "Wire the cache"}, nil),
		"with_velocity":               decompose(full, "", "", "", domain.TechStack{}, nil, velocity),
		"with_provider_context":       decompose(full, "Provider: linear, issue ORCA-12", "", "", domain.TechStack{}, nil, nil),
		"all_sources":                 decompose(full, "Provider: linear, issue ORCA-12", "orca", "https://example.com/orca.git", stack, []string{"Write the cache"}, velocity),
		"description_without_context": decompose(domain.Task{Title: "T", Description: "D"}, "", "", "", domain.TechStack{}, nil, nil),
		"context_without_description": decompose(domain.Task{Title: "T", AIContext: "C"}, "", "", "", domain.TechStack{}, nil, nil),
	}
	checkPromptGoldens(t, "buildDecomposePrompt", rendered)
}

func TestPromptGolden_buildPromptGenerationPrompt(t *testing.T) {
	generation := func(task domain.Task, providerCtx string) string {
		t.Helper()
		got, err := renderPromptGenerationPrompt(task, providerCtx)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	rendered := map[string]string{
		"title_only":            generation(domain.Task{Title: "Just a title"}, ""),
		"with_description":      generation(domain.Task{Title: "T", Description: "Cache the landing page."}, ""),
		"with_ai_context":       generation(domain.Task{Title: "T", AIContext: "The service is written in Go."}, ""),
		"with_provider_context": generation(domain.Task{Title: "T"}, "Provider: linear, issue ORCA-12"),
		"all_sections":          generation(domain.Task{Title: "Add the cache layer", Description: "Cache the landing page.", AIContext: "The service is written in Go."}, "Provider: linear, issue ORCA-12"),
	}
	checkPromptGoldens(t, "buildPromptGenerationPrompt", rendered)
}
