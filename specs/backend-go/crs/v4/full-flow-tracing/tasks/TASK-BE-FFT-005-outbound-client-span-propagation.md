# TASK-BE-FFT-005: Outbound gRPC client span propagation — mỗi service tự sửa `Dial` riêng

**Solution:** BE-FFT-SOL-001 | **CR:** [CR-FFT-001](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-001-otel-span-instrumentation.md)
**Service:** Mọi service có gọi service khác qua gRPC (per-service `Dial`, không có package chung)
**Depends on:** TASK-BE-FFT-002 (mềm)
**Status:** ✅ DONE (2026-09-09)

---

## Mục tiêu

Một request đi qua `api-gateway` → N service phải là **1 trace, không phải
N log rời** — span downstream phải là child span cùng trace ID. Điều này
đòi hỏi outbound gRPC client dial cũng gắn OTel stats handler để inject
`traceparent` vào metadata khi gọi service khác.

## Phát hiện quan trọng khi viết solution — đọc kỹ trước khi bắt đầu

**Không có 1 package `common/grpcclient` dùng chung** như bảng "Changes
Required" gốc của CR-FFT-001 gợi ý. Mỗi service tự định nghĩa hàm `Dial`
cục bộ trong package riêng của nó. Đã xác nhận trực tiếp bằng đọc code (2026-09-09):

| Service | File | Hàm |
|---|---|---|
| `api-gateway` | `internal/adapter/grpc/dial.go` | `func Dial(addr string) (*grpc.ClientConn, error)` |
| `git-gateway-service` | `internal/adapter/grpcclient/resolver.go` | `func Dial(addr string) (*grpc.ClientConn, error)` |
| `task-service` | `internal/adapter/grpcclient/dial.go` | `func Dial(addr string) (*grpc.ClientConn, error)` |

Các service khác có thư mục `internal/adapter/grpcclient/` (`auth-service`,
`ai-provider-service`, `project-service`, `automation-service`) — **task
này phải tự audit** từng package đó khi implement (tìm hàm `Dial`/tương
đương `grpc.NewClient` call site), không giả định đã biết đủ 17/17. Vì
đây là N thay đổi độc lập trên N file khác nhau (không phải 1 symbol dùng
chung), **không CRITICAL** — mỗi thay đổi review độc lập, rollback độc lập.

## gitnexus

Không có symbol CRITICAL nào bị sửa ở task này (mỗi `Dial` là 1 hàm riêng
của từng service, blast radius chỉ trong service đó). Khuyến khích chạy
`mcp__gitnexus__query({search_query: "grpc dial outbound client connection", repo: "orca"})`
trước khi bắt đầu để liệt kê đầy đủ hơn các nơi có `grpc.NewClient` — dùng
kết quả này để bổ sung bảng trên, không tự bịa thêm service nếu query
không xác nhận.

## Files cần sửa (audit lại đầy đủ trước khi implement — danh sách dưới là baseline đã xác nhận, chưa chắc đủ)

1. `backend-go/services/api-gateway/internal/adapter/grpc/dial.go`
2. `backend-go/services/git-gateway-service/internal/adapter/grpcclient/resolver.go`
3. `backend-go/services/task-service/internal/adapter/grpcclient/dial.go`
4. Mọi `Dial`/`grpc.NewClient` khác tìm thấy qua audit (ghi rõ trong PR description danh sách đầy đủ đã tìm được — không để lại "TODO audit sau" trong code).

## Nội dung (áp dụng theo đúng khuôn cho mỗi file)

```go
// vd. backend-go/services/task-service/internal/adapter/grpcclient/dial.go
import "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

func Dial(addr string) (*grpc.ClientConn, error) {
    conn, err := grpc.NewClient(addr,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithStatsHandler(otelgrpc.NewClientHandler()), // NEW
    )
    if err != nil {
        return nil, fmt.Errorf("grpcclient: dial %q: %w", addr, err)
    }
    return conn, nil
}
```

Với `api-gateway/internal/adapter/grpc/dial.go`, cùng pattern nhưng giữ
nguyên comment gốc về mTLS gap (không thuộc phạm vi task này).

## Test cases cần cover

- Với ít nhất 1 `Dial` đại diện (vd. `task-service`): test tích hợp gọi 1
  RPC thật qua connection đã dial với `otelgrpc.NewClientHandler()`, xác
  nhận downstream service (đã có `StatsHandler()` server-side từ
  TASK-BE-FFT-001) nhận được span với **cùng TraceID** với span client gửi
  đi — đây là bằng chứng thực nghiệm cho tiêu chí chấp nhận CR-FFT-001 "span
  downstream là child span cùng trace ID".
- **Regression bắt buộc** — mỗi `Dial` sửa xong vẫn dial thành công tới 1
  target giả (httptest/bufconn hoặc tương đương), không đổi behavior khi
  target unreachable (giữ đúng "grpc.NewClient không block/error khi
  target chưa sẵn sàng" đã ghi trong comment gốc của các file này).

## Verify

```bash
cd backend-go && go build ./...   # PHẢI sạch cho mọi service đã sửa
# với mỗi service đã sửa:
cd backend-go/services/<service> && go test ./...
gofmt -l $(git diff --name-only | grep '\.go$')
```

## gitnexus (sau khi sửa)

```
mcp__gitnexus__detect_changes({ scope: "compare", base_ref: "main", repo: "orca" })
```
Xác nhận scope đúng bằng danh sách file đã audit + sửa, không có file
ngoài dự kiến.

## Blocking

Không có task nào phụ thuộc cứng vào task này. Nên hoàn tất cùng đợt với
TASK-BE-FFT-001/002/004 trước khi coi BE-FFT-SOL-001 DONE, vì đây là phần
còn thiếu của tiêu chí chấp nhận "1 trace xuyên N service", nhưng có thể
trễ hơn nếu effort audit 17/17 service lớn hơn dự kiến — ghi rõ trạng thái
PARTIAL (bao nhiêu/bao nhiêu service đã xong) nếu không kịp làm hết trong
1 lần.

## Kết quả thực tế (2026-09-09)

- Audit đầy đủ bằng `grep -rln "grpc.NewClient(" services/*/internal
  services/*/cmd --include="*.go"` (không chỉ 3 file baseline task doc
  liệt kê) — tìm ra **20 call site**, nhiều hơn hẳn 3 file baseline:
  - 11 file trong `internal/adapter/{grpcclient,grpc,scmstarcheck,infrafleetclient}/`:
    `api-gateway`, `auth-service` (×2: `tenant_resolver.go`,
    `tenant_provisioner.go`), `git-gateway-service`, `project-service`
    (×4: `workflow_execution_checker.go`, `task_execution_checker.go`,
    `infra_fleet_dev_server_lister.go`, `dev_server_relay.go`),
    `tenant-service` (`scmstarcheck/grpc_adapter.go`), `task-service`,
    `workflow-service` (`infrafleetclient/relay_client.go`).
  - 9 `main.go` composition roots dial trực tiếp (không qua helper
    `Dial()`): `annotation-service`, `ai-provider-service` (×2 dial:
    `brokerConn` + `infraFleetConn`), `automation-service`,
    `project-service`, `task-service`, `scm-integration-service`,
    `infra-fleet-service`, `notification-service`,
    `issue-tracking-service`.
  - Tổng: 20 call site trên 19 file, tất cả cùng 1 pattern hệt nhau
    `grpc.NewClient(<addr>, grpc.WithTransportCredentials(insecure.NewCredentials()))`
    — xác nhận bằng đếm occurrence khớp với số call site tìm được trước
    khi sửa.
- Sửa cả 20/20: thêm `grpc.WithStatsHandler(otelgrpc.NewClientHandler())`
  làm option thứ 3, giữ nguyên comment/logic khác. Không có "TODO audit
  sau" nào còn sót — danh sách đầy đủ đã liệt kê ở trên.
- **Bổ sung ngoài kế hoạch task doc gốc**: task doc chỉ liệt kê sửa code,
  không nhắc go.mod. Theo đúng nguyên tắc đã áp dụng ở TASK-BE-FFT-004
  (module nào import trực tiếp thì module đó khai báo direct dependency,
  không dựa ngầm vào Go workspace mode hợp nhất module graph), đã thêm
  `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.69.0`
  vào `require` trực tiếp của cả 14 `go.mod` service liên quan (mỗi
  service chỉ có 1 dòng thêm, dùng `go mod edit -fmt` để sắp xếp lại) —
  không service nào cần thay đổi `go.sum` (checksum đã cache từ
  TASK-BE-FFT-001).
- Test mới `task-service/internal/adapter/grpcclient/dial_test.go` (test
  đại diện theo đúng yêu cầu task doc "ít nhất 1 Dial đại diện"):
  - `TestDial_TraceIDPropagatesToServer` — **bằng chứng thực nghiệm bắt
    buộc**: dựng 1 gRPC server thật (TCP thật trên `127.0.0.1:0`, không
    phải bufconn — vì `Dial(addr string)` chỉ nhận address, không nhận
    dialer tuỳ chỉnh) với `grpcmw.ChainUnary(logger) +
    grpcmw.StatsHandler()` (TASK-BE-FFT-001), client dial qua chính hàm
    `Dial()` thật của package (không mock), tạo 1 span thật qua
    `sdktrace.NewTracerProvider()`, gọi RPC — server đọc
    `trace.SpanContextFromContext(ctx).TraceID()` và trả về, test so
    khớp đúng bằng TraceID phía client. PASS — chứng minh
    `otelgrpc.NewClientHandler()` (client) + `otelgrpc.NewServerHandler()`
    (server, qua `StatsHandler()`) nối đúng 1 trace qua 1 hop gRPC thật.
  - `TestDial_UnreachableTargetDoesNotBlockOrError` — regression bắt
    buộc: dial tới `127.0.0.1:1` (unreachable) vẫn không lỗi/không block,
    đúng hành vi lazy-dial gốc.
- `go test ./internal/adapter/grpcclient/... -run TestDial -v`: 2/2 PASS.
  `go test ./...` toàn bộ `task-service` sau khi thêm global OTel state
  trong test: vẫn PASS hết, không leak sang test khác trong cùng package.
- Chạy `go test ./...` cho **cả 14 service đã sửa**: tất cả PASS, không
  regression.
- `gofmt -l`: sạch trên toàn bộ file đã sửa (1 file —
  `notification-service/cmd/server/main.go` — cần `gofmt -w` để sắp lại
  thứ tự import, đã sửa).
- Build lại toàn bộ 17 module (`common`, `proto`, 15 service khác không
  đụng tới trong task này): tất cả OK, 0 lỗi.
- Scope xác nhận qua `git status --porcelain`: đúng 20 file `.go` sửa +
  14 file `go.mod` sửa (thêm 1 dòng mỗi file) + 1 file test mới — không
  file nào khác bị đụng (`usage-service/go.mod` xuất hiện `M` trong git
  status nhưng là thay đổi có sẵn từ công việc F26/F31 trước đó trong
  cùng session, không phải do task này).
