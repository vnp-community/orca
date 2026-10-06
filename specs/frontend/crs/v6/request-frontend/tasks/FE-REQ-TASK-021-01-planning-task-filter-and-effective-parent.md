# FE-REQ-TASK-021-01: Lọc `plan`/`phase` khỏi Board, cây và DAG; `parentId` hiệu dụng

**From Solution:** [FE-REQ-SOL-021](../solutions/FE-REQ-SOL-021-plan-phase-tree-and-approval-ui.md) mục 2.5
**Priority:** P0
**Area:** frontend / task
**File:** `frontend/src/shared/task-hierarchy.ts` (mới), `frontend/src/renderer/src/hooks/useTasks.ts` (sửa: dòng 63, 78), `components/task/TaskGraph.tsx` (sửa), `components/task/TaskTreeView.tsx` (sửa), `components/task/TaskCard.tsx` (sửa); test `task-hierarchy.test.ts`, `hooks/useTasks.test.ts`, `components/task/__tests__/{TaskTreeView,TaskBoardView,TaskDAGView,TaskGraph,TaskCard}.test.tsx`
**Depends on:** FE-REQ-TASK-018-06 (`TaskType` `plan|phase`, `OrcaTask.requestId`)
**Status:** [ ] TODO

## Context

- `TaskGraph.tsx` truyền `filteredTasks` cho `TaskTreeView`, `TaskBoardView`, `TaskDAGView` (dòng 247-268): một chỗ lọc là đủ.
- `TaskTreeView.renderLevel` lấy `tasks.filter(t => t.parentId === parentId)` bắt đầu từ `null` (dòng 10-30): Task con của Phase bị lọc sẽ MẤT. CR-021 tưởng "nâng lên gốc" tự xảy ra, thực tế phải sửa.
- Chạy `impact` (GitNexus) cho `useTasks`, `TaskTreeView` trước khi sửa (FE-TASK-001 từng ghi LOW; chưa chạy lại).
- Dự án không dùng Request không được đổi hành vi.

## Việc cần làm

1. `task-hierarchy.ts`: `isPlanningTask(t)`, `isWorkTask(t)`, `computeEffectiveParents(tasks): Map<string, {parentId: string|null, trail: string[]}>` (đi lên qua tổ tiên `plan|phase` bị lọc đến tổ tiên đầu tiên hiển thị hoặc `null`; chống vòng lặp bằng tập đã thăm).
2. `useTasks.ts`: thêm state `showPlanningTasks` (mặc định `false`) và trả về cùng `setShowPlanningTasks`; `filteredTasks` bỏ `isPlanningTask` khi tắt; khi tắt và dữ liệu có Task `plan|phase` thì map `parentId := effectiveParent` và đính `planPath` (tên Plan / Phase) cho `TaskCard`. Khi dữ liệu không có `plan|phase` thì trả đúng như cũ (không map).
3. `TaskGraph.tsx`: công tắc "Hiện Plan/Phase" (`ui/toggle.tsx` hoặc `Checkbox`) trong toolbar, chỉ hiện khi dữ liệu có Task `plan|phase`.
4. `TaskCard.tsx`: chip nhỏ đường dẫn `Plan / Phase` khi có `planPath`.
5. `selectedIds` và chạy hàng loạt (`TaskBatchExecution`) bỏ qua Plan/Phase.
6. Rà `tasks.filter`/đếm Task ở thống kê dự án và thông báo: `rg "tasks\.filter|allTasks" frontend/src/renderer/src`; ghi kết quả vào PR, sửa nơi đếm sai.

## Kiểm thử

- `task-hierarchy.test.ts`: chuỗi `Plan > Phase > Task`; Phase lồng; vòng lặp `parentId`; không có Plan.
- `useTasks.test.ts`: tắt/bật công tắc; không có `plan|phase` thì giữ nguyên tham chiếu mảng (không tạo bản sao).
- `TaskTreeView.test.tsx`: Task con của Phase bị lọc xuất hiện ở gốc; không nhân đôi khi bật công tắc.
- `TaskBoardView.test.tsx`/`TaskDAGView.test.tsx`: không có Plan/Phase mặc định; bật công tắc thì có.
- Chạy: `pnpm --filter orca-frontend test -- src/shared/task-hierarchy src/renderer/src/hooks/useTasks src/renderer/src/components/task`.

## Tiêu chí hoàn thành

- [ ] Board, cây, DAG mặc định không có `plan|phase`; công tắc hiển thị chúng.
- [ ] Task dưới Phase vẫn thấy (nâng lên gốc) kèm chip đường dẫn.
- [ ] Không đổi hành vi khi dữ liệu không có `plan|phase`.
- [ ] Không thêm `max-lines` disable; chuỗi i18n `TaskGraph.showPlanning` đủ 5 locale.

## Rủi ro và lưu ý

- `TaskGraph.test.tsx`, `TaskTreeView.test.tsx` có thể phụ thuộc cấu trúc cũ.
- Hiệu năng: `computeEffectiveParents` chạy O(n) mỗi lần lọc; memo hoá.
