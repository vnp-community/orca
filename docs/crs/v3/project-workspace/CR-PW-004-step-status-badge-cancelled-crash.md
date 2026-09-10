# CR-PW-004 — `StepStatusBadge` crash khi status = `'cancelled'`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-PW-004 |
| **Tên** | `StepStatusBadge` xử lý `'cancelled'` (`WorkflowExecutionStatus`), không chỉ `StepStatus` |
| **Loại** | Bug Fix |
| **Priority** | 🔴 P0 (crash) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-06 |
| **Trạng thái** | ✅ Implemented — xem [FE-SOL-004](../../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-004-step-status-badge-cancelled.md) |
| **Tác giả** | Follow-up đã ghi nhận sẵn ở CR-PW-003's "Không thuộc phạm vi" |
| **Tác động HLD** | F38 — Project Workspace |
| **Tác động Features** | Workflows tab, `ExecutionMonitor` |

---

## Bối cảnh & Vấn đề

`ExecutionMonitor.tsx` gọi `<StepStatusBadge status={execution.status as any} />` (dòng ~35) để
hiển thị trạng thái tổng của 1 execution. `execution.status` có type `WorkflowExecutionStatus`
(`pending | running | completed | failed | cancelled` —
`frontend/src/shared/workflow-types.ts:47`), nhưng `StepStatusBadge`'s `STEP_STATUS` map chỉ có
key cho `StepStatus` (`pending | running | completed | failed | skipped` — cùng file, dòng kế
tiếp). Cast `as any` che giấu lỗi kiểu tại compile-time; ở runtime, `STEP_STATUS['cancelled']`
trả về `undefined`, và:

```typescript
const { icon, className, label } = STEP_STATUS[status] // throws: Cannot destructure
                                                         // property 'icon' of undefined
```

→ Bất kỳ execution nào bị cancel (qua nút Cancel có sẵn trong `ExecutionMonitor`, hoặc do bị huỷ
từ nơi khác) đều làm crash toàn bộ `ExecutionMonitor` khi render lại — user không xem được kết
quả cuối cùng của execution đó nữa.

Bug này đã được phát hiện và ghi nhận (không sửa) trong CR-PW-003's "Không thuộc phạm vi CR này"
— CR này là follow-up thực hiện đúng như đã hẹn.

## Giải pháp (tóm tắt — chi tiết ở FE-SOL-004)

Mở rộng `StepStatusBadge`'s prop type thành union `StepStatus | WorkflowExecutionStatus` và thêm
1 entry `cancelled` vào `STEP_STATUS` map. Bỏ cast `as any` ở `ExecutionMonitor.tsx` — TypeScript
giờ tự xác nhận `execution.status` khớp prop type mà không cần ép kiểu.

## Không thuộc phạm vi CR này

- Không đổi `useWorkflowExecution.ts`'s live-update transport — đó là CR-PW-006.
- Không đổi cách `WorkflowMonitor.tsx` (CR-PW-003) tự vẽ label riêng cho danh sách — component đó
  cố ý không dùng lại `StepStatusBadge` (đã ghi rõ trong code comment của nó), không liên quan gì
  đến bug này.
- Backend/backend-go/agent: không cần đổi gì — đây là bug thuần UI component.

## Liên quan

- `frontend/src/renderer/src/components/workflow/StepStatusBadge.tsx`
- `frontend/src/renderer/src/components/workflow/ExecutionMonitor.tsx`
- `frontend/src/shared/workflow-types.ts`
- [CR-PW-003](./CR-PW-003-workflows-tab-wiring.md) — nơi bug này được phát hiện lần đầu
- [FE-SOL-004](../../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-004-step-status-badge-cancelled.md)
