# FE-REQ-TASK-018-06: Gỡ `backlog` khỏi `TaskStatus`, chuẩn hoá trạng thái, thêm `plan|phase`

**From Solution:** [FE-REQ-SOL-018](../solutions/FE-REQ-SOL-018-request-frontend-foundation.md) mục 2.6
**Priority:** P0
**Area:** frontend / task
**File:** `frontend/src/shared/task-types.ts` (sửa: dòng 14, 24, 151), `frontend/src/shared/task-status-normalization.ts` (mới), `components/task/TaskBoardView.tsx` (dòng 11), `TaskDetail.tsx` (dòng 31), `TaskStatusBadge.tsx` (dòng 9), `TaskDAGView.tsx` (dòng 32), `hooks/useTasks.ts`, `hooks/useTask.ts`; test `task-status-normalization.test.ts`, `components/task/__tests__/{TaskBoardView,TaskStatusBadge,TaskDAGView,TaskDetail}.test.tsx`
**Depends on:** không (không cần backend; có thể làm sớm nhất)
**Status:** [ ] TODO

## Context

- Backend chỉ có 6 giá trị `open, blocked, in_progress, review, done, cancelled` (README v6 mục 1; migration `0003`). Frontend còn `todo` và `backlog` (`task-types.ts:19-28`); D4/O3 gỡ `backlog`. `todo` ngoài phạm vi (README mục 8 số 11).
- `TaskType` hiện `'epic'|'story'|'task'|'subtask'|'bug'|'spike'` (`task-types.ts:14`), lệch backend (`feature`); chỉ thêm `plan|phase`, không dọn lệch.
- `useTasks.ts` đặt `setTasks(response.tasks ?? [])` (dòng 63); `filteredTasks` dòng 78.

## Việc cần làm

1. `task-types.ts`: bỏ `'backlog'` khỏi `TaskStatus`; bỏ khoá `backlog` khỏi `TASK_STATUS_PROGRESS`; thêm `'plan'|'phase'` vào `TaskType`; thêm `requestId?: string` vào `OrcaTask` (CR-REQ-021 dùng).
2. `task-status-normalization.ts`: `normalizeTaskStatus(raw: unknown): TaskStatus`: giá trị hợp lệ giữ nguyên, `'backlog'` và giá trị lạ thành `'open'`; `normalizeTask(task)` áp dụng cho một `OrcaTask`.
3. `useTasks.ts`: map `normalizeTask` trước `setTasks`; `useTask.ts` tương tự.
4. Bỏ `'backlog'` khỏi `STATUS_ORDER` (`TaskBoardView.tsx`), `TASK_STATUSES` (`TaskDetail.tsx`), `STATUS_CONFIG` (`TaskStatusBadge.tsx`), `STATUS_COLORS` (`TaskDAGView.tsx`). `TaskStatusBadge` fallback đổi `todo` thành `open`.
5. Rà phần còn lại: `rg "'backlog'" frontend/src docs/ui tests/e2e` rồi sửa hoặc ghi chú; cột Board `backlog` biến mất, ghi vào `docs/ui/pages/tasks.md` nếu có nhắc.
6. Chạy `tsc` và sửa `switch` không đầy đủ do `plan|phase` (thêm nhánh hiển thị trung tính, không đổi hành vi Board; lọc thật là việc của FE-REQ-TASK-021-01).

## Kiểm thử

- `task-status-normalization.test.ts`: `'backlog'`, `'weird'`, `undefined` thành `open`; 7 giá trị hợp lệ giữ nguyên.
- `TaskBoardView.test.tsx`: không còn cột Backlog; task nhận `status:'backlog'` từ dữ liệu cũ xuất hiện ở cột Open (không mất).
- `TaskStatusBadge.test.tsx`, `TaskDAGView.test.tsx`, `TaskDetail.test.tsx` cập nhật theo.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/task src/shared/task-status-normalization src/renderer/src/hooks/useTasks` rồi `pnpm --filter orca-frontend exec tsc --noEmit -p tsconfig.json` (chưa kiểm chứng).

## Tiêu chí hoàn thành

- [ ] `rg "'backlog'" frontend/src/renderer/src/components/task frontend/src/shared/task-types.ts` không còn kết quả ngoài file chuẩn hoá và test.
- [ ] Task cũ mang `backlog` vẫn hiện ở Board (cột Open).
- [ ] `tsc` không thêm lỗi so với baseline (ghi số lỗi baseline vào PR; FE-TASK-001 từng ghi 114 lỗi sẵn có).
- [ ] Test xanh.

## Rủi ro và lưu ý

- `TaskPage.tsx` lớn: không sửa file này ở task này.
- Chạy GitNexus `impact` cho `TaskStatus`, `TaskStatusBadge`, `useTasks` trước khi sửa (chưa chạy).
