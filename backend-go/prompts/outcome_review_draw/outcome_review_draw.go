// Package outcome_review_draw renders the prompt of a draw run: a read-only agent uses the diagram-design skill to turn the
// diagram models of an Outcome Review into drawings and returns them in a nonce result block.
package outcome_review_draw

import (
	"embed"
	"fmt"
	"text/template"

	"github.com/stablyai/orca-go/prompts/output_language"
	"github.com/stablyai/orca-go/prompts/render"
	"github.com/stablyai/orca-go/prompts/request_context"
)

// ID names this prompt for provenance.
const ID = "outcome_review_draw/draw"

// Version is part of every preset digest: a drawing made with other wording is another drawing.
const Version = "outcome_review_draw/1"

const (
	ModelRunes      = 12000
	ViolationRunes  = 400
	MaxItems        = 4
	MaxViolations   = 30
	MaxHTMLKiB      = 56
	LabelRunes      = 80
	IDPrefixPattern = "rv-<section>-<variant>-"
)

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(funcs(), files, request_context.Files)

func funcs() template.FuncMap {
	f := request_context.Funcs()
	f["modelRunes"] = func() int { return ModelRunes }
	f["violationRunes"] = func() int { return ViolationRunes }
	return f
}

// Item is one diagram to draw. ModelJSON is the model as Orca checked it, so it is data from an earlier agent run and is fenced.
type Item struct {
	Section     string
	Variant     string
	Name        string
	Type        string
	InputDigest string
	ModelJSON   string
	// Violations are what Orca found wrong in this diagram's previous drawing; empty on the first attempt.
	Violations []string
}

// Style is the part of the drawing profile that changes the picture.
type Style struct {
	Detail   string
	Audience string
	Size     string
	Motion   bool
	Sketchy  bool
	Icons    bool
	// MaxNodes is the node budget of a diagram; the model is already within it unless the profile lowered it.
	MaxNodes int
	// Roles maps a role (accent, diff-add, diff-mod, diff-del) to the name of an Orca colour token.
	Roles map[string]string
}

// Input carries only data the model may see. It has no credential field by design.
type Input struct {
	Items       []Item
	Style       Style
	Language    output_language.Language
	ResultNonce string
}

type view struct {
	Input
	Version string
	Single  bool
}

// Render builds the prompt. At least one item and the nonce are required.
func Render(in Input) (string, error) {
	if err := render.Require(map[string]string{"ResultNonce": in.ResultNonce}); err != nil {
		return "", err
	}
	if len(in.Items) == 0 || len(in.Items) > MaxItems {
		return "", fmt.Errorf("outcome_review_draw: %d items; a run draws from 1 to %d", len(in.Items), MaxItems)
	}
	for _, it := range in.Items {
		if it.Section == "" || it.Variant == "" || it.Type == "" || it.InputDigest == "" || it.ModelJSON == "" {
			return "", fmt.Errorf("outcome_review_draw: an item needs section, variant, type, digest and model")
		}
	}
	for i := range in.Items {
		if len(in.Items[i].Violations) > MaxViolations {
			in.Items[i].Violations = in.Items[i].Violations[:MaxViolations]
		}
	}
	return set.Execute("draw.tmpl", view{Input: in, Version: Version, Single: len(in.Items) == 1})
}
