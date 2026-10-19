// Package git_pull_request renders the ai.complete prompt that asks a model for a pull request title and description.
package git_pull_request

import (
	"embed"
	"text/template"

	"github.com/stablyai/orca-go/prompts/render"
)

// ID names this prompt for provenance.
const ID = "git_pull_request/fields"

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(template.FuncMap{}, files)

// Input is the diff against the base branch. The prompt has no block that wraps it, so there is nothing to fence.
type Input struct {
	Diff string
}

// Render returns the instruction followed directly by the diff. The reply's first line becomes the title, so the
// instruction must stay in front of the diff.
func Render(in Input) (string, error) {
	return set.Execute("fields.tmpl", in)
}
