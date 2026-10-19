// Package request_context holds what several prompt flows say about the Request: its data, the partial templates that fence it, and the length caps.
package request_context

import (
	"embed"
	"text/template"
)

// Files are the partial templates (_request_block, _project, _feedback, _retry_note, _approved_decision).
//
//go:embed *.tmpl
var Files embed.FS

// Rune caps for text written by outside parties. A longer text is cut before it is fenced.
const (
	BodyRunes     = 12000
	ArtifactRunes = 8000
	FeedbackRunes = 2000
	RetryRunes    = 1000
)

// Funcs names the caps for the partial templates.
func Funcs() template.FuncMap {
	return template.FuncMap{
		"bodyRunes":     func() int { return BodyRunes },
		"artifactRunes": func() int { return ArtifactRunes },
		"feedbackRunes": func() int { return FeedbackRunes },
		"retryRunes":    func() int { return RetryRunes },
	}
}

// Request is the part of a Request every prompt shows. Type, Size, Urgency and SourceProvider are the domain's string values.
type Request struct {
	Title                string
	Type                 string
	Size                 string
	Urgency              string
	SourceProvider       string
	ClassificationReason string
	Body                 string
	// AcceptanceCriteria are the active criteria only; the model cites them by id.
	AcceptanceCriteria []Criterion
}

// Criterion is one acceptance criterion of the Request.
type Criterion struct {
	ID   string
	Text string
}

// Project names the repository the work is for. Both fields empty means the block is left out.
type Project struct {
	Name    string
	RepoURL string
}
