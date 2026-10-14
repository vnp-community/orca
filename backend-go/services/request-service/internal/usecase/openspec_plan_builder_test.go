package usecase

import (
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

var builderCtx = TaskPromptContext{Ref: "REQ-142", ChangeID: "req-142-nen-ai", Branch: "request/142-proposal"}

func TestBuildTaskPromptTemplate_Golden(t *testing.T) {
	with := domain.TaskProposal{Title: "Dựng client", Satisfies: []string{"AC-1", "AC-2"}}
	without := domain.TaskProposal{Title: "Hồi quy"}
	checkGolden(t, "openspec_task_prompt_with_ac.golden", BuildTaskPromptTemplate(builderCtx, "T1.2", with))
	checkGolden(t, "openspec_task_prompt_without_ac.golden", BuildTaskPromptTemplate(builderCtx, "T3.1", without))
}

func TestBuildTaskPromptTemplate_Contract(t *testing.T) {
	p := BuildTaskPromptTemplate(builderCtx, "T1.2", domain.TaskProposal{Satisfies: []string{"AC-7"}})
	for _, must := range []string{
		"task T1.2 of the OpenSpec change req-142-nen-ai", "Request REQ-142", "git show request/142-proposal:<path>", "NOT checked out",
		"acceptance criteria AC-7", "docs/tdd/", ".tdd-agent/", "tdd-knowledge-writeback", "Run the Verify command",
	} {
		if !strings.Contains(p, must) {
			t.Fatalf("prompt lacks %q:\n%s", must, p)
		}
	}
	if strings.Contains(BuildTaskPromptTemplate(builderCtx, "T1.1", domain.TaskProposal{}), "acceptance criteria") {
		t.Fatal("a task without ac= must not claim criteria")
	}
	if len([]rune(p)) > 2000 {
		t.Fatalf("a standing prompt stays short: %d runes", len([]rune(p)))
	}
}

func TestFillTaskPrompts_NumbersTasksPerContainerAndKeepsTheRest(t *testing.T) {
	phased := domain.PlanProposal{Title: "P", Phases: []domain.PhaseProposal{
		{Title: "a", Tasks: []domain.TaskProposal{{Title: "x"}, {Title: "y", DependsOnIndices: []int{0}}}},
		{Title: "b", DependsOnPhaseIndices: []int{0}, Tasks: []domain.TaskProposal{{Title: "z"}}},
	}}
	got := FillTaskPrompts(phased, builderCtx)
	if !strings.Contains(got.Phases[0].Tasks[1].PromptTemplate, "task T1.2 ") || !strings.Contains(got.Phases[1].Tasks[0].PromptTemplate, "task T2.1 ") {
		t.Fatal("ids are T<phase>.<n>")
	}
	if len(got.Phases[0].Tasks[1].DependsOnIndices) != 1 || got.Phases[1].DependsOnPhaseIndices[0] != 0 || got.Title != "P" {
		t.Fatalf("nothing but the prompt changes: %+v", got)
	}
	if phased.Phases[0].Tasks[0].PromptTemplate != "" {
		t.Fatal("the input proposal must not be modified")
	}
	flat := FillTaskPrompts(domain.PlanProposal{Tasks: []domain.TaskProposal{{Title: "only"}}}, builderCtx)
	if !strings.Contains(flat.Tasks[0].PromptTemplate, "task T1.1 ") || len(flat.Phases) != 0 {
		t.Fatalf("%+v", flat)
	}
	if empty := FillTaskPrompts(domain.PlanProposal{}, builderCtx); len(empty.Tasks) != 0 || len(empty.Phases) != 0 {
		t.Fatalf("%+v", empty)
	}
}
