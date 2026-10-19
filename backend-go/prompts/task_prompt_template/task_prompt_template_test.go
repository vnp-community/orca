package task_prompt_template

import (
	"strings"
	"testing"
)

var base = Input{TaskID: "T1.2", Ref: "REQ-142", ChangeID: "req-142-cache", Branch: "orca/req-142-cache"}

func TestRender_NamesTheTaskTheBranchAndTheChange(t *testing.T) {
	out, err := Render(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"task T1.2 of the OpenSpec change req-142-cache (Request REQ-142)", "local branch orca/req-142-cache under openspec/changes/req-142-cache/",
		"`git show orca/req-142-cache:<path>`", "\nRules:\n- Change only what the Code line names", "tdd-knowledge-writeback).\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q", want)
		}
	}
	if strings.Contains(out, "acceptance criteria") {
		t.Error("a task without criteria must not claim any")
	}
}

func TestRender_ListsTheCriteriaTheTaskServes(t *testing.T) {
	in := base
	in.Satisfies = []string{"AC-1", "AC-3"}
	out, err := Render(in)
	if err != nil || !strings.Contains(out, "serves the acceptance criteria AC-1, AC-3; the scenarios") {
		t.Fatalf("%q, %v", out, err)
	}
	if !strings.Contains(out, "must hold.\n\nRules:") {
		t.Error("a blank line separates the criteria from the rules")
	}
}
