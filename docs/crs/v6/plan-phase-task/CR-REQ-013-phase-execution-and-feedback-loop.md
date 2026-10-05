# CR-REQ-013 — Thực thi theo Phase và phản hồi ngược lên Plan, Solution, Request

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-013 |
| **Tên** | `StartPhase`, điều phối task lá, consumer `ReportTaskOutcome`, cập nhật Phase/Plan/Request, trả Request backlog khi lỗi |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-011 (container, `request_id`, sự kiện `statuschanged`), CR-REQ-012 (cây Plan), CR-REQ-009 (Approval `phase`), CR-REQ-003 (trigger `plan_approved`, `execution_finished`), CR-REQ-006 (`ReturnToBacklog`), CR-REQ-001 (`processed_events`, outbox) |
| **Mở khoá** | CR-REQ-014, CR-REQ-015, CR-REQ-021 |
| **Tác động** | `backend-go/services/request-service` (usecase, consumer, repository, migration, proto); `backend-go/services/task-service` (usecase `execute_task.go`, `report_execution_result.go`, `execution_lease.go`, adapter `execution_leases.go`, `repository.go` hai dialect) |

## 1. Bối cảnh và vấn đề

Đọc code `task-service` ngày 2026-10-05, các sự thật chi phối thiết kế:

- Chỉ `UpdateTask` phát sự kiện outbox (`orca.task.task.statuschanged`, `orca.task.task.completed`) và chỉ khi người dùng đổi status. `ClaimForExecution`, `CompleteExecution`, `ReleaseExecution`, `UpdateStatus` (dùng ở `ExecuteTask`, `dispatchDirectAgentAsync`, `ReportTaskExecutionResult`, vòng phục hồi) không ghi outbox. Vì vậy `request-service` hiện không có cách biết task đã chạy, lỗi hay xong. Đây là thiếu sót cần bổ sung.
- Payload `statuschanged` hiện chỉ có `task_id`, `project_id`, `worktree_id`, `previous_status`, `new_status`; tenant nằm ở envelope `eventbus.Event.TenantID`, id sự kiện ở `Event.ID` (dùng để khử trùng). Chưa có `request_id`, `task_type`, lý do lỗi.
- Run lỗi trả task về `previous_status` (CR-TG-008), tức không có trạng thái `failed` ở task; lỗi chỉ lộ qua `execution_links.status_mirror='failed'`. `execution_links` không lưu thông điệp lỗi (`ReportTaskExecutionResultRequest.error_message` chỉ được log).
- Run thành công đưa task lá về `review`, không `done`. Task phụ thuộc chỉ được mở khoá (`blocked → open`) trong bước un-block của `UpdateTask` khi dependency đạt `done` (`update_task.go`). Một task `review` không mở khoá task sau.
- `EnsureWorktree` (`adapter/grpcclient/worktree_provisioner.go`) tạo worktree riêng, nhánh `task/<task_id>`, cho mỗi task chưa có `worktree_id`; tái dùng nếu `Task.WorktreeID` đã có. Hai task trong cùng Phase không tự thấy code của nhau.
- `ExecuteTask` cần quyền `execute` của người gọi (`ResolvePermission`), dev server của project đang kết nối (`TASK_EXECUTE_NO_CONNECTION`), và chặn `TASK_EXECUTE_ALREADY_IN_PROGRESS` khi đang chạy. Đường này đã hỗ trợ SSH/remote qua dev server; CR không thêm đường chạy mới (README v6 mục 6).

README v6 giao cho CR này: `StartPhase`, chạy task lá của Phase đã duyệt, consumer `ReportTaskOutcome`, cập nhật Phase/Plan/Solution/Request, và gọi use case CR-REQ-006 khi lỗi.

## 2. Giải pháp đề xuất

### 2.1 Đơn vị chạy

Task lá của Phase (con trực tiếp của `phase`) hoặc, với Plan không có Phase, con trực tiếp của `plan`; riêng `hotfix` là task `single_task`. Không bao giờ `Execute` trên `plan`/`phase` (bị chặn ở CR-REQ-011). Task có `depends_on` đi Engine 2 qua `selectEngine`, task thường Engine 1; `request-service` không chọn engine.

### 2.2 `task-service`: phát sự kiện đầy đủ cho task thuộc Request

Chỉ áp cho task có `request_id` khác rỗng, để không thêm tải và nhiễu cho task thường. Payload `taskStatusChangedPayload` (CR-REQ-011 đã thêm `task_type`, `parent_id`, `request_id`, `cause`) thêm `execution_link_id`, `engine`, `error_message` (omitempty). Giá trị `cause`:

| `cause` | Nơi phát |
|---|---|
| `execute_claim` | sau `ClaimForExecution` thành công (`in_progress`) |
| `execution_completed` | `CompleteExecution` (direct_agent) và `ReportTaskExecutionResult` thành công (`review`) |
| `execution_failed` | hoàn tác về `previous_status` ở `dispatchDirectAgentAsync`, nhánh dispatch lỗi của `ExecuteTask`, `ReportTaskExecutionResult` thất bại; mang `error_message` |
| `recovery` | `ReleaseExecution` do vòng phục hồi (lease hết hạn, run mồ côi) |
| `user_update`, `derived` | CR-REQ-011 |

Để sự kiện và ghi trạng thái cùng transaction (mất sự kiện "lỗi" thì Request kẹt `executing`), thêm tham số `events []domain.OutboxEvent` cho `ClaimForExecution`, `CompleteExecution`, `ReleaseExecution` ở `usecase/ports.go`, `usecase/execution_lease.go` và hai adapter (`adapter/postgres/repository.go`, `execution_leases.go`, `adapter/mysql/...`), mẫu `Repository.Update`. Các nhánh `UpdateStatus` hoàn tác trong `ExecuteTask` đổi sang `ReleaseExecution`-kiểu có sự kiện. `ReleaseUnlinkedInProgress` (hàng loạt, không trả id) không phát; bù bằng bước đối soát ở 2.8.

### 2.3 `StartPhase`

```
message StartPhaseRequest  { string request_id = 1; string phase_task_id = 2; }  // phase_task_id rỗng: Plan không có Phase
message StartPhaseResponse { string phase_task_id = 1; bool already_started = 2; repeated string dispatched_task_ids = 3; }
```

Các bước (`internal/usecase/start_phase.go`, mới):
1. `tenant.RequireTenantID`; đọc Request; phải ở `executing` (`REQUEST_NOT_EXECUTING`).
2. Đọc Phase từ `task-service` (`ListTasks task_types=[phase], request_ids=[id]`); phải đúng `request_id` và `parent_id = plan_task_id` (`REQUEST_PHASE_NOT_IN_PLAN`). `phase_task_id` rỗng chỉ hợp lệ khi Plan không có Phase.
3. Nếu `FlowFor(type).ExecutionGates` có `phase`: phải có Approval `phase` `approved` cho `phase_task_id` (`REQUEST_PHASE_NOT_APPROVED`); loại khác (bug/refactor size L) không yêu cầu, Approval `plan` đã bao.
4. Thứ tự: mọi Phase mà Phase này `depends_on` phải `done` hoặc `cancelled` (`REQUEST_PHASE_PREDECESSOR_NOT_DONE`). Kiểm bằng `GetDependencies` hoặc `GetSubtree` (RPC có sẵn).
5. Claim idempotent: `INSERT` vào `phase_starts (tenant_id, phase_task_id)` (Postgres `ON CONFLICT DO NOTHING`, MySQL `INSERT IGNORE`); chèn được thì phát `orca.request.phase.started`; không thì `already_started=true`. Dù thế nào cũng chạy bước 6 (chạy lại an toàn).
6. `AdvanceExecution(container)` (2.4).

Không có Phase (Plan trực tiếp, `task_list`, `hotfix`): consumer sự kiện `orca.request.request.status_changed` với `to=executing` tự gọi `StartExecution` (cùng `AdvanceExecution`, không cần người bấm). Với `change_request`, người bấm "Bắt đầu Phase" (CR-REQ-021) sau khi Phase được duyệt.

### 2.3b `SubjectHandler` cho `phase` và `ExecutionGuard`

`internal/usecase/phase_subject_handler.go` (mới), đăng ký cho `subject_type=phase` (CR-REQ-009 giao cho CR-REQ-012/013; CR này nhận):
- `ValidateForRequest`: Request `executing`; Phase thuộc Plan của Request (`task-service` `ListTasks`); `digest` = SHA-256 của `(id, title)` các task con của Phase và cạnh `depends_on` giữa chúng.
- `OnApproved`: không đổi trạng thái Request, chỉ ghi nhận (sự kiện `approval.decided` đủ); người bấm `StartPhase` sau đó.
- `OnRejected`: `ReturnToBacklog(stage=phase, category=rejected, reason=comment)` (CR-REQ-006), trong cùng transaction vì chỉ ghi DB `request-service`.
- `OnClosedWithoutDecision`: không làm gì.

`ExecutionGuard` (cổng của CR-REQ-005, `false` cho đến CR này; CR-REQ-006 dùng để chặn trả backlog/huỷ khi còn task chạy): hiện thực bằng `ListTasks(request_ids=[id], task_types=[task,bug,feature])` và kiểm có task `in_progress`.

### 2.4 `AdvanceExecution` (idempotent, gọi từ `StartPhase`, consumer, đối soát)

1. Liệt kê task lá của container (`ListTasks parent_id=...`).
2. Chọn sẵn sàng: `status = open` (task có phụ thuộc chưa xong đang `blocked`, không chọn). Trừ số task đang `in_progress` khỏi `REQUEST_MAX_PARALLEL_TASKS` (mặc định 1, xem 2.7).
3. Với mỗi task chọn được: hỏi `TypePolicy.PreExecutionGate(task)` (CR-REQ-014; mặc định cho qua). Cổng chưa duyệt thì `OpenApproval` (CR-REQ-009, idempotent theo chủ thể đang `pending`) và dừng task đó, không báo lỗi. `approval.decided` đã duyệt của `pre_deploy` (ops_request) kích lại `AdvanceExecution` qua consumer idempotent dùng `processed_events`.
4. Worktree dùng chung (xem 2.7): task đầu tiên của Plan chạy trước; sau khi có `worktree_id`, `UpdateTask(worktree_id)` cho các task còn lại trước khi chạy.
5. `task-service` `Execute(task_id, request_id = "req:<request_id>:<task_id>:<attempt>")`. Lỗi `TASK_EXECUTE_ALREADY_IN_PROGRESS` coi như thành công. `TASK_EXECUTE_NO_CONNECTION`, `TASK_EXECUTE_WORKTREE_FAILED`, `TASK_EXECUTE_FAILED`: không đếm vào số lần thử; thử lại ở vòng đối soát tối đa `REQUEST_DISPATCH_RETRY_WINDOW` (mặc định 15 phút) rồi trả backlog (2.6).
6. Danh tính khi gọi: người duyệt Phase (`decided_by`), hoặc người duyệt Plan khi không có Phase; cần quyền `execute` trên task (Grant kế thừa theo cây từ người tạo Plan). Thiếu quyền: `REQUEST_EXECUTE_FORBIDDEN`, không tự đổi sang danh tính khác. Cơ chế danh tính service-to-service chưa kiểm chứng (xem mục 6).

### 2.5 Consumer `ReportTaskOutcome`

`internal/adapter/eventbus/task_outcome_consumer.go` (mới) đăng ký bền (durable) `orca.task.task.statuschanged` trên stream `TASK`, bỏ qua bản ghi `request_id` rỗng. Gọi use case `ReportTaskOutcome`; RPC nội bộ `ReportTaskOutcome` (README 3.6) là vỏ mỏng quanh cùng use case, dành cho đối soát và test, chỉ nhận lời gọi từ service nội bộ.

```
message ReportTaskOutcomeRequest {
  string event_id = 1; string request_id = 2; string task_id = 3; string task_type = 4;
  string previous_status = 5; string new_status = 6; string cause = 7;
  string execution_link_id = 8; string error_message = 9; google.protobuf.Timestamp occurred_at = 10;
}
```

Use case (`internal/usecase/report_task_outcome.go`, mới), một `TxRunner.InTx` cho phần ghi:
1. Khử trùng: `INSERT` `processed_events(event_id)` (CR-REQ-001); đã có thì trả thành công, không làm gì.
2. Đọc Request; không có thì ghi log và bỏ qua (`request_id` mồ côi). Request không còn `executing` (đã `request_backlog`, `cancelled`, `completed`) thì chỉ ghi `task_run_outcomes` rồi dừng.
3. Phân loại (bảng 2.5.1), ghi `task_run_outcomes`, rồi hành động.

**2.5.1 Phân loại và hành động**

| `task_type`, `cause`, `new_status` | Kết quả | Hành động |
|---|---|---|
| lá, `execute_claim`, `in_progress` | `started` | không (chỉ ghi) |
| lá, `execution_completed`, `review` | `succeeded` | nếu `REQUEST_AUTO_COMPLETE_TASKS` (mặc định bật): `UpdateTask(status=done)` (sinh un-block cho task sau, sự kiện `done` quay lại consumer); tắt: chờ người đặt `done` |
| lá, `user_update`, `done` | `succeeded` | `AdvanceExecution(parent)` |
| lá, `execution_failed` | `failed` | đếm lần lỗi của task (`task_run_outcomes` `failed`); `< REQUEST_MAX_TASK_ATTEMPTS` (mặc định 2) thì `AdvanceExecution` chạy lại; ngược lại trả backlog (2.6) |
| lá, `recovery` | `failed` | như `execution_failed`, lý do `recovery` |
| lá, `cancelled` (người huỷ) | `cancelled` | nếu mọi con của container `cancelled` thì trả backlog stage `phase` hoặc `plan` |
| `phase`, `derived`, `done` | `phase_done` | phát `orca.request.phase.completed`; nếu còn Phase sau: `OpenApproval` `phase` cho Phase kế (CR-REQ-009), Request vẫn `executing`; nếu hết Phase: bước 4 |
| `plan`, `derived`, `done` | `plan_done` | bước 4 |
| không có Phase, mọi lá `done` | `plan_done` | bước 4 |

4. Hoàn tất: `TypePolicy.CompletionChecks(request)` (CR-REQ-014; mặc định rỗng). Có kiểm tra chưa đạt: không chuyển, chờ; kiểm tra `failed`: trả backlog stage `task`. Đạt hết: `TransitionRequest(execution_finished, ExpectedFrom=executing)` (CR-REQ-003), sinh `request.completed`, rồi `TypePolicy.OnCompleted` (hotfix sinh Request theo dõi, CR-REQ-014).

**Solution:** README 3.5 không có trạng thái sau thực thi cho `solutions`; CR giữ `status=approved`, không ghi thêm. Kết quả nằm ở sự kiện `request.completed` và `task_run_outcomes` (Q2).

### 2.6 Trả Request backlog khi lỗi

Chỉ gọi `ReturnToBacklog` (CR-REQ-006) với `stage`, `category`, `reason`; không tự đặt trạng thái:

| Nguồn | `returned_from_stage` | `category` (CR-REQ-006) | `reason` (mẫu) |
|---|---|---|---|
| Task hết lần thử | `task` | `other` | `Task "<title>" lỗi sau N lần: <error_message cuối>` |
| Không dispatch được quá cửa sổ thử lại | `task` | `blocked_dependency` | `Không chạy được: <mã lỗi>` (ví dụ `TASK_EXECUTE_NO_CONNECTION`) |
| Phase bị từ chối (handler `phase`) | `phase` | `rejected` | `comment` của người duyệt |
| Mọi task của container bị huỷ | `phase` hoặc `plan` | `other` | `Toàn bộ task đã huỷ` |
| Kiểm tra hoàn tất `failed` (CR-REQ-014) | `task` | `infeasible` (đo) hoặc `other` (test) | tên kiểm tra và tóm tắt |

`actor_kind=system`. Nếu `ExecutionGuard` chặn (`REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION`, còn task khác chạy khi song song > 1) thì thử lại ở vòng đối soát; với `REQUEST_MAX_PARALLEL_TASKS=1` không xảy ra.

Plan bị từ chối do CR-REQ-003 (`plan_rejected`), không thuộc CR này. Task đang chạy khi Request bị trả/huỷ vẫn chạy tới cùng (không có RPC dừng run); kết quả của nó chỉ được ghi (2.5 bước 2).

### 2.7 Worktree dùng chung và song song

Vì mỗi task mặc định có worktree riêng, task sau không thấy thay đổi của task trước. CR đặt: toàn Plan dùng một worktree. Task đầu `Execute` để `EnsureWorktree` tạo; `request-service` đọc `worktree_id` của task đó và `UpdateTask(worktree_id)` cho các task còn lại của Plan (đường `WorktreeID` có sẵn trong `UpdateTaskInput`). Hệ quả: chạy tuần tự (`REQUEST_MAX_PARALLEL_TASKS=1`, mặc định và hiện là giá trị duy nhất an toàn). Chưa kiểm chứng: project-service có chấp nhận một worktree gắn nhiều task (`Worktree.TaskID`), và hai run nối tiếp trong một worktree hoạt động đúng.

### 2.8 Đối soát

`ReconcileExecutingRequests` (`request-service`, chu kỳ `REQUEST_RECONCILE_INTERVAL` mặc định 60 giây, khoá bằng `SELECT ... FOR UPDATE SKIP LOCKED` hoặc `GET_LOCK` MySQL; MySQL cần ≥ 8.0.1): với Request `executing` không có sự kiện trong 5 phút, liệt kê task bằng `ListTasks` và chạy lại bước 3 của 2.5 theo trạng thái hiện thời (task `open` không có run, dispatch lỗi tạm thời, phase `done` mà chưa phát `phase.completed`). Đây là lưới an toàn cho sự kiện mất (`ReleaseUnlinkedInProgress`, NATS không kết nối lúc khởi động như `task-service/cmd/server/main.go` cho phép).

### 2.9 Dữ liệu `request-service` (migration kế tiếp sau `0002_request_core`, số chưa xác định; cả hai dialect)

- `phase_starts`: `tenant_id` uuid NOT NULL, `phase_task_id` uuid NOT NULL, `request_id` uuid NOT NULL, `started_by` uuid NOT NULL, `started_at` timestamptz NOT NULL; PK `(tenant_id, phase_task_id)`.
- `task_run_outcomes`: `id` uuid PK, `tenant_id`, `request_id`, `task_id`, `container_id` NULL, `outcome` CHECK (`started`,`succeeded`,`failed`,`cancelled`,`phase_done`,`plan_done`), `cause`, `execution_link_id` NULL, `error_message` TEXT NOT NULL DEFAULT '', `event_id` uuid NOT NULL, `occurred_at`; chỉ mục `(tenant_id, request_id, occurred_at)` và `(tenant_id, task_id, outcome)`. MySQL: `CHAR(36)`, `TIMESTAMP(6)`.
- Không FK sang `task-service`.

### 2.10 Sự kiện và cấu hình

`orca.request.phase.started` `{request_id, phase_task_id, started_by, dispatched}`, `orca.request.phase.completed` `{request_id, phase_task_id, task_count}`. Tên subject chờ chốt (xem Q1 của `request-service-foundation/README.md`). Biến môi trường: `REQUEST_MAX_PARALLEL_TASKS=1`, `REQUEST_MAX_TASK_ATTEMPTS=2`, `REQUEST_AUTO_COMPLETE_TASKS=true`, `REQUEST_DISPATCH_RETRY_WINDOW=15m`, `REQUEST_RECONCILE_INTERVAL=60s` (mặc định đều là đề xuất, chưa kiểm chứng).

### 2.11 Lỗi

`REQUEST_NOT_EXECUTING`, `REQUEST_PHASE_NOT_IN_PLAN`, `REQUEST_PHASE_NOT_APPROVED`, `REQUEST_PHASE_PREDECESSOR_NOT_DONE`, `REQUEST_EXECUTE_FORBIDDEN` (PermissionDenied), còn lại FailedPrecondition. Quyền gọi `StartPhase`: người có quyền duyệt Phase hoặc chủ Request (CR-REQ-010); gateway kiểm (CR-REQ-016).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Chạy task lá, không chạy Phase | README v6; `ComplexExecutor` chỉ mở một cấp con |
| Phát sự kiện trong cùng transaction, chỉ cho task có `request_id` | Mất sự kiện lỗi làm Request kẹt; tránh nhiễu cho task thường |
| Dùng lại `statuschanged`, không thêm subject mới | Một kênh, `cause` phân biệt; WS bridge và notification đã nhận subject này |
| Tự đặt lá `done` sau `review` (cờ) | Chỉ `done` mới mở khoá task sau (`update_task.go`); duyệt con người nằm ở Approval Plan/Phase |
| Một worktree cho cả Plan, tuần tự | Task sau cần thấy code task trước; hiện không có cơ chế merge giữa nhánh `task/<id>` |
| `phase_starts` làm khoá idempotency | Nhấn hai lần không chạy hai lần; chạy lại `AdvanceExecution` vẫn an toàn |
| `ReturnToBacklog` là đường duy nhất ra backlog | CR-REQ-006 sở hữu trạng thái và lý do |
| Đối soát định kỳ | Sự kiện at-least-once nhưng có thể mất ở vòng quét hàng loạt |

## 4. Tiêu chí chấp nhận

- [ ] Task có `request_id` phát đúng một sự kiện `statuschanged` cho mỗi chuyển trạng thái ở các nhánh `execute_claim`, `execution_completed`, `execution_failed`, `recovery`, trong cùng transaction với ghi status; task không có `request_id` không phát thêm gì (so sánh số dòng outbox trước/sau).
- [ ] `StartPhase` chưa duyệt Phase, sai Request, hoặc Phase trước chưa `done` bị đúng mã lỗi ở 2.3; gọi hai lần cùng Phase trả `already_started=true`, không `Execute` trùng và chỉ một `phase.started`.
- [ ] Task `open` được `Execute` đúng `REQUEST_MAX_PARALLEL_TASKS`; task `blocked` không bị chạy; task thành công `review` được đặt `done` và task phụ thuộc chuyển `open` rồi được chạy.
- [ ] Task lỗi lần 1 được chạy lại; lần `REQUEST_MAX_TASK_ATTEMPTS` lỗi thì Request vào `request_backlog` với `returned_from_stage=task` và lý do chứa `error_message`.
- [ ] Sự kiện giao hai lần (cùng `Event.ID`) cho cùng kết quả một lần (`processed_events`).
- [ ] Tất cả task của Phase `done` thì Phase `done` (CR-REQ-011), có `phase.completed`, Approval Phase kế được tạo một lần; hết Phase thì `execution_finished` đưa Request về `completed` và có `request.completed`.
- [ ] Plan không Phase, `task_list`, `hotfix` tự chạy khi Request vào `executing`, không cần `StartPhase`.
- [ ] Phase bị từ chối đưa Request về backlog stage `phase`.
- [ ] Mất sự kiện (tắt consumer 2 phút rồi bật, hoặc `ReleaseUnlinkedInProgress`): vòng đối soát đưa Request về trạng thái đúng trong một chu kỳ.
- [ ] Task thứ hai của Plan chạy trong cùng `worktree_id` với task đầu (kiểm trên `GetTask`).
- [ ] Mọi mã lỗi ở 2.11 có test; hai dialect cùng kết quả.

## 5. Kiểm thử

- **Unit:** bảng phân loại 2.5.1 (table-driven); `AdvanceExecution` với fake `TaskClient` (chọn sẵn sàng, song song, lỗi `ALREADY_IN_PROGRESS`, hết cửa sổ thử lại); `StartPhase` từng điều kiện; đếm lần lỗi.
- **Integration, cả hai dialect:** `task-service` ghi trạng thái + outbox cùng transaction (chèn lỗi outbox, status không đổi); `phase_starts` đua 8 goroutine; `processed_events` giao lặp; consumer đọc từ NATS thật cho một vòng đời task; đối soát với sự kiện bị bỏ.
- **Hợp đồng:** payload `statuschanged` có đủ trường cho consumer; proto `buf breaking`.
- **E2E (cần CR-REQ-009, 011, 012):** Request `change_request` từ Plan duyệt đến `completed`; một task lỗi cố ý đến backlog.
- Chưa chạy test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- Worktree dùng chung (2.7) là giả định lớn nhất: chưa kiểm chứng hành vi `project-service` khi một worktree gắn nhiều task, cũng như `EnsureWorktree` tái dùng đúng đường dẫn cho task có `worktree_id` do người khác đặt. Nếu sai, cả pipeline cần cơ chế khác (một task lớn mỗi Phase, hoặc merge giữa nhánh) và CR này phải sửa.
- `ReportTaskExecutionResult` ghi chú chưa có kiểm tra danh tính service gọi (`adapter/grpc/server.go`); RPC `ReportTaskOutcome` nội bộ gặp cùng vấn đề. Chưa chọn cơ chế.
- Tự đặt `done` bỏ qua bước review tay của task; `UpdateTask(done)` phát `orca.task.task.completed` cho mỗi task lá nên `notification-service` có thể đẩy thông báo hàng loạt. Cần quyết định có gắn cờ bỏ thông báo cho task thuộc Request.
- Run Engine 2/3 báo xong bất đồng bộ sau khi link bị đánh dấu `completed` ngay lúc dispatch (CR-TG-008); chỉ `ReportTaskExecutionResult` mới là tín hiệu thật. Phụ thuộc orchestration-service gọi callback đúng (CR-TG-008 ghi chưa thử đầu cuối).
- Không có RPC dừng run: huỷ Request không dừng agent đang chạy.
- Quyền `execute` cho người duyệt Phase chưa chắc có trên task do người khác tạo; lỗi ở runtime là `REQUEST_EXECUTE_FORBIDDEN`.

## 7. Câu hỏi mở

- **Q1.** Request có ở `executing` trong suốt thời gian chờ duyệt Phase thứ hai trở đi (CR-REQ-003 chỉ có `awaiting_plan_approval` cho Plan)? CR giả định ở lại `executing`.
- **Q2.** Cần trường kết quả trên `solutions` (ví dụ `executed_at`) hay chỉ sự kiện là đủ? README 3.5 chưa có; CR không thêm cột.
- **Q3.** Cờ `REQUEST_AUTO_COMPLETE_TASKS` mặc định bật có phù hợp, hay mặc định tắt để người đóng task?
- **Q4.** Ánh xạ `category` ở 2.6 (ví dụ lỗi môi trường là `blocked_dependency`) là đề xuất; cần CR-REQ-006 xác nhận ý nghĩa năm giá trị.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.3, 3.4, 3.6, 3.7
- `/opt/repos/orca/docs/crs/v6/plan-phase-task/CR-REQ-011-task-service-plan-phase-task-types.md`, `CR-REQ-012-plan-phase-task-generation-from-solution.md`
- `/opt/repos/orca/docs/crs/v6/request-lifecycle/CR-REQ-003-request-state-machine-and-flow-registry.md`
- `/opt/repos/orca/docs/crs/v4/task-graph/CR-TG-008-jira-source-link-and-durable-direct-agent.md`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/execute_task.go`, `report_execution_result.go`, `mirror_execution_status.go`, `execution_lease.go`, `update_task.go`, `ports.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/postgres/execution_leases.go`, `repository.go`; `adapter/mysql/execution_leases.go`, `repository.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/grpcclient/worktree_provisioner.go`, `complex_executor.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/eventbus/consumer.go`, `publisher.go`; `cmd/server/main.go` (stream `TASK`, subject `orca.task.>`)
- `/opt/repos/orca/backend-go/common/eventbus/eventbus.go` (`Event.ID`, `Event.TenantID`)
- `/opt/repos/orca/backend-go/proto/orca/task/v1/task.proto`
- Mới: `request-service/internal/usecase/start_phase.go`, `advance_execution.go`, `report_task_outcome.go`, `reconcile_executing_requests.go`, `internal/adapter/eventbus/task_outcome_consumer.go`, `internal/adapter/grpcclient/task_client.go`, migration `phase_execution` hai dialect
