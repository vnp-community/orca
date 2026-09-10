# TASK-BE-FFT-003: `correlatingHandler.Handle` — đọc `trace_id`/`span_id` từ span context

**Solution:** BE-FFT-SOL-001 | **CR:** [CR-FFT-001](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-001-otel-span-instrumentation.md)
**Service:** `common/logging` (dùng bởi 17/17 service qua `logging.New`)
**Depends on:** Không
**Status:** ✅ DONE (2026-09-09)

---

## Mục tiêu

Hoàn thành đúng lời hứa đã có sẵn trong doc comment của chính
`correlatingHandler` ("once tracing is wired via common/tracing, trace_id")
— thêm `trace_id`/`span_id` vào mọi log record nằm trong 1 span hợp lệ,
không panic/không đổi shape khi ngoài span.

## gitnexus — BẮT BUỘC trước khi sửa

`correlatingHandler` là symbol CRITICAL cao nhất trong 3 symbol (37
impacted). Chạy lại ngay trước khi sửa:

```
mcp__gitnexus__impact({ target: "correlatingHandler", direction: "upstream", repo: "orca", summaryOnly: true })
```

Kỳ vọng: CRITICAL, ~37 impacted (3 direct @ depth 1 — `New`, `Handle` chính
nó, và 1 symbol khác; depth 2/3 lan ra 17 service qua `logging.New`). Nếu
lệch, dừng lại và báo cáo.

## Files cần sửa

1. `backend-go/common/logging/logging.go` (MODIFY — chỉ bên trong `Handle()`)

## Nội dung

```go
// backend-go/common/logging/logging.go
import "go.opentelemetry.io/otel/trace" // NEW import

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

Struct `correlatingHandler` (fields), `Enabled`/`WithAttrs`/`WithGroup` —
**không đổi**. Doc comment ở dòng 31-33 (package-level, phía trên struct)
đã đúng nội dung từ trước ("once tracing is wired... trace_id") — không
cần sửa comment, chỉ cần code khớp với lời hứa đó.

## Test cases cần cover

`common/logging` hiện **chưa có file `_test.go` nào** — file mới:
`backend-go/common/logging/logging_test.go`.

- `TestCorrelatingHandler_AddsTraceIDWhenSpanValid` — tạo 1 span giả (dùng
  `sdktrace.NewTracerProvider()` no-op hoặc `trace.ContextWithSpanContext`
  với `SpanContext` hợp lệ tự construct), gọi `Handle`, parse JSON output,
  xác nhận có `trace_id`/`span_id` đúng giá trị.
- `TestCorrelatingHandler_NoSpanContext_NoTraceIDField_NoPanic` —
  **regression bắt buộc**: gọi `Handle` với `context.Background()` (không
  có span), xác nhận JSON output **không có** field `trace_id`/`span_id`,
  và **không panic** — đúng tiêu chí chấp nhận CR-FFT-001 ("không có field
  này khi ngoài span... không panic trong cả 2 trường hợp").
- `TestCorrelatingHandler_TenantUserFieldsUnaffected` — xác nhận
  `tenant_id`/`user_id` vẫn hoạt động y hệt trước khi thêm logic mới (test
  cả 3 field cùng lúc: có tenant+user, không có span → chỉ 2 field đầu
  xuất hiện).
- `TestCorrelatingHandler_WithAttrsWithGroup_StillDelegateCorrectly` — xác
  nhận 2 method còn lại (`WithAttrs`, `WithGroup`) không bị ảnh hưởng bởi
  thay đổi trong `Handle`.

## Verify

```bash
cd backend-go/common && go build ./... && go test -race ./logging/...
gofmt -l common/logging/logging.go common/logging/logging_test.go
```

## gitnexus (sau khi sửa)

```
mcp__gitnexus__detect_changes({ scope: "compare", base_ref: "main", repo: "orca" })
```
Xác nhận scope chỉ gồm `common/logging/logging.go` (+ file test mới) —
struct fields/method signature không đổi nghĩa là không có call site nào
trong 17 service cần sửa.

## Blocking

Không có task nào phụ thuộc cứng vào task này riêng biệt, nhưng nó là 1
trong 3 điều kiện của CR-FFT-001's tiêu chí chấp nhận tổng thể ("log JSON
có field trace_id/span_id khi trong span") — nên hoàn tất cùng đợt với
TASK-BE-FFT-001/002 trước khi coi BE-FFT-SOL-001 DONE.

## Kết quả thực tế (2026-09-09)

- `impact({target: "correlatingHandler", direction: "upstream"})` chạy lại
  ngay trước khi sửa: CRITICAL, 37 impacted / 3 direct — khớp 100% kỳ
  vọng.
- Thêm import `go.opentelemetry.io/otel/trace` + khối
  `if sc := trace.SpanContextFromContext(ctx); sc.IsValid() { ... }` bên
  trong `Handle`, đúng vị trí sau `user_id`. Struct fields, `Enabled`,
  `WithAttrs`, `WithGroup` không đổi.
- File test mới hoàn toàn `logging_test.go` (xác nhận trước bằng `find`:
  package chưa có `_test.go`) — construct `*correlatingHandler` trực tiếp
  trong test (cùng package, field `inner` unexported nhưng truy cập được)
  ghi vào `bytes.Buffer` để decode JSON output thật, thay vì chỉ kiểm tra
  gọi không lỗi. 4 test: có span hợp lệ → trace_id/span_id đúng giá trị;
  không có span → 2 field vắng mặt + không panic (regression bắt buộc);
  tenant_id/user_id không đổi hành vi; WithAttrs/WithGroup vẫn delegate
  đúng.
- `go test -race ./logging/... -v`: 4/4 PASS, không race.
- `gofmt -l`: sạch. Build lại toàn bộ 17 module riêng lẻ: tất cả OK.
- Scope xác nhận qua `git status --porcelain`: chỉ
  `common/logging/logging.go` (M) + `common/logging/logging_test.go`
  (mới) — không lan sang service nào, đúng dự kiến (không đổi struct
  fields/method signature).
