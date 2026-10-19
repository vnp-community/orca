package usecase

import (
	"fmt"

	"github.com/stablyai/orca-go/prompts/task_prompt_template"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// TaskPromptContext says where the authored change lives, so each task can point its agent at the spec.
type TaskPromptContext struct {
	Ref      string
	ChangeID string
	Branch   string
}

// renderTaskPrompt maps one task to prompts/task_prompt_template.
func renderTaskPrompt(c TaskPromptContext, taskID string, t domain.TaskProposal) (string, error) {
	return builtPrompt(task_prompt_template.Render(task_prompt_template.Input{
		TaskID: taskID, Ref: c.Ref, ChangeID: c.ChangeID, Branch: c.Branch, Satisfies: t.Satisfies,
	}))
}

// FillTaskPrompts sets PromptTemplate and the spec label on every task of the proposal. Ids are positional, like the ones in tasks.md.
func FillTaskPrompts(p domain.PlanProposal, c TaskPromptContext) (domain.PlanProposal, error) {
	out := p
	var err error
	if out.Tasks, err = fillTasks(p.Tasks, 1, c); err != nil {
		return domain.PlanProposal{}, err
	}
	out.Phases = nil
	for i, ph := range p.Phases {
		cp := ph
		if cp.Tasks, err = fillTasks(ph.Tasks, i+1, c); err != nil {
			return domain.PlanProposal{}, err
		}
		out.Phases = append(out.Phases, cp)
	}
	return out, nil
}

func fillTasks(in []domain.TaskProposal, container int, c TaskPromptContext) ([]domain.TaskProposal, error) {
	if in == nil {
		return nil, nil
	}
	out := make([]domain.TaskProposal, len(in))
	for j, t := range in {
		ref := fmt.Sprintf("T%d.%d", container, j+1)
		text, err := renderTaskPrompt(c, ref, t)
		if err != nil {
			return nil, err
		}
		t.PromptTemplate = text
		// The label is how tasks.md is ticked later: the task carries the line it came from.
		t.Labels = append(append([]string(nil), t.Labels...), domain.SpecLabel(ref))
		out[j] = t
	}
	return out, nil
}
