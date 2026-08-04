# TASK-FE-018.1: Instrument `useTask().aiDecompose()` (BL-TG-02, AI decompose)

**Phase:** 3
**SOL Ref:** [SOL-FE-TRACE-018 §2.2](../solutions/SOL-FE-TRACE-018-task-graph.md)
**CR Ref:** [CR-TRACE-018](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001 — tracer `uiTaskGraphAiPlanFlow`/`uiTaskGraphExecuteFlow` đã đăng ký) + external TASK-BE-000
**Status:** ✅ Done (2026-08-04) — implemented as specced; `uiTaskGraphAiPlanFlow` already existed in tracers.ts (reused, no new entry added); tsc clean, 5/5 new tests pass in `useTask.test.ts`

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "useTask"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "useTask", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Click "🤖 Decompose with AI" trong `TaskAIDecompose` (`data-testid="decompose-btn"`) → `useTask(taskId).aiDecompose(instruction)` → RPC `tasks.aiPlan`. Tái sử dụng nguyên xi mô hình `parentTraceId` đã thiết lập ở [TASK-FE-017.2](./TASK-FE-017.2-workflow-cancel-execution.md) nếu cần nhóm nhiều thao tác trên cùng 1 task (optional, KHÔNG implement trong task này — xem §5 solution).

Field `promptLength` (không phải `instruction` đầy đủ) tránh log nội dung hướng dẫn AI dài — không phải bí mật nhưng không cần thiết trong trace field.

## File: `src/renderer/src/hooks/useTask.ts` [MODIFY]

```typescript
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { Tracers } from '../../../shared/trace/tracers'
import type { OrcaTask } from '../types/task-types'

export function useTask(taskId: string) {
  const task = useAppStore(s => s.tasks.find((t: OrcaTask) => t.id === taskId))

  const updateTask = async (patch: Partial<OrcaTask>) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'tasks.update', { taskId, ...patch })
    useAppStore.getState().updateTask(taskId, patch)
  }

  const deleteTask = async () => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'tasks.delete', { taskId })
    useAppStore.getState().removeTask(taskId)
  }

  const aiDecompose = async (instruction?: string) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // BL-TG-02: field `promptLength` thay vì instruction đầy đủ — tránh log nội
    // dung hướng dẫn AI dài, đúng tinh thần "chỉ trace field cần cho debug".
    const span = Tracers.uiTaskGraphAiPlanFlow.start({ taskId, hasInstruction: !!instruction, promptLength: instruction?.length ?? 0 })
    try {
      const result = await callRuntimeRpc(target, 'tasks.aiPlan', { taskId, instruction, traceId: span.id }) as { subtasks: Partial<OrcaTask>[] }
      span.ok({ taskId, subtaskCount: result.subtasks.length })
      return result.subtasks
    } catch (err) {
      span.fail(err, { taskId })
      throw err
    }
  }

  const acceptSubtasks = async (subtasks: Partial<OrcaTask>[], projectId: string) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // "Accept All" (tasks.createSubtasks) là CRUD hàng loạt đơn giản (INSERT N dòng,
    // không băng qua relay/AI) — không traced riêng.
    const createdSubtasks = await callRuntimeRpc(target, 'tasks.createSubtasks', { taskId, subtasks }) as OrcaTask[]
    for (const created of createdSubtasks) {
      useAppStore.getState().addTask(created)
    }
  }

  return { task, updateTask, deleteTask, aiDecompose, acceptSubtasks }
}
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/hooks/__tests__/useTask.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `useTask().aiDecompose()` tạo `ui:taskGraph.aiPlan` span, field `promptLength` phản ánh độ dài instruction (không log nội dung instruction đầy đủ), forward `traceId` vào `tasks.aiPlan`
- [ ] `aiDecompose()` không có instruction → `hasInstruction: false, promptLength: 0`
- [ ] `span.ok()` của `aiDecompose` mang `subtaskCount` khớp số lượng phần tử `result.subtasks` trả về
- [ ] RPC lỗi → `span.fail(err, { taskId })` trước khi re-throw
- [ ] `acceptSubtasks()`/`updateTask()`/`deleteTask()` KHÔNG được instrument trong task này (CRUD đơn, ngoài phạm vi BL-TG-02)
- [ ] Test suite đạt ≥ 5 test case mới theo Test Plan SOL-FE-TRACE-018 §4 (start với/không có instruction, traceId trong params, ok với subtaskCount, fail trước re-throw)
