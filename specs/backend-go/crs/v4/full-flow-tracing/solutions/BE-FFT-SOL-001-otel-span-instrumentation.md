# BE-FFT-SOL-001: OTel span thật ở lớp gRPC/HTTP interceptor + fix `trace_id` trong log

> **🔲 Designed — chưa implement.** Nền tảng — mọi solution sau phụ thuộc
> cứng vào đây.

**CR:** [CR-FFT-001](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-001-otel-span-instrumentation.md)
**Service:** tất cả 17 service backend-go (`common/tracing`, `common/grpcmw`, `common/logging` dùng chung; mỗi `cmd/server/main.go` tự wire)
**TDD tham chiếu:** `specs/backend-go/tdd/architecture/09-observability-reliability.md` §"Every inbound gRPC/HTTP request starts a span"

---

## 1. Re-verify trạng thái hiện tại (bắt buộc trước khi thiết kế — 2026-09-09)

Đọc trực tiếp code (không dùng lại mô tả CR nếu chưa đối chiếu) xác nhận
CR-FFT-001's mô tả **vẫn đúng 100%, chưa có gì thay đổi**:

- `backend-go/common/tracing/tracing.go:32-59` — `Init(ctx, serviceName,
  otlpEndpoint string) (Shutdown, error)` chỉ dựng `TracerProvider` (`sdktrace.
  NewTracerProvider(opts...)`, `otel.SetTracerProvider(tp)`), không gọi
  `otel.SetTextMapPropagator` ở đâu cả — mặc định OTel dùng no-op propagator.
- `backend-go/common/grpcmw/grpcmw.go:113-119` — `ChainUnary` thật:
  ```go
  func ChainUnary(logger *slog.Logger) grpc.ServerOption {
      return grpc.ChainUnaryInterceptor(
          RecoveryInterceptor(logger),
          TenantExtractionInterceptor(),
          LoggingInterceptor(logger),
      )
  }
  ```
  Đúng 3 interceptor, không có span creation nào, dù package doc comment
  (dòng 1-7) trích spec hứa "OpenTelemetry span creation".
- `backend-go/common/logging/logging.go:42-50` — `correlatingHandler.Handle`
  chỉ đọc `tenant.TenantID(ctx)`/`tenant.UserID(ctx)`, không có dòng nào đọc
  `trace.SpanContextFromContext(ctx)`.
- Grep toàn repo `otelgrpc\|otelhttp` trong `.go` (loại `go.mod`/`go.sum`):
  **0 kết quả** — xác nhận đúng "hạ tầng gần có nhưng chưa nối dây".
  `backend-go/common/go.mod:96` có `otelhttp v0.69.0` nhưng đang
  `// indirect`.
- 17/17 `cmd/server/main.go` gọi `grpc.NewServer(grpcmw.ChainUnary(logger))`
  (xác nhận mẫu tại `git-gateway-service/cmd/server/main.go:214`) — không
  service nào truyền thêm `grpc.StatsHandler` nào khác.
- `api-gateway` không chạy gRPC server (đúng comment trong chính
  `main.go`'s package doc — "Unlike every other service, api-gateway runs
  no gRPC server of its own") — nghĩa là `ChainUnary`'s 16 direct
  caller **không bao gồm** `api-gateway`; HTTP edge span của
  `api-gateway` là 1 việc riêng (mục 4 dưới đây), không đi qua `ChainUnary`.

## 2. Impact analysis — re-verify bằng `impact()` thật (MCP), không dùng lại số liệu cũ

| Symbol | Direction | Risk | Impacted | Khớp với CR gốc? |
|---|---|---|---|---|
| `ChainUnary` (`common/grpcmw/grpcmw.go`) | upstream | 🔴 **CRITICAL** | 32 (16 direct, 9 process affected, 3 module affected) | **Khớp 100%** |
| `Init` (`common/tracing/tracing.go`) | upstream | 🔴 **CRITICAL** | 34 (17 direct, 10 process affected, 4 module affected) | **Khớp 100%** |
| `correlatingHandler` (`common/logging/logging.go`) | upstream | 🔴 **CRITICAL** | 37 (3 direct @ depth 1, 17 @ depth 2, 17 @ depth 3; 10 process affected) | **Khớp 100%** |
| `mountTraceRoutes` (tham chiếu, thuộc CR-FFT-003) | upstream | 🟢 LOW | 3 (1 direct) | Khớp 100% |

**Không có gì giảm nhẹ được** — cả 3 symbol vẫn CRITICAL đúng như CR gốc
cảnh báo. Chiến lược additive-only dưới đây là bắt buộc, không phải tuỳ
chọn thiết kế.

## 3. Giải pháp

### 3.1. `grpcmw.StatsHandler()` — server-side span, KHÔNG sửa `ChainUnary`

```go
// backend-go/common/grpcmw/grpcmw.go — thêm hàm mới, ChainUnary giữ nguyên
// từng ký tự.

import (
    "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

// StatsHandler returns the OTel gRPC stats handler every service should
// pass to grpc.NewServer alongside ChainUnary — kept as a separate
// grpc.ServerOption (not folded into ChainUnary) so adding it doesn't touch
// ChainUnary's signature, which 16 services call identically today (see
// CR-FFT-001's impact() re-verify: CRITICAL risk, 32 impacted).
func StatsHandler() grpc.ServerOption {
    return grpc.StatsHandler(otelgrpc.NewServerHandler())
}
```

Mỗi `main.go` (16 service có gRPC server) tự thêm:

```go
grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())
```

(chỉ thêm 1 tham số vào lệnh gọi `grpc.NewServer(...)` đã có — `grpc.
NewServer` chấp nhận nhiều `grpc.ServerOption` biến-đổi, không có giới hạn
tương thích nào ở đây).

### 3.2. `tracing.Init` — set propagator (additive, 1 dòng)

```go
// backend-go/common/tracing/tracing.go
import (
    "go.opentelemetry.io/otel/propagation"
)

func Init(ctx context.Context, serviceName, otlpEndpoint string) (Shutdown, error) {
    // ... không đổi phần dựng resource/exporter/TracerProvider hiện có ...
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{}) // NEW — W3C traceparent, mặc định trước đây là no-op propagator
    return func(ctx context.Context) error { return tp.Shutdown(ctx) }, nil
}
```

Signature `Init` **không đổi** ở bước này (CR-FFT-002 mới thêm `opts
...Option` — xem BE-FFT-SOL-002, tách riêng để review độc lập).

### 3.3. `correlatingHandler.Handle` — đọc span context

```go
// backend-go/common/logging/logging.go
import "go.opentelemetry.io/otel/trace"

func (h *correlatingHandler) Handle(ctx context.Context, record slog.Record) error {
    if tid, ok := tenant.TenantID(ctx); ok {
        record.AddAttrs(slog.String("tenant_id", tid))
    }
    if uid, ok := tenant.UserID(ctx); ok {
        record.AddAttrs(slog.String("user_id", uid))
    }
    if sc := trace.SpanContextFromContext(ctx); sc.IsValid() { // NEW
        record.AddAttrs(
            slog.String("trace_id", sc.TraceID().String()),
            slog.String("span_id", sc.SpanID().String()),
        )
    }
    return h.inner.Handle(ctx, record)
}
```

Struct fields/method signature không đổi — chỉ thêm logic bên trong thân
hàm.

### 3.4. `api-gateway`'s HTTP edge span

```go
// backend-go/services/api-gateway/cmd/server/main.go — bọc router trước khi đưa cho http.Server
handler := otelhttp.NewHandler(httpgateway.NewRouter(deps), "api-gateway")
publicServer := &http.Server{Addr: ..., Handler: handler}
```

Không sửa `NewRouter`/`Deps` — chỉ bọc kết quả của nó tại composition root
(`main.go`), đúng vị trí `publicServer := &http.Server{...}` đã có sẵn
(dòng ~325).

### 3.5. Outbound propagation — mỗi service tự sửa `Dial` riêng của mình

**Khác với mô tả CR gốc** ("`backend-go/common/grpcclient` (hoặc package
dial dùng chung mỗi service)") — re-verify xác nhận **không có 1 package
chung** cho việc này. Mỗi service tự định nghĩa `Dial` cục bộ, cùng khuôn:

```go
// vd. backend-go/services/task-service/internal/adapter/grpcclient/dial.go (đã có)
func Dial(addr string) (*grpc.ClientConn, error) {
    conn, err := grpc.NewClient(addr,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithStatsHandler(otelgrpc.NewClientHandler()), // NEW
    )
    ...
}
```

Đã xác nhận tồn tại ít nhất ở `api-gateway/internal/adapter/grpc/dial.go`,
`git-gateway-service/internal/adapter/grpcclient` (hàm `Dial` trong file
`resolver.go`), `task-service/internal/adapter/grpcclient/dial.go` — cùng
1 khuôn `func Dial(addr string) (*grpc.ClientConn, error)`. Các service
khác có thư mục `grpcclient` (`auth-service`, `ai-provider-service`,
`project-service`, `automation-service`) cần audit riêng lúc implement
(tìm `func Dial` hoặc tương đương trong package đó) — không liệt kê đầy đủ
17/17 ở đây vì đây là N thay đổi độc lập, thấp rủi ro (mỗi service edit
riêng file của mình, không có symbol dùng chung để CRITICAL hoá).

## 4. Không thuộc phạm vi solution này

- Span con chi tiết theo domain (DB query, Vault call, NATS publish) — chỉ
  span boundary-level (gRPC/HTTP).
- RED-metrics qua OTel Metrics SDK.
- Sampling strategy production — giữ always-sample khi không có OTLP
  endpoint, không đổi.
- Thêm `WithTraceEventPublisher` (đó là BE-FFT-SOL-002).

## 5. Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| `ChainUnary` chạm bởi 16 service | 🔴 CRITICAL (theo blast radius) nhưng **không sửa hàm này** | Additive — an toàn nếu tuân thủ đúng, nhưng PHẢI re-verify `impact()` ngay trước khi bất kỳ ai chạm `grpcmw.go` |
| `Init` chạm bởi 17 service | 🔴 CRITICAL | Chỉ thêm 1 dòng nội bộ, signature không đổi ở CR này |
| `correlatingHandler` chạm bởi 17 service (qua `logging.New`) | 🔴 CRITICAL | Chỉ thêm logic trong `Handle()`, phải test log JSON shape không đổi khi ngoài span |
| Outbound `Dial` — N service riêng lẻ | 🟢 Thấp (per-service, không dùng chung) | Không tìm thấy hết 17/17 nơi cần sửa trong solution này — task thực thi phải tự audit `grpcclient`/`internal/adapter/grpc` của từng service khi tới lượt |
| Race giữa nhiều `Init` gọi song song trong test | Trung bình | `otel.SetTextMapPropagator` là global — cần test không panic/không race khi gọi `Init` nhiều lần (unit test song song) |

## Liên quan

- `backend-go/common/tracing/tracing.go:32-59`
- `backend-go/common/grpcmw/grpcmw.go:113-119`
- `backend-go/common/logging/logging.go:42-50`
- `backend-go/common/go.mod:11-13,95-100`
- `backend-go/services/git-gateway-service/cmd/server/main.go:214` (mẫu `ChainUnary` call site)
- `backend-go/services/api-gateway/cmd/server/main.go` (composition root, HTTP edge)
- `backend-go/services/api-gateway/internal/adapter/grpc/dial.go`,
  `backend-go/services/task-service/internal/adapter/grpcclient/dial.go` (mẫu outbound `Dial`)
- [BE-FFT-SOL-002](./BE-FFT-SOL-002-span-to-nats-trace-event-bridge.md) — phụ thuộc solution này
- [README.md](./README.md) — cảnh báo rủi ro CRITICAL tổng hợp
