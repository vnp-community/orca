# CR-PW-003 — Tab Workflows trong Project Workspace là stub tĩnh, không lấy dữ liệu từ đâu cả

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-PW-003 |
| **Tên** | Nối tab Workflows vào `workflow.listExecutions` RPC đã có sẵn ở backend |
| **Loại** | Feature Completion (không phải tính năng mới) |
| **Priority** | 🟡 P1 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-06 |
| **Trạng thái** | ✅ Implemented — xem [FE-SOL-003](../../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-003-wire-workflow-monitor-to-listExecutions.md) |
| **Tác giả** | Investigation từ câu hỏi user: "tab Tasks và Workflows lấy dữ liệu từ đâu?" |
| **Tác động HLD** | F38 — Project Workspace |
| **Tác động Features** | Workflows tab |

---

## Bối cảnh & Vấn đề

Tab "Workflows" trong Project Workspace hiện là **stub tĩnh**, toàn bộ implementation:

```typescript
// frontend/src/renderer/src/components/workflow/WorkflowMonitor.tsx
// WorkflowMonitor.tsx — Workflow monitor stub (full implementation in TASK-V5-08)
export function WorkflowMonitor() {
  return <div ...>Workflow Monitor</div>
}
```

Không gọi RPC nào, không đọc store nào — hiển thị y hệt cho mọi project, mọi trạng thái.

## Backend đã sẵn sàng — chỉ thiếu người gọi

`backend/src/main/workflow/workflow-rpc-handler.ts` có đầy đủ `workflow.listExecutions`
(filter theo `projectId`/`triggeredBy`/`status`), `workflow.getExecution`, `workflow.cancel`,
`workflow.pause`, `workflow.resume`, backed bởi DAG orchestrator thật + Postgres
(`orca_workflow_executions`, migration `0014_workflow_pause_state.ts`). Grep xác nhận **0 caller**
của `workflow.listExecutions` trong toàn bộ `frontend/src`.

Đã có sẵn 1 phần UI thật cho **1 execution đơn lẻ**: `ExecutionMonitor.tsx` +
`useWorkflowExecution.ts` (waves, step status, cancel button) — nhưng không có UI nào liệt kê
NHIỀU execution để chọn vào xem. Đây chính là mảnh còn thiếu.

Bằng chứng thêm, comment thật trong code (`frontend/src/renderer/src/store/slices/workflow.ts:34-36`):

> *"...this slice had it too, just never live-triggered yet since WorkflowMonitor's fetches don't
> succeed on this deployment."*

— xác nhận: đội ngũ trước đã biết `WorkflowMonitor` **được kỳ vọng** fetch dữ liệu, nhưng chưa bao
giờ làm.

## Giải pháp (tóm tắt — chi tiết ở FE-SOL-003)

`WorkflowMonitor.tsx` nhận `projectId`, gọi `workflow.listExecutions({ projectId })`, hiển thị
danh sách (status, thời điểm bắt đầu, người trigger); bấm vào 1 dòng mở `<ExecutionMonitor
executionId=... />` đã có sẵn — tái dùng, không viết lại phần theo dõi step/wave.

## Không thuộc phạm vi CR này

- Sửa `useWorkflowExecution.ts`/`ExecutionMonitor.tsx`'s phụ thuộc vào `window.api.on` (bridge
  Electron-only) — nghi vấn không hoạt động ở Web mode (b15.openledger.vn), nhưng là 1 lớp bug
  khác (live push event, không phải "list dữ liệu ban đầu"); ghi nhận làm follow-up riêng, không
  fix ở CR này để tránh mở rộng phạm vi ngoài câu hỏi gốc.
- `StepStatusBadge` không xử lý status `'cancelled'` của `WorkflowExecution` (chỉ có
  `pending/running/completed/failed/skipped`) — bug tiềm ẩn có sẵn trong `ExecutionMonitor.tsx`,
  không phải do CR này gây ra; solution ở đây **không dùng lại** component đó cho danh sách để
  tránh lặp lại lỗi tương tự, nhưng không sửa `ExecutionMonitor.tsx` gốc — follow-up riêng.
- Backend/backend-go: **không cần đổi gì** — RPC đã đủ. Không có agent-side impact.

## Liên quan

- `backend/src/main/workflow/workflow-rpc-handler.ts`
- `frontend/src/renderer/src/store/slices/workflow.ts`
- [FE-SOL-003](../../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-003-wire-workflow-monitor-to-listExecutions.md)
