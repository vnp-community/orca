# TASK-FE-E2E-007: Cây task: phân biệt rỗng, lỗi và không có quyền

**From Solution:** SOL-FE-E2E-003 · **Resolves:** FE-E2E-E, BUG-E2E-006
**Priority:** P2 · **Area:** frontend
**Files:**
- frontend/src/renderer/src/components/task/ (cây con)
- frontend/src/renderer/src/hooks/useTask*.ts
**Depends on:** TASK-E2E-011, 012
**Status:** `[~]` PARTIAL: mã + vitest xong (2026-10-10); tab Plan đã thấy Plan thật qua UI live; chưa thử với task cũ sau backfill TASK-E2E-012

---

## Context

`task.getSubtree` từng trả `{}` cho cả chủ task; UI không có cách phân biệt.

## Việc cần làm

- [x] Xác minh cách UI dùng `getSubtree`/`list` để dựng cây (đọc mã trước). Kết quả: **UI không gọi `task.getSubtree`**. Có hai đường dựng cây: (1) trang Task (`TaskGraph` ← `useTasks`) gọi `task.list {projectId}` và dựng cây từ `parentId`; (2) tab Plan của Request (`usePlanTree`) gọi `task.list` phân trang rồi `buildPlanSubtree`. Cả hai lọc phía máy chủ theo quyền nên task thiếu grant chủ sở hữu (BUG-E2E-006) chỉ đơn giản không có trong danh sách.
- [x] Trạng thái rỗng khác trạng thái lỗi:
  - Trang Task: `useTasks` nuốt mọi lỗi (`.catch(() => { /* silently fail */ })`) nên lỗi trông như "chưa có task". Nay có `loadError` + `loaded`; `TaskLoadStates` hiện "No tasks yet.", "No tasks match the current search or filter.", "Only Plan and Phase tasks exist...", "Loading tasks...", hoặc khối lỗi có nguyên nhân (`forbidden` có câu riêng "do not have access"), mã lỗi và "Retry"; nếu đã có danh sách cũ thì giữ lại và báo "Could not refresh...".
  - Tab Plan: đã phân biệt sẵn `error` (thử lại), `forbidden`, `empty`; bổ sung cho `not_found` một gợi ý "đã bị xoá hoặc bạn không có quyền với task của nó" và nút "Try again" (trước đó là câu cụt "Plan not found.").
- [x] Xoá task: `TASK_NOT_FOUND` coi là thành công (`shared/task-errors.ts`: nhận `TASK_NOT_FOUND`, `NOT_FOUND`, `code = NotFound`; không nhận `TASK_NOT_FOUND_BY_NUMBER`/`PROJECT_NOT_FOUND`). **Kèm sửa lỗi thật**: `useTask.deleteTask` gửi `{taskId}` còn handler `task.delete` giải mã `{id}`, nên id tới task-service luôn rỗng (`TASK_MISSING_ID`) và xoá không bao giờ chạy; nay gửi `{id}`.

## Tiêu chí hoàn thành

- [x] Test render ba trạng thái (rỗng / lỗi / không quyền) cho trang Task (`TaskLoadStates.test.tsx`, `TaskGraph.test.tsx`) và tab Plan (`RequestPlanTab.test.tsx`).
- [ ] "Thử với task cũ sau backfill": **chưa làm**, cần stack thật sau TASK-E2E-012.

## Kiểm thử

Vitest.

## Kết quả thực thi

**File đổi / mới**
- `shared/task-errors.ts` (+ test), `hooks/useTask.ts` (`deleteTask`), `hooks/useTasks.ts` (`loadError`, `loaded`, `totalTaskCount`), `components/task/TaskLoadStates.tsx` (mới), `components/task/TaskGraph.tsx` (gắn `TaskLoadStates`, vẫn trong ngưỡng max-lines), `components/request/plan/PlanStates.tsx` (gợi ý + nút ở `not_found`), `shared/request-errors.ts` (`TASK_NOT_FOUND`→`not_found`, `TASK_FORBIDDEN`→`forbidden`).
- Locale 5 ngôn ngữ: `auto.components.task.TaskLoadStates.*` (9 khoá), `plan.PlanStates.notFoundHint`.
- Test: `useTask.test.ts` (+3: gửi `{id}`, `TASK_NOT_FOUND` = thành công, lỗi khác vẫn ném và giữ task), `useTasks.test.ts` (+3), `TaskLoadStates.test.tsx` (7), `TaskGraph.test.tsx` (+1), `task-errors.test.ts` (14), `RequestPlanTab.test.tsx` (sửa 1 ca).

**Selector Playwright**: `task-load-error` (role=alert, `data-error-kind`), `task-load-error-code`, `task-empty`, `task-empty-filtered`, `task-empty-planning-hidden`, `task-loading`; tab Plan: `plan-state-not_found`, `plan-not-found-hint`, `plan-not-found-retry`, `plan-error`, `plan-state-forbidden`, `plan-state-empty`.

**Lệnh**: `pnpm exec vitest run --config config/vitest.config.ts src/renderer/src/components/task/__tests__/TaskLoadStates.test.tsx src/renderer/src/components/task/__tests__/TaskGraph src/renderer/src/hooks/__tests__/useTasks.test.ts src/renderer/src/hooks/__tests__/useTask.test.ts src/shared/task-errors.test.ts src/renderer/src/components/request/plan` → xanh.

**Blast radius (rà tay)**: `useTasks` có hai caller: `TaskGraph` (hiển thị trạng thái mới) và `WorktreeTaskLinkPicker` (chỉ dùng `filteredTasks`, không đổi). Trường trả về chỉ được thêm. `deleteTask` chưa có caller UI nào (chỉ hook và test) nên sửa `{id}` không đổi hành vi hiển thị hiện có. Mã `TASK_*` mới trong `CODE_TO_KIND` chỉ ảnh hưởng nơi gọi `classifyRequestRpcError`.

**Chưa làm / chưa kiểm**
- Không biết chắc định dạng văn bản lỗi của kênh `task.*` qua gateway (kênh này trả lỗi gRPC thô); `isTaskNotFoundError` đoán theo các dấu hiệu mà `suite_not_found_contract.py` cũng chấp nhận (`NOT_FOUND`, `TASK_NOT_FOUND`, `NotFound`).
- Trạng thái "không có quyền xem" phân biệt được khi máy chủ trả `forbidden`; còn trường hợp máy chủ **lọc im lặng** (task không có grant, trả danh sách thiếu) thì UI chỉ có thể báo "rỗng" hoặc, với Plan, `not_found` kèm gợi ý nói trên. Đó là giới hạn của hợp đồng, không phải của UI.

**Bổ sung (TASK-FE-E2E-009, 2026-10-10)**: tab Plan nay gọi `task.list` kèm `requestId` và `taskTypes: ['plan','phase','task','bug','feature','epic']` (nguyên nhân tab Plan luôn "Plan not found." trên stack thật); đã có Playwright mock cho Plan (ca h) và cho Clarification bị chặn (ca g) trong `request-solution-start-and-ai-setup.web.e2e.ts`.
