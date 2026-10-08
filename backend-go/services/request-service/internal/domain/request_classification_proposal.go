package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"unicode/utf8"
)

const (
	// MaxClassificationAttempts caps AI calls per request, failures included.
	MaxClassificationAttempts = 5
	maxProposalReasonRunes    = 2000
)

type ClassificationProposal struct {
	Type       RequestType
	Size       RequestSize
	Urgency    Urgency
	Confidence float64
	Reason     string
}

// ProposalInvalidError means the model output did not satisfy the proposal contract.
// Reason is for logs only; it never carries model output verbatim.
type ProposalInvalidError struct{ Reason string }

func (e *ProposalInvalidError) Error() string { return "invalid classification proposal: " + e.Reason }

// Is makes errors.Is(err, ErrProposalInvalid) true for any ProposalInvalidError.
func (e *ProposalInvalidError) Is(target error) bool {
	_, ok := target.(*ProposalInvalidError)
	return ok
}

var ErrProposalInvalid error = &ProposalInvalidError{Reason: "invalid"}

func invalidProposal(format string, a ...any) error {
	return &ProposalInvalidError{Reason: fmt.Sprintf(format, a...)}
}

type rawProposal struct {
	Type       *string  `json:"type"`
	Size       *string  `json:"size"`
	Urgency    *string  `json:"urgency"`
	Confidence *float64 `json:"confidence"`
	Reason     *string  `json:"reason"`
}

// ParseClassificationProposal accepts model text that may wrap the JSON in prose or
// markdown, takes the first JSON object and validates it strictly: unknown fields, a
// missing field or a value outside the enums rejects the whole proposal.
func ParseClassificationProposal(raw []byte) (ClassificationProposal, error) {
	obj, ok := firstJSONObject(raw)
	if !ok {
		return ClassificationProposal{}, invalidProposal("no JSON object in output")
	}
	var rp rawProposal
	dec := json.NewDecoder(bytes.NewReader(obj))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rp); err != nil {
		return ClassificationProposal{}, invalidProposal("decode: %v", err)
	}
	if rp.Type == nil || rp.Size == nil || rp.Urgency == nil || rp.Confidence == nil || rp.Reason == nil {
		return ClassificationProposal{}, invalidProposal("missing field")
	}
	typ, err := ParseRequestType(*rp.Type)
	if err != nil {
		return ClassificationProposal{}, invalidProposal("type outside enum")
	}
	size, err := ParseSize(*rp.Size)
	if err != nil {
		return ClassificationProposal{}, invalidProposal("size outside enum")
	}
	urgency, err := ParseUrgency(*rp.Urgency)
	if err != nil {
		return ClassificationProposal{}, invalidProposal("urgency outside enum")
	}
	c := *rp.Confidence
	if math.IsNaN(c) || c < 0 || c > 1 {
		return ClassificationProposal{}, invalidProposal("confidence outside [0,1]")
	}
	if utf8.RuneCountInString(*rp.Reason) > maxProposalReasonRunes {
		return ClassificationProposal{}, invalidProposal("reason too long")
	}
	return ClassificationProposal{Type: typ, Size: size, Urgency: urgency, Confidence: c, Reason: *rp.Reason}, nil
}

// firstJSONObject scans for the first balanced {...}, honouring strings and escapes so a
// brace inside a string value does not end the object early.
func firstJSONObject(b []byte) ([]byte, bool) {
	start := bytes.IndexByte(b, '{')
	if start < 0 {
		return nil, false
	}
	depth, inString, escaped := 0, false, false
	for i := start; i < len(b); i++ {
		c := b[i]
		switch {
		case inString:
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
		case c == '"':
			inString = true
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return b[start : i+1], true
			}
		}
	}
	return nil, false
}
