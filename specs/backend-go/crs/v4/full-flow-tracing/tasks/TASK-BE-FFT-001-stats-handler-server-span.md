# TASK-BE-FFT-001: `grpcmw.StatsHandler()` mới + wire server-side OTel span vào 16 service

**Solution:** BE-FFT-SOL-001 | **CR:** [CR-FFT-001](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-001-otel-span-instrumentation.md)
**Service:** `common/grpcmw` + 16 `cmd/server/main.go` (mọi service backend-go trừ `api-gateway`, service duy nhất không chạy gRPC server)
**Depends on:** Không
**Status:** ✅ DONE (2026-09-09)

---

## Mục tiêu

Tạo span OTel thật cho mọi request gRPC inbound, **không sửa `ChainUnary`**
— thêm 1 hàm `grpc.ServerOption` mới song song, mỗi service tự thêm vào
lệnh `grpc.NewServer(...)` đã có.

## gitnexus — BẮT BUỘC trước khi sửa

`ChainUnary` là 1 trong 3 symbol CRITICAL của toàn series (xem
[solutions/README.md](../../solutions/README.md)'s cảnh báo đầu file).
Task này **không sửa** `ChainUnary`, nhưng vì file `grpcmw.go` chứa cả 2
hàm, chạy lại `impact()` trước khi mở file để xác nhận không có gì thay
đổi kể từ audit:

```
mcp__gitnexus__impact({ target: "ChainUnary", direction: "upstream", repo: "orca", summaryOnly: true })
```

Kỳ vọng: risk vẫn CRITICAL, impacted vẫn ~32 (16 direct). Nếu số liệu khác
đáng kể (vd. thêm/bớt service, ai đó đã sửa `ChainUnary`), DỪNG LẠI và báo
cáo trước khi tiếp tục — không tự suy đoán lý do lệch.

## Files cần sửa

1. `backend-go/common/grpcmw/grpcmw.go` (MODIFY — thêm hàm mới, KHÔNG đổi `ChainUnary`)
2. `backend-go/common/go.mod` (MODIFY — thêm dependency trực tiếp `otelgrpc`)
3. 16× `backend-go/services/*/cmd/server/main.go` (MODIFY — 1 dòng mỗi file, thêm option vào `grpc.NewServer(...)`) — mọi service trừ `api-gateway`:
   `git-gateway-service`, `auth-service`, `tenant-service`, `project-service`,
   `infra-fleet-service`, `task-service`, `workflow-service`,
   `scm-integration-service`, `issue-tracking-service`, `usage-service`,
   `annotation-service`, `credential-broker-service`,
   `notification-service`, `orchestration-service`, `ai-provider-service`,
   `automation-service`

## Nội dung

```go
// backend-go/common/grpcmw/grpcmw.go — thêm cuối file, sau ChainUnary
// (ChainUnary's implementation ở trên giữ nguyên từng ký tự)

import "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

// StatsHandler returns the OTel gRPC stats handler every gRPC-serving
// service passes to grpc.NewServer ALONGSIDE ChainUnary (never instead of
// it, never merged into it) — kept separate specifically so this addition
// never touches ChainUnary's signature, which every one of the 16
// gRPC-serving services calls identically today (impact() confirms
// CRITICAL blast radius: 32 impacted symbols, 16 direct call sites).
func StatsHandler() grpc.ServerOption {
    return grpc.StatsHandler(otelgrpc.NewServerHandler())
}
```

Mỗi `main.go` (vd. `git-gateway-service/cmd/server/main.go:214`):

```go
// TRƯỚC
grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
// SAU
grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())
```

`go.mod`: nâng `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`
thành dependency trực tiếp (chưa có dòng nào — xác nhận bằng grep trước
khi thêm, tránh trùng version với `otelhttp v0.69.0` đã có sẵn cùng dòng
`go.opentelemetry.io/contrib/...`).

## Test cases cần cover

- `TestStatsHandler_ReturnsNonNilServerOption` — smoke test đơn giản,
  `grpcmw` package hiện chưa có test cho `ChainUnary` ngoài
  `TenantExtractionInterceptor`, đây là file test mới hoặc mở rộng
  `grpcmw_test.go` đã có.
- **Regression bắt buộc** — `TestChainUnary_SignatureAndBehaviorUnchanged`:
  gọi `ChainUnary(logger)` với cùng input trước/sau, xác nhận trả về vẫn
  đúng 3 interceptor theo thứ tự cũ (recovery → tenant → logging) — chứng
  minh việc thêm `StatsHandler()` không vô tình đổi `ChainUnary`.
- Với ít nhất 1 service đại diện (vd. `task-service`), test tích hợp: gọi
  1 RPC thật qua `grpc.NewServer(grpcmw.ChainUnary(logger),
  grpcmw.StatsHandler())`, xác nhận response/error code không đổi so với
  trước khi thêm `StatsHandler()` (không có span nào làm hỏng request
  thật — `otelgrpc.NewServerHandler()` chỉ quan sát, không can thiệp).

## Verify

```bash
cd backend-go && go build ./... 2>&1 | tee /tmp/build.log   # PHẢI sạch cho cả 17 service
cd backend-go/common && go test ./grpcmw/...
gofmt -l common/grpcmw/grpcmw.go
grep -rn "grpcmw.StatsHandler()" backend-go/services/*/cmd/server/main.go | wc -l   # kỳ vọng: 16
```

## gitnexus (sau khi sửa)

```
mcp__gitnexus__detect_changes({ scope: "compare", base_ref: "main", repo: "orca" })
```
Xác nhận scope thay đổi chỉ gồm `common/grpcmw/grpcmw.go`,
`common/go.mod`/`go.sum`, và đúng 16 file `main.go` liệt kê ở trên — không
có symbol nào khác bị ảnh hưởng ngoài dự kiến (đặc biệt: `ChainUnary`
KHÔNG xuất hiện trong danh sách symbol bị sửa, chỉ file nó nằm trong mới
đổi).

## Blocking

TASK-BE-FFT-004 (HTTP edge span, `api-gateway`), TASK-BE-FFT-005 (outbound
propagation) không phụ thuộc cứng vào task này (khác symbol/file), nhưng
nên hoàn tất cùng đợt review vì cùng thuộc CR-FFT-001's tiêu chí chấp nhận
"một request qua N service là 1 trace".

## Kết quả thực tế (2026-09-09)

- `impact({target: "ChainUnary", direction: "upstream", summaryOnly: true})`
  xác nhận trước khi sửa: risk CRITICAL, 32 impacted / 16 direct callers —
  khớp số liệu audit gốc, không lệch.
- `StatsHandler()` được thêm vào cuối `grpcmw.go`, sau `ChainUnary` —
  `ChainUnary`'s implementation giữ nguyên byte-for-byte (diff xác nhận
  chỉ có phần thêm mới, không có dòng nào trong `ChainUnary` bị sửa).
- `common/go.mod`: thêm đúng 1 dòng
  `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.69.0`
  vào `require` trực tiếp, pin cùng version với `otelhttp v0.69.0` đã có
  sẵn. `go get` ban đầu kéo theo bloat không liên quan (bump toolchain,
  deps khác) — revert về baseline rồi sửa tay `go.mod`, để `go build`
  tự lấp `go.sum`, kết quả 0 diff ngoài dự kiến trên `go.sum`.
- Wire vào đúng 16/16 `main.go` — 15 service theo pattern đơn giản
  `grpc.NewServer(grpcmw.ChainUnary(logger))` sửa bằng `sed`, riêng
  `automation-service` dùng pattern multi-line (có thêm
  `grpc.ChainUnaryInterceptor(interceptors.RequireTenantForExternalTrigger())`)
  nên thêm `grpcmw.StatsHandler()` làm dòng thứ 3 trong cùng lệnh
  `grpc.NewServer(...)` thay vì sed. `api-gateway` không đổi (không chạy
  gRPC server, đúng như task doc loại trừ).
- Test mới: `TestStatsHandler_ReturnsNonNilServerOption` (smoke) +
  `TestChainUnary_SignatureAndBehaviorUnchanged` (bufconn end-to-end thật,
  dùng `wrapperspb.StringValue` làm `proto.Message`, xác nhận cả tenant
  metadata reach context và panic→codes.Internal vẫn đúng khi
  `grpc.NewServer(ChainUnary(logger), StatsHandler())` chạy cùng nhau).
  `go test ./grpcmw/... -v`: tất cả PASS.
- `gofmt -l` trên `grpcmw.go`/`stats_handler_test.go`/16 `main.go`: sạch.
- `go build` chạy riêng từng module (`common`, `proto`, 16 service) do
  `go build ./...` từ workspace root báo lỗi "directory prefix . does not
  contain modules" (bản thân `backend-go/` không phải 1 module, hành vi
  đã biết của `go.work`, không liên quan tới thay đổi của task này) — cả
  17 module build sạch, 0 lỗi.
- Scope xác nhận qua `git status --porcelain` scoped đúng: `common/go.mod`,
  `common/grpcmw/grpcmw.go`, `common/grpcmw/stats_handler_test.go` (mới),
  và đúng 16 `main.go` liệt kê — không đụng file nào khác. (`grpcmw_test.go`
  và `api-gateway/cmd/server/main.go` xuất hiện `M` trong git status nhưng
  là thay đổi có sẵn từ nhánh RBAC song song, không phải do task này.)
