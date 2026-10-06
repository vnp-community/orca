# FE-REQ-TASK-021-05: `RequestPlanTab`, trạng thái rỗng/lỗi và khoá Chạy theo Phase

**From Solution:** [FE-REQ-SOL-021](../solutions/FE-REQ-SOL-021-plan-phase-tree-and-approval-ui.md) mục 2.4, 2.6
**Priority:** P0
**Area:** frontend / request / plan + task
**File:** `frontend/src/renderer/src/components/request/plan/{RequestPlanTab,PlanStates}.tsx` (mới); `components/request/RequestDetailPane.tsx` (sửa: cắm tab); `store/slices/request.ts` (sửa: `executionGateByTaskId`, `setTaskExecutionGates`); `components/task/TaskDetail.tsx` (sửa quanh dòng 123-125); test cùng tên và `TaskDetail.test.tsx`
**Depends on:** FE-REQ-TASK-021-03, 021-04, 018-04
**Status:** [ ] TODO

## Context

- `TaskDetail()` không có props; `isRunning` ở dòng 123, `handleRunAgent` dòng 125 gọi `task.execute` (dòng 142). Chạy `impact` cho `TaskDetail` trước khi sửa (chưa chạy).
- Backend (CR-REQ-013) là nguồn chân lý cho điều kiện chạy; UI chỉ ẩn nút sớm, không thay thế kiểm tra.
- Tab ẩn với loại không có Plan (`REQUEST_FLOW_REGISTRY[type].plan==='none'`).

## Việc cần làm

1. `RequestPlanTab({request})`: `usePlanTree(request)`; render `PlanSummaryHeader` + `PlanApprovalBar` + `PlanTree` + `PlanGateChips`; công tắc "Hiện Plan/Phase" của Board không liên quan ở đây.
2. `PlanStates`: bảng SOL-021 2.6: "Chưa lập Plan" (Request chưa `planning`, kèm bước hiện tại từ `buildStageTimeline`); Skeleton "AI đang lập Plan" (`planning`); lỗi sinh + "Sinh lại Plan"; "Plan trống"; `forbidden`; `planTaskId` rỗng hoặc Plan không có trong danh sách ("Chưa tìm thấy Plan", không lỗi đỏ); `truncated` cảnh báo; `network` có "Thử lại".
3. Cắm `RequestPlanTab` vào `RequestDetailPane` (019-03); ẩn khi loại không có Plan.
4. `executionGateByTaskId`: trong `RequestPlanTab` (effect), ghi `setTaskExecutionGates(requestId, {taskId: 'phase_not_approved'|'plan_not_approved'})` cho Task dưới Phase/Plan chưa `approved`; xoá khi unmount hoặc khi Approval đổi.
5. `TaskDetail.tsx`: đọc `executionGateByTaskId[task.id]`; khi có thì khoá "Run agent" kèm tooltip `TaskDetail.phaseNotApproved`; không đổi hành vi khi không có gate.
6. Quay lại Request từ Task: giữ nguyên điều hướng hiện có (`openRequestPage`), không thêm đường chạy mới.

## Kiểm thử

- Component: mỗi trạng thái trong bảng; tab ẩn cho `question`/`spike`; Plan hiện sau `plan.generated` (event bus giả).
- `TaskDetail.test.tsx`: Task có gate thì nút Chạy khoá và có tooltip; Task không gate không đổi.
- Slice: `setTaskExecutionGates` hợp nhất và dọn theo `requestId`.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request/plan src/renderer/src/components/task/__tests__/TaskDetail src/renderer/src/store/slices/request`.

## Tiêu chí hoàn thành

- [ ] Mọi trạng thái rỗng/tải/lỗi/không quyền/không hỗ trợ có UI, không lỗi đỏ cho runtime thiếu tính năng.
- [ ] Task dưới Phase chưa duyệt không có nút Chạy khả dụng và có tooltip.
- [ ] Sau `phase.completed` cây cập nhật không tải lại trang.
- [ ] i18n `RequestPlanTab.*`, `PlanStates.*`, `TaskDetail.phaseNotApproved` đủ 5 locale.

## Rủi ro và lưu ý

- Gate chỉ có khi `RequestPlanTab` đã mở: mở Task từ Board trước đó vẫn thấy nút Chạy (backend chặn). Ghi rõ trong PR.
- `TaskDetail` đang lớn: chỉ thêm vài dòng đọc gate.
