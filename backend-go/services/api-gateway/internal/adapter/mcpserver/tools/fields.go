package tools

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

type fieldType int

const (
	tString fieldType = iota
	tInt
	tBool
	tStrings
)

// Field is one tool input property and its wire key in the channel's args[0].
// Names are snake_case for the LLM; wire keys keep whatever the handler reads
// (they differ per channel: "worktree" vs "worktreeId").
type Field struct {
	Name, Wire, Desc string
	Type             fieldType
	Required         bool
	Prefix           string // prepended to string values (git.* selectors want "id:")
	Enum             []string
	Min, Max         *float64
	MaxLen           int
}

// FieldOpt tweaks a Field.
type FieldOpt func(*Field)

// Req marks the field required.
func Req(f *Field) { f.Required = true }

// IDSel prefixes the value with "id:" (worktree selector form of git.*).
func IDSel(f *Field) { f.Prefix = "id:" }

// OneOf restricts a string to an enum.
func OneOf(v ...string) FieldOpt { return func(f *Field) { f.Enum = v } }

// Range bounds an integer.
func Range(lo, hi float64) FieldOpt { return func(f *Field) { f.Min, f.Max = &lo, &hi } }

// Len caps a string's length.
func Len(n int) FieldOpt { return func(f *Field) { f.MaxLen = n } }

func mk(t fieldType, name, wire, desc string, opts []FieldOpt) Field {
	f := Field{Name: name, Wire: wire, Desc: desc, Type: t}
	for _, o := range opts {
		o(&f)
	}
	return f
}

func Str(name, wire, desc string, o ...FieldOpt) Field  { return mk(tString, name, wire, desc, o) }
func Int(name, wire, desc string, o ...FieldOpt) Field  { return mk(tInt, name, wire, desc, o) }
func Bool(name, wire, desc string, o ...FieldOpt) Field { return mk(tBool, name, wire, desc, o) }
func Strs(name, wire, desc string, o ...FieldOpt) Field { return mk(tStrings, name, wire, desc, o) }

func (f Field) schema() *jsonschema.Schema {
	s := &jsonschema.Schema{Description: f.Desc}
	switch f.Type {
	case tString:
		s.Type = "string"
		for _, e := range f.Enum {
			s.Enum = append(s.Enum, e)
		}
		ml := f.MaxLen
		if ml == 0 {
			ml = 8192
		}
		s.MaxLength = &ml
		if f.Required {
			one := 1
			s.MinLength = &one
		}
	case tInt:
		s.Type = "integer"
		s.Minimum, s.Maximum = f.Min, f.Max
	case tBool:
		s.Type = "boolean"
	case tStrings:
		s.Type = "array"
		s.Items = &jsonschema.Schema{Type: "string"}
		max := 200
		s.MaxItems = &max
	}
	return s
}

// inputSchema builds a closed object schema: unknown properties are rejected,
// which is also what keeps tenantId/userId out of tool input.
func inputSchema(fields []Field) *jsonschema.Schema {
	s := &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}}}
	for _, f := range fields {
		s.Properties[f.Name] = f.schema()
		if f.Required {
			s.Required = append(s.Required, f.Name)
		}
	}
	return s
}

// defaultArgs maps validated input onto the wire object.
func defaultArgs(fields []Field, consts map[string]any, in json.RawMessage) ([]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if len(in) > 0 && string(in) != "null" {
		if err := json.Unmarshal(in, &raw); err != nil {
			return nil, fmt.Errorf("arguments must be an object: %w", err)
		}
	}
	out := make(map[string]any, len(fields)+len(consts))
	for _, f := range fields {
		v, ok := raw[f.Name]
		if !ok {
			continue
		}
		if f.Prefix != "" {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return nil, fmt.Errorf("%s must be a string", f.Name)
			}
			out[f.Wire] = f.Prefix + s
			continue
		}
		out[f.Wire] = v
	}
	for k, v := range consts {
		out[k] = v
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return []json.RawMessage{b}, nil
}
