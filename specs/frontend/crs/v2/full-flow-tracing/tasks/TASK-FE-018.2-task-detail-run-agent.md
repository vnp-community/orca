# TASK-FE-018.2: Instrument `TaskDetail.handleRunAgent()` (BL-TG-04, entry "Execute with Agent")

**Phase:** 3
**SOL Ref:** [SOL-FE-TRACE-018 §1, §2.3](../solutions/SOL-FE-TRACE-018-task-graph.md)
**CR Ref:** [CR-TRACE-018](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001 — tracer `uiTaskGraphExecuteFlow` đã đăng ký)
**Status:** ✅ Done (2026-08-04) — implemented as specced; `uiTaskGraphExecuteFlow` reused (already in tracers.ts, shared with TASK-FE-018.3); `tasks.runAgent` (plural) drift vs `TaskPromptEditor`'s `task.runAgent` confirmed unchanged/not fixed; tsc clean, 7/7 tests pass in `TaskDetail.test.tsx` (3 pre-existing + 4 new)

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "TaskDetail"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "TaskDetail", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

**Cảnh báo doc/code drift quan trọng — 2 RPC method khác nhau cho cùng 1 hành động "chạy agent trên task":** `TaskDetail.tsx:41` gọi `tasks.runAgent` (số nhiều, đúng theo TDD-FE-15 Addendum và HLD E.7). Method này khác với `task.runAgent` (số ít) mà `TaskPromptEditor.tsx` gọi (xem TASK-FE-018.3). Cả hai đều là entry point hợp lệ vào `TaskAgentExecutor.executeTask()` phía backend — task này instrument `TaskDetail.tsx`, dùng CHUNG tracer `ui:taskGraph.execute` với TASK-FE-018.3, phân biệt bằng field `entryPoint`.

## File: `src/renderer/src/components/task/TaskDetail.tsx` [MODIFY]

```typescript
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { Tracers } from '../../../../shared/trace/tracers'

export function TaskDetail() {
  // ...existing activeTaskId/task/updateTask/localTitle/activeTab/deps state unchanged...

  const handleRunAgent = async () => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // field `entryPoint: 'task-detail'` phân biệt với TaskPromptEditor (TASK-FE-018.3) —
    // 2 nút UI khác nhau cùng dẫn vào 1 tracer chung (BL-TG-04).
    const span = Tracers.uiTaskGraphExecuteFlow.start({ taskId: task.id, entryPoint: 'task-detail' })
    try {
      await callRuntimeRpc(target, 'tasks.runAgent', { taskId: task.id, traceId: span.id })
      span.ok({ taskId: task.id })
      toast.success(`Agent started for: ${task.title}`)
    } catch (err: any) {
      span.fail(err, { taskId: task.id })
      toast.error('Failed to start agent: ' + err.message)
    }
  }

  // ...existing JSX unchanged...
}
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/components/task/__tests__/TaskDetail.test.tsx
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] Click `run-agent-btn` → `Tracers.uiTaskGraphExecuteFlow.start({ taskId, entryPoint: 'task-detail' })`
- [ ] `tasks.runAgent` RPC nhận `traceId: span.id`
- [ ] RPC thành công → `span.ok({ taskId })`, toast success hiển thị
- [ ] RPC lỗi → `span.fail(err, { taskId })`, toast error hiển thị
- [ ] Method name `tasks.runAgent` (số nhiều) KHÔNG bị sửa trong task này — chỉ thêm `traceId` (doc/code drift với `task.runAgent` số ít được ghi nhận, dành cho 1 CR dọn dẹp riêng)
- [ ] Test suite đạt ≥ 4 test case mới theo Test Plan SOL-FE-TRACE-018 §4 (click → start với đúng entryPoint, traceId trong params, thành công → ok + toast, lỗi → fail + toast)
