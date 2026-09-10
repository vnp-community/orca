# TASK-BE-FFT-002: `tracing.Init` — set W3C `TraceContext` propagator (additive, 1 dòng)

**Solution:** BE-FFT-SOL-001 | **CR:** [CR-FFT-001](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-001-otel-span-instrumentation.md)
**Service:** `common/tracing` (dùng bởi 17/17 service qua `Init`)
**Depends on:** Không
**Status:** ✅ DONE (2026-09-09)

---

## Mục tiêu

Không có span nào nối tiếp qua boundary gRPC nếu propagator vẫn là no-op
mặc định của OTel. Set `propagation.TraceContext{}` (W3C `traceparent`)
1 lần trong `Init`, giữ nguyên signature.

## gitnexus — BẮT BUỘC trước khi sửa

`Init` là 1 trong 3 symbol CRITICAL. Chạy lại **ngay trước khi sửa**,
không dùng số liệu đã ghi trong solution:

```
mcp__gitnexus__impact({ target: "Init", direction: "upstream", file_path: "backend-go/common/tracing/tracing.go", repo: "orca", summaryOnly: true })
```

Kỳ vọng: CRITICAL, ~34 impacted (17 direct). Nếu TASK-BE-FFT-007 (CR-FFT-002)
đã chạy trước và thêm `opts ...Option`, số liệu vẫn nên giữ nguyên (thêm
tham số biến-đổi không đổi call site cũ) — nhưng vẫn phải tự xác nhận, không
giả định.

## Files cần sửa

1. `backend-go/common/tracing/tracing.go` (MODIFY)

## Nội dung

```go
// backend-go/common/tracing/tracing.go
import (
    "go.opentelemetry.io/otel/propagation" // NEW import
)

func Init(ctx context.Context, serviceName, otlpEndpoint string) (Shutdown, error) {
    // ... phần dựng resource/exporter/TracerProvider hiện có, KHÔNG đổi ...

    tp := sdktrace.NewTracerProvider(opts...)
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{}) // NEW — trước đây mặc định no-op propagator, nghĩa là traceparent header không bao giờ được extract/inject qua gRPC metadata dù span có tồn tại

    return func(ctx context.Context) error {
        return tp.Shutdown(ctx)
    }, nil
}
```

Signature `Init(ctx, serviceName, otlpEndpoint string) (Shutdown, error)`
**không đổi** ở task này.

## Test cases cần cover

`common/tracing` hiện **chưa có file `_test.go` nào** (xác nhận bằng
`find` trước khi viết — đây là file test hoàn toàn mới, không mở rộng gì
có sẵn). File mới: `backend-go/common/tracing/tracing_test.go`.

- `TestInit_SetsTraceContextPropagator` — gọi `Init`, xác nhận
  `otel.GetTextMapPropagator()` trả về instance có behavior của
  `propagation.TraceContext{}` (vd. inject/extract round-trip 1
  `traceparent` header giả).
- `TestInit_SignatureUnchanged_ExistingCallSitesCompile` — không phải test
  runtime, mà là compile-check: giữ nguyên 1 dòng gọi
  `Init(ctx, "svc", "")` (2-arg như 17 call site hiện có) trong test để
  đảm bảo không ai vô tình đổi signature bắt buộc thêm tham số.
- **Regression bắt buộc** — `TestInit_NoOTLPEndpoint_StillWorksSameAsBefore`:
  gọi `Init(ctx, "svc", "")` (rỗng), xác nhận không lỗi/không panic, vẫn
  trả về `Shutdown` hợp lệ — giữ đúng hành vi "always-sample,
  exporter-less" fallback đã có trước task này.
- `TestInit_NoRaceWhenCalledConcurrently` — chạy `Init` trong nhiều
  goroutine (`go test -race`), xác nhận `otel.SetTextMapPropagator` không
  gây race — vì đây là global state, đúng tiêu chí chấp nhận CR-FFT-001 đã
  ghi ("không có race giữa nhiều lần `Init` gọi song song trong test").

## Verify

```bash
cd backend-go/common && go build ./... && go test -race ./tracing/...
gofmt -l common/tracing/tracing.go common/tracing/tracing_test.go
```

## gitnexus (sau khi sửa)

```
mcp__gitnexus__detect_changes({ scope: "compare", base_ref: "main", repo: "orca" })
```
Xác nhận scope chỉ gồm `common/tracing/tracing.go` (+ file test mới) —
không lan sang 17 `main.go` (vì không đổi signature, không call site nào
cần sửa).

## Blocking

TASK-BE-FFT-004 (HTTP edge span) và TASK-BE-FFT-005 (outbound propagation)
phụ thuộc MỀM vào task này — về mặt compile không cần, nhưng propagator
phải tồn tại thì span parent/child qua HTTP/gRPC client mới thực sự nối
được. TASK-BE-FFT-007 (CR-FFT-002, thêm `opts ...Option`) sửa lại đúng file
này lần 2 — phải hoàn tất task này trước.

## Kết quả thực tế (2026-09-09)

- `impact({target: "Init", direction: "upstream", file_path: "backend-go/common/tracing/tracing.go"})`
  chạy lại ngay trước khi sửa: CRITICAL, 34 impacted / 17 direct — khớp
  100% kỳ vọng, không lệch.
- Thêm import `go.opentelemetry.io/otel/propagation` + 1 dòng
  `otel.SetTextMapPropagator(propagation.TraceContext{})` ngay sau
  `otel.SetTracerProvider(tp)`. Signature `Init(ctx, serviceName,
  otlpEndpoint string) (Shutdown, error)` không đổi.
- File test mới hoàn toàn `tracing_test.go` (xác nhận bằng `find` trước:
  package chưa có `_test.go` nào) — 4 test: inject/extract round-trip thật
  qua `propagation.MapCarrier` (không chỉ kiểm tra non-nil), compile-guard
  cho call site 3-arg cũ, fallback không-OTLP-endpoint vẫn hoạt động, và
  race test (8 goroutine gọi `Init` đồng thời).
- `go test -race ./tracing/... -v`: 4/4 PASS, không race.
- `gofmt -l`: sạch. Build lại toàn bộ 17 module (`common` + `proto` + 16
  service riêng lẻ, vì `go build ./...` ở workspace root báo lỗi đã biết
  "directory prefix . does not contain modules" — không liên quan task
  này): tất cả OK, 0 lỗi.
- Scope xác nhận qua `git status --porcelain`: chỉ
  `common/tracing/tracing.go` (M) + `common/tracing/tracing_test.go`
  (mới) — không lan sang bất kỳ `main.go` nào, đúng dự kiến vì không đổi
  signature.
