package usecase

import "testing"

// The mapping functions return an error the real callers turn into a failed run; a test treats it as a bug in the fixture.

func mustSolutionPrompt(t *testing.T, in SolutionPromptInput) string {
	t.Helper()
	text, err := RenderSolutionPrompt(in)
	if err != nil {
		t.Fatalf("RenderSolutionPrompt: %v", err)
	}
	return text
}

func mustPlanPrompt(t *testing.T, in PlanPromptInput) string {
	t.Helper()
	text, err := RenderPlanPrompt(in)
	if err != nil {
		t.Fatalf("RenderPlanPrompt: %v", err)
	}
	return text
}

func mustOpenSpecPlanPrompt(t *testing.T, in OpenSpecPlanPromptInput) string {
	t.Helper()
	text, err := RenderOpenSpecPlanPrompt(in)
	if err != nil {
		t.Fatalf("RenderOpenSpecPlanPrompt: %v", err)
	}
	return text
}
