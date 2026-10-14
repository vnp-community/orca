package usecase

import (
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// TaskPromptContext says where the authored change lives, so each task can point its agent at the spec.
type TaskPromptContext struct {
	Ref      string
	ChangeID string
	Branch   string
}

// BuildTaskPromptTemplate is the standing instruction of one task of an OpenSpec plan. The task's own contract (Serves, Code,
// Verify, Done when) is already in its description and is not repeated here; this adds what is the same for every task: where
// the spec is, what not to touch, and what to report.
func BuildTaskPromptTemplate(c TaskPromptContext, taskID string, t domain.TaskProposal) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are implementing task %s of the OpenSpec change %s (Request %s). The task's own contract is under \"Description\" below: where to change code, how it is verified and when it is done.\n\n", taskID, c.ChangeID, c.Ref)
	fmt.Fprintf(&b, "The whole change is on the local branch %s under openspec/changes/%s/ and is NOT checked out in this worktree. Read what you need with one `git show %s:<path>` per command (tasks.md, design.md, specs/<capability>/spec.md) and never copy it into this worktree.\n", c.Branch, c.ChangeID, c.Branch)
	if len(t.Satisfies) > 0 {
		fmt.Fprintf(&b, "This task serves the acceptance criteria %s; the scenarios whose names start with those ids in the specs say what must hold.\n", strings.Join(t.Satisfies, ", "))
	}
	b.WriteString("\nRules:\n")
	b.WriteString("- Change only what the Code line names, plus the tests it needs. Do not edit openspec/, docs/tdd/ or .tdd-agent/ unless the task says so.\n")
	b.WriteString("- Before you edit a symbol, check what depends on it with the project's own tools, and follow the project's rules.\n")
	b.WriteString("- If the MCP server tdd-agent is available, use the skills tdd-orca-task and tdd-knowledge to read the requirements, rules and decisions named on the Serves line, and cite what you read. If it is not available, say so and continue from the spec and the code.\n")
	b.WriteString("- Run the Verify command and report its real output. A task is done only when that command succeeds.\n")
	b.WriteString("- If your change alters behaviour, an interface or a documented rule, list the design knowledge it affects (skill tdd-knowledge-writeback).\n")
	return b.String()
}

// FillTaskPrompts sets PromptTemplate on every task of the proposal. Ids are positional, like the ones in tasks.md.
func FillTaskPrompts(p domain.PlanProposal, c TaskPromptContext) domain.PlanProposal {
	out := p
	out.Tasks = fillTasks(p.Tasks, 1, c)
	out.Phases = nil
	for i, ph := range p.Phases {
		cp := ph
		cp.Tasks = fillTasks(ph.Tasks, i+1, c)
		out.Phases = append(out.Phases, cp)
	}
	return out
}

func fillTasks(in []domain.TaskProposal, container int, c TaskPromptContext) []domain.TaskProposal {
	if in == nil {
		return nil
	}
	out := make([]domain.TaskProposal, len(in))
	for j, t := range in {
		t.PromptTemplate = BuildTaskPromptTemplate(c, fmt.Sprintf("T%d.%d", container, j+1), t)
		out[j] = t
	}
	return out
}
