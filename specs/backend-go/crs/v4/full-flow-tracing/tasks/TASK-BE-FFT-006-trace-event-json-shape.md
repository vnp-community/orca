# TASK-BE-FFT-006: `TraceEvent` Go struct + field mapping từ OTel span

**Solution:** BE-FFT-SOL-002 | **CR:** [CR-FFT-002](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-002-span-to-nats-trace-event-bridge.md)
**Service:** `common/tracing` (type mới)
**Depends on:** Không
**Status:** ✅ DONE (2026-09-09)

---

## Mục tiêu

Định nghĩa đúng shape JSON `TraceEvent` mà frontend đã có sẵn
(`frontend/src/shared/trace/index.ts:19-34`, **không đổi frontend**) làm
Go struct, để `api-gateway` (BE-FFT-SOL-003) chỉ cần forward nguyên
payload, không dịch lại lần 2.

## gitnexus

`TraceEvent` (Go) là symbol **hoàn toàn mới** — không cần `impact()` trước
khi tạo (chỉ bắt buộc trước khi *sửa* symbol có sẵn, theo CLAUDE.md).
Không sửa file `tracing.go` ở task này (đó là TASK-BE-FFT-007).

## Files cần sửa

1. `backend-go/common/tracing/trace_event.go` (MỚI)

## Nội dung

```go
// backend-go/common/tracing/trace_event.go
package tracing

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
```

(`attribute` = `go.opentelemetry.io/otel/attribute`, đã là dependency
gián tiếp có sẵn qua `go.opentelemetry.io/otel`.)

## Test cases cần cover

File mới: `backend-go/common/tracing/trace_event_test.go`.

- `TestTraceEvent_MarshalsWithExactFrontendFieldNames` — marshal 1
  `TraceEvent{}` mẫu, xác nhận JSON key đúng `id`/`flow`/`level`/`label`/
  `fields`/`ts` (không có `elapsedMs`, không có snake_case biến thể nào).
- `TestTraceEvent_LabelOmittedWhenEmpty` — xác nhận `label` không xuất
  hiện trong JSON khi rỗng (đúng `omitempty`).
- `TestSpanAttributesToFields_DropsUnsupportedTypes` — truyền 1 slice
  attribute có string/int64/float64/bool + 1 kiểu không hỗ trợ (vd.
  `attribute.StringSlice`), xác nhận kiểu không hỗ trợ bị bỏ qua, 4 kiểu
  còn lại xuất hiện đúng giá trị.
- `TestSpanAttributesToFields_EmptyInputReturnsEmptyMap` — không nil,
  không panic với input rỗng.

## Verify

```bash
cd backend-go/common && go build ./... && go test ./tracing/...
gofmt -l common/tracing/trace_event.go common/tracing/trace_event_test.go
```

## gitnexus (sau khi sửa)

```
mcp__gitnexus__detect_changes({ scope: "compare", base_ref: "main", repo: "orca" })
```
Xác nhận scope chỉ gồm 2 file mới trong `common/tracing/` — không lan ra
service nào (type mới, chưa ai import).

## Blocking

TASK-BE-FFT-007 (`TraceEventSpanProcessor`) phụ thuộc CỨNG vào task này —
cần `TraceEvent`/`SpanAttributesToFields` tồn tại trước khi viết processor.

## Kết quả thực tế (2026-09-09)

- Symbol hoàn toàn mới, không cần `impact()`. Đối chiếu lại
  `frontend/src/shared/trace/index.ts`'s `TraceEvent`/`TraceLevel`/
  `TraceFields` thật trước khi viết struct Go — khớp 100% với mô tả task
  doc (field `id`/`flow`/`level`/`label?`/`fields`/`ts`/`elapsedMs?`,
  `TraceFields = Record<string, string | number | boolean | undefined>`).
- Tạo `trace_event.go` đúng nội dung task doc: struct `TraceEvent` với
  json tag camelCase, 3 hằng `TraceLevel*`, hàm
  `SpanAttributesToFields` lọc 4 kiểu attribute hỗ trợ.
- File test mới `trace_event_test.go` — 4 test: field name JSON chính
  xác (và xác nhận không có `elapsedMs`), `label` omitempty, lọc đúng
  kiểu không hỗ trợ (`attribute.StringSlice`) bị bỏ qua, input rỗng trả
  map rỗng không nil/không panic.
- `go test ./tracing/... -v`: 8/8 PASS (4 test mới + 4 test cũ của
  TASK-BE-FFT-002 vẫn PASS, không regression).
- `gofmt -l`: phát hiện `trace_event.go` chưa format đúng lúc viết tay —
  `gofmt -w` sửa, sau đó sạch.
- Scope xác nhận qua `git status --porcelain`: đúng 2 file mới
  `common/tracing/trace_event.go` + `trace_event_test.go` — không lan ra
  service nào (type mới, chưa ai import), đúng dự kiến.
