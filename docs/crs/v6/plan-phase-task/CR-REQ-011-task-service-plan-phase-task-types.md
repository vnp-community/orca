# CR-REQ-011 — `task-service`: type `plan`/`phase`, lọc, trạng thái suy ra từ con, không cấp số task

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-011 |
| **Tên** | Mở `task_type` cho `plan` và `phase`; lọc `ListTasks` theo type và Request; plan/phase suy trạng thái từ con; plan/phase không nhận `TaskNumber`; thêm cột `request_id` |
| **Loại** | Feature (thay đổi nhỏ ở service có sẵn) |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | Không phụ thuộc kỹ thuật vào `request-service`; theo hợp đồng README v6 (D2, O1, O2, mục 3.5) |
| **Mở khoá** | CR-REQ-012, 013, 015; frontend CR-REQ-021 |
| **Tác động** | `backend-go/services/task-service` (domain, usecase, adapter/grpc, adapter/postgres, adapter/mysql, migrations 0015, 0016), `backend-go/proto/orca/task/v1/task.proto`, `api-gateway` (chỉ chuyển tiếp tham số mới, thuộc CR-REQ-016) |

## 1. Bối cảnh và vấn đề

D2 chốt dùng lại Task cho Plan và Phase. Đọc code ngày 2026-10-05 cho thấy năm chỗ phải sửa, và hai chỗ README v6 chưa nêu:

1. `task_type` bị khoá: Postgres `CHECK (task_type IN ('task','bug','feature','epic'))` ghi inline ở `migrations/postgres/0003_task_fields_and_comments.up.sql` (tên mặc định `tasks_task_type_check`); MySQL có CHECK cùng tên ở `migrations/mysql/0003_task_fields_and_comments.up.sql` (`ADD CONSTRAINT tasks_task_type_check`), chỉ được thực thi từ MySQL 8.0.16.
2. `domain.Task.Type` là `string` không kiểm tra; `CreateTask` không validate type. Tệ hơn, handler gRPC `CreateTask` trong `internal/adapter/grpc/server.go` chỉ chuyển `Title`, `ParentId`, `ProjectId`, `CreatorId` vào `CreateTaskInput`: `task_type`, `description`, `priority`, `estimated_hours`, `prompt_template`, `ai_context` có trong `CreateTaskRequest` nhưng bị bỏ. Hiện chưa ai tạo được task `type=bug` qua gRPC `CreateTask` (chỉ `AIApply` truyền `Type` trực tiếp ở tầng usecase).
3. `TaskNumber` do repository gán trong `Create`: Postgres `nextval('task.task_number_seq')` nằm ngay trong câu `INSERT`; MySQL `INSERT INTO task_number_seq VALUES (NULL)` rồi `LastInsertId`. Mọi task đều tiêu một số.
4. `ListTasks` chỉ lọc `project_id`, `List(ctx, tenantID, projectID, pageToken, pageSize)` trong `usecase/ports.go`. Khi có plan/phase, Board và danh sách task cũ sẽ lẫn hai loại này.
5. Trạng thái: `Task.SetStatus` từ chối `in_progress` (`ErrCannotSetInProgress`) và mọi chuyển khỏi `done`/`cancelled` (`ErrTerminalStatus`). Cascade tiến độ `RecalculateProgress` chỉ chạy từ `UpdateTask` khi con đạt `done`/`cancelled`, không chạy từ đường `ExecuteTask`, `ReportTaskExecutionResult` hay vòng phục hồi. Và `BatchUpdateProgress` chỉ ghi `progress_percent`, không ghi `done_subtasks`/`total_subtasks`.
6. Chưa nêu trong README: `ExecuteTask` không kiểm type; vòng quét `ReleaseUnlinkedInProgress` (cả hai dialect, `adapter/*/execution_leases.go`) trả mọi task `in_progress` không có link quá 30 phút về `open`. Nếu plan/phase mang `in_progress` suy ra từ con thì bị vòng này đặt lại.
7. Chưa nêu trong README: `AddEdge` tự đặt task phụ thuộc thành `blocked` khi đích chưa `done`; áp cho plan/phase thì trạng thái suy ra bị ghi đè.

## 2. Giải pháp đề xuất

### 2.1 Domain

`domain/task_type.go` (mới): hằng `TypeTask`, `TypeBug`, `TypeFeature`, `TypeEpic`, `TypePlan`, `TypePhase` (giá trị chuỗi như README); `ParseTaskType(s string) (string, error)` (rỗng thành `task`, lạ thành `ErrInvalidTaskType`); `IsContainerType(s)` đúng với `plan`, `phase`. `Task.Type` giữ kiểu `string` để không vỡ adapter và proto.

`domain/task.go`: thêm `RequestID string` (id Request ở `request-service`, không FK, bất biến sau khi tạo); sửa comment `Type // task|bug|feature|epic|plan|phase`.

Lỗi domain mới: `ErrInvalidTaskType`, `ErrPlanCannotHaveParent`, `ErrPhaseRequiresPlanParent`, `ErrContainerUnderWorkTask`, `ErrContainerStatusDerived`.

### 2.2 Migration (số kế tiếp ở cả hai dialect là 0015; hiện có 0001 đến 0014)

**`0015_task_type_plan_phase`**

- Postgres up: `ALTER TABLE task.tasks DROP CONSTRAINT tasks_task_type_check; ALTER TABLE task.tasks ADD CONSTRAINT tasks_task_type_check CHECK (task_type IN ('task','bug','feature','epic','plan','phase'));` (cùng cách `0003` đổi `tasks_status_check`).
- MySQL up: `ALTER TABLE tasks DROP CHECK tasks_task_type_check; ALTER TABLE tasks ADD CONSTRAINT tasks_task_type_check CHECK (task_type IN (...6 giá trị...));`. Cột `VARCHAR(10)` đủ cho `feature` (7), `plan`, `phase`.
- Down (cả hai): đổi `plan`/`phase` về `epic` (loại container gần nhất đã có) rồi khôi phục CHECK cũ. Ghi rõ trong comment migration là mất phân biệt, nên chỉ dùng khi rollback toàn bộ v6.

**`0016_task_request_id`**

- Postgres: `ALTER TABLE task.tasks ADD COLUMN request_id UUID;` `CREATE INDEX idx_tasks_request ON task.tasks (tenant_id, request_id) WHERE request_id IS NOT NULL;` và chỉ mục duy nhất `CREATE UNIQUE INDEX uq_tasks_active_plan_per_request ON task.tasks (tenant_id, request_id) WHERE task_type = 'plan' AND status <> 'cancelled';`.
- MySQL: `ALTER TABLE tasks ADD COLUMN request_id CHAR(36) NULL, ADD COLUMN active_plan_request_id CHAR(36) GENERATED ALWAYS AS (CASE WHEN task_type = 'plan' AND status <> 'cancelled' THEN request_id END) STORED;` `CREATE INDEX idx_tasks_request ON tasks (tenant_id, request_id);` `CREATE UNIQUE INDEX uq_tasks_active_plan_per_request ON tasks (tenant_id, active_plan_request_id);` (MySQL coi NULL là khác nhau, cùng ý với `project_key` ở `mysql/0012_task_sources.up.sql`).
- Không FK, không RLS thêm (bảng `task.tasks` đã có RLS). Down: bỏ chỉ mục và cột.
- Mục đích chỉ mục duy nhất: mỗi Request có tối đa một Plan chưa huỷ; chặn hai lần `GeneratePlan` đua nhau (CR-REQ-012).

### 2.3 `CreateTask`: kiểm type, phân cấp, ghi `request_id`

Sửa `usecase/create_task.go` (sau bước đọc parent đã có):

| Type của task mới | Quy tắc | Lỗi (FailedPrecondition trừ khi ghi khác) |
|---|---|---|
| `plan` | `ParentID` rỗng; `ProjectID` bắt buộc | `TASK_PLAN_CANNOT_HAVE_PARENT`; `TASK_PLAN_PROJECT_REQUIRED` (InvalidArgument) |
| `phase` | parent tồn tại, `parent.Type = plan`, cùng `project_id` | `TASK_PHASE_REQUIRES_PLAN_PARENT` |
| `task`, `bug`, `feature`, `epic` | parent là `plan`, `phase` hoặc task làm việc đều được; `plan`/`phase` không được làm con của task làm việc | `TASK_CONTAINER_UNDER_WORK_TASK` (khi tạo plan/phase dưới task làm việc) |
| khác | từ chối | `TASK_INVALID_TYPE` (InvalidArgument) |

- `request_id` chỉ đặt lúc tạo, không có đường sửa (`UpdateTaskInput` không có trường này). Task con kế thừa `request_id` của parent nếu không truyền.
- Handler `server.go` `CreateTask`: chuyển đủ `TaskType`, `Description`, `Priority`, `EstimatedHours`, `PromptTemplate`, `AiContext`, `Visibility`, `RequestId`, `Labels` vào `CreateTaskInput` (thêm `RequestID`, `Labels` vào struct). Hệ quả: client cũ đang gửi các trường này sẽ bắt đầu được lưu; trước đây bị bỏ im lặng.
- `AIApply` (`usecase/ai_apply.go`): `SubtaskProposal.Type` do AI sinh, hiện đi thẳng vào DB. Sau CR này AI có thể trả `plan` và tạo container không số. Chuẩn hoá: type ngoài `task|bug|feature` đổi về `task`.
- `domain.NewTask` không đổi chữ ký (giữ tương thích với test hiện có).

### 2.4 O2: plan/phase không nhận `TaskNumber`

Sửa tại hai nơi duy nhất cấp số: `adapter/postgres/repository.go` `Create` và `adapter/mysql/repository.go` `Create`.

- Postgres: hai câu INSERT, nhánh container bỏ cột `task_number` (cột nullable từ `0008`, `COALESCE(task_number, 0)` ở `scanTask` đã xử lý NULL), không gọi `nextval`.
- MySQL: nhánh container không `INSERT INTO task_number_seq`.
- Chỉ mục `idx_tasks_project_task_number` không cần đổi: NULL không vi phạm duy nhất ở cả hai dialect (comment ở `mysql/0008`).
- `toProtoTask` trả `task_number = 0` cho container; frontend không hiện `#TG-N` khi bằng 0 (CR-REQ-021 đã giả định). `FindTaskByNumber` không bao giờ trả plan/phase.

### 2.5 `ListTasks`: lọc theo type, Request, parent; tương thích ngược

Proto `ListTasksRequest` (thêm, không đổi số cũ 1 đến 3): `repeated string task_types = 4; repeated string request_ids = 5; string parent_id = 6;`.

- `task_types` rỗng: trả `task, bug, feature, epic`, tức không trả `plan`, `phase`. Client cũ nhận đúng tập kết quả như trước vì chưa từng có hai loại này. Muốn plan/phase phải liệt kê tường minh.
- `request_ids`: tối đa 100 phần tử (`TASK_LIST_TOO_MANY_REQUEST_IDS`, InvalidArgument); gateway ánh xạ `requestId` đơn lẻ thành mảng một phần tử.
- `parent_id`: lấy con trực tiếp (lấy cây con theo Plan: dùng `task_types=[phase]` rồi lặp, hoặc `GetSubtree`).
- Port đổi từ `List(ctx, tenantID, projectID, pageToken, pageSize)` sang `List(ctx, tenantID, ListFilter)` với `ListFilter{ProjectID, TaskTypes, RequestIDs, ParentID, PageToken, PageSize}` (struct nằm ở `usecase/ports.go`). Caller production duy nhất là `usecase/list_tasks.go`; cập nhật các fake trong `fakes_test.go`.
- SQL: bỏ kiểu `($2 = '' OR project_id::text = $2)` cho bộ lọc mới vì làm chỉ mục không dùng được. Dựng `WHERE` động trong file `adapter/postgres/task_list_query.go` và `adapter/mysql/task_list_query.go` (mới). Postgres dùng `task_type = ANY($n::text[])`, `request_id = ANY($n::uuid[])`; MySQL sinh `IN (?,?,...)` theo số phần tử.
- Phân trang giữ `ORDER BY id`, `page_token` là id cuối. Chưa có trần `page_size`; CR không thêm để khỏi đổi hành vi cũ (xem mục 6).
- `ListTasks` hiện không lọc theo grant của người gọi; CR không đổi (xem mục 6).

### 2.6 O1: trạng thái plan/phase suy ra từ con

**Quy tắc suy ra** (`domain/container_status.go`, mới): `DeriveContainerStatus(children []Status) (Status, bool)`. Bỏ qua con `cancelled`; theo thứ tự:

| Thứ tự | Điều kiện trên các con còn lại | Kết quả |
|---|---|---|
| 0 | không có con nào | giữ nguyên (`false`) |
| 1 | có con nhưng tất cả `cancelled` | `cancelled` |
| 2 | tất cả `done` | `done` |
| 3 | có con `in_progress` | `in_progress` |
| 4 | tất cả thuộc {`done`,`review`} và có ít nhất một `review` | `review` |
| 5 | có con thuộc {`done`,`review`} nhưng còn con `open`/`blocked` | `in_progress` |
| 6 | tất cả `blocked` | `blocked` |
| 7 | còn lại | `open` |

Container đang `cancelled` không bao giờ bị suy ra lại. Container đang `done` có thể mở lại khi có con mới chưa xong (ghi log mức Warn).

**Không vi phạm `ErrCannotSetInProgress`:** `SetStatus` giữ nguyên. Giá trị suy ra được ghi bằng đường repository trực tiếp (như `ExecuteTask` làm với `UpdateStatus`), không qua `SetStatus`. `UpdateTask` trên plan/phase chỉ cho `status` bằng nil hoặc `cancelled`; giá trị khác trả `TASK_CONTAINER_STATUS_DERIVED` (InvalidArgument).

**Use case `SyncContainerStatus`** (`usecase/sync_container_status.go`, mới), gọi với id của task vừa đổi:
1. `Get` task; nếu `ParentID` rỗng hoặc parent không phải container thì dừng.
2. `ListChildStatuses(ctx, tenant, parentID)` (phương thức mới của `TaskRepository`: `SELECT status FROM tasks WHERE tenant_id = ? AND parent_id = ?`), tính `DeriveContainerStatus`.
3. Nếu đổi: `UpdateContainerStatus(ctx, tenant, id, from, to, events)` là compare-and-set `UPDATE ... SET status = to, updated_at = now WHERE id = ? AND status = from AND task_type IN ('plan','phase')` kèm `INSERT` outbox cùng transaction (mẫu `Repository.Update`). Mất CAS thì đọc lại, thử tối đa 3 lần.
4. Gọi `RecalculateProgress.Execute(parentID)` (best-effort, như `UpdateTask`).
5. Lặp với parent làm "task vừa đổi" (phase lên plan). Dừng khi parent không phải container.

**Điểm gọi** (mọi nơi con đổi status): `UpdateTask` sau `repo.Update` (bỏ điều kiện chỉ khi `reachedTerminal` cho container; giữ cho cascade tiến độ cũ); `ExecuteTask` sau claim thành công và ở bốn nhánh hoàn tác về `previousStatus`; `dispatchDirectAgentAsync` sau `CompleteExecution`/hoàn tác; `ReportTaskExecutionResult` cả hai nhánh. Vòng phục hồi (`RecoverInterruptedExecutions`) không trả id task nên không gọi trực tiếp: thêm bước `ReconcileContainerStatuses` vào cùng vòng quét 30 giây, chọn container có con mới hơn: `container.updated_at < MAX(child.updated_at)`; tính lại, nếu không đổi thì "chạm" `updated_at` để khỏi quét lại.

**Sự kiện:** khi đổi, phát `orca.task.task.statuschanged` (không phát `orca.task.task.completed` cho container, tránh thông báo đẩy trùng). Payload `taskStatusChangedPayload` (trong `usecase/update_task.go`) thêm bốn trường `omitempty`: `task_type`, `parent_id`, `request_id`, `cause` (`user_update`, `derived`; CR-REQ-013 thêm các giá trị `execute_claim`, `execution_completed`, `execution_failed`, `recovery`). Tiêu thụ hiện có (`api-gateway/.../workspace_events.go`, `notification-service` rule WS-only) bỏ qua trường lạ.

**Chặn tác dụng phụ trên container:**
- `AddEdge` (`usecase/add_edge.go`, `addEdgeWithinTx`): không đặt `blocked` khi `edge.FromTaskID` là container. Thứ tự giữa các Phase do CR-REQ-013 kiểm khi `StartPhase`.
- `ReleaseUnlinkedInProgress` (postgres và mysql `execution_leases.go`): thêm `AND task_type NOT IN ('plan','phase')`, nếu không container `in_progress` suy ra bị đặt về `open` sau 30 phút.
- `HasActiveExecutions` và `RecentCompletedTasks` (`adapter/*/repository.go`, `velocity.go`): loại plan/phase (container `in_progress` không có nghĩa là có run; plan/phase `done` làm nhiễu prompt velocity).

### 2.7 `ExecuteTask` trên plan/phase

Chặn. Trong `usecase/execute_task.go`, ngay sau khi `Get` task và trước pre-check `in_progress`: nếu `IsContainerType(task.Type)` trả `TASK_EXECUTE_CONTAINER_NOT_EXECUTABLE` (FailedPrecondition). Lý do: `selectEngine` coi task có con `parent_child` là Engine 2, và `ComplexExecutor.buildSpec` chỉ mở một cấp con, nên chạy Phase sẽ dùng một worktree và một coordinator cho cả phase, trái với README v6 (CR-REQ-013 chạy task lá). `selectEngine` không đổi cho task làm việc: có con hoặc có cạnh `depends_on` thì Engine 2, ngược lại Engine 1, `workflow_template_id` thì Engine 3. Task lá có `depends_on` đi Engine 2 với spec một nút (đọc `complex_executor.go`; chưa chạy).

### 2.8 Proto (thêm, không đổi số cũ; `buf breaking` phải xanh)

`Task`: `string request_id = 31;`. `CreateTaskRequest`: `string request_id = 15; repeated string labels = 16;`. `ListTasksRequest`: xem 2.5. Cập nhật comment `task_type` ghi 6 giá trị.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Trạng thái container tính lại từ con, không tăng dần | Idempotent, chịu được giao lặp và thứ tự đảo; mất một lần gọi thì lần sau tự sửa |
| Ghi qua repository, không nới `SetStatus` | Giữ nguyên bất biến TASK-223 (client không đặt `in_progress`) |
| `UpdateTask` chỉ cho container `cancelled` | Huỷ Plan là việc của người dùng (CR-REQ-006); mọi trạng thái khác là suy ra |
| Phát `statuschanged` cho container | Một nguồn sự kiện cho `request-service` (CR-REQ-013) và UI |
| Mặc định `ListTasks` ẩn plan/phase | Tương thích ngược, Board không đổi |
| Cột `request_id` nullable, không FK | README 3.5; hai service khác DB |
| Chỉ mục duy nhất "một Plan hoạt động mỗi Request" | Chống đua ở `GeneratePlan`; MySQL dùng generated column như `task_sources` |
| Bỏ `blocked` tự động cho container | Trạng thái do suy ra là nguồn duy nhất |
| Chặn `ExecuteTask` trên container | Tránh coordinator một cấp và một worktree cho cả Phase |

## 4. Tiêu chí chấp nhận

- [ ] `0015` và `0016` up/down chạy sạch trên Postgres và MySQL; chèn `task_type='plan'` và `'phase'` thành công, `'xyz'` bị từ chối (MySQL từ 8.0.16).
- [ ] Gọi gRPC `CreateTask` với `task_type=bug`, `description`, `priority` lưu đúng và trả lại ở `GetTask`.
- [ ] Tạo `plan` có parent bị `TASK_PLAN_CANNOT_HAVE_PARENT`; `phase` dưới task làm việc bị `TASK_PHASE_REQUIRES_PLAN_PARENT`; `plan` dưới task làm việc bị `TASK_CONTAINER_UNDER_WORK_TASK`.
- [ ] Plan và phase có `task_number = 0` (cột NULL); task thường vẫn nhận số tăng dần; 20 tạo đồng thời không trùng số (cả hai dialect).
- [ ] `ListTasks` không truyền `task_types` không trả plan/phase; `task_types=[plan]` + `request_ids=[X]` trả đúng Plan của X; hơn 100 `request_ids` bị `TASK_LIST_TOO_MANY_REQUEST_IDS`.
- [ ] Bảng suy ra mục 2.6 đúng cho từng dòng (table-driven), gồm trường hợp không con và toàn con `cancelled`.
- [ ] Con đổi `open → in_progress → review → done` qua `ExecuteTask`/`ReportTaskExecutionResult`/`UpdateTask` làm phase và plan đổi theo, mỗi lần đổi có đúng một dòng outbox `orca.task.task.statuschanged` kèm `task_type` và `cause=derived`.
- [ ] `UpdateTask` với `status=done` hoặc `in_progress` trên plan/phase trả `TASK_CONTAINER_STATUS_DERIVED`; `cancelled` được chấp nhận.
- [ ] Container `in_progress` không có link không bị `ReleaseUnlinkedInProgress` đặt về `open` sau grace.
- [ ] `ExecuteTask` trên plan/phase trả `TASK_EXECUTE_CONTAINER_NOT_EXECUTABLE`, không ghi link, không đổi status.
- [ ] `AddEdge` depends_on giữa hai phase không đặt phase thành `blocked`.
- [ ] `AIApply` với proposal `type=plan` tạo task `type=task`.
- [ ] `HasActiveExecutions` và `RecentCompletedTasks` bỏ qua container.
- [ ] `buf lint` và `buf breaking` xanh; không có file tên `helpers`, `utils`, `common`, `misc`; không thêm `max-lines` disable.

## 5. Kiểm thử

- **Unit:** `ParseTaskType`, `IsContainerType`; `DeriveContainerStatus` (bảng 8 dòng + cạnh); quy tắc phân cấp `CreateTask`; `SyncContainerStatus` với fake repository (thua CAS, lên hai cấp, không con); `AddEdge` bỏ qua container; handler `CreateTask` chuyển đủ trường.
- **Integration, cả hai dialect (`-tags=integration`, theo `repository_test.go`):** migration up/down; CHECK; chỉ mục duy nhất Plan hoạt động (hai `plan` cùng `request_id` thì lần hai lỗi; sau khi `cancelled` thì tạo được); cấp số bỏ qua container; `List` với từng bộ lọc; `UpdateContainerStatus` CAS với 8 goroutine; vòng quét `ReleaseUnlinkedInProgress` không đụng container; `ReconcileContainerStatuses`.
- **Hợp đồng:** test đọc `information_schema` so cột `request_id`, chỉ mục, CHECK ở hai DB; test proto giữ số trường cũ.
- Chưa chạy test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- `UpdateTask` ghi lại toàn bộ hàng (`Repository.Update` ghi `status` từ bản đọc trước đó): sửa tiêu đề container đua với ghi suy ra có thể đè trạng thái. Giảm nhẹ: `SyncContainerStatus` chạy sau mọi `UpdateTask` và vòng `Reconcile`; chưa đo.
- `done_subtasks`/`total_subtasks` không được `RecalculateProgress` cập nhật (đã có từ trước). UI plan/phase chỉ nên dùng `progress_percent`.
- `ListTasks` không lọc theo grant của người gọi; `request-service` phải tự lọc Request trước khi gọi (CR-REQ-015, 016). Chưa kiểm chứng gateway có lọc thêm.
- `DeleteTask` một Plan xoá cascade phase và task con (FK `parent_id ON DELETE CASCADE`), kèm `execution_links`. CR không thêm chặn; CR-REQ-006 nên dùng `cancelled`, không xoá.
- Mô tả MySQL generated column `STORED` với `CASE` chưa chạy trên MySQL/TiDB mục tiêu.
- Chuyển `List` sang `ListFilter` đổi chữ ký port; chỉ `usecase/list_tasks.go` gọi, nhưng cần grep lại trước khi sửa vì chỉ mục GitNexus có thể cũ.
- Hành vi mới của `CreateTask` gRPC (lưu thêm trường) có thể làm lộ dữ liệu client cũ đã gửi sai: `priority` ngoài `low|medium|high|urgent` làm CHECK DB báo lỗi `TASK_CREATE_FAILED`. Cần thêm kiểm `priority`, `visibility` ở domain (đề xuất làm trong CR này).

## 7. Câu hỏi mở

- **Q1.** Container `done` được phép mở lại khi thêm con mới (replan thêm task vào phase đã xong)? CR cho phép và ghi Warn; cần xác nhận.
- **Q2.** Huỷ Plan có cần cascade `cancelled` xuống con? CR để `request-service` làm (CR-REQ-006, 012); nếu muốn ở `task-service` thì thêm RPC.
- **Q3.** Có cần `page_size` tối đa cho `ListTasks`? CR-REQ-015 có thể yêu cầu trang lớn.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 2 (D2, O1, O2), 3.5
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/task.go`, `progress.go`, `outbox.go`, `task_edge.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/create_task.go`, `list_tasks.go`, `update_task.go`, `execute_task.go`, `report_execution_result.go`, `recalculate_progress.go`, `ai_apply.go`, `add_edge.go`, `execution_lease.go`, `ports.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/grpc/server.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/postgres/repository.go`, `execution_leases.go`, `velocity.go`, `subtree.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/mysql/repository.go`, `execution_leases.go`
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0003_task_fields_and_comments.up.sql`, `0008_task_outbox_and_number.up.sql`, `0012_task_sources.up.sql` và bản `mysql`
- `/opt/repos/orca/backend-go/proto/orca/task/v1/task.proto`
- `/opt/repos/orca/docs/crs/v4/task-graph/CR-TG-008-jira-source-link-and-durable-direct-agent.md`
- Mới: `domain/task_type.go`, `domain/container_status.go`, `usecase/sync_container_status.go`, `adapter/postgres/task_list_query.go`, `adapter/mysql/task_list_query.go`, `migrations/{postgres,mysql}/0015_task_type_plan_phase.*`, `0016_task_request_id.*` (đều dưới `backend-go/services/task-service/`)
