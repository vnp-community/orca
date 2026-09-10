package usecase

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestInterpolate_SubstitutesTopLevelInput(t *testing.T) {
	inputs := map[string]any{"feature_description": "add dark mode"}
	got, err := Interpolate("Implement: {{feature_description}}", inputs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Implement: add dark mode" {
		t.Errorf("got %q", got)
	}
}

func TestInterpolate_UnknownInputReferenceErrors(t *testing.T) {
	_, err := Interpolate("{{does_not_exist}}", map[string]any{}, nil)
	if err == nil {
		t.Fatal("expected an error for an unknown top-level input key")
	}
}

func TestInterpolate_SubstitutesStepOutputField(t *testing.T) {
	outputs := map[string]domain.StepResult{
		"stepX": {Status: domain.ResultStatusCompleted, OutputJSON: `{"branch":"feature/dark-mode"}`},
	}
	got, err := Interpolate("Deploy branch {{outputs.stepX.branch}}", nil, outputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Deploy branch feature/dark-mode" {
		t.Errorf("got %q", got)
	}
}

func TestInterpolate_SubstitutesNestedStepOutputField(t *testing.T) {
	outputs := map[string]domain.StepResult{
		"stepX": {Status: domain.ResultStatusCompleted, OutputJSON: `{"data":{"branch":"nested-value"}}`},
	}
	got, err := Interpolate("{{outputs.stepX.data.branch}}", nil, outputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "nested-value" {
		t.Errorf("got %q", got)
	}
}

func TestInterpolate_ReferencingStepThatHasNotCompletedErrors(t *testing.T) {
	_, err := Interpolate("{{outputs.stepX.branch}}", nil, map[string]domain.StepResult{})
	if err == nil {
		t.Fatal("expected a clear error, not a silent empty string, for a step that hasn't completed")
	}
}

func TestInterpolate_OutputsReferenceMissingFieldAfterStepIDErrors(t *testing.T) {
	outputs := map[string]domain.StepResult{"stepX": {OutputJSON: `{}`}}
	_, err := Interpolate("{{outputs.stepX}}", nil, outputs)
	if err == nil {
		t.Fatal("expected an error for an outputs reference missing a field")
	}
}

func TestInterpolate_UnknownOutputFieldErrors(t *testing.T) {
	outputs := map[string]domain.StepResult{"stepX": {OutputJSON: `{"branch":"main"}`}}
	_, err := Interpolate("{{outputs.stepX.does_not_exist}}", nil, outputs)
	if err == nil {
		t.Fatal("expected an error for an unknown output field")
	}
}

func TestInterpolate_NoTemplatesPassesThroughUnchanged(t *testing.T) {
	got, err := Interpolate("just a plain string", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "just a plain string" {
		t.Errorf("got %q", got)
	}
}

func TestInterpolateStepConfig_WalksNestedJSONObject(t *testing.T) {
	inputs := map[string]any{"env_name": "staging"}
	raw := `{"prompt":"Deploy to {{env_name}}","env":{"TARGET":"{{env_name}}"},"tags":["{{env_name}}","fixed"]}`

	got, err := interpolateStepConfig(raw, inputs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"env":{"TARGET":"staging"},"prompt":"Deploy to staging","tags":["staging","fixed"]}`
	if !jsonObjectsEqual(t, got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInterpolateStepConfig_EmptyConfigPassesThrough(t *testing.T) {
	got, err := interpolateStepConfig("", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected an empty config to pass through unchanged, got %q", got)
	}
}

func TestInterpolateStepConfig_InvalidJSONErrors(t *testing.T) {
	_, err := interpolateStepConfig("not json", nil, nil)
	if err == nil {
		t.Fatal("expected an error for a non-JSON step config")
	}
}

// jsonObjectsEqual compares two JSON strings structurally — key order in a
// re-marshaled map[string]any is not guaranteed, so a literal string
// comparison would be flaky.
func jsonObjectsEqual(t *testing.T, a, b string) bool {
	t.Helper()
	var va, vb any
	if err := json.Unmarshal([]byte(a), &va); err != nil {
		t.Fatalf("unmarshal %q: %v", a, err)
	}
	if err := json.Unmarshal([]byte(b), &vb); err != nil {
		t.Fatalf("unmarshal %q: %v", b, err)
	}
	return reflect.DeepEqual(va, vb)
}
