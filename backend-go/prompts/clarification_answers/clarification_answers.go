// Package clarification_answers renders the block that tells the model which open questions the reviewer already settled.
package clarification_answers

import (
	"embed"
	"text/template"

	"github.com/stablyai/orca-go/prompts/render"
)

// ID names this prompt for provenance.
const ID = "clarification_answers/answers"

// Rune caps for the question text and the answer value; both are redacted after the cut.
const (
	QuestionRunes = 300
	AnswerRunes   = 1000
)

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(template.FuncMap{
	"questionRunes": func() int { return QuestionRunes },
	"answerRunes":   func() int { return AnswerRunes },
}, files)

// Answer is one answered question. Answer is the JSON the reviewer gave.
type Answer struct {
	Key      string
	Question string
	Answer   string
}

type Input struct {
	Answers []Answer
}

// Render returns the empty string when nothing was answered, so a caller can append the result unconditionally.
func Render(in Input) (string, error) {
	return set.Execute("answers.tmpl", in)
}
