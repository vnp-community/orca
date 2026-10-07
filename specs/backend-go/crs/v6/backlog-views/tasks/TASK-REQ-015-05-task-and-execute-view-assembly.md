# TASK-REQ-015-05: View TASK và EXECUTE: lắp ráp từ `ListTasks`, Approval, `ListExecutionStates`, `task_run_outcomes`

**From Solution:** BE-REQ-SOL-015
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/list_backlog_tasks.go` (mới), `internal/usecase/ports.go`, `internal/adapter/grpcclient/task_client.go` (thêm `ListExecutionStates`, dùng chung với SOL-013), `internal/adapter/postgres/backlog_approvals.go`, `internal/adapter/mysql/backlog_approvals.go` (mới), `internal/usecase/list_backlog_tasks_test.go` (mới)
**Depends on:** TASK-REQ-015-01, 02, 03, 04; SOL-011 task 04 (`ListTasks` lọc); SOL-013 task 03 (`task_run_outcomes.LatestFailed`); CR-REQ-009 (`approvals`)
**Status:** `[x] DONE`

---

## Context

- Trình tự một lần gọi, không N+1 (solution 2.5): (1) trang Request ứng viên; (2) một `ListTasks` (cộng trang) cho `request_ids` với `task_types=[task,bug,feature,plan,phase]`; (3) một truy vấn `approvals`; (4) một `ListExecutionStates` cho task ứng viên; (5) một truy vấn `LatestFailed`.
- `ListTasks` hiện không lọc theo grant (chỉ tenant): lọc quyền xem phải do `request-service` làm trên danh sách Request trước bước 2.
- Loại trừ nhau: task chưa qua cổng thuộc TASK; qua cổng thì thuộc EXECUTE khi đủ điều kiện (`open|blocked`, hoặc link gần nhất `failed` với `status NOT IN (done,cancelled)`).
- Request ứng viên: TASK cần `awaiting_plan_approval` hoặc `executing`; EXECUTE cần `executing`.
- `task-service` không sẵn: trả lỗi toàn bộ (`REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE`), không trả từng phần.

## Việc cần làm

1. `TaskClient` thêm `ListExecutionStates(ctx, taskIDs []string) (map[string]ExecutionStateView, error)` (chia lô ≤ 500 id).
2. Cổng `ApprovalGateReader.ListGateApprovals(ctx, tenantID string, requestIDs []string) ([]domain.Approval, error)`: `WHERE tenant_id = ? AND request_id IN (...) AND subject_type IN ('plan','task_list','phase','pre_deploy')` (chỉ mục `(tenant_id, request_id, created_at)` của CR-REQ-009); hai adapter.
3. `list_backlog_tasks.go`: `ListBacklogTasks.Execute(ctx, in) (groups []BacklogGroup, next string, err error)`:
   - Chọn trang Request ứng viên (keyset như task 04 nhưng theo trạng thái của view và cờ `project_id`, `request_types`, `request_id`).
   - `ListTasks` một lần và nhóm theo Request/Plan/Phase; mỗi Plan tối đa 100 task (nếu vượt, cắt và ghi log).
   - `ApprovalIndex` từ bước (3) → `ResolveTaskGate` cho từng task làm việc.
   - View TASK: giữ task `open|blocked` mà cổng chưa duyệt; nhóm theo Plan rồi Phase; `gate_status` của nhóm.
   - View EXECUTE: giữ task cổng đã duyệt và đủ điều kiện; gọi `ListExecutionStates` cho tập này; điền `blocked_by_task_ids`, `last_engine`, `last_link_status`, `failed_attempts`; `last_error` từ `LatestFailed`; nhóm theo Phase (hoặc Plan khi không Phase).
   - Lọc `phase_task_id`, `plan_task_id`, `assignee_id` ở bộ nhớ sau khi lắp.
4. Lỗi `Unavailable` từ `TaskClient` bọc thành `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE`.
5. Đếm lời gọi: use case nhận `TaskClient` qua interface để test đếm số lần `ListTasks`/`ListExecutionStates`.
6. Không xuất hiện task `done`/`cancelled` ở bất kỳ view nào.

## Kiểm thử

- Table-driven với fake `TaskClient`, `ApprovalGateReader`, `BacklogRequestReader`:
  - `TestBacklogTasks_OpenUnderUnapprovedPlan_InTaskNotExecute`; duyệt Plan xong thì chuyển sang EXECUTE ở lần gọi sau.
  - `_ChangeRequest_PhaseNotApproved_StaysInTask`; Phase duyệt thì sang EXECUTE.
  - `_BugSizeL_NoPhaseYet_InTask`; sau chia Phase và duyệt Plan thì EXECUTE.
  - `_TaskDocsApprovedPlanShell_InExecute`, `_HotfixAfterPreDeploy_InExecute`.
  - `_BlockedTask_HasBlockedBy`, `_FailedLink_ShowsAttemptsAndError`, `_DoneAndCancelledNeverShown`.
  - `_CallCounts_OneListTasksOneStatesOneApprovals` (20 Request).
  - `_TaskServiceDown_ReturnsUnavailable_NoPartial`.
  - `_PlanWith150Tasks_CappedAt100`.
- Integration: truy vấn `ListGateApprovals` hai dialect (đúng bộ lọc, tenant isolation).
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... -run BacklogTasks -v`.

## Tiêu chí hoàn thành

- [x] Task `open` dưới Plan chưa duyệt ở TASK, không ở EXECUTE; duyệt xong thì chuyển.
- [x] `change_request`: task dưới Phase chưa duyệt vẫn ở TASK dù Plan đã duyệt.
- [x] Một lần `ListBacklog` TASK/EXECUTE cho 20 Request dùng đúng một `ListTasks` (cộng trang), một `ListExecutionStates` và một truy vấn `approvals`.
- [x] Task `blocked` ở EXECUTE có `blocked_by_task_ids` đúng; task link gần nhất `failed` kèm `failed_attempts`, `last_error`.
- [x] `task-service` không sẵn thì lỗi `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE`, không dữ liệu một phần.

## Rủi ro và lưu ý

- Hiệu năng: tối đa 2.000 hàng mỗi lần (20 Request × 100 task) và `IN` lớn; chưa đo.
- `last_link_status` của Engine 2/3 không phản ánh run xong thật (CR-TG-008); dùng `status` task làm nguồn chính.
- Task `review` (khi `REQUEST_AUTO_COMPLETE_TASKS` tắt) không thuộc EXECUTE (Q3 của CR); đổi khi chủ CR quyết khác.
