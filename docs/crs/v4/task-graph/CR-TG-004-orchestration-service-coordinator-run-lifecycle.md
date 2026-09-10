# CR-TG-004 — Orchestration-Service Coordinator Run Lifecycle

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TG-004 |
| **Tên** | Thiết kế + build 6 RPC còn thiếu của `orchestration-service` + vòng lặp nền autonomous advance |
| **Loại** | Feature / Architecture (thiết kế mới — chưa có solution doc nào trước) |
| **Priority** | P0 — chặn CR-TG-005's `ComplexExecutor` thật |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | Không bắt buộc kỹ thuật, nhưng logic gate cho [CR-TG-005](./CR-TG-005-task-agent-execution-permission-and-complex-executor.md) |
| **Tham chiếu** | `specs/backend-go/tdd/services/orchestration-service.md` §3 (sơ đồ 11-RPC gốc), `specs/backend-go/bugs/task-v1/BUG-TASKV1-005-*.md` |
| **Tác động** | `backend-go/services/orchestration-service/` (proto, usecase mới, `cmd/server/main.go`), migration cho `orchestration.messages`/coordinator-run state nếu cần cột mới |

---

## 1. Vấn đề

Đây là **gap duy nhất trong toàn bộ audit F37 không có solution doc sẵn** —
`orchestration-service` chỉ implement 7/11 RPC theo sơ đồ TDD gốc
(`orchestration.proto:10-43`):

**Đã có:** `CreateDispatchContext`, `CreateGate`, `ResolveGate`,
`UpdateTaskStatusAndPromote`, `FailDispatch`, `GetDispatchContextForTask`,
`ListActiveDispatchContextsForUser`.

**Thiếu:** `StartCoordinatorRun`, `GetCoordinatorRun`, `CompleteCoordinatorRun`,
`FailCoordinatorRun`, `RecordHeartbeat`, `ListPendingDecisionGates`.

Nghiêm trọng hơn tên RPC thiếu: **service không có bất kỳ vòng lặp nền nào**
— `cmd/server/main.go:112-129` xác nhận chỉ chạy gRPC listener + HTTP health
check (`grep -n "time.Ticker\|time.NewTicker\|for {" ` = 0 kết quả). Bảng
`orchestration.messages` (`migrations/0001_init.up.sql:101-115`) tồn tại
nhưng hoàn toàn chết — không Go code nào đọc/ghi.

Hệ quả: dù `CreateDispatchContext`/`CreateGate`/`ResolveGate` hoạt động đúng
khi được gọi thủ công, **không có cơ chế nào tự động tiến các
`orchestration_tasks` qua pending→ready→dispatched→completed** — mọi
orchestration run cần 1 client bên ngoài liên tục poll và gọi RPC thủ công,
điều không ai đang làm trong production.

## 2. Giải pháp đề xuất (thiết kế mới, tham chiếu TDD §3)

### 2.1 6 RPC còn thiếu

```protobuf
// orchestration.proto — bổ sung
rpc StartCoordinatorRun(StartCoordinatorRunRequest) returns (StartCoordinatorRunResponse);
rpc GetCoordinatorRun(GetCoordinatorRunRequest) returns (GetCoordinatorRunResponse);
rpc CompleteCoordinatorRun(CompleteCoordinatorRunRequest) returns (google.protobuf.Empty);
rpc FailCoordinatorRun(FailCoordinatorRunRequest) returns (google.protobuf.Empty);
rpc RecordHeartbeat(RecordHeartbeatRequest) returns (google.protobuf.Empty);
rpc ListPendingDecisionGates(ListPendingDecisionGatesRequest) returns (ListPendingDecisionGatesResponse);

message StartCoordinatorRunRequest {
  string tenant_id = 1;
  string task_id = 2;       // task-service gọi vào đây khi ExecuteTask chọn nhánh complex
  string request_id = 3;    // idempotency key
}
message StartCoordinatorRunResponse {
  string run_id = 1;
}
```

`StartCoordinatorRun` tạo 1 `CoordinatorRun` row (`status='running'`), tạo
`DispatchContext` gốc cho root task, KHÔNG tự dispatch ngay — vòng lặp nền
(§2.2) mới là nơi thật sự dispatch.

### 2.2 Vòng lặp nền autonomous advance

```go
// cmd/server/main.go — thêm bên cạnh gRPC listener hiện có
func runCoordinatorScanLoop(ctx context.Context, svc *CoordinatorService, interval time.Duration) {
    ticker := time.NewTicker(interval) // gợi ý 5s, cần load-test trước khi chốt
    defer ticker.Stop()
    for {
        select {
        case &lt;-ctx.Done():
            return
        case &lt;-ticker.C:
            if err := svc.AdvancePendingRuns(ctx); err != nil {
                log.Error("coordinator scan tick failed", "err", err) // không panic — tick sau retry
            }
        }
    }
}
```

`AdvancePendingRuns`: với mỗi `CoordinatorRun` đang `running`, tìm
`DispatchContext` con đã `ready` (dependency đã `completed`) chưa
`dispatched`, gọi `task-service.ExecuteTask` (hoặc trực tiếp gRPC call tương
đương) cho từng subtask, cập nhật `orchestration.messages` làm audit trail.
Khi toàn bộ subtask `completed`/`failed` → gọi `CompleteCoordinatorRun`/
`FailCoordinatorRun`, rồi (nếu có `origin_task_id`, xem CR-FLOW-TASK-002) gọi
callback `task-service.ReportTaskExecutionResult`.

**Chống double-dispatch khi nhiều instance `orchestration-service` chạy
song song:** dùng `SELECT ... FOR UPDATE SKIP LOCKED` khi lấy batch
`DispatchContext` ready trong mỗi tick — không dựa vào giả định chỉ có 1
instance chạy.

### 2.3 `RecordHeartbeat` — phát hiện run bị treo

Task-service (hoặc chính worker dispatch) gọi định kỳ trong lúc 1 subtask
đang chạy; nếu 1 `CoordinatorRun` không nhận heartbeat quá N phút, scan loop
tự đánh dấu `FailCoordinatorRun` với lý do `HEARTBEAT_TIMEOUT` thay vì treo
vô thời hạn.

### 2.4 `ListPendingDecisionGates`

RPC đọc cho UI hiển thị "đang chờ quyết định" (gate cần approve thủ công) —
dùng bảng `orchestration_gates` đã có (`CreateGate`/`ResolveGate` real).

## 3. Rủi ro / Không thuộc phạm vi

- **Đây là thiết kế mới, chưa qua review kiến trúc** — khác các CR khác trong
  series (áp dụng SOL doc có sẵn), CR này cần 1 vòng review riêng về
  concurrency (multi-instance dispatch), interval polling (5s là gợi ý, không
  phải quyết định cuối), và cách chọn `SKIP LOCKED` vs message-queue thật
  (Postgres LISTEN/NOTIFY hoặc outbox-poll) trước khi implement.
- Không thiết kế lại 7 RPC đã có (`CreateDispatchContext` v.v.) — chỉ bổ sung
  phần thiếu.
- Không tự quyết định polling interval production cuối cùng — cần load-test
  với số lượng `CoordinatorRun` đồng thời thực tế trước khi chốt.
- Không thuộc phạm vi: `task-service.ComplexExecutor` gọi `StartCoordinatorRun`
  — đó là CR-TG-005, CR này chỉ cung cấp RPC để CR-TG-005 gọi vào.

## Acceptance Criteria

- [ ] 6 RPC còn thiếu implement đầy đủ, có test cho từng RPC.
- [ ] Vòng lặp nền chạy độc lập gRPC listener, không chặn service khởi động
      nếu tick đầu tiên lỗi.
- [ ] `AdvancePendingRuns` idempotent khi 2 instance `orchestration-service`
      cùng tick 1 lúc (test bằng cách chạy 2 goroutine giả lập 2 instance,
      xác nhận 1 `DispatchContext` chỉ dispatch đúng 1 lần).
- [ ] `RecordHeartbeat`/timeout detection hoạt động — test: 1 run không
      heartbeat quá ngưỡng tự chuyển `failed` với lý do `HEARTBEAT_TIMEOUT`.
- [ ] `orchestration.messages` không còn là dead schema — mỗi lần dispatch/
      complete/fail đều ghi 1 message audit trail.
- [ ] `ListPendingDecisionGates` trả đúng danh sách gate `unresolved`.
- [ ] `detect_changes()` xác nhận thay đổi không ảnh hưởng 7 RPC hiện có.
