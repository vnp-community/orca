package domain

import "log/slog"

// SecretValue holds plaintext secret material (an external MCP server env or
// header value). Every formatting path prints "[redacted]" so an accidental
// %v, %+v, %#v, slog attribute or JSON marshal cannot leak it. Reveal is the
// single explicit way out, used only at the broker/child-process boundary.
type SecretValue struct{ b []byte }

func NewSecretValue(s string) SecretValue { return SecretValue{b: []byte(s)} }

func (s SecretValue) String() string               { return "[redacted]" }
func (s SecretValue) GoString() string             { return "[redacted]" }
func (s SecretValue) LogValue() slog.Value         { return slog.StringValue("[redacted]") }
func (s SecretValue) MarshalJSON() ([]byte, error) { return []byte(`"[redacted]"`), nil }
func (s SecretValue) MarshalText() ([]byte, error) { return []byte("[redacted]"), nil }
func (s SecretValue) Len() int                     { return len(s.b) }
func (s SecretValue) Reveal() []byte               { return s.b }

// Zero overwrites the buffer; call after the value was handed over.
func (s SecretValue) Zero() {
	for i := range s.b {
		s.b[i] = 0
	}
}
