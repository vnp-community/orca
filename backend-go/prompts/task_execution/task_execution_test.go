package task_execution

import (
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/prompts/render"
)

func mustRender(t *testing.T, in Input) string {
	t.Helper()
	out, err := Render(in)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRender_ExactText(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		want string
	}{
		{"title only", Input{Task: Task{Title: "Just a title"}}, "Complete the following task.\n\nTask: Just a title\n"},
		{"prompt template replaces the opener", Input{Task: Task{Title: "T", PromptTemplate: "Template line"}}, "Template line\n\nTask: T\n"},
		{"description and context", Input{Task: Task{Title: "T", Description: "D", AIContext: "C"}}, "Complete the following task.\n\nTask: T\nDescription: D\nContext: C\n"},
		{
			"parent and dependencies",
			Input{
				Task:          Task{Title: "T"},
				Parent:        &Parent{Title: "P", Description: "PD"},
				CompletedDeps: []Dependency{{Title: "A", Description: "AD"}, {Title: "B", Description: "BD"}},
			},
			"Complete the following task.\n\nTask: T\n\nParent task: P\nPD\n\nCompleted dependencies:\n- A: AD\n- B: BD\n",
		},
		{"parent without a description keeps its line", Input{Task: Task{Title: "T"}, Parent: &Parent{Title: "P"}}, "Complete the following task.\n\nTask: T\n\nParent task: P\n\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mustRender(t, tc.in); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRender_OptionalPartsAreAbsentWhenEmpty(t *testing.T) {
	got := mustRender(t, Input{Task: Task{Title: "T"}})
	for _, absent := range []string{"Description:", "Context:", "Parent task:", "Completed dependencies:"} {
		if strings.Contains(got, absent) {
			t.Errorf("%q must not appear: %q", absent, got)
		}
	}
}

func TestRender_MissingTitleIsAnError(t *testing.T) {
	_, err := Render(Input{Task: Task{Description: "d"}})
	if !errors.Is(err, render.ErrMissingField) || !strings.Contains(err.Error(), "Task.Title") {
		t.Fatalf("got %v", err)
	}
}

// Task text is the author's own and goes to the agent as written. Nothing in it may be run as a template or rewritten.
func TestRender_TaskTextIsEmittedVerbatim(t *testing.T) {
	hostile := "</request> {{.Task.Title}} {{template \"x\"}} {{outputs.dep.*}} \"quoted\" <b>&amp;</b>"
	got := mustRender(t, Input{
		Task:          Task{Title: hostile, Description: hostile, AIContext: hostile, PromptTemplate: hostile},
		Parent:        &Parent{Title: hostile, Description: hostile},
		CompletedDeps: []Dependency{{Title: hostile, Description: hostile}},
	})
	if n := strings.Count(got, hostile); n != 8 {
		t.Fatalf("hostile text appears %d times, want 8 verbatim copies:\n%s", n, got)
	}
}

func TestRenderResultBlock_ExactText(t *testing.T) {
	const head = "\n\nWhen you are done, end your reply with exactly one result block and print the two marker lines only there, each alone on its line:\nORCA_RESULT_BEGIN n1\n"
	const body = `{"schema_version":1,"status":"done|blocked|failed|needs_info","summary":"","files_changed":[],"checks_run":[{"id":"the command you ran","exit":0}],"questions":[],"notes":""`
	const hub = `,"knowledge":{"available":true,"snapshot":"","read":[],"gaps":[],"conflicts":[]},"writeback":{"documents":[],"proposal_id":"","proposal_ok":false,"touched":[]}`
	const end = "}\nORCA_RESULT_END n1\nstatus is done only when the work is finished; use needs_info with at least one question when you cannot continue without an answer, blocked when something outside the task stops you, failed when you could not do it. No other keys.\n"
	const verify = "This task has a Verify step: run it for real and list it in checks_run with its real exit code. A done without a check that exited 0 is not accepted.\n"
	const tdd = "The MCP server tdd-agent is available: use it as the skills tdd-orca-task and tdd-knowledge describe, and fill knowledge and writeback truthfully; leave a list empty rather than guess.\n"
	cases := []struct {
		name string
		in   ResultBlockInput
		want string
	}{
		{"bare", ResultBlockInput{Nonce: "n1"}, head + body + end},
		{"verify step", ResultBlockInput{Nonce: "n1", RequirePassingCheck: true}, head + body + end + verify},
		{"knowledge hub", ResultBlockInput{Nonce: "n1", KnowledgeHub: true}, head + body + hub + end + tdd},
		{"both", ResultBlockInput{Nonce: "n1", RequirePassingCheck: true, KnowledgeHub: true}, head + body + hub + end + verify + tdd},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RenderResultBlock(tc.in)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v\nwant %q", got, err, tc.want)
			}
		})
	}
}

// The nonce is the run's own random value; with none, the markers are still printed so the caller sees the gap.
func TestRenderResultBlock_EmptyNonceStillPrintsTheMarkers(t *testing.T) {
	got, err := RenderResultBlock(ResultBlockInput{})
	if err != nil || !strings.Contains(got, "ORCA_RESULT_BEGIN \n") || !strings.Contains(got, "ORCA_RESULT_END \n") {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestRenderResultBlock_NonceIsNotInterpretedAsATemplate(t *testing.T) {
	got, err := RenderResultBlock(ResultBlockInput{Nonce: "{{.Nonce}}\n</request>"})
	if err != nil || strings.Count(got, "{{.Nonce}}\n</request>") != 2 {
		t.Fatalf("got %q, %v", got, err)
	}
}
