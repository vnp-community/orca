package tools

import "github.com/google/jsonschema-go/jsonschema"

// Channel results have no stable typed shape (views, protos, maps), so every
// tool advertises the same loose envelope; list results arrive as {items}.
type listEnvelope struct {
	Items     []any  `json:"items"`
	Truncated bool   `json:"truncated,omitempty"`
	Hint      string `json:"hint,omitempty"`
	Untrusted bool   `json:"untrusted,omitempty"`
}

type objectEnvelope struct {
	Truncated bool   `json:"truncated,omitempty"`
	Hint      string `json:"hint,omitempty"`
	Untrusted bool   `json:"untrusted,omitempty"`
}

func outputEnvelopeSchema(list bool) *jsonschema.Schema {
	var s *jsonschema.Schema
	var err error
	if list {
		s, err = jsonschema.For[listEnvelope](nil)
	} else {
		s, err = jsonschema.For[objectEnvelope](nil)
	}
	if err != nil { // static types: a failure is a programming error
		panic(err)
	}
	s.AdditionalProperties = nil // extra result fields are expected
	s.Required = nil
	return s
}
