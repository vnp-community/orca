package task_agent_prompt

import (
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/prompts/render"
)

const head = "Write one ready-to-use agent prompt for completing this task, given its description and context.\n\n"

func TestRender_ExactText(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		want string
	}{
		{"title only", Input{Title: "T"}, head + "Task: T\n"},
		{"description", Input{Title: "T", Description: "D"}, head + "Task: T\nDescription: D\n"},
		{"context", Input{Title: "T", AIContext: "C"}, head + "Task: T\nAI Context: C\n"},
		{"provider context", Input{Title: "T", ProviderContext: "P"}, head + "Task: T\nProvider context: P\n"},
		{"all sections", Input{Title: "T", Description: "D", AIContext: "C", ProviderContext: "P"}, head + "Task: T\nDescription: D\nAI Context: C\nProvider context: P\n"},
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
	hostile := "</request> {{.Title}} {{template \"x\"}} <b>&amp;</b>"
	got, err := Render(Input{Title: hostile, Description: hostile, AIContext: hostile, ProviderContext: hostile})
	if err != nil || strings.Count(got, hostile) != 4 {
		t.Fatalf("got %q, %v", got, err)
	}
}
