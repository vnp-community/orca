// Package task_decompose renders the ai.complete prompt that asks a model to break a task into subtasks.
package task_decompose

import (
	"embed"
	"text/template"

	"github.com/stablyai/orca-go/prompts/render"
)

// ID names this prompt for provenance.
const ID = "task_decompose/decompose"

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(template.FuncMap{}, files)

// Input is the five context sources the prompt draws on: the task, its project, the project's tech stack, the subtasks it
// already has and the titles of recently finished tasks. Empty sources are left out. The text is the author's own and is
// sent as written: the prompt has no block that wraps it, so there is nothing to fence.
type Input struct {
	Title       string
	Description string
	AIContext   string
	// ProviderContext is what the AI provider integration knows about the task.
	ProviderContext string
	ProjectName     string
	RepoURL         string
	Languages       []string
	Frameworks      []string
	ExistingTitles  []string
	VelocityTitles  []string
}

// Render returns the prompt; the model's answer must be the JSON shape it states.
func Render(in Input) (string, error) {
	if err := render.Require(map[string]string{"Title": in.Title}); err != nil {
		return "", err
	}
	return set.Execute("decompose.tmpl", in)
}
