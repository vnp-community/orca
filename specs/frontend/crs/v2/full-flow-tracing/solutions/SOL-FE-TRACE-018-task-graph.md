# SOL-FE-TRACE-018: Task Graph — Frontend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-018](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md)
**TDD Ref:** [TDD-FE-15](../../../../tdd/v5/15-task-graph-ui.md) (Task Graph UI, F37, ADR-010)
**Status:** Proposed
**Dependency:** F40 core tracing infra (đã implement); core API `Tracer.start(fields?, resume?)` từ CR-TRACE-000 §3.1 (**chưa ship** — xem SOL-FE-TRACE-015 §2.0); mô hình `parentTraceId` field nghiệp vụ đã thiết lập ở [SOL-FE-TRACE-017 §2](./SOL-FE-TRACE-017-workflow-orchestration.md#2-mô-hình-parenttraceid--góc-nhìn-frontend-đọchiển-thị) — solution này tái sử dụng nguyên xi cách tiếp cận đó (không định nghĩa lại)

---

## 1. Điểm khởi tạo trace trong Renderer

| BL | Hành động user | Component kích hoạt | Hook thực thi RPC | RPC method | File:line hiện tại |
|----|-----------------|----------------------|--------------------|------------|----------------------|
| BL-TG-02 (AI decompose) | Click "🤖 Decompose with AI" trong `TaskAIDecompose` | `TaskAIDecompose.tsx:36` (`onClick={decompose}`, `data-testid="decompose-btn"`) | `useTask(taskId).aiDecompose(instruction)` | `tasks.aiPlan` | `src/renderer/src/hooks/useTask.ts:20-27` |
| BL-TG-04 (agent execution — path 1) | Click "▶ Execute with Agent" trong `TaskDetail` | `TaskDetail.tsx:63` (`onClick={handleRunAgent}`, `data-testid="run-agent-btn"`) | inline trong `TaskDetail.tsx`, **không qua `useTask` hook** | `tasks.runAgent` | `src/renderer/src/components/task/TaskDetail.tsx:38-48` |
| BL-TG-04 (agent execution — path 2) | Click "▶ Run with Agent" trong tab "AI Agent" (`TaskPromptEditor`, hiển thị khi user vào tab con của `TaskDetail`) | `TaskPromptEditor.tsx:37` (`onClick={runWithAgent}`, `data-testid="run-agent-btn"`) | inline trong `TaskPromptEditor.tsx` | `task.runAgent` (**số ít** — xem cảnh báo bên dưới) | `src/renderer/src/components/task/TaskPromptEditor.tsx:14-26` |
| BL-TG-01 (add dependency/edge) | — không tìm thấy UI trigger | — | — | `tasks.addDependency`/`task.addEdge` | **Chưa xác định — cần điều tra thêm khi triển khai.** Grep `src/renderer/src/components/task/*.tsx` và `useTask*.ts` không tìm thấy lời gọi RPC nào tên `addEdge`/`addDependency` — UI cho thao tác này (nếu có) chưa được implement, khớp với CR-TRACE-018's chính nó không nêu component FE cụ thể. `TaskDetail.tsx:31` chỉ có `tasks.getDependencies` (đọc, không phải ghi cạnh mới) |
| BL-TG-03 (grant resolution) | — không có UI trigger riêng | — | — | — | Không traced riêng phía FE — xem §5 |

**Cảnh báo quan trọng — doc/code drift: 2 RPC method khác nhau cho cùng 1 hành động "chạy agent trên task":**

- `TaskDetail.tsx:41` gọi **`tasks.runAgent`** (số nhiều, đúng theo TDD-FE-15 Addendum "tasks.runAgent() — Full Flow" và HLD E.7 `'tasks.runAgent'`)
- `TaskPromptEditor.tsx:18` gọi **`task.runAgent`** (số ít — không khớp bất kỳ tài liệu nào đã đọc)

Đây là 2 call site độc lập, cả hai đều thực sự tồn tại trong code và cả hai đều là entry point hợp lệ vào BL-TG-04 (`TaskAgentExecutor.executeTask()` phía backend, theo CR-TRACE-018 §4). Solution này **instrument cả 2** (không giả định cái nào "đúng" hơn), nhưng đánh dấu khuyến nghị dọn dẹp ở §5 vì đây rất có thể là 2 route khác nhau trên backend (`task.runAgent` singular có thể 404/không tồn tại) — nếu đúng vậy, `TaskPromptEditor`'s "Run with Agent" button đang bị lỗi từ trước khi có CR này, độc lập với tracing.

## 2. Full Implementation

### 2.1. Thêm tracer phía browser vào `src/shared/trace/tracers.ts`

```typescript
// src/shared/trace/tracers.ts
export const Tracers = {
  // ...existing entries (uiProfileUpdateFlow, uiProfileResolveFlow, uiAiProviderWriteCredFlow,
  //                       uiAiProviderTestConnFlow, uiWorkflowTemplateSaveFlow,
  //                       uiWorkflowExecuteFlow, uiWorkflowCancelFlow, ...)...
  // ...existing backend entries từ CR-TRACE-018 (taskGraphEdgeFlow, taskGraphAiPlanFlow,
  //                       taskGraphGrantFlow, taskGraphExecuteFlow)...

  /** Browser-initiated: click "Decompose with AI" trong TaskAIDecompose (BL-TG-02) */
  uiTaskGraphAiPlanFlow:  createTracer('ui:taskGraph.aiPlan'),
  /** Browser-initiated: click "Execute/Run with Agent" — cả 2 call site (TaskDetail +
   *  TaskPromptEditor) dùng CHUNG tracer này, phân biệt bằng field `entryPoint` (BL-TG-04) */
  uiTaskGraphExecuteFlow: createTracer('ui:taskGraph.execute'),
} as const
```

Prefix `ui:` giữ nguyên lý do như các solution trước (tránh trùng badge `isBackend` trong `TracePanel.tsx:42` với `taskGraph:aiPlan`/`taskGraph:execute` phía backend).

### 2.2. `useTask.ts` — instrument `aiDecompose()` (BL-TG-02)

```typescript
// src/renderer/src/hooks/useTask.ts
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
    // BL-TG-02: field `promptLength` thay vì instruction đầy đủ — tránh log nội dung
    // hướng dẫn AI dài (không phải bí mật nhưng không cần thiết trong trace field, đúng
    // tinh thần "chỉ trace field cần cho debug" của CR-TRACE-000 §5).
    const span = Tracers.uiTaskGraphAiPlanFlow.start({
      taskId, hasInstruction: !!instruction, promptLength: instruction?.length ?? 0,
    })
    try {
      const result = await callRuntimeRpc(target, 'tasks.aiPlan', { taskId, instruction, traceId: span.id }) as {
        subtasks: Partial<OrcaTask>[]
      }
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
    // không băng qua relay/AI) — không traced riêng theo nguyên tắc §5 CR-TRACE-000.
    const createdSubtasks = await callRuntimeRpc(target, 'tasks.createSubtasks', {
      taskId, subtasks
    }) as OrcaTask[]
    for (const created of createdSubtasks) {
      useAppStore.getState().addTask(created)
    }
  }

  return { task, updateTask, deleteTask, aiDecompose, acceptSubtasks }
}
```

### 2.3. `TaskDetail.tsx` — instrument `handleRunAgent()` (BL-TG-04, entry point "Execute with Agent")

```typescript
// src/renderer/src/components/task/TaskDetail.tsx
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { Tracers } from '../../../../shared/trace/tracers'
// ...existing imports unchanged...

export function TaskDetail() {
  // ...existing activeTaskId/task/updateTask/localTitle/activeTab/deps state unchanged...

  const handleRunAgent = async () => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // field `entryPoint: 'task-detail'` phân biệt với TaskPromptEditor bên dưới —
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

### 2.4. `TaskPromptEditor.tsx` — instrument `runWithAgent()` (BL-TG-04, entry point "AI Agent tab")

```typescript
// src/renderer/src/components/task/TaskPromptEditor.tsx
import { useState } from 'react'
import { useWorkspace } from '../../context/WorkspaceContext'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { Tracers } from '../../../../shared/trace/tracers'
// ...existing imports unchanged...

export function TaskPromptEditor({ task }: { task: OrcaTask }) {
  const [prompt, setPrompt]       = useState(task.agentPrompt ?? '')
  const [isRunning, setIsRunning] = useState(false)
  const { project }               = useWorkspace()

  const runWithAgent = async () => {
    setIsRunning(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const span = Tracers.uiTaskGraphExecuteFlow.start({
      taskId: task.id, entryPoint: 'prompt-editor', promptLength: prompt.length,
    })
    try {
      // NOTE (doc/code drift — xem §1): method 'task.runAgent' (số ít) khác với
      // 'tasks.runAgent' mà TaskDetail dùng. Giữ nguyên method name hiện tại vì sửa
      // routing RPC không thuộc phạm vi CR tracing này — chỉ thêm traceId.
      await callRuntimeRpc(target, 'task.runAgent', {
        taskId: task.id,
        prompt: prompt || task.agentPrompt,
        projectId: project!.id,
        traceId: span.id,
      })
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

## 3. Sử dụng `parentTraceId` để nhóm nhiều thao tác trên cùng 1 task

Theo CR-TRACE-018 §5 ("Tương tự CR-TRACE-017"): nếu user thực hiện `aiDecompose` rồi `handleRunAgent` liên tiếp trên cùng 1 task, 2 span độc lập (`ui:taskGraph.aiPlan` rồi `ui:taskGraph.execute`) có thể được nhóm bằng field `parentTraceId` — **optional**, không bắt buộc như ở CR-TRACE-017 (không có khái niệm "1 root span cho toàn bộ session làm việc trên task" trong CR gốc). Solution này KHÔNG implement field này (giữ đơn giản, đúng scope CR-TRACE-018 §5 nói rõ "optional — CR này không phụ thuộc vào CR-TRACE-017"); nếu cần trong tương lai, cách làm giống hệt [SOL-FE-TRACE-017 §3.2](./SOL-FE-TRACE-017-workflow-orchestration.md#32-useworkflowexecutionts--cancelexecution--đọc-parenttraceid-từ-push-event) (`cancelExecution` đọc `rootTraceId` đã lưu từ span trước).

## 4. Test Plan (Vitest)

| File | Test case mới |
|------|----------------|
| `src/renderer/src/hooks/__tests__/useTask.test.ts` | `aiDecompose(instruction)` gọi `Tracers.uiTaskGraphAiPlanFlow.start({ taskId, hasInstruction: true, promptLength })` |
| | `aiDecompose()` không có instruction → `hasInstruction: false, promptLength: 0` |
| | `tasks.aiPlan` RPC nhận `traceId: span.id` trong params |
| | RPC thành công → `span.ok({ taskId, subtaskCount })` với đúng số lượng subtask trả về |
| | RPC lỗi → `span.fail(err, { taskId })` trước khi re-throw |
| `src/renderer/src/components/task/__tests__/TaskDetail.test.tsx` | Click `run-agent-btn` → `Tracers.uiTaskGraphExecuteFlow.start({ taskId, entryPoint: 'task-detail' })` |
| | `tasks.runAgent` RPC nhận `traceId: span.id` |
| | RPC thành công → `span.ok({ taskId })`, toast success hiển thị |
| | RPC lỗi → `span.fail(err, { taskId })`, toast error hiển thị |
| `src/renderer/src/components/task/__tests__/TaskPromptEditor.test.tsx` | Click `run-agent-btn` (trong file này) → `Tracers.uiTaskGraphExecuteFlow.start({ taskId, entryPoint: 'prompt-editor', promptLength })` |
| | `task.runAgent` RPC (method name giữ nguyên như code hiện tại) nhận `traceId: span.id` |
| | `entryPoint` khác nhau giữa 2 file test (`'task-detail'` vs `'prompt-editor'`) — verify field này phân biệt đúng 2 nguồn gọi cùng 1 tracer |

**Mục tiêu:** +5 test trong `useTask.test.ts`, +4 test trong `TaskDetail.test.tsx`, +3 test trong `TaskPromptEditor.test.tsx`.

## 5. Acceptance Criteria

- [ ] `useTask().aiDecompose()` tạo `ui:taskGraph.aiPlan` span, field `promptLength` phản ánh độ dài instruction (không log nội dung instruction đầy đủ), forward `traceId` vào `tasks.aiPlan`
- [ ] `span.ok()` của `aiDecompose` mang `subtaskCount` khớp số lượng phần tử `result.subtasks` trả về
- [ ] `TaskDetail.handleRunAgent()` và `TaskPromptEditor.runWithAgent()` dùng **chung 1 tracer** `ui:taskGraph.execute`, phân biệt bằng field `entryPoint: 'task-detail' | 'prompt-editor'`
- [ ] Cả 2 RPC (`tasks.runAgent` và `task.runAgent`) đều nhận `traceId: span.id` — method name KHÔNG bị sửa trong solution này (ngoài phạm vi tracing CR)
- [ ] Doc/code drift `tasks.runAgent` vs `task.runAgent` được ghi nhận rõ ràng trong solution (đã ở §1) làm căn cứ cho 1 CR dọn dẹp riêng (không phải CR-TRACE-018)
- [ ] BL-TG-01 (add dependency/edge) không có instrumentation nào được thêm — do không tìm thấy UI trigger tồn tại; đánh dấu "chưa triển khai — cần điều tra thêm" thay vì tạo call site giả định
- [ ] BL-TG-03 (grant resolution) không có tracer FE riêng — permission-denied errors (ví dụ `TASK_PERMISSION_DENIED`) surface qua `span.fail(err)` của `ui:taskGraph.execute` sẵn có, không cần span riêng
- [ ] Tracer flow name dùng prefix `ui:`, không trùng `taskGraph:aiPlan`/`taskGraph:execute`/`taskGraph:grantResolve` phía backend (CR-TRACE-018 §3)
- [ ] Test suite đạt tối thiểu 12 test case mới trên 3 file
