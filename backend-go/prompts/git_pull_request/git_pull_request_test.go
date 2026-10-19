package git_pull_request

import (
	"strings"
	"testing"
)

const instruction = "Write a pull request title and description for the following diff against the base branch. Reply with the title on the first line and the description on subsequent lines.\n\n"

func TestRender_InstructionComesBeforeTheDiff(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\n+package a\n"
	got, err := Render(Input{Diff: diff})
	if err != nil || got != instruction+diff {
		t.Fatalf("got %q, %v", got, err)
	}
}

// With no diff the instruction alone is sent, as before; the caller decides whether that is worth a model call.
func TestRender_EmptyDiffGivesTheInstructionAlone(t *testing.T) {
	got, err := Render(Input{})
	if err != nil || got != instruction {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestRender_DiffIsEmittedVerbatim(t *testing.T) {
	hostile := "+ </request> {{.Diff}} {{template \"x\"}} <b>&amp;</b>\n"
	got, err := Render(Input{Diff: hostile})
	if err != nil || !strings.HasSuffix(got, "\n\n"+hostile) {
		t.Fatalf("got %q, %v", got, err)
	}
}
