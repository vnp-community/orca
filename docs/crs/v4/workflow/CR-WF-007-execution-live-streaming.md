# CR-WF-007 — Workflow Execution Live Streaming (step.output/step.completed → WS)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WF-007 |
| **Tên** | Emit event thời gian thực từ `wave_dispatcher.go`, tái dùng contract của CR-FLOW-TASK-003 thay vì tự thiết kế event schema riêng |
| **Loại** | Feature / Observability |
| **Priority** | P2 — không chặn execution, chỉ chặn trải nghiệm real-time |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | **Cứng phụ thuộc** [`CR-FLOW-TASK-003`](../../v3/flow-task/CR-FLOW-TASK-003-unified-task-activity-event-catalog.md) đã merge trước — không build trước để tránh 2 event schema không tương thích |
| **Tham chiếu** | `specs/backend-go/bugs/logic-v1/BUG-WF-02-workflow-execution-partial.md`, `specs/backend-go/bugs/missing-v1/BUG-030-workflow-channels-not-implemented.md` |
| **Tác động** | `backend-go/services/workflow-service/internal/usecase/wave_dispatcher.go`, `frontend/src/renderer/src/components/workflow/ExecutionMonitor.tsx`, `hooks/useWorkflowExecution.ts` |

---

## 1. Vấn đề

`WorkflowService` không có RPC nào để push step/execution event tới client
— `workflow.proto` không có `StreamExecutionEvents` (dù TDD §3 từng sketch,
chưa bao giờ thêm vào proto thật). `channels_workflow.go`'s 11 channel không
có `step.output`/`step.completed`. Client duy nhất tự xoay xở bằng cách poll
`workflow.getExecution` mỗi 4 giây (`useWorkflowExecution.ts:8,50-52`) — tự
comment nhận đây là "interim, pre-Phase-D stopgap" thay thế 1 cầu nối
`window.api.on(...)` từng bị hỏng, âm thầm no-op trên Web mode.

`ExecutionMonitor.tsx`'s `streamingOutput` state luôn rỗng trong thực tế —
dead code path chờ 1 nguồn dữ liệu chưa bao giờ tồn tại.

## 2. Giải pháp đề xuất

### 2.1 Không tự thiết kế event schema — cắm vào catalog của CR-FLOW-TASK-003

`CR-FLOW-TASK-003` đã thiết kế đúng phần này ở mức cross-engine: outbox/
eventbus mở rộng sang Engine 2+3, gộp vào 1 kênh WS `task.activity` theo
`origin_task_id`, publish `orca.workflow.step.completed`/`.failed` từ
`wave_dispatcher.go` là điểm mở rộng CR-003 đã định vị sẵn. Việc của CR-WF-007
là **implement đúng điểm publish đó**, không phát minh lại:

```go
// wave_dispatcher.go — sau khi 1 step hoàn tất/fail (điểm CR-FLOW-TASK-003 đã chỉ định)
outbox.Publish(ctx, tx, "orca.workflow.step.completed", StepCompletedEvent{
    ExecutionID: exec.ID, StepID: step.ID, Status: result.Status,
    OriginTaskID: exec.OriginTaskID, // rỗng nếu execution không qua Task (CR-FLOW-TASK-002)
})
```

### 2.2 Kênh riêng cho execution KHÔNG qua Task (chạy độc lập)

`task.activity:{taskId}` (CR-003) chỉ có ý nghĩa khi execution có
`origin_task_id`. Với execution chạy độc lập (không qua Task — vẫn là
use-case hợp lệ của F36, xem CR-FLOW-TASK-002 §Rủi ro), cần 1 kênh tương
đương theo `executionId`:

```
workflow.activity:{executionId}  — cùng shape event, chỉ khác key subscribe
```

### 2.3 Frontend — thay polling bằng subscription, giữ polling làm fallback

```tsx
// useWorkflowExecution.ts
useEffect(() => {
  const channel = execution.originTaskId ? `task.activity:${execution.originTaskId}` : `workflow.activity:${executionId}`
  const unsub = subscribeRuntimeEvent(target, channel, (frame) => dispatch({ type: 'step-update', frame }))
  return unsub
}, [executionId])
// Giữ nguyên poll 4s làm fallback nếu subscribeRuntimeEvent không khả dụng
// (một số deploy target Web mode) — không xoá hẳn, chỉ hạ ưu tiên.
```

`ExecutionMonitor.tsx`'s `streamingOutput` state nay có nguồn dữ liệu thật để
hiển thị thay vì dead code path.

## 3. Rủi ro / Không thuộc phạm vi

- **Không build trước khi CR-FLOW-TASK-003 merge** — làm vậy sẽ tạo 2 event
  schema không tương thích, phải migrate lại sau. Đây là điều kiện tiên
  quyết cứng, không phải khuyến nghị.
- Không giải quyết streaming **liên tục** (từng chunk stdout của step
  `agent`/`shell`) — đó là phạm vi
  [`docs/crs/v4/task-graph/CR-TG-006`](../task-graph/CR-TG-006-task-execute-streaming-relay.md)'s
  cơ chế `chunk` notification, CR này chỉ xử lý event **rời rạc** (step
  bắt đầu/hoàn tất/fail), đúng theo CR-FLOW-TASK-003's phạm vi đã định.
- Không thuộc phạm vi: đổi cơ chế outbox/transactional publish (dùng nguyên
  cơ chế CR-003/SOL-PW-04 đã có).

## Acceptance Criteria

- [ ] `orca.workflow.step.completed`/`.failed` được publish đúng tại điểm
      CR-FLOW-TASK-003 đã định vị trong `wave_dispatcher.go`, đúng shape
      catalog chung.
- [ ] Execution có `origin_task_id` → event xuất hiện trên kênh
      `task.activity:{taskId}`; execution không có → xuất hiện trên
      `workflow.activity:{executionId}`.
- [ ] Frontend nhận event qua subscription, `ExecutionMonitor` cập nhật UI
      không cần chờ tới lần poll 4s kế tiếp.
- [ ] Polling 4s vẫn hoạt động như fallback khi subscription không khả dụng
      (test trên Web mode target giả lập không có WS).
- [ ] Không phát sinh event trùng lặp khi `resumeRunningExecutions()` chạy
      lại sau restart Orca Server (test riêng cho path resume).
