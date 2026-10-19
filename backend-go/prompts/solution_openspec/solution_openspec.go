// Package solution_openspec renders the prompt that restricts an agent to one OpenSpec change directory while it reads a Request.
package solution_openspec

import (
	"embed"
	"strings"
	"text/template"

	"github.com/stablyai/orca-go/prompts/render"
)

// ID names this prompt for provenance.
const ID = "solution_openspec/solution"

// BodyRunes caps the Request body.
const BodyRunes = 12000

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(template.FuncMap{
	"bodyRunes":    func() int { return BodyRunes },
	"stripControl": stripControl,
}, files)

type Input struct {
	RequestTitle string
	RequestBody  string
	ChangeID     string
}

// Render builds the prompt. ChangeID is required: it names the only directory the agent may write to.
func Render(in Input) (string, error) {
	if err := render.Require(map[string]string{"ChangeID": in.ChangeID}); err != nil {
		return "", err
	}
	return set.Execute("solution.tmpl", in)
}

// stripControl drops control characters other than newline, carriage return and tab.
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, s)
}
