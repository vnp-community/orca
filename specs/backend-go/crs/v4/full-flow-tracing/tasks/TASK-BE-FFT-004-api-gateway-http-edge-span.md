# TASK-BE-FFT-004: `api-gateway` HTTP edge span qua `otelhttp.NewHandler`

**Solution:** BE-FFT-SOL-001 | **CR:** [CR-FFT-001](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-001-otel-span-instrumentation.md)
**Service:** `api-gateway` (composition root only)
**Depends on:** TASK-BE-FFT-002 (mềm — cần propagator để span có ý nghĩa xuyên boundary)
**Status:** ✅ DONE (2026-09-09)

---

## Mục tiêu

`api-gateway` là điểm vào REST/WS duy nhất và **không chạy gRPC server**
(xem `main.go`'s package doc comment: "Unlike every other service,
api-gateway runs no gRPC server of its own") — nên `ChainUnary`/`StatsHandler`
(TASK-BE-FFT-001) không áp dụng cho nó. Span gốc của mọi request `/api/*`
phải đến từ việc bọc `otelhttp.NewHandler` quanh `chi.Router`.

## gitnexus

Không sửa symbol CRITICAL nào — `NewRouter` (`httpgateway/router.go`)
không bị sửa, chỉ bọc **kết quả** của nó tại `main.go`. Không bắt buộc
`impact()` trước task này (không sửa symbol có sẵn), nhưng khuyến khích
đọc lại `mcp__gitnexus__context({name: "run", repo: "orca"})`
(`api-gateway/cmd/server/main.go`'s `run`) để xác nhận vị trí chính xác
`publicServer := &http.Server{...}` chưa đổi kể từ audit.

## Files cần sửa

1. `backend-go/services/api-gateway/cmd/server/main.go` (MODIFY — bọc handler trước khi gán cho `http.Server`)
2. `backend-go/common/go.mod` (MODIFY — nâng `otelhttp` từ `// indirect` thành direct dependency, đã có sẵn ở `common/go.mod:96` với version `v0.69.0`)

## Nội dung

```go
// backend-go/services/api-gateway/cmd/server/main.go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

// ... sau khi router := httpgateway.NewRouter(deps) (hoặc tương đương) đã có sẵn ...
handler := otelhttp.NewHandler(router, "api-gateway") // NEW

publicServer := &http.Server{
    Addr:    fmt.Sprintf(":%d", cfg.PublicPort),
    Handler: handler, // trước đây là `router` trực tiếp
    // ... field khác giữ nguyên ...
}
```

Không sửa `httpgateway.NewRouter`/`Deps` — chỉ đổi biến truyền vào
`http.Server.Handler` tại đúng vị trí `publicServer := &http.Server{...}`
đã tồn tại (dòng ~325 tại thời điểm audit — xác nhận lại số dòng thật khi
implement vì có thể lệch do các task khác chạy trước).

**Lưu ý quan trọng**: `healthServer` (dòng ~329, `/healthz`/`/readyz`)
**không bọc** `otelhttp` — health check không cần span, và không nên phụ
thuộc OTel exporter (nếu OTel có vấn đề, health check vẫn phải trả lời
được).

## Test cases cần cover

- `TestRun_PublicServerWrappedWithOtelhttp` — hoặc test ở mức thấp hơn nếu
  `run()` khó test trực tiếp (nhiều side-effect): tối thiểu xác nhận
  `otelhttp.NewHandler(router, "api-gateway")` là 1 `http.Handler` hợp lệ,
  request đi qua nó vẫn tới đúng route bên trong (khớp status code/body
  với khi gọi `router` trực tiếp) — otelhttp chỉ observe, không đổi
  response.
- **Regression bắt buộc** — xác nhận `/healthz`/`/readyz` (health server,
  không bọc otelhttp) hoạt động y hệt trước task này.
- Test tích hợp (nếu có harness httptest sẵn trong `api-gateway`): gọi 1
  route thật (vd. `/api/trace-stream` hoặc 1 route đã mature) qua handler
  đã bọc, xác nhận response headers/status không đổi so với trước khi bọc.

## Verify

```bash
cd backend-go/services/api-gateway && go build ./... && go test ./...
gofmt -l cmd/server/main.go
cd backend-go/common && go build ./... # xác nhận go.mod otelhttp direct không phá build package khác dùng chung common
```

## gitnexus (sau khi sửa)

```
mcp__gitnexus__detect_changes({ scope: "compare", base_ref: "main", repo: "orca" })
```
Xác nhận scope chỉ gồm `api-gateway/cmd/server/main.go` + `common/go.mod`/`go.sum`.

## Blocking

Không có task nào phụ thuộc cứng — đây là nhánh riêng của CR-FFT-001, độc
lập với TASK-BE-FFT-001/003 (khác file). Nên hoàn tất trước khi coi
BE-FFT-SOL-001 DONE vì là 1 phần của tiêu chí chấp nhận CR gốc ("Một
request HTTP vào api-gateway sinh ra 1 span gốc").

## Kết quả thực tế (2026-09-09)

- Không sửa symbol có sẵn nên không bắt buộc `impact()`; xác nhận
  `publicServer := &http.Server{...}` vẫn ở đúng vị trí mong đợi (dòng
  332, lệch nhẹ so với ước tính ~325 trong task doc do các task trước
  chạy trước nó — đã tự xác nhận lại bằng grep, không giả định số dòng
  cũ).
- Thêm import `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`,
  bọc `router` thành `publicHandler := otelhttp.NewHandler(router,
  "api-gateway")`, gán `publicHandler` cho `publicServer.Handler`.
  `healthServer.Handler` (health.New().Handler()) **không đổi** — đúng
  yêu cầu không bọc otelhttp quanh health check.
- **Sai lệch so với kế hoạch task doc (đã tự sửa theo thực tế, không theo
  đúng văn bản gốc)**: task doc yêu cầu sửa `common/go.mod` để nâng
  `otelhttp` thành direct dependency. Kiểm tra thực tế cho thấy
  `common` package không hề import `otelhttp` trực tiếp ở đâu cả (grep
  xác nhận 0 kết quả) — chỉ `api-gateway` mới import trực tiếp. Ban đầu
  build thử không sửa go.mod nào cả vẫn PASS (Go workspace mode hợp nhất
  module graph của mọi thành viên `go.work`, nên `otelhttp` — vốn đã là
  indirect dependency của `common` — vẫn resolve được cho `api-gateway`
  dù không khai báo). Để đúng ngữ nghĩa Go module (module nào import trực
  tiếp thì module đó khai báo direct dependency, tránh phụ thuộc ngầm vào
  cách workspace mode hợp nhất graph — rủi ro nếu ai đó build
  `api-gateway` ngoài workspace), đã thêm dòng
  `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.69.0`
  vào `require` trực tiếp của `services/api-gateway/go.mod` thay vì
  `common/go.mod`, chạy `go mod edit -fmt` để sắp xếp lại. `go.sum` không
  đổi (checksum đã có sẵn từ trước).
- File test mới `cmd/server/otelhttp_wrap_test.go` (package `main` trong
  `cmd/server` trước đó chưa có test nào) — `run()` không test trực tiếp
  được (nhiều side-effect: dial 15 service, đọc config...), nên theo đúng
  gợi ý "test ở mức thấp hơn" của task doc: 2 test xác nhận
  `otelhttp.NewHandler` chỉ observe (status/body/header giống hệt gọi
  handler gốc trực tiếp) và không đổi hành vi 404 cho route không khớp.
- `go test ./cmd/... -v`: 2/2 PASS. `go test ./...` toàn bộ package
  `api-gateway` (bao gồm `httpgateway`, `wsbridge`, `wscompat`,
  `usecase`...): tất cả PASS, không regression.
- `gofmt -l`: sạch. `common/go.mod` build lại xác nhận không bị ảnh hưởng
  (vì cuối cùng task này không sửa file đó). Build lại toàn bộ 17 module:
  OK.
- Scope xác nhận qua `git status --porcelain`:
  `services/api-gateway/cmd/server/main.go` (M),
  `services/api-gateway/go.mod` (M, thay vì `common/go.mod` như task doc
  dự kiến — đã giải thích lý do ở trên), và
  `services/api-gateway/cmd/server/otelhttp_wrap_test.go` (mới). Không
  file nào khác bị đụng.
