package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInterpolate_InputToken(t *testing.T) {
	execCtx := ExecutionContext{Inputs: map[string]any{"feature_description": "dark mode"}}
	got, err := Interpolate(`{"prompt":"build {{feature_description}}"}`, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"prompt":"build dark mode"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInterpolate_OutputsToken(t *testing.T) {
	execCtx := ExecutionContext{Outputs: map[string]map[string]any{
		"stepA": {"field": "value-from-a"},
	}}
	got, err := Interpolate(`{"prompt":"use {{outputs.stepA.field}}"}`, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"prompt":"use value-from-a"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInterpolate_ProjectIDAndUserID(t *testing.T) {
	execCtx := ExecutionContext{ProjectID: "proj-1", UserID: "user-1"}
	got, err := Interpolate(`{"note":"{{project.id}}/{{user.id}}"}`, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"note":"proj-1/user-1"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInterpolate_NowToken(t *testing.T) {
	got, err := Interpolate(`{"ts":"{{now()}}"}`, ExecutionContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed struct {
		TS string `json:"ts"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("interpolated output is not valid JSON: %v (%s)", err, got)
	}
	if parsed.TS == "" || parsed.TS == "{{now()}}" {
		t.Errorf("expected now() to resolve to an RFC3339 timestamp, got %q", parsed.TS)
	}
}

func TestInterpolate_UnresolvableTokenLeftAsLiteralText(t *testing.T) {
	got, err := Interpolate(`{"prompt":"{{does.not.exist}}"}`, ExecutionContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "{{does.not.exist}}") {
		t.Errorf("expected the unresolvable token left visible as literal text, got %q", got)
	}
}

func TestInterpolate_TokenValueWithSpecialCharsEscapesCorrectly(t *testing.T) {
	execCtx := ExecutionContext{Inputs: map[string]any{"note": `say "hi" and \backslash`}}
	got, err := Interpolate(`{"prompt":"{{note}}"}`, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed struct {
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("interpolated output is not valid JSON: %v (%s)", err, got)
	}
	if parsed.Prompt != `say "hi" and \backslash` {
		t.Errorf("got %q, want the original special-char value round-tripped", parsed.Prompt)
	}
}

func TestInterpolate_NumericOutputStandaloneValue(t *testing.T) {
	execCtx := ExecutionContext{Outputs: map[string]map[string]any{
		"stepA": {"count": float64(42)},
	}}
	got, err := Interpolate(`{"count":{{outputs.stepA.count}}}`, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed struct {
		Count float64 `json:"count"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("interpolated output is not valid JSON: %v (%s)", err, got)
	}
	if parsed.Count != 42 {
		t.Errorf("got %v, want 42", parsed.Count)
	}
}

func TestInterpolate_MultipleTokensInOneConfig(t *testing.T) {
	execCtx := ExecutionContext{
		Inputs:  map[string]any{"a": "A"},
		Outputs: map[string]map[string]any{"s1": {"b": "B"}},
	}
	got, err := Interpolate(`{"x":"{{a}}-{{outputs.s1.b}}-{{project.id}}"}`, ExecutionContext{
		Inputs:    execCtx.Inputs,
		Outputs:   execCtx.Outputs,
		ProjectID: "P",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"x":"A-B-P"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
