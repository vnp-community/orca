# FE-TASK-004: Thêm `setExecutions` vào `WorkflowSlice`

**Domain:** project-workspace
**Solution Ref:** FE-SOL-003 Bước 1
**Priority:** 🟡 P1 — prerequisite cho FE-TASK-005
**Estimated:** 10 phút
**Status:** ✅ DONE (2026-09-06)

**Kết quả thực tế:** Đúng như kế hoạch. Cũng cập nhật lại comment cũ ở đầu file
(`"...never live-triggered yet since WorkflowMonitor's fetches don't succeed on this
deployment."`) — claim đó không còn đúng sau CR-PW-003, sửa thành trỏ tới nơi giờ đã gọi
`setExecutions` thật.

---

## Mục tiêu

Slice có `addExecution`/`updateExecutionStatus` nhưng thiếu setter bulk-replace cho kết quả
`workflow.listExecutions` ban đầu.

## Files cần sửa

1. `frontend/src/renderer/src/store/slices/workflow.ts`

## Các bước thực thi

```typescript
// Thêm vào WorkflowSlice type
setExecutions(executions: WorkflowExecution[]): void

// Thêm vào createWorkflowSlice — giữ convention "return object, không mutate" đã ghi chú sẵn
// trong file (bug class BUG-FE-TASKGRAPH-SETTINGS)
setExecutions: (executions) => set(() => ({ executions })),
```

## Verify

```bash
grep -n "setExecutions" frontend/src/renderer/src/store/slices/workflow.ts
cd frontend && npx tsc --noEmit -p .
```

## Depends on
Không có

## Blocking
FE-TASK-005
