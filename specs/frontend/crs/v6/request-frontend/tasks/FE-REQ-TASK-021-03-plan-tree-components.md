# FE-REQ-TASK-021-03: Thành phần cây Plan → Phase → Task

**From Solution:** [FE-REQ-SOL-021](../solutions/FE-REQ-SOL-021-plan-phase-tree-and-approval-ui.md) mục 2.3
**Priority:** P0
**Area:** frontend / request / plan
**File:** `frontend/src/renderer/src/components/request/plan/{PlanSummaryHeader,PlanTree,PhaseNode,PlanTaskRow}.tsx` (mới); test cùng tên
**Depends on:** FE-REQ-TASK-021-02, 018-05 (badge), 018-06
**Status:** [x] DONE (verified 2026-10-07: vitest components/request/plan/PlanTree.test.tsx (10 tests) pass; oxlint+tsc clean)

**Ghi chú:** `TaskDetail` trong `Sheet` mới kiểm chứng với `TaskDetail` được stub; chưa xem bố cục thật. `PlanTree` nạp Task vào store (`addTask`) nếu chưa có vì `TaskDetail` đọc từ store. Dùng `TaskStatusBadge`/`ExecutionEngineBadge` hiện có (còn emoji/màu thô, không sửa).

## Context

- Tái dùng `components/task/TaskStatusBadge.tsx`, `ExecutionEngineBadge.tsx`, `TaskDetail.tsx` (đọc `activeTaskId` từ store, không có props), `ui/progress.tsx`, `ui/collapsible.tsx`, `ui/sheet.tsx`.
- Plan/Phase không có `#TG-N` (O2); trạng thái suy từ con (O1): chỉ đọc.
- `progressPercent` từ backend (cascade, CR-REQ-011 chưa triển khai); dự phòng `TASK_STATUS_PROGRESS`.

## Việc cần làm

1. `PlanSummaryHeader({tree, approvals})`: tên Plan, `ApprovalStatusBadge`, `Progress` (nếu `progressPercent` trống tính từ `TASK_STATUS_PROGRESS` và gắn nhãn "ước tính"), số Phase/Task. Ghim ở trên (`sticky`), thân cuộn bên dưới.
2. `PlanTree({tree})`: loại có Phase thì liệt kê `PhaseNode`; loại `task_list` thì danh sách phẳng `PlanTaskRow`.
3. `PhaseNode({phase, tasks, approval})`: `Collapsible`; tiêu đề, `TaskStatusBadge`, `Progress`, `computePhaseStats` ("3/5 task xong", chip `blocked` dùng `destructive`, đang chạy dùng `primary`); `ArrowRight`/`ArrowLeft` mở/thu khi hàng có tiêu điểm; `aria-expanded`. Chỗ cắm `PhaseApprovalBar` (021-04).
4. `PlanTaskRow({task})`: tiêu đề, `taskNumber` chỉ khi có (Task làm việc), `TaskStatusBadge`, `ExecutionEngineBadge`, `estimatedHours`, "bị chặn bởi" (từ `task.getDependencies` chỉ khi mở chi tiết; ở hàng chỉ chip `blocked`). Bấm → `setActiveTask(task.id)` và mở `Sheet` có `TaskDetail`; đóng `Sheet` thì `setActiveTask(null)`. Chưa kiểm chứng `TaskDetail` hiển thị đúng trong `Sheet`.
5. Không có dropdown trạng thái, không kéo-thả, không nút Chạy riêng ở cây (Chạy theo `TaskDetail`, 021-05).
6. Chỉ token màu, icon lucide; không hex; thông tin trạng thái có chữ, không chỉ màu.

## Kiểm thử

- Component: cây 2 Phase × 3 Task; `task_list` phẳng; mở/thu bằng bàn phím; thống kê Phase; Plan/Phase không hiện `#TG-N`; không có dropdown trạng thái; bấm Task đặt `activeTaskId` và mở `Sheet`; tiến độ dự phòng có nhãn "ước tính".
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request/plan`.

## Tiêu chí hoàn thành

- [ ] Cây hiển thị đúng thứ tự Plan → Phase → Task; tiến độ khớp `progressPercent`.
- [ ] Phím mũi tên mở/thu; tiêu điểm nhìn thấy.
- [ ] i18n `PlanSummaryHeader.*`, `PlanTaskRow.*`, `PhaseNode.*` đủ 5 locale.

## Rủi ro và lưu ý

- `TaskDetail` trong `Sheet` có thể vỡ bố cục: nếu vậy mở bằng `openTaskPage` hoặc tách phần nhìn của `TaskDetail` (quyết định khi triển khai, ghi vào PR).
- Cây lớn (hàng trăm Task): cân nhắc ảo hoá nếu đo chậm (chưa đo).
