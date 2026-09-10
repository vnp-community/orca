package tracing

import "go.opentelemetry.io/otel/attribute"

// TraceEvent mirrors frontend/src/shared/trace/index.ts's TraceEvent type
// field-for-field so api-gateway (CR-FFT-003) can forward this JSON
// byte-for-byte over SSE without a second translation layer. Field names
// use the TS camelCase-via-json-tag convention (not Go's usual snake_case
// wire convention) specifically because this struct's ONLY consumer is
// that TS type — never treat this as the general backend-go event
// envelope convention (compare common/eventbus.Event, which does use
// snake_case).
//
// elapsedMs (optional on the TS side) is intentionally omitted: no OTel
// boundary-span equivalent exists until child "step" spans are added
// (out of scope for CR-FFT-001/002) — a TS consumer parses missing
// optional fields as undefined, so this is not a wire-format bug.
type TraceEvent struct {
	ID     string         `json:"id"`
	Flow   string         `json:"flow"`
	Level  string         `json:"level"` // "start" | "step" | "ok" | "fail" — see TraceLevel below
	Label  string         `json:"label,omitempty"`
	Fields map[string]any `json:"fields"`
	TS     int64          `json:"ts"` // unix ms
}

// TraceLevel values TraceEvent.Level can take — OTel span boundary-level
// instrumentation (CR-FFT-001) only ever produces "start"/"ok"/"fail";
// "step" requires nested child spans, out of scope here.
const (
	TraceLevelStart = "start"
	TraceLevelOK    = "ok"
	TraceLevelFail  = "fail"
)

// SpanAttributesToFields flattens an OTel span's attributes into
// TraceEvent.Fields, dropping any attribute whose value isn't a
// string/int64/float64/bool — TraceFields on the TS side only accepts
// those 4 kinds (frontend/src/shared/trace/index.ts's TraceFields type).
func SpanAttributesToFields(attrs []attribute.KeyValue) map[string]any {
	fields := make(map[string]any, len(attrs))
	for _, kv := range attrs {
		switch kv.Value.Type() {
		case attribute.STRING:
			fields[string(kv.Key)] = kv.Value.AsString()
		case attribute.INT64:
			fields[string(kv.Key)] = kv.Value.AsInt64()
		case attribute.FLOAT64:
			fields[string(kv.Key)] = kv.Value.AsFloat64()
		case attribute.BOOL:
			fields[string(kv.Key)] = kv.Value.AsBool()
			// other kinds (slice types, etc.) dropped — not representable in
			// TraceFields
		}
	}
	return fields
}
