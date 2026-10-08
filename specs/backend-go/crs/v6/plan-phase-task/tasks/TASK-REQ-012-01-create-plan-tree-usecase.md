# TASK-REQ-012-01: `task-service` use case `CreatePlanTree` (nguyên tử, idempotent, supersede)

**From Solution:** BE-REQ-SOL-012
**Priority:** P0
**Service:** `task-service`
**File:** `internal/usecase/create_plan_tree.go` (mới), `internal/usecase/ports.go`, `proto/orca/task/v1/task.proto`, `internal/usecase/create_plan_tree_test.go` (mới), `internal/usecase/fakes_test.go`
**Depends on:** TASK-REQ-011-01, TASK-REQ-011-03, TASK-REQ-011-07
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./...` + `go test -tags integration ./internal/adapter/...` (Postgres 16 và MySQL 8) trong task-service)

---

## Context

- Mẫu bắt buộc theo: `usecase/ai_apply.go` (đọc toàn file ngày 2026-10-06). `AIApply` mở `txRunner.RunInTx(ctx, func(ctx, tasks TaskRepository, edges EdgeRepository) error {...})`, trong đó dựng `NewCreateTask(tasks, nil)` và `NewAddEdge(tasks, edges)`, tạo task rồi cạnh `parent_child`, lượt hai thêm `depends_on`, rồi `UpdateAIPlanJSON`. Lỗi bất kỳ làm rollback toàn bộ (`TestAIApply_MidLoopFailure_RollsBackEntireSubtree`).
- `CreateTask` trong tx truyền `grants=nil` nên không `Grant` được; owner grant phải làm sau commit bằng `GrantRepository` ngoài tx.
- `AddEdge.Execute` kiểm vòng và tự đặt `blocked` khi đích chưa `done`; SOL-011 task 07 bỏ `blocked` khi `from` là container, nên cạnh `depends_on` giữa hai Phase không chặn.
- Chỉ mục `uq_tasks_active_plan_per_request` (SOL-011 task 01) chống hai Plan hoạt động cho một Request; vi phạm duy nhất (Postgres `23505`, MySQL `1062`) phải đọc lại và trả `already_exists`.
- `GetSubtree` có sẵn (`usecase/get_subtree.go`) để đọc cây đã tồn tại.

## Việc cần làm

1. Proto `task.proto`: thêm `rpc CreatePlanTree` và các message `CreatePlanTreeRequest`, `PlanTreePhase`, `PlanTreeTask`, `CreatePlanTreeResponse` đúng như solution 2.6. `make proto-gen && make proto-lint`.
2. `ports.go`: cổng `PlanTreeGrantor interface { Grant(ctx, tenantID string, g domain.Grant) (string, error) }` (có thể dùng lại `GrantRepository`); cổng đọc cây cho nhánh `already_exists`: `PlanTreeReader` (hoặc dùng `GetSubtree`).
3. `create_plan_tree.go`:
   ```go
   type CreatePlanTreeInput struct { RequestID, ProjectID, Title, Description, AIPlanJSON, CreatorID, SupersedesPlanID string
       Phases []PlanTreePhase; Tasks []PlanTreeTask }
   type CreatePlanTreeResult struct { Plan domain.Task; Phases, Tasks []domain.Task; AlreadyExists bool }
   func NewCreatePlanTree(tx TxRunner, find ActivePlanFinder, grants GrantRepository) *CreatePlanTree
   func (uc *CreatePlanTree) Execute(ctx context.Context, in CreatePlanTreeInput) (CreatePlanTreeResult, error)
   ```
4. Kiểm đầu vào ngoài tx: `RequestID`, `ProjectID`, `Title` bắt buộc; không đồng thời có `Phases` và `Tasks` (`TASK_PLAN_TREE_MIXED_CHILDREN`); chỉ số `depends_on` trong phạm vi.
5. Trong `RunInTx`: (a) `find.ActivePlan(ctx, tenant, requestID)`; có và `SupersedesPlanID==""` thì đọc cây trả `AlreadyExists=true` (không ghi); có và khớp thì kiểm không con `in_progress` (`TASK_PLAN_HAS_RUNNING_TASKS`) rồi đặt Plan cũ và mọi con chưa `done` thành `cancelled` (qua `UpdateContainerStatus` cho container, `UpdateStatus` cho lá); có mà không khớp `TASK_PLAN_SUPERSEDE_MISMATCH`. (b) Tạo Plan (`Type=plan`, `RequestID`), từng Phase (`ParentID=plan`), task (`ParentID` là Phase hoặc Plan) bằng `NewCreateTask(tasks, nil)`; cạnh `parent_child` bằng `AddEdge`. (c) Lượt hai: `depends_on` giữa task anh em và giữa Phase. (d) `tasks.UpdateAIPlanJSON(plan.ID, in.AIPlanJSON)`.
6. Bắt lỗi vi phạm duy nhất: nhận diện bằng hàm `isUniqueViolation(err)` đặt ở adapter và bọc thành `ErrActivePlanExists`; usecase gặp lỗi này thì đọc lại bằng `find` ngoài tx và trả `AlreadyExists=true`.
7. Sau commit: `grants.Grant(ctx, tenant, domain.Grant{TaskID: plan.ID, SubjectID: in.CreatorID, Level: domain.GrantLevelOwner, ApplyTree: true})` best-effort (log lỗi, không fail).
8. Mã lỗi: `TASK_PLAN_TREE_INVALID` (InvalidArgument), `TASK_PLAN_TREE_MIXED_CHILDREN`, `TASK_PLAN_HAS_RUNNING_TASKS`, `TASK_PLAN_SUPERSEDE_MISMATCH` (FailedPrecondition), `TASK_PLAN_TREE_FAILED` (Internal).

## Kiểm thử

`create_plan_tree_test.go` (fake `TxRunner` có hỗ trợ rollback như ở `ai_apply_test.go`): `TestCreatePlanTree_PlanPhasesTasks_EdgesAndLabels`, `_DirectTasksUnderPlan`, `_MidTreeFailure_RollsBackAll` (chèn lỗi ở task thứ N, không còn task nào), `_SecondCall_AlreadyExists`, `_UniqueViolation_RereadsExisting`, `_Supersede_CancelsOldTree`, `_Supersede_RunningTask_Rejected`, `_SupersedeMismatch`, `_PhaseDependsOnPhase_NoBlocked`, `_TaskDependsOnTask_Blocked`, `_Cycle_Rejected`, `_GrantsOwnerAfterCommit`, `_GrantFailure_DoesNotFail`.
Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/task-service/internal/usecase/... -run CreatePlanTree -v`.

## Tiêu chí hoàn thành

- [x] Một transaction tạo đủ Plan, Phase, task, cạnh `parent_child` và `depends_on`; lỗi giữa chừng không để lại task nào.
- [x] Gọi lặp cho cùng `request_id` trả `AlreadyExists=true`, không Plan thứ hai.
- [x] Task phụ thuộc task chưa xong là `blocked`; Phase phụ thuộc Phase không `blocked`.
- [x] Supersede huỷ Plan cũ và con chưa `done`; có con `in_progress` thì bị từ chối.
- [x] Owner grant với `ApplyTree=true` gán sau commit.

## Rủi ro và lưu ý

- Chưa kiểm chứng `RunInTx` MySQL xử lý nhiều `AddEdge` trong một tx mà không deadlock (`ListByKindForUpdate` khoá rộng); task 02 có test tải.
- Cây lớn (tối đa 100 task) tạo tới vài trăm câu SQL trong một tx; giữ tx ngắn bằng cách kiểm đầu vào trước.
- `ActivePlanFinder` có thể là phương thức mới của `TaskRepository` (`FindActivePlanByRequest`) hoặc dùng `List` với `ListFilter{TaskTypes:[plan], RequestIDs:[x]}`: ưu tiên cái sau để không thêm port, nếu SOL-011 task 04 đã xong.

## Ghi chú triển khai

- Test đúng tên trong `create_plan_tree_test.go` (cộng `_InvalidInput_Rejected`, `_MixedChildren_Rejected`, `_Supersede_RetryAfterSuccess_AlreadyExists`, `_TxConflict_RetriesWholeTx`). Cổng đọc Plan hoạt động dùng `TaskRepository.List` (không thêm port). Giới hạn 100 task, 50 phase mỗi cây.
- Lệch: thêm `ErrTxConflict` (deadlock MySQL 1213/1205, Postgres 40001/40P01 do `RunInTx` gắn nhãn) và retry cả giao dịch tối đa 5 lần; không có thì 8 goroutine cùng `request_id` trên MySQL bị deadlock ở chỉ mục duy nhất. Retry xong thì bước (a) thấy Plan đã commit và trả `already_exists`.
- Lệch: `UpdateContainerStatus` của hai adapter nay dùng giao dịch của `RunInTx` khi gọi lồng (trước đây mở tx riêng trên pool nên huỷ Plan cũ không rollback cùng cây mới).
- Lệch: gọi lại với `supersedes_plan_id` đã `cancelled` còn Plan mới đang hoạt động thì trả `already_exists` (retry an toàn sau replan thành công).
- Phase `done` của cây cũ giữ `done`; Plan cũ luôn thành `cancelled` (để giải phóng chỉ mục duy nhất).
- Owner grant sau commit best-effort bằng `GrantRepository` ngoài tx.
