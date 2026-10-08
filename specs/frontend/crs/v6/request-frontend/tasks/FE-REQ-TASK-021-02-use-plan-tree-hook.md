# FE-REQ-TASK-021-02: Hook `usePlanTree` và mô hình ghép Approval

**From Solution:** [FE-REQ-SOL-021](../solutions/FE-REQ-SOL-021-plan-phase-tree-and-approval-ui.md) mục 2.2
**Priority:** P0
**Area:** frontend / hooks
**File:** `frontend/src/renderer/src/hooks/usePlanTree.ts` (mới), `frontend/src/shared/task-hierarchy.ts` (sửa: `buildPlanSubtree`), `frontend/src/renderer/src/components/request/plan/plan-approval-model.ts` (mới); test `usePlanTree.test.ts`, `plan-approval-model.test.ts`, `task-hierarchy.test.ts`
**Depends on:** FE-REQ-TASK-021-01, 018-03 (`useApprovals`), 018-02 (event bus)
**Status:** [x] DONE (verified 2026-10-07: vitest hooks/usePlanTree (6), plan-approval-model (4), task-hierarchy buildPlanSubtree pass)

**Ghi chú:** `usePlanTree` gọi `callRuntimeRpc` `task.list` trực tiếp (không qua `callRequestRpc` vì `task.list` không thuộc `REQUEST_RPC_METHODS`); thêm `resolveProgress`, `listPlanWorkTasks`, `computeExecutionGates` vào `plan-approval-model.ts`. Lưu ý: `useApprovals({requestId})` vẫn ghi đè `pendingApprovalCount` toàn cục bằng danh sách của một Request (hành vi có sẵn, chưa sửa).

## Context

- `task.list` (`channels_automation_task.go:343`) chỉ nhận `{projectId, pageToken, pageSize}` và trả `{tasks, nextPageToken}`; không lọc theo Request hay Plan (đã đọc). Cột `request_id` backend (CR-REQ-011) chưa dùng được qua WS.
- `useTasks` thay toàn `tasks` của store: không dùng.
- Dữ liệu Plan: `OrcaRequest.planTaskId`; Approval subject `plan|task_list|phase|pre_deploy` với `subjectId` là id Task (README v6 3.5, mục 8 số 3).

## Việc cần làm

1. `buildPlanSubtree(tasks, planTaskId)` → `{plan|null, phases: OrcaTask[], tasksByPhase: Record<string, OrcaTask[]>, flatTasks: OrcaTask[]}`; BFS có tập đã thăm; Task con trực tiếp của Plan là `flatTasks` khi loại `task_list`.
2. `usePlanTree(request)`: lặp `task.list {projectId, pageSize:200, pageToken}` tối đa 20 trang (cờ `truncated`), `normalizeTask`; `useApprovals({requestId: request.id})`; trả `{tree, approvals, isLoading, error, truncated, refetch}`. Chỉ gọi khi `request.planTaskId` có giá trị; ngược lại trả `tree=null`.
3. Làm mới: nghe event bus (`plan.generated`, `phase.started`, `phase.completed`, `approval.*`, `request.status_changed`) cho `request.id`; khi `request.status==='executing'` và không có luồng sự kiện thì polling 15 giây, dừng khi `document.visibilityState!=='visible'`; huỷ khi unmount; một lần tải ở mỗi thời điểm (bỏ qua nếu đang tải).
4. `attachApprovals(tree, approvals)`: gắn `approval` `pending|approved|rejected` mới nhất cho Plan (`plan`|`task_list`), mỗi Phase (`phase`), danh sách `pre_deploy` (theo `subjectId`, hoặc theo Request nếu rỗng).
5. `computePhaseStats(phase, tasks)`: `{done, total, blocked, running}` từ `status`.
6. Đánh dấu tạm trong mã: comment "đổi sang requestId/planTaskId khi CR-REQ-011/016 có" (ngắn, theo AGENTS.md).

## Kiểm thử

- `usePlanTree`: nhiều trang nối đúng; quá 20 trang đặt `truncated`; `planTaskId` rỗng không gọi RPC; huỷ khi unmount; polling dừng khi tab ẩn; `forbidden`.
- `buildPlanSubtree`: vòng lặp, mồ côi, Plan không có trong danh sách, `task_list`.
- `attachApprovals`: nhiều Approval cùng subject lấy bản mới nhất; `pre_deploy`.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/hooks/usePlanTree src/renderer/src/components/request/plan/plan-approval-model src/shared/task-hierarchy`.

## Tiêu chí hoàn thành

- [ ] Cây dựng đúng từ danh sách phẳng, không ghi đè store `tasks`.
- [ ] Không rò polling khi unmount hoặc tab ẩn.
- [ ] Thay nguồn dữ liệu (khi backend có lọc) chỉ cần sửa trong `usePlanTree`.

## Rủi ro và lưu ý

- Tải toàn dự án đắt với dự án lớn; chấp nhận tạm, theo dõi.
- Mapping `subjectId` ↔ Task id là suy luận từ README; xác nhận khi có CONTRACT.
