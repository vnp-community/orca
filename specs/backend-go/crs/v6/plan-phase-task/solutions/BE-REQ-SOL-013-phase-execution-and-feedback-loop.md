# BE-REQ-SOL-013: Thực thi theo Phase, sự kiện task, `ReportTaskOutcome` và phản hồi ngược lên Request

> **✅ Đã triển khai (kiểm chứng 2026-10-08, 7/7 task).** Phần `request-service` chạy với task-service giả qua bufconn; chưa chạy với task-service thật (xem IMPLEMENTATION-NOTES).

**CR:** [CR-REQ-013](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-013-phase-execution-and-feedback-loop.md)
**Service:** `task-service` · `request-service` (mới) · `proto/orca/request/v1`, `proto/orca/task/v1`
**TDD tham chiếu:** [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (outbox cùng transaction, consumer idempotent), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (JetStream, subject, durable consumer), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (retry, đối soát), [`services/task-service.md`](../../../../tdd/services/task-service.md), [`services/orchestration-service.md`](../../../../tdd/services/orchestration-service.md) (Engine 2, callback), [`services/project-service.md`](../../../../tdd/services/project-service.md) (worktree)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-06: `task-service/internal/usecase/execute_task.go` (claim dòng 229 đến 236, hoàn tác dòng 250, 262, 316, 376, `dispatchDirectAgentAsync` dòng 335 đến 395), `report_execution_result.go` (toàn file), `execution_lease.go` (`TaskExecutionClaimer` dòng 238, `TaskExecutionReleaser` dòng 113, `RecoverInterruptedExecutions` dòng 141 đến 240), `update_task.go` (payload dòng 181, un-block dòng 131 đến 160), `adapter/postgres/execution_leases.go` (`ClaimForExecution` dòng 77, `ReleaseExecution` dòng 163), `adapter/mysql/execution_leases.go`, `adapter/eventbus/consumer.go`, `common/eventbus/eventbus.go` (`Subscribe(ctx, streamName, consumerName, subject, fn)` dòng 128, `PublishDedup`), `cmd/server/main.go` (stream `TASK`, dòng ~409), proto `TaskServiceExecuteRequest` (`request_id = 2` đã có), `ReportTaskExecutionResultRequest`.

Khớp với CR gốc:
- Chỉ `UpdateTask` ghi outbox. `ClaimForExecution` (cổng riêng `TaskExecutionClaimer`), `CompleteExecution` (trên `TaskRepository`, `ports.go` dòng 60), `ReleaseExecution` (`TaskExecutionReleaser`) và các `UpdateStatus` hoàn tác không ghi outbox.
- Engine 1 (`direct_agent`) chạy trong goroutine (`dispatchDirectAgentAsync`), lỗi dispatch chỉ vào log (`dispatch_error`); Engine 2/3 báo xong qua `ReportTaskExecutionResult`, và ghi chú trong code nói callback chưa kiểm tra danh tính service gọi.
- Task xong chỉ đến `review` (`StatusReview`); chỉ `UpdateTask(done)` mới mở khoá dependent (`update_task.go` khối un-block).
- Chưa có consumer của `orca.task.task.statuschanged` ở service nào ngoài `api-gateway` (ephemeral) và `notification-service`.

### Correction relative to CR-REQ-013

1. CR viết `ReleaseExecution` và `ClaimForExecution` "ở `usecase/ports.go`": thực tế hai cổng nằm ở `execution_lease.go` (`TaskExecutionClaimer`, `TaskExecutionReleaser`). Thêm tham số `events` ở đó; đồng thời đổi mọi fake (`fakes_test.go`, `execution_lease_test.go`, `adapter/grpc/server_test.go`) và hai adapter.
2. `task-service` đã có consumer bền (`adapter/eventbus/consumer.go`, `Subscribe` với `consumerName`); `request-service` dùng cùng khuôn với `consumerName="request-service-task-outcome"` (tên giả định) và `Event.ID` để khử trùng qua `processed_events`.
3. README v6 mục 8 điều 8: task xong chỉ ở `review`, cờ `REQUEST_AUTO_COMPLETE_TASKS` quyết định tự đặt `done`, Request ở lại `executing` khi chờ Phase kế. Solution theo đó.
4. Migration `request-service` kế tiếp **chưa có số cố định**: các CR khác cùng dùng `0002` (`request_core`, `processed_events`, `request_sync_state`) và `0005_outbox`. Task migration dùng quy tắc "số lớn nhất hiện có + 1" lúc tạo file.

## 2. Giải pháp

### 2.1 Cây file

```
task-service/internal/
  usecase/execution_lease.go                (sửa)  events []OutboxEvent cho Claim/Release
  usecase/ports.go                          (sửa)  CompleteExecution(..., events)
  usecase/task_run_events.go                (mới)  dựng payload statuschanged theo cause
  usecase/{execute_task,report_execution_result}.go (sửa)
  adapter/{postgres,mysql}/execution_leases.go, repository.go (sửa)
request-service/
  migrations/{postgres,mysql}/NNNN_phase_execution.{up,down}.sql   (mới)  phase_starts, task_run_outcomes
  internal/domain/{phase_start,task_run_outcome,task_outcome_class}.go (mới)
  internal/usecase/start_phase.go, advance_execution.go, start_execution.go,
                  report_task_outcome.go, reconcile_executing_requests.go,
                  phase_subject_handler.go, execution_guard.go                (mới)
  internal/adapter/grpcclient/task_client.go                                  (mới, dùng chung SOL-015)
  internal/adapter/eventbus/task_outcome_consumer.go, request_status_consumer.go (mới)
  internal/adapter/{postgres,mysql}/{phase_starts,task_run_outcomes}.go       (mới)
```

### 2.2 `task-service`: sự kiện đầy đủ cho task có `request_id`

`taskStatusChangedPayload` (SOL-011) thêm `execution_link_id`, `engine`, `error_message` (`omitempty`). Giá trị `cause`: `execute_claim`, `execution_completed`, `execution_failed`, `recovery` (cộng `user_update`, `derived` của SOL-011). Chỉ phát khi `task.RequestID != ""`.

```go
// ports: thêm tham số events, ghi cùng transaction với UPDATE (mẫu Repository.Update)
ClaimForExecution(ctx, tenantID, taskID string, from domain.Status, events []domain.OutboxEvent) (bool, error)
CompleteExecution(ctx, tenantID, id, status string, actualHours float64, events []domain.OutboxEvent) error
ReleaseExecution(ctx, tenantID, taskID, linkID string, to domain.Status, events []domain.OutboxEvent) (bool, error)
```

Các nhánh `UpdateStatus(previousStatus)` hoàn tác trong `ExecuteTask` đổi sang `ReleaseExecution`-kiểu có sự kiện `execution_failed` kèm `error_message` (cắt tối đa 1 KB). `ReleaseUnlinkedInProgress` là update hàng loạt không trả id nên không phát; bù bằng đối soát (2.8). Task không có `request_id` không thêm dòng outbox nào.

### 2.3 `StartPhase`

```proto
rpc StartPhase(StartPhaseRequest) returns (StartPhaseResponse);
message StartPhaseRequest { string request_id = 1; string phase_task_id = 2; } // rỗng: Plan không Phase
message StartPhaseResponse { string phase_task_id = 1; bool already_started = 2; repeated string dispatched_task_ids = 3; }
```

Bước: `RequireTenantID`; Request `executing` (`REQUEST_NOT_EXECUTING`); Phase thuộc Plan của Request (`ListTasks task_types=[phase], request_ids=[id]`, `REQUEST_PHASE_NOT_IN_PLAN`); nếu `FlowFor(type).ExecutionGates` có `phase` thì cần Approval `phase` `approved` (`REQUEST_PHASE_NOT_APPROVED`); mọi Phase mà nó `depends_on` phải `done/cancelled` (`REQUEST_PHASE_PREDECESSOR_NOT_DONE`, kiểm bằng `GetSubtree`); claim idempotent bằng `INSERT` vào `phase_starts` (Postgres `ON CONFLICT DO NOTHING`, MySQL `INSERT IGNORE`), chèn được thì outbox `orca.request.phase.started`, không thì `already_started=true`; dù thế nào cũng chạy `AdvanceExecution` (chạy lại an toàn). Plan không Phase, `task_list`, `hotfix` không cần người bấm: consumer `request_status_consumer` thấy `status_changed` với `to=executing` thì gọi `StartExecution` (cùng `AdvanceExecution`).

### 2.4 `AdvanceExecution` (idempotent)

1. Liệt kê task lá của container (`ListTasks parent_id=...`).
2. Chọn `status = open` (task `blocked` bỏ qua), trừ số `in_progress` khỏi `REQUEST_MAX_PARALLEL_TASKS` (mặc định 1).
3. Hỏi `TypePolicy.PreExecutionGate(task)` (SOL-014; mặc định cho qua); cổng chưa duyệt thì `OpenApproval` rồi dừng task đó, không báo lỗi.
4. Worktree dùng chung: task đầu của Plan chạy trước; có `worktree_id` thì `UpdateTask(worktree_id)` cho các task còn lại.
5. `task-service.Execute(task_id, request_id="req:<request_id>:<task_id>:<attempt>")`; `TASK_EXECUTE_ALREADY_IN_PROGRESS` coi là thành công; `TASK_EXECUTE_NO_CONNECTION`, `TASK_EXECUTE_WORKTREE_FAILED`, `TASK_EXECUTE_FAILED` không tính vào số lần thử, thử lại ở đối soát trong `REQUEST_DISPATCH_RETRY_WINDOW` (15 phút) rồi trả backlog.
6. Danh tính: người duyệt Phase (hoặc Plan khi không Phase); thiếu quyền `execute` thì `REQUEST_EXECUTE_FORBIDDEN`, không đổi danh tính. Cơ chế danh tính service-to-service chưa kiểm chứng.

### 2.5 `ReportTaskOutcome`

Consumer bền `orca.task.task.statuschanged` trên stream `TASK`, bỏ bản ghi `request_id` rỗng, đặt tenant từ `Event.TenantID`, gọi use case; RPC nội bộ `ReportTaskOutcome` là vỏ mỏng cho đối soát và test. Use case: một `TxRunner.InTx` - (1) khử trùng `processed_events(event_id)`; (2) đọc Request, không có thì log và bỏ; Request không còn `executing` thì chỉ ghi `task_run_outcomes`; (3) phân loại và ghi:

| `task_type`, `cause`, `new_status` | Kết quả | Hành động |
|---|---|---|
| lá, `execute_claim`, `in_progress` | `started` | chỉ ghi |
| lá, `execution_completed`, `review` | `succeeded` | bật `REQUEST_AUTO_COMPLETE_TASKS` (mặc định) thì `UpdateTask(done)`; tắt thì chờ người |
| lá, `user_update`, `done` | `succeeded` | `AdvanceExecution(parent)` |
| lá, `execution_failed` hoặc `recovery` | `failed` | `< REQUEST_MAX_TASK_ATTEMPTS` (2) thì chạy lại; ngược lại trả backlog |
| lá, `cancelled` | `cancelled` | mọi con của container huỷ thì trả backlog |
| `phase`, `derived`, `done` | `phase_done` | `orca.request.phase.completed`; còn Phase thì `OpenApproval` `phase` kế; hết thì bước 4 |
| `plan`, `derived`, `done` | `plan_done` | bước 4 |

(4) hoàn tất: `TypePolicy.CompletionChecks`; `failed` thì trả backlog stage `task`; đạt thì `TransitionRequest(execution_finished, ExpectedFrom=executing)`, outbox `request.completed`, rồi `TypePolicy.OnCompleted`. Solution (`solutions`) giữ `approved`, không ghi thêm (CR Q2).

### 2.6 Trả backlog khi lỗi

Chỉ gọi `ReturnToBacklog` (CR-REQ-006), `actor_kind=system`: task hết lần thử (`stage=task`, `category=other`, lý do có `error_message` cuối); không dispatch được quá cửa sổ (`blocked_dependency`, mã lỗi); Phase bị từ chối (`phase`, `rejected`); mọi task huỷ (`other`); kiểm hoàn tất `failed` (`infeasible` khi đo, `other` khi test). Nếu `ExecutionGuard` chặn (`REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION`) thì thử lại ở đối soát. Không có RPC dừng run: task đang chạy vẫn chạy tới cùng, kết quả chỉ được ghi.

### 2.7 Worktree dùng chung, song song

Mỗi task mặc định có worktree riêng (`EnsureWorktree`, nhánh `task/<id>`); task sau không thấy code task trước. Quyết định: cả Plan dùng một worktree, tuần tự (`REQUEST_MAX_PARALLEL_TASKS=1`). Chưa kiểm chứng `project-service` chấp nhận một worktree gắn nhiều task.

### 2.8 Đối soát và dữ liệu

`ReconcileExecutingRequests` (chu kỳ `REQUEST_RECONCILE_INTERVAL=60s`, khoá `FOR UPDATE SKIP LOCKED` hoặc `GET_LOCK` MySQL ≥ 8.0.1): với Request `executing` không có sự kiện trong 5 phút, liệt kê task và chạy lại bước 3 của 2.5. Bảng: `phase_starts(tenant_id, phase_task_id, request_id, started_by, started_at)` PK `(tenant_id, phase_task_id)`; `task_run_outcomes(id, tenant_id, request_id, task_id, container_id NULL, outcome CHECK, cause, execution_link_id NULL, error_message, event_id, occurred_at)` với chỉ mục `(tenant_id, request_id, occurred_at)` và `(tenant_id, task_id, outcome)`. Không FK.

Cấu hình: `REQUEST_MAX_PARALLEL_TASKS=1`, `REQUEST_MAX_TASK_ATTEMPTS=2`, `REQUEST_AUTO_COMPLETE_TASKS=true`, `REQUEST_DISPATCH_RETRY_WINDOW=15m`, `REQUEST_RECONCILE_INTERVAL=60s` (mặc định là đề xuất). Lỗi: `REQUEST_NOT_EXECUTING`, `REQUEST_PHASE_NOT_IN_PLAN`, `REQUEST_PHASE_NOT_APPROVED`, `REQUEST_PHASE_PREDECESSOR_NOT_DONE`, `REQUEST_EXECUTE_FORBIDDEN` (PermissionDenied).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Chạy task lá, không chạy Phase | SOL-011 chặn `ExecuteTask` trên container |
| Sự kiện cùng transaction, chỉ cho task có `request_id` | Mất sự kiện lỗi làm Request kẹt `executing`; tránh nhiễu task thường |
| Dùng lại `statuschanged`, thêm `cause` | Một kênh; gateway và notification đã nhận subject này |
| Đổi chữ ký cổng thay vì thêm hàm song song | Một đường ghi duy nhất, không hai đường lệch nhau |
| Một worktree, tuần tự | Task sau cần thấy code trước; chưa có cơ chế merge nhánh |
| `phase_starts` làm khoá idempotency | Bấm hai lần không chạy hai lần |
| Đối soát định kỳ | Sự kiện at-least-once nhưng vẫn mất ở vòng quét hàng loạt |

## 4. Phụ thuộc và thứ tự

Nửa task-service (task 01, 02) cần SOL-011 task 05, 06 (cùng điểm gọi `SyncContainerStatus`); nửa request-service (task 03 đến 07) cần SOL-012, CR-REQ-003, 006, 009 và `processed_events`, outbox của CR-REQ-001. Thứ tự: 01, 02 song song với SOL-012; 03; 04; 05 và 06 song song sau 04; 07 cuối. Mở khoá SOL-014, 015.

## 5. Kiểm thử

- **Unit:** bảng phân loại 2.5 table-driven; `AdvanceExecution` với fake `TaskClient` (chọn sẵn sàng, song song, `ALREADY_IN_PROGRESS`, hết cửa sổ); `StartPhase` từng điều kiện; đếm lần lỗi.
- **Integration hai dialect:** ghi status và outbox cùng transaction (chèn lỗi outbox thì status không đổi); `phase_starts` đua 8 goroutine; `processed_events` giao lặp; consumer đọc NATS thật; đối soát với sự kiện bị bỏ.
- **E2E:** `change_request` từ Plan duyệt tới `completed`; một task lỗi cố ý đến backlog. Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/task-service/... ./services/request-service/...` và bản `-tags=integration`.

## 6. Rủi ro và điểm chưa kiểm chứng

- Worktree dùng chung là giả định lớn nhất; sai thì phải đổi mô hình (một task lớn mỗi Phase hoặc merge nhánh).
- `ReportTaskExecutionResult` và RPC nội bộ `ReportTaskOutcome` chưa kiểm tra danh tính service gọi.
- Tự đặt `done` bỏ qua review tay của task; `UpdateTask(done)` phát `orca.task.task.completed` hàng loạt gây thông báo đẩy: cần quyết định gắn cờ bỏ thông báo.
- Engine 2/3 báo xong bất đồng bộ sau khi link bị đánh dấu `completed` lúc dispatch (CR-TG-008); callback đầu cuối chưa thử.
- Đổi chữ ký ba cổng chạm nhiều fake và hai adapter; cần `gitnexus_impact` trước khi sửa.
- Không có RPC dừng run; huỷ Request không dừng agent.

## 7. Câu hỏi mở

- Request ở lại `executing` trong lúc chờ duyệt Phase kế (README v6 mục 8 điều 8): xác nhận với CR-REQ-003.
- `REQUEST_AUTO_COMPLETE_TASKS` mặc định bật hay tắt?
- Ánh xạ `category` ở 2.6 cần CR-REQ-006 xác nhận ý nghĩa.
- Tham chiếu tiến: CR bổ sung về ExecutionResult và `Failure.class` (trong dải 026 đến 036, số chưa chốt) sẽ thêm phân loại lỗi có cấu trúc vào kết quả chạy, nên `error_message` ở đây có thể được thay bằng `failure_class`; CR-REQ-029 (ReadinessGate) có thể chặn dispatch khi task chưa sẵn sàng. Solution này chỉ để chỗ nối: cột `error_message` và `cause`, không dựng phân loại.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.3, 3.4, 3.7, 8 (điều 3, 4, 8, 9)
- `/opt/repos/orca/docs/crs/v6/plan-phase-task/CR-REQ-011-task-service-plan-phase-task-types.md`, `CR-REQ-012-plan-phase-task-generation-from-solution.md`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/execute_task.go`, `report_execution_result.go`, `execution_lease.go`, `update_task.go`, `ports.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/postgres/execution_leases.go`, `repository.go`; `adapter/mysql/execution_leases.go`, `repository.go`; `adapter/eventbus/consumer.go`
- `/opt/repos/orca/backend-go/common/eventbus/eventbus.go`
- `/opt/repos/orca/backend-go/proto/orca/task/v1/task.proto`
- `/opt/repos/orca/docs/crs/v4/task-graph/CR-TG-008-jira-source-link-and-durable-direct-agent.md`
- `/opt/repos/orca/specs/backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-011-task-service-plan-phase-task-types.md`, `BE-REQ-SOL-012-plan-phase-task-generation-from-solution.md`
