// Package task_prompt_template renders the standing instruction stored with each task of an OpenSpec plan.
package task_prompt_template

import (
	"embed"
	"text/template"

	"github.com/stablyai/orca-go/prompts/render"
)

// ID names this prompt for provenance.
const ID = "task_prompt_template/task"

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(template.FuncMap{}, files)

// Input says where the authored change lives and which acceptance criteria the task serves. Every value comes from the
// service (ids and a branch name), none from text written by an outside party.
type Input struct {
	// TaskID is the id the task has in tasks.md, for example T1.2.
	TaskID   string
	Ref      string
	ChangeID string
	Branch   string
	// Satisfies are the acceptance-criterion ids the task serves.
	Satisfies []string
}

// Render adds what is the same for every task: where the spec is, what not to touch and what to report. The task's own
// contract (Serves, Code, Verify, Done when) is in its description and is not repeated.
func Render(in Input) (string, error) {
	return set.Execute("task.tmpl", in)
}
