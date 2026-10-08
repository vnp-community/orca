# BE-REQ-SOL-015: API đọc ba view backlog (`ListBacklog`, `ListExecutionStates`)

> **✅ Đã triển khai (kiểm chứng 2026-10-08, 6/6 task).** Bảng cổng 2.4 vẫn cần chủ CR-REQ-015 xác nhận; quyền xem Request dùng quy tắc tạm của CR-REQ-035 (xem IMPLEMENTATION-NOTES).

**CR:** [CR-REQ-015](../../../../../../docs/crs/v6/backlog-views/CR-REQ-015-backlog-read-views.md)
**Service:** `request-service` (mới, `ListBacklog`) · `task-service` (`ListExecutionStates`) · `proto/orca/request/v1`, `proto/orca/task/v1`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (usecase đọc qua port), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (không FK chéo, hai dialect, keyset), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`services/task-service.md`](../../../../tdd/services/task-service.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-06: `task-service/migrations/{postgres,mysql}/0010_execution_links.up.sql`, `internal/domain/execution_link.go` (`StatusMirror`, `StartedAt`, `CompletedAt`, `PreviousStatus`), `migrations/postgres/0001_init.up.sql` (`task_edges`, `task_edges_from_idx (tenant_id, from_task_id, edge_type)`), `usecase/list_tasks.go`, `usecase/get_dependencies.go`, proto `GetDependenciesRequest/Response` (một task mỗi lần), `adapter/{postgres,mysql}/execution_links.go`.

Khớp với CR gốc:
- Không RPC nào trả `execution_links`; `Task.active_execution_link_id` chỉ trỏ link mới nhất, không kèm trạng thái.
- `GetDependencies` chỉ trả đích `depends_on` của một task; không có bản theo lô.
- Chỉ mục `idx_execution_links_task (task_id, started_at DESC)` có ở cả hai dialect, nên "link gần nhất theo task" hiệu quả.
- `status_mirror` nhận `in_progress`, `completed`, `failed` (cột `VARCHAR(20)` default `in_progress`); run lỗi trả task về `previous_status` (CR-TG-008) nên "task lỗi" chỉ nhận ra qua link gần nhất `failed`. Link không lưu thông điệp lỗi: `last_error` lấy từ `task_run_outcomes` (SOL-013), chỉ có cho task thuộc Request.
- `ListTasks` chỉ kiểm tenant, không kiểm grant người gọi (xem 6).
- `backend-go/services/request-service` chưa tồn tại; mọi đường dẫn của nó là "(mới)".

### Correction relative to CR-REQ-015

1. CR viết `task.execution_links` và `task.task_edges` cho Postgres: đúng schema `task`; bản MySQL không có tiền tố schema (`execution_links`, `task_edges`, `tasks`). Hai adapter dùng tên khác nhau.
2. CR-REQ-015 dựa vào `returned_category` và `request_return_history` của CR-REQ-006 mà README v6 mục 3.5 chưa nêu (CR đã ghi ở Q2); README mục 8 chưa cập nhật điều này. Solution giả định hai thứ đó tồn tại.
3. README v6 mục 8 điều 7 đã chấp nhận mở rộng Execute backlog bằng "cổng của task" theo `FlowFor`; phần "cần xác nhận" của CR coi như đã chốt, nhưng điều 7 chưa nói rõ bảng 2.3: bảng đó vẫn cần chủ CR xác nhận.
4. Cổng `TaskClient` (`ListTasks`, `ListExecutionStates`) dùng chung với SOL-013 (`task_client.go`); không tạo file thứ hai.

## 2. Giải pháp

### 2.1 Cây file

```
backend-go/proto/orca/request/v1/request_backlog.proto            (mới)  ListBacklog*, BacklogView, rows
backend-go/proto/orca/task/v1/task.proto                          (sửa)  ListExecutionStates*
backend-go/services/task-service/
  internal/usecase/list_execution_states.go                       (mới)
  internal/usecase/ports.go                                       (sửa)  ExecutionStateReader
  internal/adapter/postgres/execution_states.go                   (mới)
  internal/adapter/mysql/execution_states.go                      (mới)
  internal/adapter/grpc/server_execution_states.go                (mới)
backend-go/services/request-service/
  internal/domain/backlog_gate.go                                 (mới)  cổng task, hàm thuần
  internal/domain/backlog_page_token.go                           (mới)  mã hoá keyset
  internal/usecase/list_backlog.go                                (mới)
  internal/usecase/ports.go                                       (sửa)  BacklogRequestReader, ApprovalGateReader
  internal/adapter/postgres/backlog_requests.go, backlog_approvals.go   (mới)
  internal/adapter/mysql/backlog_requests.go, backlog_approvals.go      (mới)
  internal/adapter/grpcclient/task_client.go                      (sửa)  ListExecutionStates
  internal/adapter/grpc/server_backlog.go                         (mới)
```

### 2.2 RPC `ListBacklog`

```proto
enum BacklogView { BACKLOG_VIEW_UNSPECIFIED = 0; REQUEST = 1; TASK = 2; EXECUTE = 3; }
message ListBacklogRequest { BacklogView view = 1; string project_id = 2; repeated string request_types = 3;
  string request_id = 4; string plan_task_id = 5; string phase_task_id = 6; string assignee_id = 7;
  int32 page_size = 8; string page_token = 9; repeated string categories = 10; }
message ListBacklogResponse { repeated BacklogRequestRow request_rows = 1; repeated BacklogGroup groups = 2; string next_page_token = 3; }
```

`view` bắt buộc (`REQUEST_BACKLOG_INVALID_VIEW`). Chỉ đọc, không sinh sự kiện. `BacklogRequestRow`, `BacklogGroup`, `BacklogTaskRow` đúng như CR (số trường 1 đến 13, 1 đến 7, 1 đến 10). `page_size` mặc định 20, tối đa 100 Request; `page_token` là base64 `(updated_at, id)` của Request cuối (`REQUEST_BACKLOG_BAD_PAGE_TOKEN`).

### 2.3 View REQUEST

Một truy vấn lên `requests` với chỉ mục `(tenant_id, status, updated_at DESC)`, keyset khai triển cho cả hai dialect (MySQL tối ưu kém dạng so sánh hàng):

```sql
SELECT ... FROM requests
WHERE tenant_id = :t AND status = 'request_backlog'
  AND (updated_at < :ts OR (updated_at = :ts AND id < :id))
ORDER BY updated_at DESC, id DESC LIMIT :n + 1
```

Thêm lọc `project_id`, `type IN`, `returned_category IN (categories)`. `parent_request_ids` từ một truy vấn `request_links ... WHERE child_request_id IN (...)`; `returned_by`, `returned_at` từ `request_return_history` (`action='returned'`, lấy `at` lớn nhất mỗi Request ở bộ nhớ, không dùng `updated_at`). Không gọi `task-service`.

### 2.4 Cổng của task (hàm thuần `backlog_gate.go`)

Task làm việc = `task_type` ∈ {`task`,`bug`,`feature`} có `request_id`. Container cổng = cha gần nhất (`phase`, `plan`, hoặc không có cha với hotfix). Bản đầu chỉ xét task là con trực tiếp của `phase`/`plan` và task `single_task`; task lồng sâu hơn bỏ qua.

| Task nằm ở | "Đã duyệt" khi |
|---|---|
| dưới `phase`, `ExecutionGates` có `phase` (`change_request`) | Approval `phase` `approved` trên Phase **và** `plan` `approved` trên Plan |
| dưới `phase`, loại không có `phase` gate (bug/refactor size L) | Approval `plan` `approved` trên Plan cha của Phase |
| dưới `plan`, `PhasesFor(size)` sai | Approval `plan` hoặc `task_list` `approved` theo `Plan.Kind`; `security` dùng `pre_deploy` trên Plan |
| dưới `plan`, `PhasesFor(size)` đúng (chưa chia Phase) | không bao giờ đạt: "Plan chưa chia Phase" |
| `single_task` (hotfix) | Approval `pre_deploy` `approved` trên task |

Bản ghi mới nhất theo `created_at` của `(subject_type, subject_id)` quyết định; `pending` hoặc `rejected` là chưa duyệt. Đọc Approval bằng một truy vấn `WHERE tenant_id = ? AND request_id IN (...) AND subject_type IN ('plan','task_list','phase','pre_deploy')`, lắp ráp trong bộ nhớ.

### 2.5 View TASK và EXECUTE

Loại trừ nhau theo cổng. **TASK:** Request `awaiting_plan_approval` hoặc `executing`; task làm việc `open|blocked` mà cổng chưa duyệt; nhóm theo Plan rồi Phase. **EXECUTE:** Request `executing`; cổng đã duyệt; và (`open|blocked` hoặc link gần nhất `failed` với `status NOT IN (done,cancelled)`); nhóm theo Phase (hoặc Plan). Trình tự một lần gọi, không N+1: (1) chọn trang Request ứng viên; (2) `task-service.ListTasks(request_ids, task_types=[task,bug,feature,plan,phase])` một lần (SOL-011), phân trang nội bộ đến hết (mỗi Plan tối đa 100 task, SOL-012); (3) một truy vấn `approvals`; (4) `ListExecutionStates(task_ids)`; (5) lắp ráp; `last_error` từ `task_run_outcomes` (`LatestFailed`, SOL-013 task 03); `gate_status` ∈ `approved|pending|rejected|none`. Phân trang theo Request: trang có thể ít nhóm hơn `page_size`; Request không có task thuộc view vẫn tiêu một chỗ.

### 2.6 `task-service`: `ListExecutionStates`

```proto
rpc ListExecutionStates(ListExecutionStatesRequest) returns (ListExecutionStatesResponse);
message ListExecutionStatesRequest { repeated string task_ids = 1; }  // tối đa 500: TASK_STATES_TOO_MANY_IDS
message ExecutionState { string task_id = 1; repeated string blocked_by_task_ids = 2; string last_engine = 3;
  string last_link_status = 4; google.protobuf.Timestamp last_started_at = 5; google.protobuf.Timestamp last_completed_at = 6; int32 failed_attempts = 7; }
```

Cổng `ExecutionStateReader` ở `ports.go`; use case `list_execution_states.go`. Link gần nhất: Postgres `SELECT DISTINCT ON (task_id) ... ORDER BY task_id, started_at DESC, id DESC`; MySQL `ROW_NUMBER() OVER (PARTITION BY task_id ORDER BY started_at DESC, id DESC)` rồi `rn = 1` (MySQL ≥ 8.0). Số lần lỗi: Postgres `count(*) FILTER (WHERE status_mirror = 'failed')`, MySQL `SUM(status_mirror = 'failed')`. `blocked_by`: `task_edges` `edge_type='depends_on'` nối `tasks` đích có `status NOT IN ('done','cancelled')`, dùng `task_edges_from_idx`. Mọi truy vấn lọc `tenant_id`.

### 2.7 Lỗi

`REQUEST_BACKLOG_INVALID_VIEW`, `REQUEST_BACKLOG_BAD_PAGE_TOKEN` (InvalidArgument), `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE` (Unavailable; view TASK/EXECUTE không trả dữ liệu từng phần), `TASK_STATES_TOO_MANY_IDS`.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Một RPC `ListBacklog` với `view` | Gateway và MCP chỉ cần kênh `backlog.*` |
| Truy vấn ở `request-service`, `task-service` chỉ đưa dữ liệu | Approval và loại Request thuộc `request-service` |
| Chỉ một RPC mới ở `task-service` | `ListTasks` (SOL-011) đủ lấy cây; thiếu mỗi link và cạnh theo lô |
| Phân trang theo Request, tối đa 100 task mỗi Plan | Nhóm không vỡ giữa các trang |
| Hai view task loại trừ nhau theo cổng | Một task xuất hiện đúng một nơi |
| Lỗi toàn bộ khi `task-service` không sẵn | Trả một nửa bị hiểu nhầm là backlog rỗng |
| `last_error` từ `task_run_outcomes` | Link không lưu thông điệp lỗi |

## 4. Phụ thuộc và thứ tự

Task 01, 02 (task-service) cần SOL-011 task 01 (không bắt buộc) và độc lập SOL-013; task 03, 04 cần CR-REQ-002, 006, 003; task 05 cần SOL-011 (task 04), SOL-013 (task 03 `LatestFailed`), CR-REQ-009; task 06 cuối. Thứ tự: 01 → 02; 03 song song 01; 04; 05; 06. Mở khoá CR-REQ-016 (kênh `backlog.*`) và CR-REQ-023.

## 5. Kiểm thử

- **Unit:** hàm cổng cho 11 loại và các size (table-driven, dùng `FlowFor` thật); lắp ráp view với fake `TaskClient`, `ApprovalGateReader`; mã hoá/giải mã `page_token`.
- **Integration hai dialect:** REQUEST với keyset và 1.000 hàng; `ListExecutionStates` (nhiều link mỗi task, link cùng `started_at`, task không link, cạnh tới task `done`); truy vấn approvals dùng chỉ mục.
- **Hợp đồng:** `make proto-lint`; mỗi điều kiện README 3.8 có một fixture.
- **E2E (CR-REQ-025):** Request qua vòng đời, task chuyển view đúng lúc. Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/task-service/... ./services/request-service/...` (chưa chạy).

## 6. Rủi ro và điểm chưa kiểm chứng

- `ListTasks` và `ListExecutionStates` chỉ kiểm tenant, không kiểm grant người gọi: lọc quyền xem phải làm ở `request-service` trước khi gọi, chưa có quy tắc rõ ở CR-REQ-010/016.
- Hiệu năng chưa đo: tối đa `page_size × 100` = 2.000 hàng và `IN (...)` lớn trên MySQL.
- `execution_links` tăng không giới hạn; chưa có dọn dẹp.
- Quy tắc "bản mới nhất theo `created_at`" giả định `approvals` giữ nhiều bản ghi cho một chủ thể (CR-REQ-009 gợi ý đúng); chưa kiểm chứng.
- Link `completed` của Engine 2/3 bị đánh dấu ngay sau dispatch (CR-TG-008) nên `last_link_status` không đủ để biết run xong thật; dùng `status` task làm nguồn chính, link chỉ để phát hiện `failed`.
- Window function MySQL ≥ 8.0; TiDB chưa kiểm chứng.

## 7. Câu hỏi mở

- Xác nhận bảng cổng 2.4 với chủ CR-REQ-015 và cập nhật README v6 mục 3.8 (Execute backlog "dưới Phase đã approved" chưa phủ loại không có Phase).
- Task `review` (khi `REQUEST_AUTO_COMPLETE_TASKS` tắt) có thuộc Execute backlog? Hiện loại.
- Subtask lồng dưới task làm việc có cần hiển thị? Bản đầu bỏ.
- `ListBacklog` có cần cho luồng không Request (task đơn lẻ)? Hiện chỉ task có `request_id`.
- Tham chiếu tiến: các CR bổ sung (026 đến 036) có thể thêm trạng thái "chưa sẵn sàng" (ReadinessGate, CR-REQ-029) hoặc câu hỏi làm rõ (CR-REQ-028); chúng có thể muốn hiện trong backlog. Solution này không phụ thuộc; `BacklogTaskRow` và `gate_status` chỉ thêm trường mới (additive) khi cần.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.5, 3.6, 3.8, 8 (điều 7, 9, 12)
- `/opt/repos/orca/docs/crs/v6/plan-phase-task/CR-REQ-011-task-service-plan-phase-task-types.md`, `CR-REQ-013-phase-execution-and-feedback-loop.md`; `/opt/repos/orca/docs/crs/v6/request-lifecycle/CR-REQ-003-request-state-machine-and-flow-registry.md`, `CR-REQ-006-return-to-backlog-reopen-cancel-child-requests.md`; `/opt/repos/orca/docs/crs/v6/approval/CR-REQ-009-generic-approval-domain-and-api.md`
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0001_init.up.sql`, `0010_execution_links.up.sql`; bản `mysql`
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/execution_link.go`, `usecase/list_tasks.go`, `usecase/get_dependencies.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/postgres/execution_links.go`; `adapter/mysql/execution_links.go`
- `/opt/repos/orca/backend-go/proto/orca/task/v1/task.proto`
- `/opt/repos/orca/specs/backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-011-task-service-plan-phase-task-types.md`, `BE-REQ-SOL-013-phase-execution-and-feedback-loop.md`
