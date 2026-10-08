package domain

import (
	"encoding/json"
	"strings"
)

type FindingsDocument struct {
	SchemaVersion  int       `json:"schema_version"`
	Kind           string    `json:"kind"`
	Question       string    `json:"question,omitempty"`
	Summary        string    `json:"summary"`
	Findings       []Finding `json:"findings"`
	Recommendation struct {
		Statement string `json:"statement,omitempty"`
		Reason    string `json:"reason,omitempty"`
	} `json:"recommendation"`
	FollowUps []FindingFollowUp `json:"follow_ups,omitempty"`
	Unknowns  []string          `json:"unknowns,omitempty"`
}

type Finding struct {
	ID         string          `json:"id"`
	Statement  string          `json:"statement"`
	Confidence float64         `json:"confidence"`
	Evidence   []SupportingRef `json:"evidence,omitempty"`
}

// FindingFollowUp is prefill data for SpawnChildRequest; nothing creates a child from it automatically.
type FindingFollowUp struct {
	Title         string `json:"title"`
	SuggestedType string `json:"suggested_type"`
	Reason        string `json:"reason,omitempty"`
	Body          string `json:"body,omitempty"`
}

func ParseFindingsDocument(raw []byte) (FindingsDocument, error) {
	var d FindingsDocument
	err := decodeAnalysisDocument("findings", raw, &d)
	return d, err
}

func (d FindingsDocument) Marshal() ([]byte, error) { return json.Marshal(d) }

func (d FindingsDocument) Validate() error {
	const k = "findings"
	if d.SchemaVersion != 1 {
		return documentErr(k, "schema_version must be 1")
	}
	if d.Kind != k {
		return documentErr(k, "kind must be %q, got %q", k, d.Kind)
	}
	if len(d.Findings) == 0 {
		return documentErr(k, "at least one finding is required")
	}
	for i, f := range d.Findings {
		if f.Statement == "" {
			return documentErr(k, "findings[%d].statement is required", i)
		}
		if err := validateConfidence(k, "findings confidence", f.Confidence); err != nil {
			return err
		}
		if err := validateEvidence(k, f.Evidence, "file", "doc", "command"); err != nil {
			return err
		}
		if len(f.Evidence) == 0 && !d.mentionedInUnknowns(f) {
			return documentErr(k, "findings[%d] has no evidence and is not listed in unknowns", i)
		}
	}
	for i, f := range d.FollowUps {
		switch f.SuggestedType {
		case "change_request", "task":
		default:
			return documentErr(k, "follow_ups[%d].suggested_type must be change_request or task", i)
		}
	}
	return nil
}

func (d FindingsDocument) mentionedInUnknowns(f Finding) bool {
	for _, u := range d.Unknowns {
		if (f.ID != "" && strings.Contains(u, f.ID)) || strings.Contains(u, f.Statement) {
			return true
		}
	}
	return false
}
