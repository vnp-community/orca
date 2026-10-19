// Package git_agent_prompt renders the prompt typed into the agent session that starts in a worktree created from an issue.
package git_agent_prompt

import (
	"embed"
	"html"
	"regexp"
	"text/template"

	"github.com/stablyai/orca-go/prompts/render"
)

// ID names this prompt for provenance.
const ID = "git_agent_prompt/issue"

// FieldBytes is the length one issue field is cut to, counted in bytes.
const FieldBytes = 4000

var htmlTag = regexp.MustCompile(`<[^>]*>`)

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(template.FuncMap{"sanitize": Sanitize}, files)

// Input is an issue as the tracker returned it: text written by outside parties, cleaned by Sanitize inside the template.
type Input struct {
	Title              string
	Description        string
	AcceptanceCriteria string
	Comments           []string
}

// Render returns the prompt; sections whose raw value is empty are left out.
func Render(in Input) (string, error) {
	return set.Execute("issue.tmpl", in)
}

// Sanitize removes HTML tags, unescapes entities and cuts the result to FieldBytes bytes. A hostile or huge issue body
// must not reach the agent as markup or blow its context budget (BR-PI-05).
func Sanitize(s string) string {
	s = htmlTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	if len(s) <= FieldBytes {
		return s
	}
	return s[:FieldBytes]
}
