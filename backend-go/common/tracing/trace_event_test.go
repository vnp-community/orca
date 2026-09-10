package tracing

import (
	"encoding/json"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

// TestTraceEvent_MarshalsWithExactFrontendFieldNames guards the wire
// contract with frontend/src/shared/trace/index.ts's TraceEvent type —
// key names must match exactly, no elapsedMs, no snake_case variants.
func TestTraceEvent_MarshalsWithExactFrontendFieldNames(t *testing.T) {
	ev := TraceEvent{
		ID:     "span-1",
		Flow:   "devServer:browseDir",
		Level:  TraceLevelOK,
		Label:  "relay",
		Fields: map[string]any{"k": "v"},
		TS:     1234567890,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	want := []string{"id", "flow", "level", "label", "fields", "ts"}
	if len(got) != len(want) {
		t.Fatalf("field count: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for _, k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("missing expected field %q in %v", k, got)
		}
	}
	if _, ok := got["elapsedMs"]; ok {
		t.Error("elapsedMs must not be present — not populated by OTel boundary-span instrumentation")
	}
}

// TestTraceEvent_LabelOmittedWhenEmpty guards the omitempty contract.
func TestTraceEvent_LabelOmittedWhenEmpty(t *testing.T) {
	ev := TraceEvent{
		ID:     "span-1",
		Flow:   "devServer:browseDir",
		Level:  TraceLevelStart,
		Fields: map[string]any{},
		TS:     1234567890,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := got["label"]; ok {
		t.Errorf("expected label to be omitted when empty, got %v", got["label"])
	}
}

// TestSpanAttributesToFields_DropsUnsupportedTypes confirms only the 4
// TraceFields-compatible kinds survive; unsupported kinds (e.g. a string
// slice) are silently dropped, not erroring or panicking.
func TestSpanAttributesToFields_DropsUnsupportedTypes(t *testing.T) {
	attrs := []attribute.KeyValue{
		attribute.String("s", "hello"),
		attribute.Int64("i", 42),
		attribute.Float64("f", 3.14),
		attribute.Bool("b", true),
		attribute.StringSlice("unsupported", []string{"a", "b"}),
	}

	got := SpanAttributesToFields(attrs)

	if len(got) != 4 {
		t.Fatalf("want 4 fields (unsupported kind dropped), got %d: %v", len(got), got)
	}
	if got["s"] != "hello" {
		t.Errorf("s: got %v, want hello", got["s"])
	}
	if got["i"] != int64(42) {
		t.Errorf("i: got %v, want 42", got["i"])
	}
	if got["f"] != 3.14 {
		t.Errorf("f: got %v, want 3.14", got["f"])
	}
	if got["b"] != true {
		t.Errorf("b: got %v, want true", got["b"])
	}
	if _, ok := got["unsupported"]; ok {
		t.Error("expected unsupported attribute.StringSlice kind to be dropped")
	}
}

// TestSpanAttributesToFields_EmptyInputReturnsEmptyMap guards against nil
// map / panic on empty input.
func TestSpanAttributesToFields_EmptyInputReturnsEmptyMap(t *testing.T) {
	got := SpanAttributesToFields(nil)
	if got == nil {
		t.Fatal("expected a non-nil empty map, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}
