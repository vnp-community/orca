# CR-REQ-015 — API đọc ba view backlog

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-015 |
| **Tên** | RPC `ListBacklog` cho Request backlog, Task backlog, Execute backlog (view tính toán, không thêm status backend) |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-002 (`requests`, `request_links`), CR-REQ-006 (`returned_category`, `request_return_history`), CR-REQ-003 (`FlowFor`, `PhasesFor`), CR-REQ-009 (`approvals`), CR-REQ-011 (`request_id`, lọc `ListTasks`), CR-REQ-013 (`task_run_outcomes`) |
| **Mở khoá** | CR-REQ-016 (kênh `backlog.*`), CR-REQ-023 (màn hình Backlog) |
| **Tác động** | `backend-go/services/request-service` (usecase, proto, repository hai dialect); `backend-go/services/task-service` (RPC `ListExecutionStates`, adapter hai dialect, proto) |

## 1. Bối cảnh và vấn đề

D4 (README v6): backlog là view tính toán, không thêm status `backlog` ở backend. Cần một truy vấn đọc cho mỗi view ở `request-service`; frontend chỉ hiển thị, không tự ghép từ nhiều RPC (nghiên cứu mục 4.5).

Dữ liệu nằm ở hai service, không FK chéo:
- `request-service`: `requests` (kèm `returned_category`), `request_return_history`, `request_links`, `approvals`, `task_run_outcomes` (CR-REQ-013).
- `task-service`: task (`parent_id`, `status`, `request_id`, `task_type`, `labels`), `task_edges` (`depends_on`), `execution_links`.

Hạn chế của `task-service` hiện tại (đọc code ngày 2026-10-05) buộc phải thêm một RPC:
- Không RPC nào trả `execution_links`. `Task.ActiveExecutionLinkID` chỉ trỏ link mới nhất, không có trạng thái.
- `GetDependencies` chỉ trả cạnh `depends_on` của một task mỗi lần, không có bản hàng loạt.
- `execution_links` có chỉ mục `(task_id, started_at DESC)` (`migrations/postgres/0010_execution_links.up.sql`, bản MySQL tương đương) nên lấy "link gần nhất theo task" hiệu quả.
- Run lỗi trả task về `previous_status` (CR-TG-008), nên "task lỗi" chỉ nhận ra qua link gần nhất có `status_mirror='failed'`. Link không lưu thông điệp lỗi; lý do lỗi lấy từ `task_run_outcomes.error_message` (CR-REQ-013), chỉ có cho task thuộc Request.

README v6 3.8 còn một chỗ cần làm rõ trước khi viết truy vấn (xem Q1): Plan không chia Phase mà vẫn duyệt xong (bug size M, task, docs, security, performance, ops_request) thì task rơi vào view nào. Phần lý do trả về đã được CR-REQ-006 bổ sung (`requests.returned_category`, bảng `request_return_history`).

## 2. Giải pháp đề xuất

### 2.1 RPC (thêm vào `orca.request.v1.RequestService`, README 3.6 `ListBacklog`)

```
enum BacklogView { BACKLOG_VIEW_UNSPECIFIED = 0; REQUEST = 1; TASK = 2; EXECUTE = 3; }
message ListBacklogRequest {
  BacklogView view = 1;
  string project_id = 2;                 // lọc tuỳ chọn
  repeated string request_types = 3;     // lọc tuỳ chọn
  string request_id = 4; string plan_task_id = 5; string phase_task_id = 6; string assignee_id = 7;
  int32 page_size = 8;                   // mặc định 20, tối đa 100, tính theo Request (xem 2.5)
  string page_token = 9; repeated string categories = 10;
}
message ListBacklogResponse {
  repeated BacklogRequestRow request_rows = 1;   // view REQUEST
  repeated BacklogGroup groups = 2;              // view TASK, EXECUTE
  string next_page_token = 3;
}
message BacklogRequestRow { string request_id = 1; int64 number = 2; string title = 3; string type = 4; string source_provider = 5; string source_ref = 6; string source_url = 7; string returned_from_stage = 8; string returned_category = 9; string return_reason = 10; string returned_by = 11; google.protobuf.Timestamp returned_at = 12; repeated string parent_request_ids = 13; }
message BacklogGroup { string request_id = 1; string plan_task_id = 2; string plan_title = 3; string phase_task_id = 4; string phase_title = 5; string gate_status = 6; repeated BacklogTaskRow tasks = 7; }
message BacklogTaskRow { string task_id = 1; string title = 2; string status = 3; google.protobuf.DoubleValue estimated_hours = 4; string assignee_id = 5; repeated string blocked_by_task_ids = 6; string last_engine = 7; string last_link_status = 8; int32 failed_attempts = 9; string last_error = 10; }
```

`view` bắt buộc (`REQUEST_BACKLOG_INVALID_VIEW`, InvalidArgument). Chỉ đọc, không ghi, không phát sự kiện. Quyền: người dùng chỉ thấy Request mình được xem theo chính sách CR-REQ-010/016; `ListBacklog` lọc theo tập đó trước khi gọi `task-service` (xem mục 6, vì `ListTasks` không lọc theo grant).

### 2.2 View Request backlog

Một truy vấn lên bảng `requests` của CR-REQ-002, dùng chỉ mục `(tenant_id, status, updated_at DESC)`:

```
SELECT ... FROM requests
WHERE tenant_id = :tenant AND status = 'request_backlog'
  [AND project_id = :p] [AND type IN (...)]
  AND (updated_at, id) < (:cursor_updated_at, :cursor_id)      -- keyset
ORDER BY updated_at DESC, id DESC LIMIT :page_size + 1
```

- Postgres dùng so sánh hàng `(updated_at, id) < ($a, $b)`. MySQL 8 chấp nhận cú pháp này nhưng tối ưu kém; dùng dạng khai triển `updated_at < ? OR (updated_at = ? AND id < ?)` cho cả hai dialect để cùng kết quả.
- `parent_request_ids`: một truy vấn `SELECT child_request_id, parent_request_id FROM request_links WHERE tenant_id = ? AND child_request_id IN (...)` cho cả trang.
- `returned_by`, `returned_at`: một truy vấn `SELECT request_id, actor_id, at FROM request_return_history WHERE tenant_id = ? AND action = 'returned' AND request_id IN (...)`, lấy dòng `at` lớn nhất của mỗi Request trong bộ nhớ (chỉ mục `(tenant_id, request_id, at)` của CR-REQ-006). Không dùng `updated_at` làm thời điểm trả.
- Gom theo lý do: trả `returned_category` (`missing_info`, `infeasible`, `blocked_dependency`, `rejected`, `other`) và `returned_from_stage`; việc gom do frontend làm. Thêm bộ lọc `categories` vào `ListBacklogRequest` (trường 10, `repeated string`), điều kiện `returned_category IN (...)`.
- Không gọi `task-service`.

### 2.3 Khái niệm "cổng của task" (dùng cho hai view còn lại)

Task làm việc: `task_type` thuộc `task`, `bug`, `feature` (không `epic`, `plan`, `phase`) có `request_id` thuộc Request ứng viên. Với mỗi task xác định **container cổng** là con đường lên gần nhất: cha là `phase` thì Phase đó, cha là `plan` thì Plan đó, không cha thì không container (hotfix). Task con của task khác tính theo container của tổ tiên lá gần nhất; bản đầu chỉ xét task là con trực tiếp của `phase`/`plan` và task `single_task`, task lồng sâu hơn bỏ qua (Q4).

Điều kiện "cổng đã duyệt" theo `FlowFor(type)` (CR-REQ-003):

| Task nằm ở | Yêu cầu để "đã duyệt" |
|---|---|
| dưới `phase`, `ExecutionGates` có `phase` (`change_request`) | Approval `phase` `approved` trên Phase đó **và** Approval `plan` `approved` trên Plan |
| dưới `phase`, loại không có `phase` gate (`bug`/`refactor` size L) | Approval `plan` `approved` trên Plan cha của Phase |
| dưới `plan`, loại `PhasesFor(size)` sai | Approval `plan` hoặc `task_list` `approved` (theo `Plan.Kind`); `security` dùng Approval `pre_deploy` `approved` trên Plan |
| dưới `plan`, loại `PhasesFor(size)` đúng (chưa chia Phase) | không bao giờ đạt: "Plan chưa chia Phase" |
| `single_task` (hotfix) | Approval `pre_deploy` `approved` trên task |

Cổng đã duyệt khi bản ghi mới nhất (`created_at`) của `(subject_type, subject_id)` là `approved`; bản mới nhất là `pending` (đã đòi duyệt lại) hoặc `rejected` thì chưa duyệt. Truy vấn theo Request, dùng chỉ mục `(tenant_id, request_id, created_at)` của CR-REQ-009, không cần chỉ mục mới: `SELECT subject_type, subject_id, status, created_at FROM approvals WHERE tenant_id = ? AND request_id IN (...) AND subject_type IN ('plan','task_list','phase','pre_deploy')`, rồi lắp ráp trong bộ nhớ. CR-REQ-009 có chỉ mục duy nhất một `pending` mỗi chủ thể nên "không có `pending`" là kiểm tra chắc chắn.

### 2.4 View Task backlog và Execute backlog

Loại trừ nhau theo cổng: task chưa qua cổng vào Task backlog; qua cổng thì vào Execute backlog nếu đủ điều kiện.

- **Task backlog**: Request ở `awaiting_plan_approval` hoặc `executing`; task làm việc `status IN (open, blocked)` mà cổng chưa duyệt (bảng 2.3). Nhóm theo Plan (rồi Phase).
- **Execute backlog**: Request `executing`; cổng đã duyệt; và (`status IN (open, blocked)` **hoặc** link gần nhất `failed` và `status NOT IN (done, cancelled)`). Nhóm theo Phase (hoặc Plan khi không có Phase).

Trình tự một lần gọi (không N+1):
1. `request-service`: chọn trang Request ứng viên (`status` phù hợp, lọc `project_id`, `request_types`, `request_id`), keyset `(updated_at DESC, id DESC)`, `page_size` Request (mặc định 20).
2. `task-service.ListTasks(request_ids=[...], task_types=[task,bug,feature,plan,phase])` (CR-REQ-011), một lần, phân trang nội bộ đến hết (mỗi Plan tối đa 100 task, CR-REQ-012).
3. `request-service`: một truy vấn `approvals` theo `request_id IN (...)` cho cả trang.
4. `task-service.ListExecutionStates(task_ids=[task làm việc ứng viên])` (mục 2.6).
5. Lắp ráp trong bộ nhớ; `last_error` lấy từ `task_run_outcomes` (bản `failed` mới nhất theo `task_id`, một truy vấn `IN`); `gate_status` ∈ `approved`, `pending`, `rejected`, `none` (`pending` khi Approval đang chờ; `rejected` khi bản mới nhất bị từ chối).

### 2.5 Phân trang

Theo Request, không theo task: một trang trả đủ nhóm của các Request đó, nên số hàng tối đa `page_size × 100`. `page_token` là base64 `(updated_at, id)` của Request cuối. Hệ quả: Request không có task thuộc view đó vẫn tiêu một chỗ trong trang; trang có thể ít nhóm hơn `page_size` (frontend gọi tiếp khi `next_page_token` còn).

### 2.6 `task-service`: RPC `ListExecutionStates` (thuộc CR này)

```
message ListExecutionStatesRequest { repeated string task_ids = 1; }       // tối đa 500: TASK_STATES_TOO_MANY_IDS
message ExecutionState {
  string task_id = 1;
  repeated string blocked_by_task_ids = 2;      // đích depends_on chưa done/cancelled
  string last_engine = 3; string last_link_status = 4;   // status_mirror của link gần nhất, rỗng nếu chưa có
  google.protobuf.Timestamp last_started_at = 5; google.protobuf.Timestamp last_completed_at = 6;
  int32 failed_attempts = 7;                    // số link status_mirror = 'failed'
}
message ListExecutionStatesResponse { repeated ExecutionState states = 1; }
```

Use case `usecase/list_execution_states.go` (mới), cổng `ExecutionStateReader` hiện thực ở hai adapter (`adapter/postgres/execution_states.go`, `adapter/mysql/execution_states.go`, mới). Mọi truy vấn có `tenant_id`.

- Link gần nhất: Postgres `SELECT DISTINCT ON (task_id) task_id, engine, status_mirror, started_at, completed_at FROM task.execution_links WHERE tenant_id = $1 AND task_id = ANY($2::uuid[]) ORDER BY task_id, started_at DESC, id DESC`. MySQL `ROW_NUMBER() OVER (PARTITION BY task_id ORDER BY started_at DESC, id DESC)` rồi lọc `rn = 1` (MySQL ≥ 8.0, cùng nền với yêu cầu `SKIP LOCKED` của CR-DB-002; chưa kiểm chứng TiDB).
- Số lần lỗi: Postgres `count(*) FILTER (WHERE status_mirror = 'failed')`, MySQL `SUM(status_mirror = 'failed')`, nhóm theo `task_id`.
- Bị chặn bởi: `SELECT e.from_task_id, e.to_task_id FROM task_edges e JOIN tasks t ON t.id = e.to_task_id WHERE e.tenant_id = ? AND e.edge_type = 'depends_on' AND e.from_task_id IN (...) AND t.status NOT IN ('done','cancelled')` (chỉ mục `task_edges_from_idx`).
- Quyền: giống `ListTasks` (chỉ kiểm tenant, xem mục 6).

### 2.7 Lỗi

`REQUEST_BACKLOG_INVALID_VIEW`, `REQUEST_BACKLOG_BAD_PAGE_TOKEN` (InvalidArgument), `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE` (Unavailable; view TASK/EXECUTE không trả dữ liệu từng phần), `TASK_STATES_TOO_MANY_IDS`.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Một RPC `ListBacklog` với `view`, đúng README 3.6 | Gateway và MCP chỉ cần một kênh `backlog.*` |
| Truy vấn chạy ở `request-service`, `task-service` chỉ đưa dữ liệu | Duyệt (Approval) và loại Request ở `request-service`; không đưa Approval sang `task-service` |
| Chỉ một RPC mới ở `task-service` | `ListTasks` (CR-REQ-011) đã đủ lấy cây; thiếu mỗi link và cạnh theo lô |
| Phân trang theo Request | Task của một Plan được duyệt cùng nhau; cắt trang giữa Plan làm nhóm vỡ |
| Hai view task loại trừ nhau theo cổng | Một task không xuất hiện hai nơi |
| Lỗi toàn bộ nếu `task-service` không sẵn | Trả một nửa gây hiểu nhầm là backlog rỗng |
| `last_error` từ `task_run_outcomes`, không từ link | Link không lưu thông điệp lỗi |

## 4. Tiêu chí chấp nhận

- [ ] View REQUEST chỉ trả Request `request_backlog`, đúng thứ tự `updated_at DESC, id DESC`, phân trang ổn định khi có hàng mới chen vào; `parent_request_ids` đúng với `request_links`.
- [ ] Task `open` dưới Plan chưa có Approval `approved` xuất hiện ở TASK, không ở EXECUTE; duyệt Plan xong (và Phase nếu `change_request`) thì chuyển sang EXECUTE ở lần gọi kế tiếp.
- [ ] `change_request`: task dưới Phase chưa duyệt vẫn ở TASK dù Plan đã duyệt; Phase duyệt xong thì sang EXECUTE.
- [ ] Task dưới Plan của `bug` size L chưa chia Phase ở TASK; sau khi chia Phase và Plan duyệt thì EXECUTE.
- [ ] Task `open` dưới Plan không Phase của `task`/`docs` đã duyệt ở EXECUTE; hotfix ở EXECUTE sau Approval `pre_deploy`.
- [ ] Task `blocked` ở EXECUTE có `blocked_by_task_ids` đúng; task có link gần nhất `failed` (kể cả đang `review` sau lần chạy lỗi) hiện kèm `failed_attempts`, `last_error`; task `done`, `cancelled` không bao giờ xuất hiện.
- [ ] Số lần gọi: một lần `ListBacklog` TASK/EXECUTE cho 20 Request dùng đúng một `ListTasks` (cộng trang), một `ListExecutionStates` và một truy vấn `approvals` (kiểm bằng đếm lời gọi trong test).
- [ ] `ListExecutionStates` đúng trên cả hai dialect: link gần nhất, đếm lỗi, `blocked_by`; hơn 500 id bị `TASK_STATES_TOO_MANY_IDS`.
- [ ] Người dùng không có quyền xem Request không thấy dòng của Request đó ở cả ba view.
- [ ] `task-service` không sẵn sàng: TASK/EXECUTE trả `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE`, REQUEST vẫn chạy.
- [ ] Không có file mới tên `helpers`, `utils`, `common`, `misc`.

## 5. Kiểm thử

- **Unit:** hàm xác định cổng theo bảng 2.3 cho 11 loại và size (table-driven, dùng `FlowFor` thật); lắp ráp view với fake `TaskClient`, `ApprovalReader`; mã hoá và giải mã `page_token`.
- **Integration, cả hai dialect:** truy vấn REQUEST với keyset và 1.000 hàng; `ListExecutionStates` với dữ liệu: nhiều link mỗi task, link cùng `started_at`, task không link, cạnh tới task `done`; truy vấn `approvals` dùng chỉ mục (`EXPLAIN` không phải test bắt buộc, ghi nhận kế hoạch).
- **Hợp đồng:** proto `buf breaking`; test đối chiếu mỗi điều kiện README 3.8 với một fixture.
- **E2E (CR-REQ-025):** Request qua đủ vòng đời, kiểm task chuyển view đúng thời điểm.
- Chưa chạy test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- `ListTasks` và `ListExecutionStates` chỉ kiểm tenant, không kiểm grant người gọi (xác nhận ở `usecase/list_tasks.go`). Lọc theo quyền phải làm ở `request-service` bằng danh sách Request được xem; chưa có quy tắc rõ trong CR-REQ-010/016.
- Hiệu năng: chưa đo với số Request lớn; mỗi lượt gọi tối đa `page_size` Request, mỗi Request tới 100 task, nên tối đa 2.000 hàng, và `IN (...)` lớn trên MySQL. Chưa đo.
- `execution_links` tăng không giới hạn theo số lần chạy; truy vấn dựa chỉ mục `(task_id, started_at DESC)`. Chưa có dọn dẹp.
- Quy tắc "bản ghi mới nhất theo `created_at`" giả định `approvals` giữ nhiều bản ghi cho một chủ thể (CR-REQ-009 có `created_at` và chỉ mục một `pending` mỗi chủ thể, gợi ý đúng); chưa kiểm chứng.
- Link `completed` của Engine 2/3 được đánh dấu ngay sau dispatch (CR-TG-008), nên `last_link_status` không đủ để biết run đã thật sự xong; view dùng `status` của task làm nguồn chính, link chỉ để phát hiện `failed`.

## 7. Câu hỏi mở

- **Q1.** README 3.8 viết Execute backlog là "task dưới Phase đã approved", nhưng nhiều loại không có Phase (README 3.4: `task`, `docs`, `hotfix`, `security`, `performance`, `ops_request`, và `bug`, `refactor` khi size không phải L). CR này mở rộng: "cổng đã duyệt" theo bảng 2.3. Cần xác nhận cách đọc, và sửa README.
- **Q2.** README v6 3.5 thiếu `returned_category` và `request_return_history` mà CR-REQ-006 đã thêm; cần cập nhật README. CR này giả định hai thứ đó tồn tại.
- **Q3.** Task `review` (khi `REQUEST_AUTO_COMPLETE_TASKS` tắt) chưa xong có thuộc Execute backlog? CR loại (chỉ `open`, `blocked`, hoặc link `failed`).
- **Q4.** Task lồng dưới task làm việc (subtask) có cần hiển thị? CR bản đầu bỏ qua.
- **Q5.** `ListBacklog` cần cho cả luồng không Request (task đơn lẻ)? CR chỉ phục vụ task có `request_id`.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.5, 3.6, 3.8
- `/opt/repos/orca/docs/research/receive-request/request-pipeline-existing-capabilities-and-build-scope.md` mục 4.5
- `/opt/repos/orca/docs/crs/v6/request-service-foundation/CR-REQ-002-request-data-model-and-repositories.md`, `/opt/repos/orca/docs/crs/v6/request-lifecycle/CR-REQ-003-request-state-machine-and-flow-registry.md`, `CR-REQ-006-return-to-backlog-reopen-cancel-child-requests.md`, `/opt/repos/orca/docs/crs/v6/approval/CR-REQ-009-generic-approval-domain-and-api.md`
- `/opt/repos/orca/docs/crs/v6/plan-phase-task/CR-REQ-011-task-service-plan-phase-task-types.md`, `CR-REQ-012-plan-phase-task-generation-from-solution.md`, `CR-REQ-013-phase-execution-and-feedback-loop.md`
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0010_execution_links.up.sql`, `0013_execution_leases.up.sql` và bản `mysql`; `0001_init.up.sql` (`task_edges`, chỉ mục)
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/execution_link.go`, `task_edge.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/list_tasks.go`, `get_dependencies.go`, `report_execution_result.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/postgres/execution_links.go`, `adapter/mysql/execution_links.go`
- `/opt/repos/orca/backend-go/proto/orca/task/v1/task.proto`
- `/opt/repos/orca/docs/crs/v4/task-graph/CR-TG-008-jira-source-link-and-durable-direct-agent.md`
- Mới: `request-service/internal/usecase/list_backlog.go`, `internal/domain/backlog_gate.go`, `internal/adapter/grpcclient/task_client.go` (dùng chung với CR-REQ-013); `task-service/internal/usecase/list_execution_states.go`, `adapter/postgres/execution_states.go`, `adapter/mysql/execution_states.go`
