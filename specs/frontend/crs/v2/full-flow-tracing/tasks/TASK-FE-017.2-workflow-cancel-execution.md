# TASK-FE-017.2: Instrument `useWorkflowExecution().cancelExecution()` + đọc `parentTraceId`

**Phase:** 3
**SOL Ref:** [SOL-FE-TRACE-017 §3.2](../solutions/SOL-FE-TRACE-017-workflow-orchestration.md)
**CR Ref:** [CR-TRACE-017](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001 — tracer `uiWorkflowCancelFlow` đã đăng ký) + TASK-FE-017.1 (field `rootTraceId` trên `WorkflowExecution`)
**Status:** ✅ Done (2026-08-04) — Implemented as specced, no drift (current source matched the task doc's pre-state exactly). `useWorkflowExecution.test.ts` did not exist — created new, 5/5 passing. `pnpm tsc --noEmit` clean.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "useWorkflowExecution"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "useWorkflowExecution", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

BL-WF-02 mở rộng — hành động "Cancel" không có trong CR-TRACE-017 gốc nhưng bổ sung vì cùng vòng đời execution. Click "Cancel" trong `ExecutionMonitor` (khi `status === 'running'`) → `cancelExecution()` → RPC `workflow.cancel`. Field `parentTraceId` (đọc từ `execution.rootTraceId` đã lưu ở TASK-FE-017.1) nhóm thao tác cancel này vào cùng execution trong TracePanel — **đây là field nghiệp vụ tự do, KHÔNG dùng cơ chế `resume`** (span cancel vẫn có id riêng của chính nó).

Cùng bug tiền tồn tại về signature `callRuntimeRpc` như TASK-FE-017.1 — sửa đúng trong file này.

## File: `src/renderer/src/hooks/useWorkflowExecution.ts` [MODIFY]

```typescript
import { useEffect, useCallback } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { Tracers } from '../../../shared/trace/tracers'

export function useWorkflowExecution(executionId: string) {
  const { stepStatuses, streamingOutput } = useAppStore(s => ({
    stepStatuses:    s.stepStatuses[executionId] ?? {},
    streamingOutput: s.streamingOutput,
  }))
  const execution = useAppStore(s => s.executions.find(e => e.id === executionId))

  useEffect(() => {
    if (!(window as any).api?.on) return
    const unsubs = [
      // Backend push event — nếu companion backend solution đính kèm `parentTraceId`
      // (= rootTraceId của execution) trong payload, lưu lại để hiển thị. Không bắt
      // buộc cho status hoạt động đúng — chỉ phục vụ hiển thị/debug.
      (window as any).api.on('workflow:stepStatus', ({ execId, stepId, status }: any) => {
        if (execId !== executionId) return
        useAppStore.getState().setStepStatus(execId, stepId, status)
      }),
      (window as any).api.on('workflow:stepOutput', ({ execId, line }: any) => {
        if (execId !== executionId) return
        useAppStore.getState().appendStreamLine(execId, line)
      }),
      (window as any).api.on('workflow:complete', ({ execId, status }: any) => {
        if (execId !== executionId) return
        useAppStore.getState().updateExecutionStatus(execId, status)
      }),
    ]
    return () => unsubs.forEach((u: any) => u?.())
  }, [executionId])

  const cancelExecution = useCallback(async () => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const rootTraceId = useAppStore.getState().executions.find(e => e.id === executionId)?.rootTraceId
    // Field `parentTraceId` (không phải resume) nhóm thao tác cancel này vào cùng
    // execution trong TracePanel, dù span này có id riêng.
    const span = Tracers.uiWorkflowCancelFlow.start({ executionId, parentTraceId: rootTraceId })
    try {
      await callRuntimeRpc(target, 'workflow.cancel', { executionId, traceId: span.id })
      useAppStore.getState().updateExecutionStatus(executionId, 'cancelled')
      span.ok({ executionId })
    } catch (err) {
      span.fail(err, { executionId })
      throw err
    }
  }, [executionId])

  return { execution, stepStatuses, streamingOutput, cancelExecution }
}
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/hooks/__tests__/useWorkflowExecution.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `useWorkflowExecution().cancelExecution()` tạo `ui:workflow.cancel` span với field `parentTraceId` trỏ về `execution.rootTraceId` (nhóm hiển thị, KHÔNG dùng cơ chế `resume`)
- [ ] `traceId: span.id` forward vào `workflow.cancel` params (đúng chữ ký `callRuntimeRpc(target, method, params)`)
- [ ] RPC `workflow.cancel` reject → `span.fail(err, { executionId })`, status KHÔNG chuyển `'cancelled'`
- [ ] Thành công → `updateExecutionStatus(executionId, 'cancelled')` rồi `span.ok({ executionId })`
- [ ] Không tạo span FE riêng cho từng `workflow:stepExecute` — step-level tracing là trách nhiệm của backend, hook chỉ hiển thị qua SSE/push event (`workflow:stepStatus`, `workflow:stepOutput`, `workflow:complete`) mà không traced
- [ ] Test suite đạt ≥ 3 test case mới: `start({ executionId, parentTraceId })` lấy đúng từ `execution.rootTraceId`; forward `traceId`; reject → `fail()`, status không đổi
