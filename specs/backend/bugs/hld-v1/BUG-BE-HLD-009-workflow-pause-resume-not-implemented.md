# BUG-BE-HLD-009 — Workflow pause/resume (user-triggered) hoàn toàn không tồn tại — chỉ có crash-recovery resume

**Mức độ:** 🟠 HIGH (Feature gap)
**Status:** 🔴 Open
**Module:** `backend/src/main/workflow/WorkflowTypes.ts`, `WorkflowOrchestrator.ts`, `workflow-rpc-handler.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.15/F36, §2.10)

---

## Mô tả

`docs/features/F36-multi-server-workflow-orchestration.md` mô tả nút "[Pause]" trong Workflow Execution UI và `StepState`/`WorkflowExecution.status` có giá trị `'paused'`. `docs/hld/backend-server-architecture.md` §9 cũng liệt kê RPC `workflows.pause`/`workflows.resume`.

Thực tế:
- `WorkflowStatus` type (`backend/src/main/workflow/WorkflowTypes.ts:16`) chỉ có `'pending'|'running'|'completed'|'failed'|'cancelled'` — **không có `'paused'`**.
- `WorkflowOrchestrator` không có method `pause()`. Chỉ có `cancel()` (abort hẳn, không thể resume lại) và `resumeRunningExecutions()` (`WorkflowOrchestrator.ts:238-257`) — nhưng đây là **crash-recovery khi Orca Server restart** (đọc lại execution có status `running`, tiếp tục từ wave dở dang), **không phải** cơ chế user-triggered pause/resume qua UI.
- RPC namespace `workflow.*` thực tế chỉ có 7 method: `execute, getExecution, listExecutions, cancel, template.create, template.list, template.resolve` — không có `pause`/`resume`.

## Hậu quả

- Nút "[Pause]" trong UI (nếu tồn tại ở frontend) không có backing RPC/service nào — không hoạt động.
- User không thể tạm dừng 1 workflow execution đang chạy để can thiệp thủ công (vd sửa 1 file trước khi bước tiếp theo chạy) — chỉ có thể `cancel()` (huỷ hẳn, mất tiến trình).

## Bằng chứng

- `backend/src/main/workflow/WorkflowTypes.ts:16` — enum `WorkflowStatus` thiếu `'paused'`.
- `backend/src/main/workflow/WorkflowOrchestrator.ts:212-232` (`cancel()`) vs `:238-257` (`resumeRunningExecutions()`, crash-recovery only).
- `backend/src/main/workflow/workflow-rpc-handler.ts:5-8` — danh sách 7 method thật, không có pause/resume.

## Đề xuất fix

1. Thêm `'paused'` vào `WorkflowStatus`, thêm cột tương ứng nếu cần trong `orca_workflow_executions`.
2. Implement `WorkflowOrchestrator.pause(executionId)` — dừng dispatch wave tiếp theo, giữ nguyên state hiện tại (không phải rollback).
3. Implement RPC `workflow.pause`/`workflow.resume` (user-triggered), phân biệt rõ với `resumeRunningExecutions()` (internal crash-recovery, giữ nguyên).

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.15 (F36), §2.10, §6 mục 6 (Top 10)
- Doc gốc: `docs/features/F36-multi-server-workflow-orchestration.md`, `docs/hld/backend-server-architecture.md` §9
- Liên quan: [BUG-BE-HLD-008](./BUG-BE-HLD-008-workflow-provider-selection-not-implemented.md), [BUG-WF-001](../workflow-orchestration/BUG-WF-001-server-spec-not-implemented.md) (bug cũ cùng domain, `server:<devServerId>` dispatch chưa implement)
