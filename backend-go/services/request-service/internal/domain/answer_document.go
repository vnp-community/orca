package domain

import "encoding/json"

const maxAnswerRunes = 8000

type AnswerDocument struct {
	SchemaVersion  int        `json:"schema_version"`
	Kind           string     `json:"kind"`
	AnswerMarkdown string     `json:"answer_markdown"`
	Confidence     float64    `json:"confidence"`
	Citations      []Citation `json:"citations,omitempty"`
	Limitations    []string   `json:"limitations,omitempty"`
	// SuggestedFollowUp is display-only; it never changes the request type.
	SuggestedFollowUp *struct {
		Type  string `json:"type"`
		Title string `json:"title"`
	} `json:"suggested_follow_up,omitempty"`
}

type Citation struct {
	Ref     string `json:"ref"`
	Excerpt string `json:"excerpt,omitempty"`
}

func ParseAnswerDocument(raw []byte) (AnswerDocument, error) {
	var d AnswerDocument
	err := decodeAnalysisDocument("answer", raw, &d)
	return d, err
}

func (d AnswerDocument) Marshal() ([]byte, error) { return json.Marshal(d) }

func (d AnswerDocument) Validate() error {
	const k = "answer"
	if d.SchemaVersion != 1 {
		return documentErr(k, "schema_version must be 1")
	}
	if d.Kind != k {
		return documentErr(k, "kind must be %q, got %q", k, d.Kind)
	}
	if n := runeLen(d.AnswerMarkdown); n < 1 || n > maxAnswerRunes {
		return documentErr(k, "answer_markdown must be 1..%d characters, got %d", maxAnswerRunes, n)
	}
	if err := validateConfidence(k, "confidence", d.Confidence); err != nil {
		return err
	}
	for i, c := range d.Citations {
		if runeLen(c.Excerpt) > maxEvidenceExcerpt {
			return documentErr(k, "citations[%d] excerpt exceeds %d characters", i, maxEvidenceExcerpt)
		}
	}
	if f := d.SuggestedFollowUp; f != nil && f.Type != "change_request" && f.Type != "task" {
		return documentErr(k, "suggested_follow_up.type must be change_request or task")
	}
	return nil
}
