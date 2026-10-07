# TASK-REQ-013-04: `TaskClient`, use case `StartPhase` và `AdvanceExecution` (kèm `StartExecution`)

**From Solution:** BE-REQ-SOL-013
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/adapter/grpcclient/task_client.go` (mới), `internal/usecase/start_phase.go` (mới), `internal/usecase/advance_execution.go` (mới), `internal/usecase/start_execution.go` (mới), `internal/usecase/ports.go`, `internal/adapter/grpc/server_phase.go` (mới), `internal/usecase/start_phase_test.go`, `advance_execution_test.go` (mới), `proto/orca/request/v1/request_execution.proto` (mới)
**Depends on:** TASK-REQ-013-03, SOL-011 (task 04 `ListTasks` lọc), SOL-012 (cây Plan), CR-REQ-003 (`FlowFor`), CR-REQ-009 (`OpenApproval`, đọc Approval `phase`)
**Status:** `[x] DONE`

---

## Context

- `TaskServiceExecuteRequest` đã có `request_id = 2` (proto dòng 317 đến 327); `ExecuteTask` dùng nó làm mã yêu cầu chuyển cho agent. Dùng chuỗi `req:<request_id>:<task_id>:<attempt>`.
- Mã lỗi `task-service` cần nhận diện: `TASK_EXECUTE_ALREADY_IN_PROGRESS` (coi là thành công), `TASK_EXECUTE_NO_CONNECTION`, `TASK_EXECUTE_WORKTREE_FAILED`, `TASK_EXECUTE_FAILED` (tạm thời, retry ở đối soát), `TASK_EXECUTE_CONTAINER_NOT_EXECUTABLE` (lỗi lập trình: không bao giờ được gọi trên container).
- `ExecuteTask` cần quyền `execute` của **người gọi** (`ResolvePermission`, `execute_task.go`): `request-service` phải chuyển danh tính người duyệt (metadata tenant và user qua gRPC). Cơ chế danh tính service-to-service chưa kiểm chứng; không tự nghĩ ra: dùng đường `tenant.WithUserID` giống `dispatchCtx` của task-service và ghi rõ trong Rủi ro.
- `TaskClient` được dùng chung với SOL-012 (`task_plan_writer.go`) và SOL-015: tạo một interface nhỏ theo nhu cầu (`ListTasks`, `Execute`, `UpdateTask`, `GetSubtree`), không gom mọi RPC.
- Theo SOL-013 2.3 và 2.4; bảng cổng `TypePolicy.PreExecutionGate` do SOL-014; ở đây dùng cổng `PreExecutionGate` mặc định cho qua.

## Việc cần làm

1. `task_client.go`: `type TaskClient interface { ListTasks(ctx, ListTasksQuery) ([]TaskView, string, error); Execute(ctx, taskID, requestID string) error; SetWorktree(ctx, taskID, worktreeID string) error; Subtree(ctx, rootID string) (SubtreeView, error) }` và hiện thực gRPC; chuyển lỗi `TASK_*` về kiểu lỗi domain (`ErrTaskAlreadyRunning`, `ErrTaskDispatchTransient`, `ErrTaskForbidden`).
2. Proto `request_execution.proto`: `StartPhaseRequest/Response`; thêm RPC `StartPhase` vào `RequestService` (README v6 3.6).
3. `advance_execution.go`: `AdvanceExecution.Execute(ctx, requestID, containerID string, actor Actor) (dispatched []string, err error)` theo 2.4: liệt kê lá, chọn `open`, trừ `in_progress` khỏi `MaxParallelTasks`, hỏi `PreExecutionGate`, thiết lập worktree dùng chung, `Execute`. Số lần thử (`attempt`) = `CountFailed + 1`. Lỗi tạm thời ghi `dispatch_retry_until` (lưu ở `task_run_outcomes` loại `failed` với `cause=dispatch_error` hoặc cột phụ, theo chỗ SOL-013 chốt; dùng `cause`).
4. `start_phase.go`: các bước của 2.3, trả lỗi `REQUEST_NOT_EXECUTING`, `REQUEST_PHASE_NOT_IN_PLAN`, `REQUEST_PHASE_NOT_APPROVED`, `REQUEST_PHASE_PREDECESSOR_NOT_DONE`; claim bằng `PhaseStartRepository.TryStart`; outbox `orca.request.phase.started` `{request_id, phase_task_id, started_by, dispatched}` cùng transaction với `TryStart`.
5. `start_execution.go`: `StartExecution.Execute(ctx, requestID)` cho Plan không Phase, `task_list`, `hotfix`: gọi `AdvanceExecution` với container là Plan (hoặc không container với hotfix: danh sách task theo `request_ids`, `task_types=[task,bug,feature]`).
6. Worktree dùng chung: sau lần `Execute` đầu cho Plan, đọc `worktree_id` của task đó (`ListTasks`) rồi `SetWorktree` cho các task còn lại trước khi chạy tiếp. Ghi comment về giả định chưa kiểm chứng (một worktree gắn nhiều task).
7. `server_phase.go`: RPC `StartPhase` tự kiểm quyền (README v6 mục 8 điều 13): người có quyền duyệt Phase hoặc chủ Request (CR-REQ-010).

## Kiểm thử

- `TestStartPhase_NotExecuting`, `_PhaseOfOtherRequest`, `_NotApproved_ChangeRequest`, `_NoPhaseGate_BugSizeL_OK`, `_PredecessorNotDone`, `_TwiceReturnsAlreadyStarted_NoDoubleExecute`.
- `TestAdvanceExecution_SelectsOnlyOpen`, `_RespectsMaxParallel`, `_AlreadyInProgressIsSuccess`, `_TransientErrorNotCountedAsAttempt`, `_ShareWorktreeAfterFirst`, `_GateBlocks_OpensApprovalAndStops`, `_Idempotent_RerunDispatchesNothingNew`.
- Fake `TaskClient` ghi lại thứ tự lời gọi; không gọi mạng.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... -run 'StartPhase|AdvanceExecution|StartExecution' -v`.

## Tiêu chí hoàn thành

- [x] `StartPhase` sai điều kiện trả đúng mã; gọi hai lần trả `already_started=true`, không `Execute` trùng và chỉ một `phase.started`.
- [x] Task `open` được `Execute` đúng `MaxParallelTasks`; task `blocked` không bị chạy.
- [x] Task thứ hai của Plan dùng cùng `worktree_id` với task đầu (kiểm qua fake).
- [x] Không gọi `Execute` trên `plan`/`phase`.
- [x] `AdvanceExecution` chạy lặp không sinh lời gọi `Execute` thừa cho task đang chạy.

## Rủi ro và lưu ý

- Giả định worktree dùng chung (chưa kiểm chứng `project-service`): nếu sai, task này phải đổi (mỗi Phase một task lớn hoặc cơ chế merge).
- Danh tính khi gọi `Execute`: người duyệt có thể không có quyền `execute` trên task do người khác tạo; `REQUEST_EXECUTE_FORBIDDEN`, không đổi danh tính.
- CR-REQ-029 (ReadinessGate) có thể chặn dispatch; để chỗ nối bằng cổng `PreExecutionGate`, không thêm logic ở đây.
