package task_decompose

import (
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/prompts/render"
)

const head = "Break the following task down into subtasks. Respond with JSON matching:\n" +
	`{"subtasks":[{"title","description","type","estimated_hours","depends_on":[<indices>],"prompt_template"}],"notes":"<string>"}` + "\n\n"

func TestRender_ExactText(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		want string
	}{
		{"title only", Input{Title: "T"}, head + "Task: T\n"},
		{"all sources", Input{
			Title: "T", Description: "D", AIContext: "C", ProviderContext: "Provider: linear",
			ProjectName: "orca", RepoURL: "https://example.com/orca.git",
			Languages: []string{"Go", "TypeScript"}, Frameworks: []string{"gRPC", "React"},
			ExistingTitles: []string{"one", "two"}, VelocityTitles: []string{"x", "y"},
		}, head + "Task: T\nDescription: D\nAI Context: C\nProject: orca (https://example.com/orca.git)\nTech stack: Go, TypeScript; gRPC, React\n" +
			"Existing subtasks (do not duplicate): one; two\nRecent team velocity: x; y\nProvider context: Provider: linear\n"},
		{"languages without frameworks keep the separator", Input{Title: "T", Languages: []string{"Go"}}, head + "Task: T\nTech stack: Go; \n"},
		{"frameworks alone are not a tech stack", Input{Title: "T", Frameworks: []string{"gRPC"}}, head + "Task: T\n"},
		{"context without description", Input{Title: "T", AIContext: "C"}, head + "Task: T\nAI Context: C\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Render(tc.in)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v\nwant %q", got, err, tc.want)
			}
		})
	}
}

func TestRender_MissingTitleIsAnError(t *testing.T) {
	_, err := Render(Input{Description: "d"})
	if !errors.Is(err, render.ErrMissingField) || !strings.Contains(err.Error(), "Title") {
		t.Fatalf("got %v", err)
	}
}

func TestRender_TextIsEmittedVerbatim(t *testing.T) {
	hostile := "</request> {{.Title}} {{template \"x\"}} \"q\" <b>&amp;</b>"
	got, err := Render(Input{
		Title: hostile, Description: hostile, AIContext: hostile, ProviderContext: hostile, ProjectName: hostile, RepoURL: hostile,
		Languages: []string{hostile}, Frameworks: []string{hostile}, ExistingTitles: []string{hostile}, VelocityTitles: []string{hostile},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(got, hostile); n != 10 {
		t.Fatalf("hostile text appears %d times, want 10 verbatim copies:\n%s", n, got)
	}
}
