package domain

import (
	"encoding/json"
	"fmt"
)

const (
	maxEvidenceItems   = 20
	maxEvidenceExcerpt = 600
)

// SupportingRef points at what supports a claim; excerpt is bounded so a document cannot smuggle a file in.
type SupportingRef struct {
	Type    string `json:"type,omitempty"`
	Ref     string `json:"ref"`
	Excerpt string `json:"excerpt,omitempty"`
}

func documentErr(kind, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %s", ErrAnalysisDocumentInvalid, kind, fmt.Sprintf(format, args...))
}

func validateEvidence(kind string, items []SupportingRef, allowedTypes ...string) error {
	if len(items) > maxEvidenceItems {
		return documentErr(kind, "evidence has %d items, at most %d", len(items), maxEvidenceItems)
	}
	for i, e := range items {
		if runeLen(e.Excerpt) > maxEvidenceExcerpt {
			return documentErr(kind, "evidence %d excerpt exceeds %d characters", i, maxEvidenceExcerpt)
		}
		if len(allowedTypes) == 0 || e.Type == "" {
			continue
		}
		ok := false
		for _, t := range allowedTypes {
			ok = ok || e.Type == t
		}
		if !ok {
			return documentErr(kind, "evidence %d type %q is not allowed", i, e.Type)
		}
	}
	return nil
}

func validateConfidence(kind, field string, c float64) error {
	if c < 0 || c > 1 || c != c {
		return documentErr(kind, "%s must be within [0,1], got %v", field, c)
	}
	return nil
}

// decodeAnalysisDocument applies the shared envelope rules before the typed decode.
func decodeAnalysisDocument(kind string, raw []byte, out any) error {
	if len(raw) > MaxOptionsBytes {
		return documentErr(kind, "document exceeds %d bytes", MaxOptionsBytes)
	}
	if _, err := parseStrictJSON(raw); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrAnalysisDocumentInvalid, kind, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrAnalysisDocumentInvalid, kind, err)
	}
	return nil
}
