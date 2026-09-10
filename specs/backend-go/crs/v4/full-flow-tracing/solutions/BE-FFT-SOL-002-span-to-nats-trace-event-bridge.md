# BE-FFT-SOL-002: `SpanProcessor` bridge span OTel → NATS JetStream (F40 `TraceEvent`)

> **🔲 Designed — chưa implement.** Phụ thuộc cứng BE-FFT-SOL-001.

**CR:** [CR-FFT-002](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-002-span-to-nats-trace-event-bridge.md)
**Service:** `common/tracing` (thêm mới) + `api-gateway`, `usage-service`, `issue-tracking-service`, `notification-service`, `tenant-service`, `infra-fleet-service`
**TDD tham chiếu:** `specs/backend-go/tdd/architecture/08-inter-service-communication.md` §subject naming (`orca.<service>.<entity>.<event>`)

---

## 1. Re-verify trạng thái hiện tại (2026-09-09)

- `backend-go/common/eventbus/eventbus.go` xác nhận đúng 2 chế độ subscribe:
  `Subscribe` (durable, competing-consumer) và `SubscribeEphemeral`
  (dòng ~134-165, mỗi process cursor riêng — đúng primitive cho "mọi
  replica phải nhận mọi event"). Doc comment của package (dòng 1-7) khẳng
  định: *"publishing always goes through the transactional-outbox pattern
  ... rather than a direct publish call inside a request handler"*, và
  `Publisher.Publish`'s doc comment lặp lại: *"Called by an outbox relay
  loop, not directly from a request handler"*. **CR-FFT-002 publish trực
  tiếp, bỏ qua outbox** — đây là lệch khỏi convention mặc định của package,
  nhưng là lựa chọn có chủ đích đã tự biện minh trong CR (trace = diagnostic
  best-effort, không phải domain-of-record) — solution này giữ nguyên quyết
  định, không tự ý đổi sang outbox.
- `notification-service/internal/adapter/broadcaster/broadcaster.go`'s
  package doc (dòng 1-16) xác nhận đúng tiền lệ "mỗi replica
  `SubscribeEphemeral` độc lập, fan-out in-memory riêng, không cần biết về
  replica khác" — pattern BE-FFT-SOL-003 sẽ tái dùng.
- `api-gateway/internal/config/config.go` — `Config` struct **chưa có**
  field `NATSURL`; `OtherServiceAddrs` chỉ chứa 14 service address, không
  có NATS. Đúng như CR ghi "chưa kết nối NATS bao giờ".
- 5 service đã có `*eventbus.Publisher` sẵn qua `eventbus.Connect` —
  xác nhận cụ thể bằng grep `eventbus.Connect` trong `cmd/server/main.go`
  của `tenant-service` (dòng 114), `infra-fleet-service` (dòng 143),
  `notification-service` (dòng 106) — mẫu đúng như CR liệt kê. Chưa grep
  hết `usage-service`/`issue-tracking-service` trong lần re-verify này
  nhưng CR gốc đã liệt kê rõ, không có dấu hiệu sai lệch ở 3 service đã
  kiểm.
- Frontend's `TraceEvent` (`frontend/src/shared/trace/index.ts:19-34`) —
  đọc lại xác nhận field list CR-FFT-002 nêu đúng (`id`, `flow`, `level`,
  `label?`, `fields`, `ts`), nhưng **có thêm 1 field CR chưa liệt kê**:
  `elapsedMs?: number` (optional). Vì optional và JSON thiếu field optional
  vẫn parse hợp lệ ở TS, backend-go **không cần gửi** field này ở v1 — ghi
  rõ để tránh ai tưởng nhầm là thiếu.

## 2. Impact analysis — re-verify

| Symbol | Direction | Risk | Impacted | Ghi chú |
|---|---|---|---|---|
| `Init` (`common/tracing/tracing.go`) | upstream | 🔴 **CRITICAL** | 34 (17 direct) | Solution này thêm THÊM 1 tham số biến-đổi (`opts ...Option`) lên trên phần BE-FFT-SOL-001 đã thêm — vẫn additive, vẫn CRITICAL blast radius, PHẢI re-verify `impact()` lại (không dùng số liệu này) ngay trước khi sửa file, vì BE-FFT-SOL-001 có thể đã đổi nội dung hàm (dù không đổi signature) |

`TraceEventSpanProcessor`, `WithTraceEventPublisher`, `TraceEvent` (Go
struct) là symbol **mới** — không cần `impact()` trước khi tạo mới, chỉ bắt
buộc trước khi *sửa* `Init` (symbol có sẵn).

## 3. Giải pháp

### 3.1. `TraceEvent` JSON — Go struct khớp chính xác field name phía TS

```go
// backend-go/common/tracing/trace_event.go (mới)
package tracing

// TraceEvent mirrors frontend/src/shared/trace/index.ts's TraceEvent type
// field-for-field (id/flow/level/label/fields/ts) so api-gateway (CR-FFT-003)
// can forward this JSON byte-for-byte without a second translation layer.
// elapsedMs (optional on the TS side) is intentionally omitted here — no
// OTel boundary-span equivalent exists until child "step" spans are added
// (out of scope, see CR-FFT-002's "Không thuộc phạm vi").
type TraceEvent struct {
    ID     string         `json:"id"`
    Flow   string         `json:"flow"`
    Level  string         `json:"level"` // "start" | "step" | "ok" | "fail"
    Label  string         `json:"label,omitempty"`
    Fields map[string]any `json:"fields"`
    TS     int64          `json:"ts"` // unix ms
}
```

### 3.2. `TraceEventSpanProcessor` — implement `sdktrace.SpanProcessor`

```go
// backend-go/common/tracing/trace_event_processor.go (mới)
package tracing

import (
    "context"

    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/codes"

    "github.com/stablyai/orca-go/common/eventbus"
)

// TraceEventSpanProcessor bridges OTel spans onto NATS as F40 TraceEvent
// JSON, powering TracePanel via api-gateway's SSE endpoint (CR-FFT-003).
// Publishes directly (no outbox) — trace data is diagnostic/best-effort,
// not domain-of-record, so a dropped event on a mid-flight service restart
// is acceptable (see this CR's doc comment for the full rationale; this
// deliberately deviates from eventbus.Publisher.Publish's own doc comment,
// which otherwise expects an outbox relay caller).
type TraceEventSpanProcessor struct {
    serviceName string
    pub         *eventbus.Publisher // nil-safe: no-op when not configured
}

func NewTraceEventSpanProcessor(serviceName string, pub *eventbus.Publisher) *TraceEventSpanProcessor {
    return &TraceEventSpanProcessor{serviceName: serviceName, pub: pub}
}

func (p *TraceEventSpanProcessor) OnStart(ctx context.Context, s sdktrace.ReadWriteSpan) {
    p.publish(ctx, s, "start")
}

func (p *TraceEventSpanProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
    level := "ok"
    if s.Status().Code == codes.Error {
        level = "fail"
    }
    p.publish(context.Background(), s, level)
}

func (p *TraceEventSpanProcessor) Shutdown(ctx context.Context) error   { return nil } // eventbus.Publisher has no buffering to flush
func (p *TraceEventSpanProcessor) ForceFlush(ctx context.Context) error { return nil }

func (p *TraceEventSpanProcessor) publish(ctx context.Context, s interface {
    Name() string
    SpanContext() interface{ TraceID() interface{ String() string } }
}, level string) {
    if p.pub == nil {
        return
    }
    // ... build TraceEvent{ID: traceID, Flow: serviceName+":"+s.Name(), Level: level, Fields: flattenAttrs(...), TS: time.Now().UnixMilli()}
    // marshal, publish to "orca." + serviceName + ".trace.span" via p.pub.Publish
}
```

(Chữ ký `publish` ở trên rút gọn để minh hoạ shape — task thực thi dùng
đúng type thật `sdktrace.ReadWriteSpan`/`sdktrace.ReadOnlySpan`, cả 2 đều
có `Name()`, `SpanContext()`, `Attributes()`, `StartTime()`/`EndTime()`.)

Mapping field (đúng bảng CR-FFT-002 §1):

| `TraceEvent` field | Nguồn OTel |
|---|---|
| `ID` | `s.SpanContext().TraceID().String()` (TraceID, không phải SpanID) |
| `Flow` | `serviceName + ":" + s.Name()` |
| `Level` | `OnStart`→`"start"`; `OnEnd` lỗi→`"fail"`; `OnEnd` bình thường→`"ok"` |
| `Fields` | `s.Attributes()` chuyển `map[string]any`, bỏ attribute không phải string/number/bool |
| `TS` | `s.StartTime()`/`s.EndTime()` tương ứng, `.UnixMilli()` |

### 3.3. `Init` nhận thêm option, additive (chạm lại symbol CRITICAL)

```go
// backend-go/common/tracing/tracing.go
type Option func(*initOptions)
type initOptions struct{ tracePublisher *eventbus.Publisher }

// WithTraceEventPublisher wires the trace-event bridge — omit entirely to
// keep today's exporter-only behavior (existing 17 call sites unchanged).
func WithTraceEventPublisher(pub *eventbus.Publisher) Option {
    return func(o *initOptions) { o.tracePublisher = pub }
}

func Init(ctx context.Context, serviceName, otlpEndpoint string, opts ...Option) (Shutdown, error) {
    var o initOptions
    for _, opt := range opts {
        opt(&o)
    }
    // ... phần dựng resource/exporter hiện có, không đổi ...
    if o.tracePublisher != nil {
        tpOpts = append(tpOpts, sdktrace.WithSpanProcessor(
            NewTraceEventSpanProcessor(serviceName, o.tracePublisher)))
    }
    tp := sdktrace.NewTracerProvider(tpOpts...)
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{}) // từ BE-FFT-SOL-001
    return func(ctx context.Context) error { return tp.Shutdown(ctx) }, nil
}
```

17 call site hiện có (`tracing.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)`,
không truyền `opts`) **compile nguyên trạng** — Go cho phép biến-tham-số
rỗng.

### 3.4. Rollout có kiểm soát — chỉ 6/17 service bật `WithTraceEventPublisher`

| Service | Trạng thái NATS | Việc cần làm |
|---|---|---|
| `api-gateway` | Chưa kết nối bao giờ | Thêm `NATSURL` vào `Config`, gọi `eventbus.Connect` lần đầu, `EnsureStream("TRACE", []string{"orca.*.trace.span"})`, truyền `pub` vào `tracing.Init(..., tracing.WithTraceEventPublisher(pub))`, giữ `cons` cho BE-FFT-SOL-003 |
| `usage-service`, `issue-tracking-service`, `notification-service`, `tenant-service`, `infra-fleet-service` | Đã có `*eventbus.Publisher` (xác nhận 3/5 qua grep `eventbus.Connect`) | Chỉ thêm `tracing.WithTraceEventPublisher(pub)` vào lệnh `tracing.Init(...)` đã có — 1 dòng/service |
| 11 service còn lại | Chưa nối NATS | **Ngoài phạm vi** — xem "Không thuộc phạm vi" |

## 4. Không thuộc phạm vi solution này

- Bật cho 11 service chưa nối NATS (`task-service`, `workflow-service`,
  `project-service`, `auth-service`, `git-gateway-service`,
  `scm-integration-service`, `credential-broker-service`,
  `annotation-service`, `ai-provider-service`, `automation-service`,
  `orchestration-service`) — cần CR riêng cho coverage 17/17.
- `level: "step"` — cần span con lồng nhau, ngoài scope CR-FFT-001/002.
- TTL/retention cho JetStream stream `TRACE` — dùng default không giới
  hạn ở v1, tuning retention là follow-up.

## 5. Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Phụ thuộc cứng BE-FFT-SOL-001 | Cao | Cần span thật tồn tại (propagator, StatsHandler) trước khi có gì để bridge |
| `Init` bị sửa 2 lần (BE-FFT-SOL-001 rồi solution này) | 🔴 CRITICAL (không đổi) | Mỗi lần sửa PHẢI tự `impact()` lại, không cộng dồn giả định từ lần trước |
| Bỏ qua outbox pattern | Trung bình — lệch convention package | Đã tự biện minh trong CR, không phải sơ suất — nhưng review PR nên trích dẫn đúng đoạn lý do này để reviewer không tưởng nhầm là bug |
| `EnsureStream` gọi nhiều lần khi restart | Thấp | Đã idempotent theo `CreateOrUpdateStream` — cần test xác nhận không lỗi "stream already exists" |
| `api-gateway` lần đầu tiên nối NATS | Trung bình | Cần `NATSURL` config mới hoàn toàn — nếu NATS down, `Init` không được phép panic/lỗi cứng (giữ đúng "always-sample, exporter-less" fallback khi optional) |

## Liên quan

- `backend-go/common/eventbus/eventbus.go` (đặc biệt `SubscribeEphemeral`, `Publisher.Publish`'s doc comment)
- `backend-go/services/notification-service/internal/adapter/broadcaster/broadcaster.go`
- `backend-go/services/{tenant,infra-fleet,notification}-service/cmd/server/main.go` (mẫu `eventbus.Connect` đã có)
- `backend-go/services/api-gateway/internal/config/config.go` (chưa có `NATSURL`)
- `frontend/src/shared/trace/index.ts:19-34` (`TraceEvent`, không đổi)
- [BE-FFT-SOL-001](./BE-FFT-SOL-001-otel-span-instrumentation.md) — phụ thuộc cứng
- [BE-FFT-SOL-003](./BE-FFT-SOL-003-api-gateway-sse-real-forwarding.md) — tiêu thụ output solution này
- [README.md](./README.md) — cảnh báo rủi ro CRITICAL tổng hợp
