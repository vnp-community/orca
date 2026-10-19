// Package task_agent_prompt renders the ai.complete prompt that asks a model to write the agent prompt for a task.
package task_agent_prompt

import (
	"embed"
	"text/template"

	"github.com/stablyai/orca-go/prompts/render"
)

// ID names this prompt for provenance.
const ID = "task_agent_prompt/generate"

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(template.FuncMap{}, files)

// Input is the task and what the AI provider integration knows about it. Empty fields are left out. The text is the
// author's own and is sent as written: the prompt has no block that wraps it, so there is nothing to fence.
type Input struct {
	Title           string
	Description     string
	AIContext       string
	ProviderContext string
}

// Render returns the prompt.
func Render(in Input) (string, error) {
	if err := render.Require(map[string]string{"Title": in.Title}); err != nil {
		return "", err
	}
	return set.Execute("generate.tmpl", in)
}
