// Package classify_request renders the prompt that asks a model for the type, size and urgency of a new Request.
package classify_request

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/stablyai/orca-go/prompts/render"
)

// ID names this prompt for provenance.
const ID = "classify_request/classify"

// BodyRunes caps the Request body; a longer body is cut before it is wrapped.
const BodyRunes = 20000

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(template.FuncMap{"bodyRunes": func() int { return BodyRunes }}, files)

// Input is the text written by the Request's author plus the issue-tracker facts that came with it.
type Input struct {
	Title     string
	IssueType string
	Labels    []string
	Body      string
}

type view struct {
	Input
	Boundary string
	Content  string
}

// Render wraps the untrusted text in a boundary that is random per call and absent from the text, so the text cannot
// close the data block and pose as instructions.
func Render(in Input) (string, error) {
	return renderWith(in, rand.Reader)
}

func renderWith(in Input, entropy io.Reader) (string, error) {
	content, err := set.Execute("_content.tmpl", in)
	if err != nil {
		return "", err
	}
	boundary, err := newBoundary(content, entropy)
	if err != nil {
		return "", err
	}
	return set.Execute("classify.tmpl", view{Input: in, Boundary: boundary, Content: content})
}

func newBoundary(content string, entropy io.Reader) (string, error) {
	for {
		var raw [8]byte
		if _, err := io.ReadFull(entropy, raw[:]); err != nil {
			return "", fmt.Errorf("draw a prompt boundary: %w", err)
		}
		b := "<<<REQUEST_DATA_" + hex.EncodeToString(raw[:]) + ">>>"
		if !strings.Contains(content, b) {
			return b, nil
		}
	}
}
