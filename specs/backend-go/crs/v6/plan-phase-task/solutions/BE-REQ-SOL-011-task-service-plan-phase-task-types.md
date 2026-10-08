# BE-REQ-SOL-011: `task-service` mở `task_type` plan/phase, lọc `ListTasks`, trạng thái suy ra từ con, không cấp số task

> ✅ Đã triển khai (kiểm chứng 2026-10-07): 7/7 task xong; test đơn vị xanh, test integration chạy thật trên Postgres 16 và MySQL 8.0.46. Chi tiết lệch so với thiết kế: [IMPLEMENTATION-NOTES](../IMPLEMENTATION-NOTES.md).

**CR:** [CR-REQ-011](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-011-task-service-plan-phase-task-types.md)
**Service:** `task-service` · `proto/orca/task/v1/task.proto` · (`api-gateway` chỉ chuyển tiếp tham số mới, thuộc CR-REQ-016)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain thuần, usecase gọi port), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect, outbox cùng transaction), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (subject sự kiện), [`services/task-service.md`](../../../../tdd/services/task-service.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc trực tiếp ngày 2026-10-06: `migrations/{postgres,mysql}/` (cả hai dừng ở `0014_task_sources_site`, nên số kế tiếp thật là **0015**), `0003_task_fields_and_comments.up.sql`, `internal/domain/task.go` (`Type string` dòng 92; `SetStatus` dòng 232), `usecase/{create_task,list_tasks,update_task,add_edge,ai_apply,execute_task,execution_lease,ports}.go`, `adapter/{postgres,mysql}/{repository,execution_leases,velocity}.go`, `adapter/grpc/server.go` (661 dòng), `proto/orca/task/v1/task.proto`.

Khớp với CR gốc:
- Postgres `task_type` là CHECK inline ở `0003` dòng 3, tên mặc định `tasks_task_type_check`; MySQL `ADD CONSTRAINT tasks_task_type_check` ở `0003` dòng 26, cột `VARCHAR(10)`.
- `server.go` `CreateTask` (dòng 134 đến 147) chỉ chuyển `Title`, `ParentId`, `ProjectId`, `CreatorId`; `task_type`, `description`, `priority`... bị bỏ.
- `Repository.Create` Postgres gọi `nextval('task.task_number_seq')` ngay trong `INSERT`; MySQL `INSERT INTO task_number_seq VALUES (NULL)` (`mysql/repository.go` dòng 192). `scanTask` Postgres đã `COALESCE(task_number, 0)`.
- `TaskRepository.List(ctx, tenantID, projectID, pageToken, pageSize)` (`ports.go` dòng 67), chỉ `usecase/list_tasks.go` gọi.
- `AddEdge` tự đặt `blocked` ở `add_edge.go` dòng 89; `ReleaseUnlinkedInProgress` và `HasActiveExecutions` không lọc type.
- `taskStatusChangedPayload` (`update_task.go` dòng 181) chỉ có 5 trường.
- Trường cuối của `Task` proto là `share_token = 30`, của `CreateTaskRequest` là `creator_id = 14`, `ListTasksRequest` dùng 1 đến 3: số `31`, `15, 16`, `4 đến 6` còn trống.

### Quan hệ với v4 task-graph (CR-TG-001, BE-SOL-001, TASK-TG-001-0x)

CR-REQ-011 đụng cùng file với BE-SOL-001. Kiểm bằng code thật: BE-SOL-001 **đã triển khai** (có `domain/progress.go`, `usecase/recalculate_progress.go`, `usecase/get_subtree.go`, `adapter/*/subtree.go`, `TxRunner` ở `ports.go`, `labels`/`done_subtasks` ở migration `0011`). Hệ quả cho solution này:
- Không làm lại `RecalculateProgress`, `AddEdge` nguyên tử, `GetSubtree`; chỉ **tái dùng** (`SyncContainerStatus` gọi `RecalculateProgress.Execute`).
- Số migration trong tài liệu v4 (`0004_task_fields_and_comments`) đã lệch thực tế (file thật là `0003_...` và `0011_...`): không tin số trong tài liệu cũ, luôn `ls migrations/` trước khi đặt số.
- `BatchUpdateProgress` chỉ ghi `progress_percent`, không ghi `done_subtasks`/`total_subtasks` (tồn tại từ trước): UI plan/phase chỉ dùng `progress_percent`.

### Correction relative to CR-REQ-011

1. CR nói "`List` chỉ lọc project" đúng, nhưng thêm một chi tiết: cột `project_id` được so bằng `project_id::text = $2` nên chỉ mục không dùng được. Truy vấn mới phải dựng `WHERE` động và so kiểu native.
2. `ReleaseExecution` và `ClaimForExecution` nằm ở cổng riêng (`TaskExecutionReleaser`, `TaskExecutionClaimer` trong `execution_lease.go`), không ở `TaskRepository`; phần thêm tham số sự kiện thuộc BE-REQ-SOL-013, không ở đây.

## 2. Giải pháp

### 2.1 Cây file

```
backend-go/services/task-service/
  migrations/postgres/0015_task_type_plan_phase.{up,down}.sql        (mới)
  migrations/postgres/0016_task_request_id.{up,down}.sql             (mới)
  migrations/mysql/0015_task_type_plan_phase.{up,down}.sql           (mới)
  migrations/mysql/0016_task_request_id.{up,down}.sql                (mới)
  internal/domain/task_type.go              (mới)  ParseTaskType, IsContainerType, hằng
  internal/domain/container_status.go       (mới)  DeriveContainerStatus
  internal/domain/task.go                   (sửa)  RequestID, comment Type, lỗi mới
  internal/usecase/create_task.go           (sửa)  validate type/phân cấp, RequestID, Labels
  internal/usecase/list_tasks.go            (sửa)  ListFilter
  internal/usecase/sync_container_status.go (mới)
  internal/usecase/reconcile_container_statuses.go (mới)
  internal/usecase/ports.go                 (sửa)  ListFilter, ListChildStatuses, UpdateContainerStatus
  internal/usecase/{update_task,execute_task,report_execution_result,add_edge,ai_apply}.go (sửa nhỏ)
  internal/adapter/{postgres,mysql}/repository.go (sửa) taskColumns + request_id, Create bỏ số cho container
  internal/adapter/{postgres,mysql}/task_list_query.go   (mới)
  internal/adapter/{postgres,mysql}/container_status.go  (mới)
  internal/adapter/{postgres,mysql}/execution_leases.go  (sửa)  loại container khỏi sweep
  internal/adapter/{postgres/velocity.go,mysql/velocity.go} (sửa)
  internal/adapter/grpc/server.go           (sửa)  CreateTask chuyển đủ trường, ListTasks, toProtoTask
backend-go/proto/orca/task/v1/task.proto    (sửa)
```

### 2.2 Domain

```go
// domain/task_type.go
const (TypeTask = "task"; TypeBug = "bug"; TypeFeature = "feature"; TypeEpic = "epic"; TypePlan = "plan"; TypePhase = "phase")
func ParseTaskType(s string) (string, error)   // "" -> task; lạ -> ErrInvalidTaskType
func IsContainerType(s string) bool            // plan, phase
```

Lỗi domain: `ErrInvalidTaskType`, `ErrPlanCannotHaveParent`, `ErrPhaseRequiresPlanParent`, `ErrContainerUnderWorkTask`, `ErrContainerStatusDerived`. `Task.Type` giữ `string` để không vỡ adapter và proto. `Task.RequestID string` bất biến sau khi tạo.

`DeriveContainerStatus(children []Status) (Status, bool)` theo bảng 8 dòng của CR (bỏ con `cancelled`; không con thì `false`; toàn `cancelled` thì `cancelled`; toàn `done` thì `done`; có `in_progress` thì `in_progress`; toàn {`done`,`review`} có `review` thì `review`; lẫn `done/review` với `open/blocked` thì `in_progress`; toàn `blocked` thì `blocked`; còn lại `open`). Hàm thuần, không I/O.

### 2.3 Migration (cả hai dialect, số 0015 và 0016)

`0015_task_type_plan_phase`: Postgres `DROP CONSTRAINT tasks_task_type_check` rồi `ADD CONSTRAINT ... CHECK (task_type IN ('task','bug','feature','epic','plan','phase'))`; MySQL `DROP CHECK` rồi `ADD CONSTRAINT` (CHECK chỉ thực thi từ MySQL 8.0.16, ghi ở comment). Down: `UPDATE ... SET task_type='epic' WHERE task_type IN ('plan','phase')` trước khi khôi phục CHECK cũ.

`0016_task_request_id`:
```sql
-- Postgres
ALTER TABLE task.tasks ADD COLUMN request_id UUID;
CREATE INDEX idx_tasks_request ON task.tasks (tenant_id, request_id) WHERE request_id IS NOT NULL;
CREATE UNIQUE INDEX uq_tasks_active_plan_per_request ON task.tasks (tenant_id, request_id)
  WHERE task_type = 'plan' AND status <> 'cancelled';
-- MySQL
ALTER TABLE tasks ADD COLUMN request_id CHAR(36) NULL,
  ADD COLUMN active_plan_request_id CHAR(36) GENERATED ALWAYS AS
    (CASE WHEN task_type = 'plan' AND status <> 'cancelled' THEN request_id END) STORED;
CREATE INDEX idx_tasks_request ON tasks (tenant_id, request_id);
CREATE UNIQUE INDEX uq_tasks_active_plan_per_request ON tasks (tenant_id, active_plan_request_id);
```
Không FK. Chỉ mục duy nhất chống hai `CreatePlanTree` đua nhau (BE-REQ-SOL-012).

### 2.4 `CreateTask`, handler gRPC, `AIApply`

`CreateTaskInput` thêm `RequestID`, `Labels`. Quy tắc phân cấp (sau bước đọc parent đã có ở `create_task.go`):

| Type mới | Quy tắc | Mã lỗi |
|---|---|---|
| `plan` | không parent; cần `ProjectID` | `TASK_PLAN_CANNOT_HAVE_PARENT`, `TASK_PLAN_PROJECT_REQUIRED` (InvalidArgument) |
| `phase` | parent phải là `plan`, cùng `project_id` | `TASK_PHASE_REQUIRES_PLAN_PARENT` |
| `task/bug/feature/epic` | cha là `plan`, `phase` hoặc task làm việc | không lỗi |
| `plan`/`phase` dưới task làm việc | chặn | `TASK_CONTAINER_UNDER_WORK_TASK` |
| khác | chặn | `TASK_INVALID_TYPE` (InvalidArgument) |

Task con kế thừa `request_id` của parent khi không truyền. Thêm kiểm `priority` (`low|medium|high|urgent`) và `visibility` (`private|team|public`) ở domain để lỗi trả `TASK_INVALID`, không để CHECK DB nổ thành `TASK_CREATE_FAILED`. Handler `server.go` chuyển đủ `TaskType`, `Description`, `Priority`, `EstimatedHours`, `PromptTemplate`, `AiContext`, `Visibility`, `RequestId`, `Labels`; đặt phần chuyển đổi vào hàm `toCreateTaskInput` trong file mới `adapter/grpc/server_create_task.go` để không phình `server.go`. `AIApply` chuẩn hoá `SubtaskProposal.Type` ngoài `task|bug|feature` về `task`.

### 2.5 Không cấp `TaskNumber` cho container

`Create` Postgres rẽ hai câu INSERT: container bỏ cột `task_number` và `nextval`; MySQL container không `INSERT INTO task_number_seq`. `idx_tasks_project_task_number` giữ nguyên (NULL không vi phạm duy nhất ở cả hai dialect). `toProtoTask` trả `task_number = 0`; `FindTaskByNumber` không bao giờ trả plan/phase.

### 2.6 `ListTasks`

Proto `ListTasksRequest`: `repeated string task_types = 4; repeated string request_ids = 5; string parent_id = 6;`. `task_types` rỗng thì trả `task,bug,feature,epic` (client cũ không thấy plan/phase). `request_ids` tối đa 100 (`TASK_LIST_TOO_MANY_REQUEST_IDS`). Port đổi thành `List(ctx, tenantID string, f ListFilter) ([]domain.Task, string, error)` với `ListFilter{ProjectID, TaskTypes, RequestIDs, ParentID, PageToken, PageSize}`. SQL dựng động trong `task_list_query.go` mỗi dialect: Postgres `task_type = ANY($n::text[])`, `request_id = ANY($n::uuid[])`; MySQL `IN (?,...)`. `ORDER BY id`, token là id cuối.

### 2.7 Trạng thái suy ra: `SyncContainerStatus`

Port thêm vào `TaskRepository`: `ListChildStatuses(ctx, tenantID, parentID) ([]domain.Status, error)` và `UpdateContainerStatus(ctx, tenantID, id string, from, to domain.Status, events []domain.OutboxEvent) (bool, error)` (compare-and-set `WHERE id=? AND status=from AND task_type IN ('plan','phase')`, kèm INSERT `outbox_events` cùng transaction, mẫu `Repository.Update` dòng 425).

`SyncContainerStatus.Execute(ctx, changedTaskID)`: `Get` task; parent không phải container thì dừng; tính `DeriveContainerStatus`; CAS tối đa 3 lần; gọi `RecalculateProgress.Execute(parentID)` best-effort; lặp lên cấp trên. Phát `orca.task.task.statuschanged` với `cause=derived` (không phát `completed`). `taskStatusChangedPayload` thêm bốn trường `omitempty`: `task_type`, `parent_id`, `request_id`, `cause`.

Điểm gọi: `UpdateTask` sau `repo.Update`; `ExecuteTask` sau claim và ở các nhánh hoàn tác (`execute_task.go` dòng 250, 262, 316, 376); `dispatchDirectAgentAsync` sau `CompleteExecution` (dòng 385); `ReportTaskExecutionResult` hai nhánh. Vòng phục hồi thêm `ReconcileContainerStatuses` chọn container có `updated_at < MAX(child.updated_at)`.

Chặn tác dụng phụ: `AddEdge` không `blocked` khi `FromTaskID` là container; `ReleaseUnlinkedInProgress`, `HasActiveExecutions`, `RecentCompletedTasks` thêm `task_type NOT IN ('plan','phase')`; `UpdateTask` trên container chỉ nhận `status` nil hoặc `cancelled` (`TASK_CONTAINER_STATUS_DERIVED`); `ExecuteTask` trên container trả `TASK_EXECUTE_CONTAINER_NOT_EXECUTABLE` ngay sau `Get`.

### 2.8 Proto (chỉ thêm, `buf breaking` xanh)

`Task.request_id = 31`; `CreateTaskRequest.request_id = 15`, `labels = 16`; `ListTasksRequest` 4 đến 6; cập nhật comment `task_type` ghi sáu giá trị.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Trạng thái container tính lại từ toàn bộ con | Idempotent, chịu giao lặp và đảo thứ tự; lần gọi sau tự sửa lần mất |
| Ghi qua `UpdateContainerStatus`, không nới `SetStatus` | Giữ bất biến `ErrCannotSetInProgress` (TASK-223) |
| Mặc định `ListTasks` ẩn plan/phase | Tương thích ngược, Board không đổi |
| Chỉ mục duy nhất một Plan hoạt động mỗi Request | Chống đua ở `CreatePlanTree`; MySQL dùng generated column như `task_sources` |
| Tách `server_create_task.go` | `server.go` đã 661 dòng; không thêm `max-lines` disable |
| Kiểm `priority`/`visibility` ở domain | Handler bắt đầu lưu trường trước đây bị bỏ; tránh lỗi DB thô |

## 4. Phụ thuộc và thứ tự

Không phụ thuộc `request-service`. Task theo thứ tự: 01 migration; 02 domain và cột `request_id`; 03 `CreateTask`; 04 `ListTasks`; 05 `SyncContainerStatus`; 06 điểm gọi và đối soát; 07 chặn tác dụng phụ. 03 và 04 song song được sau 02. Mở khoá BE-REQ-SOL-012, 013, 015.

## 5. Kiểm thử

- **Unit:** `ParseTaskType`, `IsContainerType`, `DeriveContainerStatus` (bảng 8 dòng cộng không con, toàn `cancelled`), quy tắc phân cấp, `SyncContainerStatus` với fake repo (thua CAS, lên hai cấp, không con), handler chuyển đủ trường. Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/task-service/...`.
- **Integration hai dialect** (`-tags=integration`, theo `repository_test.go`): migration up/down, CHECK, chỉ mục duy nhất Plan, cấp số bỏ qua container (20 tạo đồng thời), `List` từng bộ lọc, CAS 8 goroutine, sweep không đụng container.
- **Hợp đồng:** `make proto-lint`; test đọc `information_schema` so cột, chỉ mục, CHECK ở hai DB.

## 6. Rủi ro và điểm chưa kiểm chứng

- `Repository.Update` ghi lại toàn hàng từ bản đọc trước: sửa tiêu đề container đua với ghi suy ra có thể đè trạng thái. Giảm nhẹ bằng `SyncContainerStatus` sau mọi `UpdateTask` và vòng `Reconcile`. Chưa đo.
- MySQL generated column `STORED` với `CASE` chưa chạy trên MySQL/TiDB mục tiêu. CHECK MySQL cần 8.0.16.
- `DeleteTask` một Plan xoá cascade phase và task con (FK `parent_id ON DELETE CASCADE`); CR-REQ-006 phải dùng `cancelled`.
- Hành vi mới của `CreateTask` gRPC làm lộ dữ liệu client cũ từng gửi sai (đã giảm bằng kiểm `priority`/`visibility`).
- Đổi chữ ký `List` cần `gitnexus_impact` và grep lại trước khi sửa vì chỉ mục có thể cũ.
- Số migration 0015/0016 có thể va nếu CR khác chen vào trước; `ls migrations/` ngay trước khi tạo file.

## 7. Câu hỏi mở

- Container `done` có mở lại khi thêm con mới (replan)? CR cho phép và log Warn.
- Huỷ Plan có cascade `cancelled` xuống con ở `task-service`? Hiện để `request-service` làm (CR-REQ-006, 012).
- Có cần trần `page_size` cho `ListTasks`? CR-REQ-015 cần trang lớn.
- Tham chiếu tiến: CR-REQ-029 (TaskSpec, ReadinessGate) có thể thêm trường vào `Task` và prompt; solution này không phụ thuộc, `Task` proto số `31` có thể bị CR đó dùng, kiểm lại khi merge.

## 8. Tham chiếu

- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0003_task_fields_and_comments.up.sql`, `0008_task_outbox_and_number.up.sql`, `0012_task_sources.up.sql`; bản `mysql`
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/task.go`, `progress.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/create_task.go`, `list_tasks.go`, `update_task.go`, `add_edge.go`, `ai_apply.go`, `execute_task.go`, `execution_lease.go`, `ports.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/postgres/repository.go`, `execution_leases.go`, `velocity.go`; `adapter/mysql/*`; `adapter/grpc/server.go`
- `/opt/repos/orca/backend-go/proto/orca/task/v1/task.proto`
- `/opt/repos/orca/specs/backend-go/crs/v4/task-graph/solutions/BE-SOL-001-orcatask-data-model-widening.md`
