package domain

import (
	"encoding/json"
	"errors"
)

// ErrAnalysisDocumentInvalid wraps every schema violation of a diagnosis, findings or answer document.
var ErrAnalysisDocumentInvalid = errors.New("invalid analysis document")

type DiagnosisDocument struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
	Summary       string `json:"summary"`
	RootCause     struct {
		Statement  string          `json:"statement"`
		Confidence float64         `json:"confidence"`
		Evidence   []SupportingRef `json:"evidence,omitempty"`
	} `json:"root_cause"`
	Reproduction struct {
		Reproducible string   `json:"reproducible,omitempty"`
		Steps        []string `json:"steps,omitempty"`
		Notes        string   `json:"notes,omitempty"`
	} `json:"reproduction"`
	Impact struct {
		Severity           string   `json:"severity"`
		Scope              string   `json:"scope,omitempty"`
		AffectedComponents []string `json:"affected_components,omitempty"`
		UserFacing         bool     `json:"user_facing"`
		DataRisk           bool     `json:"data_risk"`
	} `json:"impact"`
	FixDirections                  []FixDirection `json:"fix_directions,omitempty"`
	SuggestedSize                  string         `json:"suggested_size,omitempty"`
	SuggestEscalateToChangeRequest bool           `json:"suggest_escalate_to_change_request"`
	EscalationReason               string         `json:"escalation_reason,omitempty"`
	Measurements                   []Measurement  `json:"measurements,omitempty"`
	OpenQuestions                  []string       `json:"open_questions,omitempty"`
}

type FixDirection struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Risk    string `json:"risk,omitempty"`
}

// Measurement only reserves room in the schema; measuring is CR-REQ-014.
type Measurement struct {
	Metric string  `json:"metric"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit,omitempty"`
	Method string  `json:"method,omitempty"`
}

func ParseDiagnosisDocument(raw []byte) (DiagnosisDocument, error) {
	var d DiagnosisDocument
	err := decodeAnalysisDocument("diagnosis", raw, &d)
	return d, err
}

func (d DiagnosisDocument) Marshal() ([]byte, error) { return json.Marshal(d) }

func (d DiagnosisDocument) Validate() error {
	const k = "diagnosis"
	if d.SchemaVersion != 1 {
		return documentErr(k, "schema_version must be 1")
	}
	if d.Kind != k {
		return documentErr(k, "kind must be %q, got %q", k, d.Kind)
	}
	if d.RootCause.Statement == "" {
		return documentErr(k, "root_cause.statement is required")
	}
	if err := validateConfidence(k, "root_cause.confidence", d.RootCause.Confidence); err != nil {
		return err
	}
	if err := validateEvidence(k, d.RootCause.Evidence, "file", "log", "command", "commit"); err != nil {
		return err
	}
	switch d.Reproduction.Reproducible {
	case "", "yes", "no", "unknown":
	default:
		return documentErr(k, "reproduction.reproducible must be yes, no or unknown")
	}
	switch d.Impact.Severity {
	case "low", "medium", "high", "critical":
	default:
		return documentErr(k, "impact.severity must be low, medium, high or critical")
	}
	seen := map[string]bool{}
	for i, f := range d.FixDirections {
		if f.ID == "" || seen[f.ID] {
			return documentErr(k, "fix_directions[%d].id must be present and unique", i)
		}
		seen[f.ID] = true
		switch f.Risk {
		case "", "low", "medium", "high":
		default:
			return documentErr(k, "fix_directions[%d].risk must be low, medium or high", i)
		}
	}
	switch d.SuggestedSize {
	case "", "S", "M", "L":
	default:
		return documentErr(k, "suggested_size must be S, M or L")
	}
	return nil
}
