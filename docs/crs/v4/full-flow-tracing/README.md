# Full-Flow Tracing (backend-go) — Change Requests (v4)

> **Bối cảnh & khác biệt với `docs/crs/v2/full-flow-tracing/`:** Thư mục [`docs/crs/v2/full-flow-tracing/`](../../v2/full-flow-tracing/README.md) chứa series **CR-TRACE-000 → CR-TRACE-019**, giải quyết việc **rollout instrumentation tracing (`createTracer`/`span.start/step/ok/fail`) vào 19 luồng nghiệp vụ của kiến trúc `backend/` Electron cũ** (worktree, agent orchestration, terminal, remote dev, code review, ...) — xem [CR-TRACE-000](../../v2/full-flow-tracing/CR-TRACE-000-tracing-rollout-overview.md) để hiểu đầy đủ scope đó.
>
> **Series CR-FFT-001 → CR-FFT-003 trong thư mục này là một vấn đề hoàn toàn khác**: F40's spec gốc (`docs/features/F40-full-flow-tracing.md`) và CR-TRACE series đều giả định 1 process Node.js duy nhất (`registerTraceSink()` in-process fan-out). Khi backend chuyển sang **`backend-go`** (17 Go microservices, gRPC nội bộ + NATS JetStream), giả định đó không còn đúng — `api-gateway`'s SSE endpoint `/api/trace-stream` (`backend-go/services/api-gateway/internal/adapter/httpgateway/trace_routes.go`) hiện chỉ gửi heartbeat, có dòng TODO tự nhận "does NOT forward any real trace/debug events yet, since backend-go has no equivalent to the old backend's global `registerTraceSink()` fan-out" (trích nguyên văn, dòng 24-30). Series CR-FFT-XXX (ID prefix **mới, không phải CR-TRACE**, để không trùng/nhầm lẫn với series v2) giải quyết đúng gap này: làm sao N service backend-go độc lập fan-in được event trace thật về 1 SSE endpoint.
>
> Dùng ID prefix `CR-FFT-XXX` (Full-Flow Tracing) thay vì tiếp tục đánh số `CR-TRACE-02x` vì đây là 1 kiến trúc/tầng hoàn toàn khác (Go microservices + NATS, không phải TS Electron + WS RPC/relay.call) — tránh người đọc tưởng nhầm đây là phần tiếp theo của rollout CR-TRACE-001…019.

## Tổng quan gap đã xác nhận (bằng chứng chi tiết ở từng CR)

| Gap | Trạng thái thật | CR |
|-----|-----------------|-----|
| OTel span thật được tạo cho request gRPC/HTTP | ❌ Chưa có — `common/tracing.Init` chỉ dựng `TracerProvider` rỗng; `common/grpcmw`'s `ChainUnary` không có OTel interceptor dù doc comment hứa; không có `otelgrpc`/`otelhttp` import ở đâu trong repo | CR-FFT-001 |
| `trace_id`/`span_id` trong structured log | ❌ `common/logging`'s `correlatingHandler` tự hứa trong comment nhưng `Handle()` chưa từng đọc span context | CR-FFT-001 |
| Cơ chế fan-in event từ N service về 1 điểm | ❌ Chưa có bridge nào giữa OTel span và `common/eventbus` (NATS JetStream) — hạ tầng pub/sub duy nhất trong `backend-go`, đã có tiền lệ chạy thật ở `notification-service`'s broadcaster nhưng chưa áp dụng cho trace | CR-FFT-002 |
| SSE endpoint `/api/trace-stream` forward event thật | ❌ Heartbeat-only, TODO xác nhận trong code (`trace_routes.go:24-30`) | CR-FFT-003 |

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-FFT-001](./CR-FFT-001-otel-span-instrumentation.md) | Không có span OTel thật nào được tạo — nền tảng bắt buộc trước 2 CR sau | 🔴 P0 | Large | 🔲 Chưa triển khai |
| [CR-FFT-002](./CR-FFT-002-span-to-nats-trace-event-bridge.md) | Không có bridge span → NATS trace-event; chưa quyết định wire format | 🔴 P0 | Medium | 🔲 Chưa triển khai |
| [CR-FFT-003](./CR-FFT-003-api-gateway-sse-real-forwarding.md) | `trace_routes.go`'s TODO — SSE heartbeat-only, TracePanel trống với backend-go | 🔴 P0 | Small–Medium | 🔲 Chưa triển khai |

## Thứ tự thực thi

```
CR-FFT-001 (OTel span thật qua gRPC/HTTP interceptor)
        │  span tồn tại, nhưng chưa ai đọc được từ process khác
        ▼
CR-FFT-002 (Span → NATS JetStream, subject orca.<service>.trace.span)
        │  event đã ra NATS, nhưng api-gateway chưa consume
        ▼
CR-FFT-003 (api-gateway subscribe NATS → fan-out SSE /api/trace-stream)
        │  TracePanel nhận được event thật, đúng F40 acceptance criteria
```

Cả 3 CR phụ thuộc tuyến tính — không làm song song được, khác với CR-RBAC series (v4/team-rbac) vốn có nhiều CR độc lập nhau.

## Kiến trúc được chọn — vì sao không tự chế 1 hệ trace riêng cho backend-go

Task gốc đặt câu hỏi kiến trúc cốt lõi: *"event trace thật đến từ đâu — mỗi service tự log span ở đâu? có message bus/pub-sub nào giữa các service không?"* Khảo sát trực tiếp mã nguồn (không suy đoán) xác nhận:

1. **Có đúng 1 hạ tầng pub/sub liên-service trong `backend-go`**: `common/eventbus` (NATS JetStream), theo convention `orca.<service>.<entity>.<event>` đã định nghĩa sẵn trong `specs/backend-go/tdd/architecture/08-inter-service-communication.md`. Không có Redis/gRPC-streaming-ngược/message-bus nào khác.
2. **Có sẵn OTel SDK scaffold** (`common/tracing`) nhưng **chưa ai tạo span thật** — không phải "đã có hệ trace, chỉ thiếu forward" như giả định ban đầu trong `docs/roadmap/feature-completion-matrix.md`, mà là **chưa có dữ liệu nguồn** ở tầng sâu hơn TODO trong `trace_routes.go` mô tả.
3. **Có tiền lệ chạy thật cho đúng bài toán "N service/replica → fan-in → 1 client"**: `notification-service`'s `broadcaster.go` + `SubscribeEphemeral` — mỗi replica tự subscribe NATS, fan-out in-process tới subscriber cục bộ, không cần biết về replica khác.

Quyết định kiến trúc: **tái dùng cả 2 (OTel scaffold có sẵn + NATS JetStream có sẵn)**, nối chúng bằng 1 `SpanProcessor` publish JSON đúng shape `TraceEvent` mà frontend đã định nghĩa sẵn (không đổi frontend) — thay vì phát minh 1 giao thức span/step/ok/fail riêng cho backend-go như F40 gốc mô tả cho tầng TS, hoặc dựng 1 hệ pub/sub mới song song với NATS đã có.

**Khác biệt có chủ ý so với CR-TRACE-000's convention**: CR-TRACE-000 định nghĩa `resume.id`/`traceId` field lan truyền thủ công qua wire envelope (vì TS's `Tracer.start()` tự sinh `shortId()`, cần cơ chế "nhận id từ layer trước"). Ở backend-go, id xuyên-boundary chính là **OTel's `TraceID`**, lan truyền tự động qua W3C `traceparent` propagator (CR-FFT-001) — không cần tự tay truyền `traceId` qua từng transport như bảng ở CR-TRACE-000 §3.3, vì gRPC/NATS context propagation làm việc đó ở tầng chuẩn OTel. Hai cơ chế phục vụ cùng mục tiêu ("1 id nhất quán xuyên suốt stack") nhưng không tương thích trực tiếp về wire format — **không trộn 2 convention trong cùng 1 request** (một request đi qua cả TS Electron lẫn backend-go sẽ có 2 id độc lập ở 2 nửa stack; hợp nhất 2 id đó là vấn đề khác, ngoài phạm vi cả 2 series CR).

## Impact analysis (gitnexus, chạy trước khi viết CR — theo CLAUDE.md)

| Symbol | File | Risk | Impacted | CR |
|---|---|---|---|---|
| `mountTraceRoutes` | `api-gateway/internal/adapter/httpgateway/trace_routes.go` | 🟢 LOW | 3 (1 direct) | CR-FFT-003 |
| `ChainUnary` | `common/grpcmw/grpcmw.go` | 🔴 CRITICAL | 32 (16 direct, 9 process) | CR-FFT-001 (không sửa hàm này — thêm hàm mới song song) |
| `Init` | `common/tracing/tracing.go` | 🔴 CRITICAL | 34 (17 direct — mọi service) | CR-FFT-001, CR-FFT-002 (cả 2 CR đều additive-only) |
| `correlatingHandler` | `common/logging/logging.go` | 🔴 CRITICAL | 37 (3 direct, 10 process) | CR-FFT-001 |

**Cảnh báo bắt buộc**: 3/4 symbol có risk CRITICAL vì được dùng bởi toàn bộ 17 service backend-go. Cả CR-FFT-001 và CR-FFT-002 được thiết kế cố ý theo nguyên tắc **additive-only** (thêm hàm/option mới, không đổi signature/behavior mặc định của hàm hiện có) để giữ blast radius thực tế ở mức kiểm soát được — đúng tiền lệ đã áp dụng khi thêm OTLP exporter vào `tracing.Init` trước đây (`backend-go/docs/execution-plan.md` dòng 665). Mỗi CR phải chạy lại `impact()` ngay trước khi implement thật (không dùng số liệu ở đây nếu code đã đổi kể từ khi audit), và `detect_changes({scope:"compare", base_ref:"main"})` trước khi commit.

## Việc chưa làm ngoài bộ CR này

- Bật `WithTraceEventPublisher` (CR-FFT-002) cho 12/17 service chưa từng kết nối NATS (`task-service`, `workflow-service`, `project-service`, `auth-service`, `git-gateway-service`, `scm-integration-service`, `credential-broker-service`, `annotation-service`, `ai-provider-service`, `automation-service`, `orchestration-service`) — CR-FFT-002 chỉ bật cho `api-gateway` + 5 service đã có `*eventbus.Publisher` sẵn (`usage-service`, `issue-tracking-service`, `notification-service`, `tenant-service`, `infra-fleet-service`). Coverage đầy đủ 17/17 cần CR riêng.
- Span con chi tiết theo domain (DB query, Vault call, NATS publish là child span riêng — yêu cầu đầy đủ của `specs/backend-go/tdd/architecture/09-observability-reliability.md`) — CR-FFT-001 chỉ đảm bảo span boundary-level (gRPC/HTTP), chưa đủ để có `level: "step"` thật trong `TraceEvent`.
- Hợp nhất id giữa 2 nửa stack (TS Electron's `Tracer` id ↔ backend-go's OTel `TraceID`) khi 1 request đi qua cả 2 — xem "Kiến trúc được chọn" ở trên.
- Cập nhật `docs/roadmap/feature-completion-matrix.md`'s dòng F40 sau khi 3 CR này triển khai xong — nên làm như 1 phần acceptance của CR-FFT-003 khi merge thật, chưa sửa trong lần audit này.
