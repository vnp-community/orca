# CR-FFT-002 — Bridge span OTel → NATS JetStream dưới dạng F40 `TraceEvent`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-FFT-002 |
| **Tên** | Custom `SpanProcessor` publish span thành `TraceEvent` lên NATS JetStream (`orca.<service>.trace.span`) |
| **Loại** | Feature / Observability |
| **Priority** | 🔴 P0 (điều kiện tiên quyết của CR-FFT-003) |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu "thực thi đầy đủ F40 ở lớp backend-go" |
| **Tác động Features** | F40 |
| **Phụ thuộc** | **CR-FFT-001** (cần span thật tồn tại trước khi có gì để bridge) |

---

## Bối cảnh & Vấn đề

Sau CR-FFT-001, mỗi service tạo OTel span thật cho mọi request gRPC/HTTP — nhưng span đó chỉ chảy tới OTLP exporter (Jaeger/Tempo, khi cấu hình) hoặc biến mất (khi không cấu hình `OTLPEndpoint`, trường hợp local dev mặc định — xem `tracing.go`'s "always-sample, exporter-less provider" fallback). **Không service nào publish span/event ra một nơi mà `api-gateway` (process khác) có thể đọc được** — đây chính là câu hỏi kiến trúc cốt lõi task đặt ra: "event trace thật đến từ đâu, có message bus nào giữa các service không".

Khảo sát `backend-go/common/` xác nhận: **có sẵn đúng 1 hạ tầng pub/sub liên-service** — `common/eventbus` (NATS JetStream), theo `specs/backend-go/tdd/architecture/08-inter-service-communication.md:32-33`: *"Subject naming: `orca.<service>.<entity>.<event>`"*. Không có NATS/Redis/message-bus nào khác trong `backend-go` (grep `nats\|redis\|pubsub\|eventbus` toàn repo chỉ ra đúng 1 package `common/eventbus` + các adapter dùng nó). Đây là hạ tầng đúng để tái dùng, **không cần tự chế gRPC streaming ngược hay Redis mới**.

`common/eventbus.Publisher`/`Consumer` (`eventbus.go:34-43`) đã có 2 chế độ subscribe:
- `Subscribe` (durable, competing-consumer — 1 event xử lý đúng 1 lần toàn cluster) — dùng cho domain event cần xử lý side-effect (vd. gửi email khi `task.completed`).
- `SubscribeEphemeral` (`eventbus.go:134-165`, mỗi process tự có cursor riêng, mọi replica đều nhận full copy) — theo doc comment của chính hàm này, đây là **primitive đúng cho "mọi replica phải phản ứng với mọi event"**, và đã có 1 tiền lệ thật đang chạy: `notification-service/internal/adapter/broadcaster/broadcaster.go`'s package doc comment (dòng 1-16) mô tả chính xác pattern "mỗi replica `SubscribeEphemeral` độc lập → fan-out vào registry in-memory của riêng replica đó" để hiện thực hoá cross-replica WS broadcast mà không cần replica nào biết về replica khác.

Đây đúng là hình dạng bài toán "N service → 1 SSE endpoint fan-in" mà F40 cần, chỉ khác chiều: ở đây là "N service → 1 `api-gateway`" thay vì "N event → nhiều WS client cùng 1 service" — nhưng cơ chế NATS layer giống hệt nhau.

`common/outbox` (transactional-outbox, `outbox.go:1-23`) là pattern **bắt buộc cho domain-of-record event** (event mất là mất dữ liệu nghiệp vụ) — **không áp dụng cho trace span**: `trace_routes.go`'s chính comment đã xác nhận triết lý "trace data is diagnostic, not sensitive" (kế thừa từ `backend/src/server/trace-sse-routes.ts:63`'s "intentionally low-security"), và F40's spec không có yêu cầu "không được mất trace event" — TracePanel chỉ hiển thị best-effort, mất 1 vài event khi service restart giữa chừng là chấp nhận được (khác hẳn `task.completed` mất là treo nghiệp vụ thật). Publish trực tiếp qua `eventbus.Publisher.Publish` (bỏ qua outbox) là lựa chọn kiến trúc **có chủ đích**, không phải thiếu sót.

## Giải pháp đề xuất

### 1. Định nghĩa `TraceEvent` JSON đúng shape frontend đã có sẵn (không đổi frontend)

Frontend's `TraceEvent` (`frontend/src/shared/trace/index.ts:20-32`, không đổi — theo AGENTS.md, không có lý do sửa 1 contract đã hoạt động ở nơi khác) có shape:

```ts
type TraceEvent = {
  id: string          // span id, xuyên suốt boundary
  flow: string         // 'subsystem:operation'
  level: TraceLevel     // 'start' | 'step' | 'ok' | 'fail'
  label?: string
  fields: Record<string, string | number | boolean | undefined>
  ts: number            // unix ms
}
```

Backend-go publish JSON đúng field name này (không phải OTel's field name gốc) để `api-gateway` (CR-FFT-003) chỉ cần forward nguyên payload, không phải dịch lại lần 2:

| F40 `TraceEvent` field | Nguồn từ OTel span |
|---|---|
| `id` | `span.SpanContext().TraceID().String()` — dùng **TraceID** (không phải SpanID) làm `id` xuyên boundary, đúng tinh thần CR-TRACE-000's "1 id nhất quán xuyên suốt stack" §3.1/3.2, dù cơ chế lan truyền là OTel context propagation (CR-FFT-001) thay vì `resume.id` thủ công |
| `flow` | `<service>:<span.Name()>` (vd. `task-service:CreateTask`) — namespace theo service, không đụng namespace `domain:operation` phía TS (`worktree:`, `agentOrch:`, ... đã liệt kê ở CR-TRACE-000 §4) vì đây là 2 hệ tracer độc lập, không chia sẻ registry tên |
| `level` | `OnStart` → `"start"`; `OnEnd` với `span.Status().Code == codes.Error` → `"fail"`; `OnEnd` bình thường → `"ok"` — không có `"step"` (OTel span không có khái niệm sub-step giữa start/end; muốn có "step" thật cần span con lồng nhau, ngoài scope CR này) |
| `fields` | `span.Attributes()` chuyển thành `map[string]any` phẳng (bỏ qua attribute không phải string/number/bool, theo đúng kiểu `TraceFields` phía TS) |
| `ts` | `span.StartTime()`/`span.EndTime()` tương ứng, đổi `UnixMilli()` |

### 2. `TraceEventSpanProcessor` — implement `sdktrace.SpanProcessor`

File mới `backend-go/common/tracing/trace_event_processor.go`:

```go
// TraceEventSpanProcessor bridges OTel spans onto NATS as F40 TraceEvent
// JSON, powering TracePanel via api-gateway's SSE endpoint (CR-FFT-003).
// Publishes directly (no outbox) — see this CR's doc comment: trace data
// is diagnostic/best-effort, not domain-of-record, so a dropped event on a
// mid-flight service restart is acceptable (unlike an outbox-backed
// domain event, whose loss would be a real data-integrity bug).
type TraceEventSpanProcessor struct {
	serviceName string
	pub         *eventbus.Publisher // nil-safe: no-op when tracing publish isn't configured for this service
}

func (p *TraceEventSpanProcessor) OnStart(ctx context.Context, s sdktrace.ReadWriteSpan) { p.publish(s, "start") }
func (p *TraceEventSpanProcessor) OnEnd(s sdktrace.ReadOnlySpan)                          { p.publish(s, endLevel(s)) }
// Shutdown/ForceFlush: no-op — eventbus.Publisher has no buffering to flush.
```

Đăng ký thêm vào `sdktrace.NewTracerProvider(...)` trong `Init` bằng `sdktrace.WithSpanProcessor(processor)`, **chỉ khi** một `*eventbus.Publisher` non-nil được truyền vào — xem mục 3.

### 3. `Init` nhận thêm tham số optional, additive (không phá 17 call site)

```go
// Init's existing signature: func Init(ctx, serviceName, otlpEndpoint string) (Shutdown, error)
// New: add a variadic/functional option, e.g.
func Init(ctx context.Context, serviceName, otlpEndpoint string, opts ...Option) (Shutdown, error)

// WithTraceEventPublisher wires the trace-event bridge — omit entirely
// (existing 17 call sites unchanged) to keep today's exporter-only behavior.
func WithTraceEventPublisher(pub *eventbus.Publisher) Option
```

Mirror đúng tiền lệ "additive option, không đổi hành vi mặc định" đã dùng khi thêm OTLP exporter vào `Init` trước đây (`docs/execution-plan.md` dòng 665).

### 4. Rollout: KHÔNG bắt tất cả 17 service nối NATS trong CR này

Khảo sát xác nhận chỉ **5/17 service** hiện đã gọi `eventbus.Connect` (có `*eventbus.Publisher` sẵn trong tay): `usage-service`, `issue-tracking-service`, `notification-service`, `tenant-service`, `infra-fleet-service`. 12 service còn lại (`task-service`, `workflow-service`, `project-service`, `auth-service`, `git-gateway-service`, `scm-integration-service`, `credential-broker-service`, `annotation-service`, `ai-provider-service`, `automation-service`, `orchestration-service`, `api-gateway`) chưa từng kết nối NATS.

Vì `Init`'s option là opt-in/nil-safe, CR này chỉ bật `WithTraceEventPublisher` cho:
- **`api-gateway`**: bắt buộc, vì CR-FFT-003 cần chính service này vừa publish (span HTTP edge của chính nó) vừa consume (SSE fan-in) — cần nối NATS mới hoàn toàn (xem "Changes Required").
- 5 service đã có `*eventbus.Publisher` sẵn: chỉ cần truyền thêm option vào `tracing.Init`, chi phí gần bằng 0.

12 service còn lại: out of scope CR này, xem "Không thuộc phạm vi".

## Changes Required

| File | Thay đổi |
|------|---------|
| `backend-go/common/tracing/trace_event_processor.go` (mới) | `TraceEventSpanProcessor` implement `sdktrace.SpanProcessor`, publish JSON `TraceEvent` lên subject `orca.<service>.trace.span` |
| `backend-go/common/tracing/tracing.go` | `Init` nhận thêm `opts ...Option` (additive); đăng ký `TraceEventSpanProcessor` khi có `WithTraceEventPublisher` |
| `backend-go/common/eventbus` | Không đổi API — chỉ cần `EnsureStream` cho stream mới (vd. tên `TRACE`, subjects `["orca.*.trace.span"]`) gọi 1 lần ở mỗi service publish |
| `backend-go/services/api-gateway/internal/config/config.go` | Thêm field `NATSURL string` (chưa tồn tại — service này chưa kết nối NATS bao giờ), theo đúng convention `commonconfig.StringEnv("NATS_URL", "nats://localhost:4222")` 5 service kia đã dùng |
| `backend-go/services/api-gateway/cmd/server/main.go` | Gọi `eventbus.Connect(ctx, cfg.NATSURL)` lần đầu tiên cho service này; truyền `pub` vào `tracing.Init(..., tracing.WithTraceEventPublisher(pub))`; giữ `cons` cho CR-FFT-003 |
| `backend-go/services/{usage,issue-tracking,notification,tenant,infra-fleet}-service/cmd/server/main.go` | Thêm `tracing.WithTraceEventPublisher(pub)` vào `Init(...)` call đã có (1 dòng/service, `pub` đã tồn tại sẵn từ `eventbus.Connect`) |

## Không thuộc phạm vi CR này

- Bật `WithTraceEventPublisher` cho 12 service chưa nối NATS — cần CR riêng nếu business muốn coverage đầy đủ 17/17 service trong TracePanel; hiện tại chỉ 6 service (api-gateway + 5 đã nối sẵn) có trace forward thật, các service khác vẫn "im lặng" ở TracePanel (không lỗi, chỉ không xuất hiện) — cần ghi rõ giới hạn này trong release note.
- `level: "step"` cho sub-step giữa start/end — cần span con lồng nhau ở tầng instrumentation chi tiết hơn (ngoài scope CR-FFT-001 lẫn CR-FFT-002).
- TTL/retention policy cho JetStream stream `TRACE` — dùng default của `EnsureStream` (không giới hạn) ở v1; nên giới hạn retention ngắn (vd. `MaxAge: 1h`) cho stream này vì dữ liệu chỉ có giá trị real-time, nhưng đây là tuning, không phải correctness — để lại follow-up.

## Tiêu chí chấp nhận

- [ ] Gọi 1 gRPC method bất kỳ trên `api-gateway` với `NATSURL` cấu hình → xuất hiện đúng 1 message trên subject `orca.api-gateway.trace.span`, JSON parse được thành `TraceEvent` hợp lệ (field name khớp chính xác frontend's `TraceEvent` type).
- [ ] Khi `NATSURL` rỗng/không cấu hình (local dev không có NATS) → `Init` không lỗi, không panic, hành vi giống hệt trước CR này (giữ đúng tiền lệ "always-sample, exporter-less" fallback).
- [ ] `EnsureStream` idempotent — khởi động lại service nhiều lần không lỗi "stream already exists".
- [ ] Span lỗi (`span.Status().Code == codes.Error`) publish `level: "fail"` — không im lặng biến mất giống F40's "fail luôn phải thấy" acceptance criteria gốc.

## Impact analysis (gitnexus)

| Symbol | Direction | Risk | Impacted | Ghi chú |
|---|---|---|---|---|
| `Init` (`backend-go/common/tracing/tracing.go`) | upstream | 🔴 **CRITICAL** | 34 (17 direct) | Thêm `opts ...Option` biến-tham-số — additive, không phá call site không truyền option; đã verify không có call site nào dùng function-value-typed-đè-lên-signature-cũ theo kiểu gây lỗi compile |

`TraceEventSpanProcessor` và `WithTraceEventPublisher` là symbol **mới** — không cần `impact()` trước khi tạo (chỉ bắt buộc trước khi *sửa* symbol có sẵn, theo CLAUDE.md). Đã chạy `detect_changes()` scope dự kiến: giới hạn ở `common/tracing/*`, `common/eventbus` (không đổi), 2 file config/main.go của 6 service liệt kê ở trên — chưa chạy so với `main` vì CR này chưa implement, sẽ chạy lại ngay trước khi commit thật.

## Liên quan

- [F40-full-flow-tracing.md](../../../features/F40-full-flow-tracing.md) — mục "Sink Architecture", "SSE broadcast sink"
- [CR-TRACE-000](../../v2/full-flow-tracing/CR-TRACE-000-tracing-rollout-overview.md) §3.2 — quy ước `traceId`/wire-envelope cho TS Electron; CR này áp dụng cùng tinh thần ("1 id nhất quán") nhưng qua cơ chế khác (OTel `TraceID` + context propagation, không phải `resume.id`) vì transport khác hẳn (gRPC/NATS, không phải WS RPC/relay.call)
- `backend-go/common/eventbus/eventbus.go` (đặc biệt `SubscribeEphemeral`, dòng 134-165)
- `backend-go/services/notification-service/internal/adapter/broadcaster/broadcaster.go` — tiền lệ pattern "mỗi replica `SubscribeEphemeral` độc lập, fan-out in-memory riêng"
- `backend-go/common/outbox/outbox.go` — lý do KHÔNG dùng outbox cho trace event (xem "Bối cảnh & Vấn đề")
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md` dòng 32-33 (subject naming convention)
- [CR-FFT-001](./CR-FFT-001-otel-span-instrumentation.md) — phụ thuộc cứng
- [CR-FFT-003](./CR-FFT-003-api-gateway-sse-real-forwarding.md) — tiêu thụ output của CR này
- [README.md](./README.md)
