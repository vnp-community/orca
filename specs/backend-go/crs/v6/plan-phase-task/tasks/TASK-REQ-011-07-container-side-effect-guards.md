# TASK-REQ-011-07: Chặn tác dụng phụ trên plan/phase (`AddEdge`, sweep, velocity, `ExecuteTask`, `UpdateTask`)

**From Solution:** BE-REQ-SOL-011
**Priority:** P0
**Service:** `task-service`
**File:** `internal/usecase/add_edge.go`, `internal/usecase/execute_task.go`, `internal/usecase/update_task.go`, `internal/adapter/{postgres,mysql}/execution_leases.go`, `internal/adapter/postgres/repository.go` (`HasActiveExecutions`), `internal/adapter/mysql/repository.go`, `internal/adapter/{postgres,mysql}/velocity.go`, `internal/usecase/create_task.go`
**Depends on:** TASK-REQ-011-03, TASK-REQ-011-05
**Status:** [x] DONE (đã kiểm chứng 2026-10-07: `go test ./services/task-service/...` và `go test -tags=integration ./internal/adapter/postgres ./internal/adapter/mysql` (PG 16, MySQL 8.0.46 thật))

---

## Context

Các chỗ trong code hiện tại sẽ làm hỏng trạng thái suy ra hoặc nghĩa của "có run" nếu có container:
- `add_edge.go` dòng 82 đến 93: cạnh `depends_on` mà đích chưa `done/cancelled` thì `tasks.UpdateStatus(FromTaskID, blocked)`.
- `ReleaseUnlinkedInProgress` (`postgres/execution_leases.go` dòng 177): `UPDATE ... status='in_progress' AND active_execution_link_id IS NULL AND updated_at < now()-grace` đặt về `open`. Container `in_progress` suy ra không có link, nên bị đặt lại sau 30 phút. MySQL có bản tương đương.
- `HasActiveExecutions` (`postgres/repository.go` dòng 364): `EXISTS ... status='in_progress'`.
- `RecentCompletedTasks` (`postgres/velocity.go` dòng 16): `status='done'`, nạp vào prompt velocity.
- `ExecuteTask` không kiểm type; `selectEngine` (dòng 399) coi task có con `parent_child` là Engine 2, `ComplexExecutor.buildSpec` chỉ mở một cấp con.
- `UpdateTask` cho đặt `status` tuỳ ý (trừ `in_progress`).

## Việc cần làm

1. `AddEdge`: trong `addEdgeWithinTx`, bỏ nhánh `UpdateStatus(blocked)` khi `from` là container. Cần `tasks.Get(from)` (đã có `dep`; thêm một `Get` cho `FromTaskID`, hoặc nhận từ `TaskRepository.Get` đã gọi ở đầu hàm nếu có). Thứ tự giữa các Phase do BE-REQ-SOL-013 kiểm khi `StartPhase`.
2. Sweep: thêm `AND task_type NOT IN ('plan','phase')` vào hai câu `ReleaseUnlinkedInProgress` (cả trong subquery lẫn điều kiện ngoài).
3. `HasActiveExecutions` và `RecentCompletedTasks`: thêm cùng điều kiện ở hai dialect.
4. `ExecuteTask.Execute`: ngay sau `Get` task, trước pre-check `in_progress`, nếu `domain.IsContainerType(task.Type)` thì trả `TASK_EXECUTE_CONTAINER_NOT_EXECUTABLE` (FailedPrecondition), không ghi link, không đổi status.
5. `UpdateTask.Execute`: nếu task là container và `in.Status != nil` và giá trị khác `cancelled` thì `TASK_CONTAINER_STATUS_DERIVED` (InvalidArgument). Các trường khác (title, description) vẫn sửa được.
6. Huỷ container (`status=cancelled`) cho phép; cascade xuống con không làm ở đây (câu hỏi mở Q2).
7. Chạy `gitnexus_impact` cho `AddEdge`, `ExecuteTask.Execute`, `UpdateTask.Execute`; báo blast radius.

## Kiểm thử

- Unit: `TestAddEdge_ContainerFrom_NotBlocked`, `TestAddEdge_WorkTaskFrom_StillBlocked` (hành vi cũ), `TestExecuteTask_Container_NotExecutable` (không ghi link: kiểm fake `links`), `TestUpdateTask_ContainerDone_Rejected`, `TestUpdateTask_ContainerInProgress_Rejected`, `TestUpdateTask_ContainerCancelled_OK`, `TestUpdateTask_ContainerTitle_OK`.
- Integration hai dialect: `TestExecutionLeases_ReleaseUnlinked_SkipsContainers`, `TestHasActiveExecutions_IgnoresContainers`, `TestRecentCompletedTasks_IgnoresContainers`.
- `cd /opt/repos/orca/backend-go && go test ./services/task-service/... && go test -tags=integration ./services/task-service/internal/adapter/...`.

## Tiêu chí hoàn thành

- [x] `ExecuteTask` trên plan/phase trả `TASK_EXECUTE_CONTAINER_NOT_EXECUTABLE`, không link, không đổi status.
- [x] `UpdateTask` `done`/`in_progress`/`open` trên container bị từ chối; `cancelled` được chấp nhận.
- [x] Container `in_progress` không có link không bị sweep đặt về `open` sau grace.
- [x] `depends_on` giữa hai phase không đặt phase `blocked`; giữa hai task làm việc vẫn `blocked` như cũ.
- [x] `HasActiveExecutions` và `RecentCompletedTasks` bỏ qua container.

## Rủi ro và lưu ý

- Nếu sau này muốn chạy cả Phase bằng một coordinator, `TASK_EXECUTE_CONTAINER_NOT_EXECUTABLE` phải gỡ; ghi rõ lý do trong comment (một worktree, một coordinator cho cả phase trái README v6).
- Test `execute_task_test.go` hiện có dùng fake `TaskRepository`: giữ nguyên type mặc định `task` để không vỡ.
- Không thêm `max-lines` disable; nếu `execute_task.go` vượt ngưỡng, tách hàm sang file mới tên theo khái niệm.

## Ghi chú triển khai (2026-10-07)

- `AddEdge`: không đặt `blocked` khi `FromTaskID` là container (Get thất bại thì giữ hành vi cũ); `ReleaseUnlinkedInProgress`, `HasActiveExecutions`, `RecentCompletedTasks` thêm `task_type NOT IN ('plan','phase')` ở hai dialect; `ExecuteTask` trả `TASK_EXECUTE_CONTAINER_NOT_EXECUTABLE` ngay sau `Get`; `UpdateTask` chặn mọi status ngoài `cancelled` trên container (`TASK_CONTAINER_STATUS_DERIVED`), title vẫn sửa được.
- Test: unit `TestAddEdge_ContainerFrom_NotBlocked`, `TestAddEdge_WorkTaskFrom_StillBlocked`, `TestExecuteTask_Container_NotExecutable`, `TestUpdateTask_Container*`; integration `TestExecutionLeases_ReleaseUnlinked_SkipsContainers`, `TestHasActiveExecutions_IgnoresContainers`, `TestRecentCompletedTasks_IgnoresContainers` (cả hai DB).
- Người gọi đã kiểm: `AddEdge` (usecase, `AIApply` qua `addEdgeWithinTx`), `ExecuteTask.Execute` (gRPC `Execute`, batch), `UpdateTask.Execute` (gRPC `UpdateTask`, api-gateway), build và test xanh ở các module phụ thuộc.
