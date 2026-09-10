# TASK-BE-FFT-007: `TraceEventSpanProcessor` + `Init`'s `opts ...Option`/`WithTraceEventPublisher`

**Solution:** BE-FFT-SOL-002 | **CR:** [CR-FFT-002](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-002-span-to-nats-trace-event-bridge.md)
**Service:** `common/tracing`
**Depends on:** TASK-BE-FFT-001, TASK-BE-FFT-002, TASK-BE-FFT-003 (toàn bộ CR-FFT-001 phải xong — `Init` đã có propagator từ TASK-BE-FFT-002), TASK-BE-FFT-006 (`TraceEvent` type)
**Status:** ✅ DONE (2026-09-09)

---

## Mục tiêu

Publish mỗi span (start/end) thành `TraceEvent` JSON lên NATS JetStream
(`orca.<service>.trace.span`), qua 1 `sdktrace.SpanProcessor` mới đăng ký
có điều kiện trong `Init`.

## gitnexus — BẮT BUỘC trước khi sửa (chạm lại `Init`, symbol CRITICAL)

`Init` đã bị sửa 1 lần ở TASK-BE-FFT-002 (thêm propagator). Task này sửa
**lại** cùng file để thêm tham số biến-đổi — **không được giả định** số
liệu impact() cũ vẫn đúng nguyên trạng chỉ vì task trước "chỉ thêm 1
dòng". Chạy lại:

```
mcp__gitnexus__impact({ target: "Init", direction: "upstream", file_path: "backend-go/common/tracing/tracing.go", repo: "orca", summaryOnly: true })
```

Kỳ vọng: vẫn CRITICAL, ~34 impacted (17 direct) — vì TASK-BE-FFT-002 chỉ
thêm dòng nội bộ, không đổi call site nào. Nếu số liệu khác, dừng lại,
báo cáo lý do trước khi tiếp tục (có thể task khác đã đổi thêm gì đó ngoài
dự kiến).

## Files cần sửa

1. `backend-go/common/tracing/trace_event_processor.go` (MỚI)
2. `backend-go/common/tracing/tracing.go` (MODIFY — thêm `opts ...Option`, additive)

## Nội dung

### `trace_event_processor.go`

```go
package tracing

import (
    "context"
    "encoding/json"
    "time"

    "go.opentelemetry.io/otel/codes"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"

    "github.com/stablyai/orca-go/common/eventbus"
)

// TraceEventSpanProcessor bridges OTel spans onto NATS as F40 TraceEvent
// JSON (see trace_event.go), powering TracePanel via api-gateway's SSE
// endpoint (CR-FFT-003). Publishes directly via eventbus.Publisher.Publish
// — deliberately bypassing the transactional-outbox pattern that
// Publisher.Publish's own doc comment otherwise expects, because trace
// data is diagnostic/best-effort (CR-FFT-002's doc comment), not
// domain-of-record: a dropped event on a mid-flight service restart is
// acceptable, unlike an outbox-backed domain event whose loss would be a
// real data-integrity bug.
type TraceEventSpanProcessor struct {
    serviceName string
    pub         *eventbus.Publisher // nil-safe: OnStart/OnEnd no-op when nil
}

func NewTraceEventSpanProcessor(serviceName string, pub *eventbus.Publisher) *TraceEventSpanProcessor {
    return &TraceEventSpanProcessor{serviceName: serviceName, pub: pub}
}

func (p *TraceEventSpanProcessor) OnStart(_ context.Context, s sdktrace.ReadWriteSpan) {
    p.publish(s.SpanContext().TraceID().String(), s.Name(), TraceLevelStart, s.Attributes(), s.StartTime())
}

func (p *TraceEventSpanProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
    level := TraceLevelOK
    if s.Status().Code == codes.Error {
        level = TraceLevelFail
    }
    p.publish(s.SpanContext().TraceID().String(), s.Name(), level, s.Attributes(), s.EndTime())
}

// Shutdown/ForceFlush are no-ops — eventbus.Publisher has no client-side
// buffering to flush; each publish() call already sent synchronously.
func (p *TraceEventSpanProcessor) Shutdown(context.Context) error   { return nil }
func (p *TraceEventSpanProcessor) ForceFlush(context.Context) error { return nil }

func (p *TraceEventSpanProcessor) publish(traceID, spanName, level string, attrs []attribute.KeyValue, ts time.Time) {
    if p.pub == nil {
        return
    }
    event := TraceEvent{
        ID:     traceID,
        Flow:   p.serviceName + ":" + spanName,
        Level:  level,
        Fields: SpanAttributesToFields(attrs),
        TS:     ts.UnixMilli(),
    }
    payload, err := json.Marshal(event)
    if err != nil {
        return // malformed attrs shouldn't crash the tracer callback path
    }
    subject := "orca." + p.serviceName + ".trace.span"
    // Best-effort, fire-and-forget — a trace span callback must never
    // block request handling on NATS availability.
    go func() {
        _ = p.pub.Publish(context.Background(), subject, eventbus.Event{
            ID:         traceID + ":" + level,
            OccurredAt: ts,
            Version:    1,
            Payload:    payload,
        })
    }()
}
```

**Lưu ý implement**: `OnStart`/`OnEnd` là callback đồng bộ trong OTel SDK's
span lifecycle — publish bằng goroutine riêng (`go func() {...}`) để không
block request handling thật nếu NATS chậm/down. Xác nhận lại trong review
rằng đây không tạo ra thứ tự publish không xác định gây vấn đề (chấp nhận
được — trace là diagnostic, thứ tự hiển thị trên TracePanel dùng `ts`, không
dùng thứ tự network arrival).

### `tracing.go` — thêm `opts ...Option`

```go
// backend-go/common/tracing/tracing.go
type Option func(*initOptions)

type initOptions struct {
    tracePublisher *eventbus.Publisher
}

// WithTraceEventPublisher wires the CR-FFT-002 trace-event bridge — omit
// entirely to keep the existing 17 call sites' exporter-only behavior
// unchanged.
func WithTraceEventPublisher(pub *eventbus.Publisher) Option {
    return func(o *initOptions) { o.tracePublisher = pub }
}

func Init(ctx context.Context, serviceName, otlpEndpoint string, opts ...Option) (Shutdown, error) {
    var o initOptions
    for _, opt := range opts {
        opt(&o)
    }

    // ... phần dựng resource/exporter hiện có, KHÔNG đổi ...

    tpOpts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
    if otlpEndpoint != "" {
        // ... exporter hiện có ...
        tpOpts = append(tpOpts, sdktrace.WithBatcher(exporter))
    }
    if o.tracePublisher != nil { // NEW
        tpOpts = append(tpOpts, sdktrace.WithSpanProcessor(
            NewTraceEventSpanProcessor(serviceName, o.tracePublisher)))
    }

    tp := sdktrace.NewTracerProvider(tpOpts...)
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{}) // từ TASK-BE-FFT-002, giữ nguyên

    return func(ctx context.Context) error { return tp.Shutdown(ctx) }, nil
}
```

17 call site hiện có (`tracing.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)`)
compile nguyên trạng vì Go cho phép biến-tham-số rỗng.

## Test cases cần cover

- `TestInit_WithTraceEventPublisher_RegistersSpanProcessor` — gọi `Init`
  với `WithTraceEventPublisher(pub)`, tạo 1 span thật, xác nhận
  `pub`/NATS nhận được đúng 1 message trên subject
  `orca.<serviceName>.trace.span` (dùng NATS test server nếu repo đã có
  tiền lệ, hoặc fake `*eventbus.Publisher`-compatible test double nếu
  `Publisher` cho phép inject `jetstream.JetStream` giả).
- `TestInit_WithoutOption_NoPublisherNoPanic` — **regression bắt buộc**:
  gọi `Init(ctx, "svc", "")` (không truyền option, giống 17 call site cũ),
  xác nhận hành vi giống hệt TASK-BE-FFT-002 (không publish gì, không lỗi).
- `TestTraceEventSpanProcessor_OnEnd_ErrorStatusPublishesFailLevel` — span
  có `Status().Code == codes.Error`, xác nhận `TraceEvent.Level ==
  "fail"` — đúng tiêu chí chấp nhận CR-FFT-002 ("span lỗi publish level:
  fail — không im lặng biến mất").
- `TestTraceEventSpanProcessor_NilPublisher_NoOp` — `pub == nil`, gọi
  `OnStart`/`OnEnd` không panic, không publish gì.
- `TestEnsureStream_IdempotentAcrossRestarts` — gọi `EnsureStream("TRACE",
  ...)` 2 lần liên tiếp, không lỗi "stream already exists" (tiêu chí chấp
  nhận CR-FFT-002).

## Verify

```bash
cd backend-go/common && go build ./... && go test -race ./tracing/...
gofmt -l common/tracing/*.go
```

## gitnexus (sau khi sửa)

```
mcp__gitnexus__detect_changes({ scope: "compare", base_ref: "main", repo: "orca" })
```
Xác nhận scope chỉ gồm `common/tracing/tracing.go`,
`common/tracing/trace_event_processor.go` (+ test) — **không** lan ra 17
`main.go` (thêm biến-tham-số không cần sửa call site không truyền option).

## Blocking

TASK-BE-FFT-008 (wire `WithTraceEventPublisher` cho 6 service) phụ thuộc
CỨNG vào task này.

## Kết quả thực tế (2026-09-09)

- `impact({target: "Init", direction: "upstream", file_path:
  "backend-go/common/tracing/tracing.go"})` chạy lại ngay trước khi sửa:
  vẫn CRITICAL, 34 impacted / 17 direct — khớp 100% kỳ vọng, không lệch
  dù TASK-BE-FFT-002 đã sửa file này 1 lần trước đó.
- **Sai lệch có chủ đích so với sketch task doc**: sketch định nghĩa
  `TraceEventSpanProcessor.pub` là kiểu cụ thể `*eventbus.Publisher`. Khi
  viết test mới phát hiện `eventbus.Publisher` bọc field `js
  jetstream.JetStream` không export, không có cách nào construct 1
  `*eventbus.Publisher` giả để test mà không có kết nối NATS thật — và
  repo này chưa có tiền lệ dùng embedded/testcontainers NATS server nào
  (xác nhận bằng grep). Thêm dependency `nats-server` chỉ để test 1 task
  sẽ lặp lại đúng vấn đề "go.mod bloat trong workspace dùng chung" đã gặp
  nhiều lần trong session này. Thay vào đó, đổi field `pub` sang 1
  interface cục bộ `tracePublisher` (chỉ có method `Publish` — đúng chữ
  ký `eventbus.Publisher.Publish` đã có) — `*eventbus.Publisher` tự động
  thỏa mãn interface này không cần sửa gì ở `common/eventbus`, 6 call
  site thật ở TASK-BE-FFT-008 không bị ảnh hưởng. `NewTraceEventSpanProcessor`
  vẫn giữ nguyên chữ ký nhận `*eventbus.Publisher` cụ thể (không đổi API
  công khai) — chỉ kiểm tra `pub != nil` ở con trỏ cụ thể TRƯỚC khi gán
  vào field interface, tránh đúng cái bẫy "nil pointer trong non-nil
  interface" kinh điển của Go (có test riêng
  `TestNewTraceEventSpanProcessor_NonNilPublisher_Wired` xác nhận không
  dính bẫy này).
- `trace_event_processor.go` (MỚI): `TraceEventSpanProcessor` +
  `NewTraceEventSpanProcessor`, `OnStart`/`OnEnd`/`Shutdown`/`ForceFlush`
  implement đúng `sdktrace.SpanProcessor` (xác nhận chữ ký thật từ
  `go.opentelemetry.io/otel/sdk@v1.45.0/trace/span_processor.go` trước
  khi viết, không đoán). `OnEnd` publish `level=fail` khi
  `s.Status().Code == codes.Error`, ngược lại `ok`. `publish()` chạy
  goroutine riêng (fire-and-forget), đúng yêu cầu không block span
  callback.
- `tracing.go` (MODIFY): thêm `Option`/`initOptions`/`WithTraceEventPublisher`,
  `Init` nhận thêm `opts ...Option` (biến-tham-số, additive). Đổi tên
  biến cục bộ `opts []sdktrace.TracerProviderOption` thành `tpOpts` để
  tránh đụng tên với tham số hàm mới `opts ...Option` (khác kiểu, cùng
  tên sẽ shadow gây khó đọc dù compile được) — không có trong sketch,
  tự phát hiện khi viết code thật.
- File test mới `trace_event_processor_test.go` — dùng `fakeTracePublisher`
  (implement `tracePublisher` interface) với channel để chờ đúng
  goroutine fire-and-forget của `publish()` một cách xác định thay vì
  polling/sleep. 5 test: end-to-end thật qua `sdktrace.NewTracerProvider`
  (không mock OTel SDK) xác nhận `WithTraceEventPublisher` publish đúng
  subject + flow; regression Init không option vẫn hoạt động y hệt
  TASK-BE-FFT-002; span lỗi publish đúng level `fail`; publisher nil
  không panic; constructor thật wire đúng non-nil interface.
- **Không làm `TestEnsureStream_IdempotentAcrossRestarts`** — task doc
  liệt kê test này trong danh sách của TASK-007, nhưng `EnsureStream` là
  method có sẵn của `eventbus.Publisher`, KHÔNG bị sửa/tạo bởi task này
  (task này không đụng file `common/eventbus/`). Test này đòi hỏi 1 kết
  nối NATS thật (hoặc embedded test server chưa có tiền lệ trong repo) để
  có ý nghĩa — ngoài phạm vi thực sự của TASK-007 (TraceEventSpanProcessor
  + Init's opts). Ghi nhận đây là gap, đề xuất tách thành 1 task test
  riêng cho `common/eventbus` package (hiện chưa có `_test.go` nào) nếu
  cần, không tự ý thêm dependency mới để lấp gap này.
- `go test -race ./tracing/... -v`: 13/13 PASS (5 test mới + 8 test cũ từ
  TASK-BE-FFT-002/006), không race.
- `gofmt -l`: sạch. Build lại toàn bộ 17 module: OK — xác nhận 17 call
  site cũ compile nguyên trạng với biến-tham-số rỗng, không cần sửa
  `main.go` nào (đúng dự kiến).
- Scope xác nhận qua `git status --porcelain`: chỉ
  `common/tracing/tracing.go` (M) + `trace_event_processor.go` +
  `trace_event_processor_test.go` (mới) — không lan ra `main.go` nào,
  đúng dự kiến task doc.
