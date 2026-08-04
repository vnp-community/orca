# TASK-FE-018.3: Instrument `TaskPromptEditor.runWithAgent()` (BL-TG-04, entry "AI Agent tab")

**Phase:** 3
**SOL Ref:** [SOL-FE-TRACE-018 §1, §2.4](../solutions/SOL-FE-TRACE-018-task-graph.md)
**CR Ref:** [CR-TRACE-018](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001 — tracer `uiTaskGraphExecuteFlow` đã đăng ký) — độc lập với TASK-FE-018.2, có thể làm song song
**Status:** ✅ Done (2026-08-04) — implemented as specced; `uiTaskGraphExecuteFlow` reused (shared with TASK-FE-018.2, distinguished via `entryPoint: 'prompt-editor'`); `task.runAgent` (singular) method name kept unchanged per doc — drift vs `TaskDetail`'s `tasks.runAgent` documented, not fixed; BL-TG-01/BL-TG-03 confirmed no FE call site exists, no instrumentation added; tsc clean, 4/4 new tests pass in new `TaskPromptEditor.test.tsx`. NOTE: mid-task, a concurrent agent's write briefly reverted `useTask.ts`/`TaskDetail.tsx`/`TaskDetail.test.tsx` and clobbered `tracers.ts` to an older revision missing the `uiTaskGraph*` entries — re-applied all edits and additively re-added the two tracer entries per the collision-avoidance rule; final state re-verified via tsc + full test run after re-applying.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "TaskPromptEditor"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "TaskPromptEditor", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Click "▶ Run with Agent" trong tab "AI Agent" (`TaskPromptEditor`, `data-testid="run-agent-btn"` — cùng testid với `TaskDetail.tsx` nhưng khác file/component) → `runWithAgent()`, gọi RPC **`task.runAgent`** (số ít — khác với `tasks.runAgent` số nhiều mà `TaskDetail.tsx` dùng, xem TASK-FE-018.2). Đây là 2 call site độc lập, cả hai thực sự tồn tại trong code; task này **KHÔNG giả định cái nào "đúng" hơn** và **KHÔNG sửa routing RPC** — chỉ thêm `traceId`. Method `task.runAgent` (số ít) có thể 404/không tồn tại phía backend — nếu đúng vậy, nút "Run with Agent" này đang lỗi từ trước khi có CR tracing, độc lập với task này.

## File: `src/renderer/src/components/task/TaskPromptEditor.tsx` [MODIFY]

```typescript
import { useState } from 'react'
import { useWorkspace } from '../../context/WorkspaceContext'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { Tracers } from '../../../../shared/trace/tracers'

export function TaskPromptEditor({ task }: { task: OrcaTask }) {
  const [prompt, setPrompt]       = useState(task.agentPrompt ?? '')
  const [isRunning, setIsRunning] = useState(false)
  const { project }               = useWorkspace()

  const runWithAgent = async () => {
    setIsRunning(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const span = Tracers.uiTaskGraphExecuteFlow.start({ taskId: task.id, entryPoint: 'prompt-editor', promptLength: prompt.length })
    try {
      // NOTE (doc/code drift): method 'task.runAgent' (số ít) khác với 'tasks.runAgent'
      // mà TaskDetail dùng. Giữ nguyên method name hiện tại vì sửa routing RPC không
      // thuộc phạm vi CR tracing này — chỉ thêm traceId.
      await callRuntimeRpc(target, 'task.runAgent', { taskId: task.id, prompt: prompt || task.agentPrompt, projectId: project!.id, traceId: span.id })
      span.ok({ taskId: task.id })
    } catch (err) {
      span.fail(err, { taskId: task.id })
      throw err
    } finally {
      setIsRunning(false)
    }
  }

  // ...existing JSX unchanged...
}
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/components/task/__tests__/TaskPromptEditor.test.tsx
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] Click `run-agent-btn` (trong `TaskPromptEditor.tsx`) → `Tracers.uiTaskGraphExecuteFlow.start({ taskId, entryPoint: 'prompt-editor', promptLength })`
- [ ] `task.runAgent` RPC (method name giữ nguyên như code hiện tại — KHÔNG sửa thành `tasks.runAgent`) nhận `traceId: span.id`
- [ ] `TaskDetail.handleRunAgent()` (TASK-FE-018.2) và `TaskPromptEditor.runWithAgent()` dùng chung 1 tracer `ui:taskGraph.execute`, phân biệt bằng field `entryPoint: 'task-detail' | 'prompt-editor'` — verify bằng test entryPoint khác nhau giữa 2 file test
- [ ] Doc/code drift `tasks.runAgent` vs `task.runAgent` được ghi nhận rõ ràng (đã ở mục Mô tả) làm căn cứ cho 1 CR dọn dẹp riêng (không phải task này)
- [ ] BL-TG-01 (add dependency/edge) không có instrumentation nào được thêm — không tìm thấy UI trigger tồn tại; đánh dấu "chưa triển khai — cần điều tra thêm" thay vì tạo call site giả định
- [ ] BL-TG-03 (grant resolution) không có tracer FE riêng — permission-denied errors surface qua `span.fail(err)` của `ui:taskGraph.execute` sẵn có
- [ ] Test suite đạt ≥ 3 test case mới theo Test Plan SOL-FE-TRACE-018 §4
