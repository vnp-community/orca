# CR-FFT-001 — Bật tạo OTel span thật ở lớp gRPC/HTTP interceptor (nền tảng)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-FFT-001 |
| **Tên** | Wire OpenTelemetry span creation thật vào gRPC/HTTP interceptor + fix trace_id trong structured log |
| **Loại** | Architecture / Observability (nền tảng, không đổi behavior nghiệp vụ) |
| **Priority** | 🔴 P0 (mọi CR sau trong series này phụ thuộc cứng vào đây — không có span thì không có gì để forward) |
| **Effort** | Large (chạm interceptor dùng chung bởi 17/17 service) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu "thực thi đầy đủ F40 ở lớp backend-go" |
| **Tác động Features** | F40 |
| **Phụ thuộc** | Không — nền tảng, phải làm **trước** CR-FFT-002 và CR-FFT-003 |

---

## Bối cảnh & Vấn đề

F40's spec (`docs/features/F40-full-flow-tracing.md`) mô tả một hệ tracing isomorphic ở kiến trúc TS Electron cũ (`src/shared/trace/*`). Khi backend chuyển sang `backend-go` (microservices, gRPC nội bộ + REST edge), **không có bất kỳ span thật nào được tạo ra** ở tầng này — dù hạ tầng OTel đã được "cắm điện" một phần:

### 1. `common/tracing.Init` chỉ dựng `TracerProvider`, không ai gọi `tracer.Start()`

`backend-go/common/tracing/tracing.go:5-7` tự thừa nhận trong doc comment:

```go
// Package tracing wires OpenTelemetry for a service. With no OTLPEndpoint
// configured, Init falls back to an always-sample, exporter-less provider —
// ...
// Full span-attribute/RED-metrics work described in
// specs/backend-go/architecture/09-observability-reliability.md
// is still a follow-up; only the exporter itself is wired here.
```

Xác nhận bằng grep toàn repo: **không có lệnh gọi `otel.Tracer(...).Start(...)` nào** ngoài chính `tracing.go` (chỉ set `TracerProvider`, không tạo span). `Init` được gọi ở cả 17 `cmd/server/main.go` (`git-gateway-service`, `tenant-service`, `auth-service`, `project-service`, `infra-fleet-service`, `task-service`, `workflow-service`, `scm-integration-service`, `issue-tracking-service`, `usage-service`, `annotation-service`, `credential-broker-service`, `notification-service`, `api-gateway`, `orchestration-service`, `ai-provider-service`, `automation-service`) — nhưng vì không có interceptor/middleware nào tạo span cho request, `TracerProvider` đứng không, không sinh ra data.

### 2. `common/grpcmw`'s doc comment hứa "OpenTelemetry span creation" — nhưng `ChainUnary` không có

`backend-go/common/grpcmw/grpcmw.go:1-7` trích dẫn spec:

```go
// Package grpcmw provides the shared gRPC server interceptors every service
// wires into its adapter/grpc/ layer identically: panic recovery, tenant
// context extraction, and request logging. See
// specs/backend-go/architecture/08-inter-service-communication.md
// ("Server-side interceptors... handle: JWT validation, tenant-context
// extraction, OpenTelemetry span creation, structured request logging,
// panic recovery... No service hand-rolls this per-RPC.").
```

Nhưng `ChainUnary` thật (`grpcmw.go:113-119`) chỉ có 3 interceptor, **không có span creation**:

```go
func ChainUnary(logger *slog.Logger) grpc.ServerOption {
	return grpc.ChainUnaryInterceptor(
		RecoveryInterceptor(logger),
		TenantExtractionInterceptor(),
		LoggingInterceptor(logger),
	)
}
```

`specs/backend-go/tdd/architecture/09-observability-reliability.md:8` xác nhận yêu cầu gốc: *"Every inbound gRPC/HTTP request starts a span; every outbound gRPC call, DB query, Vault call, and NATS publish is a child span — a single request from `api-gateway` through 4 services and a DB call is one trace, not 5 disconnected logs."* — chưa có dòng nào trong số này tồn tại.

`otelgrpc`/`otelhttp` (thư viện chuẩn OTel để tự động tạo span cho interceptor gRPC/HTTP) **không được import ở đâu trong `backend-go`** (grep `otelgrpc\|otelhttp` trên toàn bộ `.go` ra rỗng). Đáng chú ý: `otelhttp` v0.69.0 đã có sẵn trong `backend-go/common/go.mod:96` nhưng chỉ là **indirect dependency** (kéo theo bởi thư viện khác, không ai import trực tiếp) — hạ tầng "gần có" nhưng chưa nối dây.

### 3. `common/logging`'s `correlatingHandler` tự hứa `trace_id` — nhưng không đọc span context

`backend-go/common/logging/logging.go:28-31`:

```go
// correlatingHandler injects tenant_id/user_id (and, once tracing is wired
// via common/tracing, trace_id) from context into every log record, so a
// handler doesn't have to remember to attach them at every call site.
```

Nhưng `Handle()` thật (`logging.go:39-46`) chỉ thêm `tenant_id`/`user_id`, **không bao giờ đọc `trace_id`/`span_id`** từ context dù comment nói "once tracing is wired" — vì đến giờ tracing vẫn chưa "wired" theo đúng nghĩa (mục 1–2 ở trên), promise này chưa từng được hiện thực hoá.

### Hệ quả với F40

`trace_routes.go`'s SSE endpoint (xem CR-FFT-003) không có event thật để forward **không chỉ** vì thiếu cơ chế fan-in — mà vì ở nguồn, backend-go chưa từng tạo ra 1 span/event nào để fan-in. CR này là điều kiện tiên quyết bắt buộc.

## Giải pháp đề xuất

Không tự chế một hệ span riêng cho backend-go (F40 gốc là spec cho tầng TS/Electron, không áp dụng nguyên trạng cho Go microservices) — tái dùng đúng OTel SDK đã có sẵn scaffold, chỉ còn thiếu bước "nối dây" interceptor:

1. **Server-side span**: thêm `otelgrpc.NewServerHandler()` (qua `grpc.StatsHandler`, cách khuyến nghị hiện tại của `otelgrpc` thay vì `UnaryServerInterceptor` đã deprecated) vào `grpc.NewServer(...)` options mỗi service — sửa `ChainUnary`/gọi `grpc.NewServer` để nhận thêm option này, giữ additive (không đổi signature `ChainUnary` hiện có, thêm option riêng ở call site `main.go`, hoặc thêm 1 hàm `grpcmw.StatsHandler()` mới cạnh `ChainUnary`).
2. **HTTP edge span**: `api-gateway` là điểm vào REST/WS duy nhất — bọc `otelhttp.NewHandler(mux, "api-gateway")` quanh `chi.Router` gốc trong composition root, để mọi request `/api/*` có span gốc.
3. **Outbound propagation**: mọi gRPC client dial (package `grpcclient` mỗi service dùng để gọi service khác) thêm `grpc.WithChainUnaryInterceptor(otelgrpc.UnaryClientInterceptor())` (hoặc `otelgrpc.NewClientHandler()`) để span nối tiếp qua boundary — đúng yêu cầu "một request qua 4 service là 1 trace, không phải 5 log rời" của spec 09.
4. **Propagator**: gọi `otel.SetTextMapPropagator(propagation.TraceContext{})` trong `tracing.Init` (hiện chưa set — mặc định OTel dùng no-op propagator) để W3C `traceparent` header thực sự được extract/inject qua gRPC metadata.
5. **Fix `correlatingHandler.Handle`**: đọc `trace.SpanContextFromContext(ctx)`, nếu `sc.IsValid()` thì thêm `slog.String("trace_id", sc.TraceID().String())` và `slog.String("span_id", sc.SpanID().String())` — hoàn thành đúng lời hứa đã có sẵn trong doc comment, không cần đổi comment.

## Changes Required

| File | Thay đổi |
|------|---------|
| `backend-go/common/tracing/tracing.go` | `Init` gọi thêm `otel.SetTextMapPropagator(propagation.TraceContext{})`; giữ nguyên signature (additive) |
| `backend-go/common/grpcmw/grpcmw.go` | Thêm hàm mới `StatsHandler() grpc.ServerOption` trả về `grpc.StatsHandler(otelgrpc.NewServerHandler())` — KHÔNG sửa `ChainUnary` hiện có (tránh đổi signature dùng bởi 17 service cùng lúc); mỗi `main.go` tự thêm option này cạnh `ChainUnary(logger)` khi gọi `grpc.NewServer(...)` |
| `backend-go/common/grpcclient` (hoặc package dial dùng chung mỗi service) | Thêm `otelgrpc.NewClientHandler()` vào `grpc.WithStatsHandler(...)` khi dial downstream service |
| `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go` (hoặc nơi khởi tạo `chi.Router` gốc) | Bọc `otelhttp.NewHandler(mux, "api-gateway")` trước khi đưa cho `http.Server` |
| `backend-go/common/logging/logging.go` | `correlatingHandler.Handle` đọc `trace.SpanContextFromContext(ctx)`, thêm `trace_id`/`span_id` khi hợp lệ |
| `backend-go/common/go.mod` | Thêm dependency trực tiếp `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`; `otelhttp` đã có sẵn as indirect (`common/go.mod:96`) — nâng thành direct |
| 17× `backend-go/services/*/cmd/server/main.go` | Thêm `grpcmw.StatsHandler()` vào `grpc.NewServer(...)` options (đứng cạnh `grpcmw.ChainUnary(logger)`, không thay thế) |

## Không thuộc phạm vi CR này

- Tạo span thủ công (`tracer.Start()`) bên trong usecase/repository cho từng bước nghiệp vụ cụ thể (DB query, Vault call, NATS publish) như spec 09 mô tả đầy đủ — CR này chỉ đảm bảo span **boundary-level** (gRPC/HTTP) tồn tại; instrumentation chi tiết hơn theo domain là follow-up riêng, ngoài scope "F40 forwarding" đang giải quyết.
- RED-metrics (rate/error/duration) qua OTel Metrics SDK — comment trong `tracing.go` đã tách rõ đây là việc khác.
- Sampling strategy cho production (hiện luôn always-sample khi không có OTLP endpoint) — chấp nhận nguyên trạng, không đổi trong CR này.

## Tiêu chí chấp nhận

- [ ] Một request HTTP vào `api-gateway` sinh ra 1 span gốc; nếu request đó gọi tiếp gRPC xuống service khác, span downstream là **child span cùng trace ID** (verify qua OTLP exporter trỏ vào Jaeger/Tempo local, hoặc log `trace_id` khớp nhau ở cả 2 service).
- [ ] `correlatingHandler`'s log JSON có field `trace_id`/`span_id` khi request nằm trong 1 span hợp lệ; không có field này khi ngoài span (background job) — không panic trong cả 2 trường hợp.
- [ ] Không service nào trong 17 service bị vỡ do đổi `ChainUnary` signature — cách làm additive (option mới, không sửa option cũ) nghĩa là mỗi `main.go` tự thêm dòng mới, không có compile-break hàng loạt.
- [ ] `otel.SetTextMapPropagator` được set đúng 1 lần, không có race giữa nhiều lần `Init` gọi song song trong test.

## Impact analysis (gitnexus)

Đã chạy `impact()` cho các symbol dự kiến sửa (bắt buộc theo CLAUDE.md trước khi edit):

| Symbol | Direction | Risk | Impacted | Ghi chú |
|---|---|---|---|---|
| `Init` (`backend-go/common/tracing/tracing.go`) | upstream | 🔴 **CRITICAL** | 34 (17 direct — mọi `cmd/server/main.go`) | Đổi thêm 1 dòng `otel.SetTextMapPropagator` bên trong `Init`, KHÔNG đổi signature → giữ additive để không phá 17 call site, đúng tiền lệ đã có (OTLP exporter fix trước đây trong `docs/execution-plan.md` dòng 665 cũng cố tình additive-only vì lý do này) |
| `ChainUnary` (`backend-go/common/grpcmw/grpcmw.go`) | upstream | 🔴 **CRITICAL** | 32 (16 direct, 9 process ảnh hưởng) | **Không sửa hàm này** — thêm hàm `StatsHandler()` mới song song, tránh chạm vào 32 symbol phụ thuộc |
| `correlatingHandler` (`backend-go/common/logging/logging.go`) | upstream | 🔴 **CRITICAL** | 37 (3 direct, 10 process ảnh hưởng — toàn bộ 17 service dùng `logging.New`) | Chỉ sửa bên trong `Handle()`, không đổi struct fields/method signature → an toàn nhưng vẫn CRITICAL theo blast radius review; phải test log JSON output không đổi shape khi ngoài span |

**⚠️ Cảnh báo bắt buộc theo CLAUDE.md**: cả 3 symbol trên đều **CRITICAL risk** (dùng bởi toàn bộ 17 service). Chiến lược implementation trong CR này được thiết kế cố ý để giữ **additive-only** (thêm option/dòng mới, không đổi signature/behavior mặc định của các hàm hiện có) nhằm giảm blast radius thực tế xuống mức kiểm soát được — nhưng vẫn cần `detect_changes({scope:"compare", base_ref:"main"})` trước khi commit để xác nhận không symbol ngoài dự kiến bị ảnh hưởng.

## Liên quan

- [F40-full-flow-tracing.md](../../../features/F40-full-flow-tracing.md)
- [CR-TRACE-000](../../v2/full-flow-tracing/CR-TRACE-000-tracing-rollout-overview.md) — convention `traceId`/wire-envelope cho kiến trúc TS Electron cũ; **không áp dụng trực tiếp** cho backend-go (transport khác hẳn: gRPC/NATS thay vì WS RPC/relay.call), nhưng cùng tinh thần "1 id xuyên suốt toàn bộ stack" — ở đây id đó là OTel's `TraceID` thay vì `Tracer.start()`'s `shortId()`.
- `specs/backend-go/tdd/architecture/09-observability-reliability.md` (dòng 7-8, yêu cầu gốc)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md` (dòng 13, 32-33 — subject/proto convention dùng lại ở CR-FFT-002)
- `backend-go/docs/execution-plan.md` dòng 465-479 (lịch sử `/api/trace-stream` heartbeat-only), dòng 665 (tiền lệ additive-only khi sửa `tracing.Init`)
- [README.md](./README.md) — tổng quan series CR-FFT
- [CR-FFT-002](./CR-FFT-002-span-to-nats-trace-event-bridge.md) — phụ thuộc vào CR này
