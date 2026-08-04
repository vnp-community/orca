# SOL-FE-TRACE-017: Workflow Orchestration — Frontend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-017](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
**TDD Ref:** [TDD-FE-14](../../../../tdd/v5/14-workflow-ui.md) (Workflow Builder & Monitor, F36, ADR-009)
**Status:** Proposed
**Dependency:** F40 core tracing infra (đã implement); core API `Tracer.start(fields?, resume?)` từ CR-TRACE-000 §3.1 (**chưa ship** — xem SOL-FE-TRACE-015 §2.0); backend `rootTraceId` persistence (`orca_workflow_executions.root_trace_id`) từ CR-TRACE-017 §4 BL-WF-02 "Lưu ý resume sau restart" (companion backend solution)

---

## 1. Điểm khởi tạo trace trong Renderer

Toàn bộ RPC nằm trong 2 hook: `useWorkflow.ts` (CRUD + execute) và `useWorkflowExecution.ts` (monitor thời gian thực). Không component nào trong `src/renderer/src/components/workflow/*.tsx` tự gọi RPC trực tiếp — chúng chỉ gọi hàm từ 2 hook này.

| BL | Hành động user | Component kích hoạt | Hook thực thi RPC | RPC method | File:line hiện tại |
|----|-----------------|----------------------|--------------------|------------|----------------------|
| BL-WF-01 (template create/update) | Click "Save" trong `WorkflowBuilder` | `WorkflowBuilder.tsx:34` (`onClick={saveTemplate}`, `data-testid="save-workflow-btn"`) | `useWorkflow(templateId).saveTemplate()` | `workflow.template.create` (mới) hoặc `workflow.template.update` (sửa) | `src/renderer/src/hooks/useWorkflow.ts:45-53` |
| BL-WF-02 (execute — khởi tạo) | Click "Run" trong `WorkflowBuilder` | `WorkflowBuilder.tsx:35` (`onClick={() => runWorkflow()}`, `data-testid="run-workflow-btn"`) | `useWorkflow(templateId).runWorkflow()` | `workflow.execute` | `useWorkflow.ts:55-59` |
| BL-WF-02 (cancel — không có trong CR gốc, bổ sung vì cùng vòng đời execution) | Click "Cancel" trong `ExecutionMonitor` (khi `status === 'running'`) | `ExecutionMonitor.tsx:27` (`onClick={cancelExecution}`, `data-testid="cancel-btn"`) | `useWorkflowExecution(executionId).cancelExecution()` | `workflow.cancel` | `src/renderer/src/hooks/useWorkflowExecution.ts:31-34` |

**Phát hiện quan trọng — bug tiền tồn tại (signature mismatch), giống SOL-FE-TRACE-016 §1:** cả 3 lời gọi `callRuntimeRpc` trong `useWorkflow.ts` (dòng 47, 49, 57) và 1 lời gọi trong `useWorkflowExecution.ts` (dòng 32) đều thiếu tham số `target` bắt buộc đầu (gọi `callRuntimeRpc('workflow.template.update', {...})` thay vì `callRuntimeRpc(target, 'workflow.template.update', {...})`). Vì solution này bắt buộc sửa các dòng này để thêm `traceId`, signature được sửa đúng luôn theo pattern `getActiveRuntimeTarget()` đã dùng nhất quán ở `useTask.ts`/`useTasks.ts`/`useAIProviders.ts`.

## 2. Mô hình `parentTraceId` — góc nhìn Frontend (đọc/hiển thị)

CR-TRACE-017 §4 (backend) định nghĩa: `workflow:execute` là **span cha** (1 per execution, id = `rootTraceId`), mỗi step chạy `workflow:stepExecute` là **span con độc lập** mang field nghiệp vụ `parentTraceId: rootTraceId` (không phải cơ chế `resume` — đây là field tuỳ ý dùng để nhóm hiển thị). Frontend cần làm 3 việc để phối hợp với thiết kế này:

1. **Tạo `rootTraceId` ở phía browser trước, không đợi backend sinh id** — theo đúng convention CR-TRACE-000 §3.3 hàng 1 ("Browser tạo `traceId` đầu tiên"), `runWorkflow()` tự sinh span `ui:workflow.execute` và gửi `span.id` làm `traceId` trong params của `workflow.execute`. Nếu backend `resume` đúng bằng id này (companion backend solution), `workflow:execute` (backend, root span) và mọi `workflow:stepExecute` con (`parentTraceId` trỏ về id này) đều mang **cùng 1 id gốc** mà browser đã biết ngay từ đầu — không cần chờ response để lấy id.
2. **Lưu `rootTraceId` vào `WorkflowExecution` trong store** — để UI (`ExecutionMonitor`) có thể hiển thị/copy id này cho user dùng làm filter trong TracePanel.
3. **Đọc field `parentTraceId` từ backend push event** (`workflow:stepStatus`, `workflow:stepOutput`) nếu backend đính kèm — không bắt buộc cho việc hiển thị step status (đã có `execId`/`stepId` riêng), nhưng cần thiết nếu muốn hiển thị badge "cùng trace" trên UI thay vì chỉ dựa vào TracePanel filter thủ công.

### 2.1. Mở rộng type `WorkflowExecution` (shared) để mang `rootTraceId`

```typescript
// src/shared/workflow-types.ts
export type WorkflowExecution = {
  id:           string
  templateId:   string
  status:       WorkflowExecutionStatus
  startedAt:    number
  endedAt?:     number
  triggeredBy:  string
  definition:   WorkflowDefinition
  /** Span id của `ui:workflow.execute` (FE) == `workflow:execute` (BE, nếu resume đúng).
   *  Dùng để filter TracePanel theo toàn bộ execution — xem CR-TRACE-017 §4. */
  rootTraceId?: string
}
```

### 2.2. Thêm tracer phía browser vào `src/shared/trace/tracers.ts`

```typescript
// src/shared/trace/tracers.ts
export const Tracers = {
  // ...existing entries (uiProfileUpdateFlow, uiProfileResolveFlow, uiAiProviderWriteCredFlow,
  //                       uiAiProviderTestConnFlow, ...)...
  // ...existing backend entries từ CR-TRACE-017 (workflowTemplateCreateFlow, workflowExecuteFlow,
  //                       workflowStepFlow, workflowShareFlow)...

  /** Browser-initiated: click "Save" trong WorkflowBuilder (BL-WF-01) */
  uiWorkflowTemplateSaveFlow: createTracer('ui:workflow.templateSave'),
  /** Browser-initiated: click "Run" — root span của toàn bộ execution nhìn từ browser (BL-WF-02) */
  uiWorkflowExecuteFlow:      createTracer('ui:workflow.execute'),
  /** Browser-initiated: click "Cancel" trên execution đang chạy */
  uiWorkflowCancelFlow:       createTracer('ui:workflow.cancel'),
} as const
```

Prefix `ui:` giữ nguyên lý do như SOL-FE-TRACE-015/016 (tránh trùng badge `isBackend` trong `TracePanel.tsx:42` với `workflow:execute`/`workflow:templateCreate` phía backend).

## 3. Full Implementation

### 3.1. `useWorkflow.ts` — BL-WF-01 (saveTemplate) + BL-WF-02 khởi tạo (runWorkflow)

```typescript
// src/renderer/src/hooks/useWorkflow.ts
import { useState, useCallback } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { Tracers } from '../../../shared/trace/tracers'
import { toast } from 'sonner'
import type { WorkflowDefinition, WorkflowStep } from '@shared/workflow-types'

export function useWorkflow(templateId?: string) {
  // ...existing templates/executions/local/updateTemplate/addStep/removeStep/updateStep unchanged...

  const saveTemplate = useCallback(async () => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // BL-WF-01: field `mode` phân biệt create/update — đối xứng với field `hasParent` mà
    // backend `workflow:templateCreate` dùng để phân biệt kế thừa (CR-TRACE-017 §4).
    const span = Tracers.uiWorkflowTemplateSaveFlow.start({
      mode: templateId ? 'update' : 'create',
      hasParent: !!local.templateId,
    })
    try {
      if (templateId) {
        await callRuntimeRpc(target, 'workflow.template.update', { templateId, ...local, traceId: span.id })
      } else {
        const created = await callRuntimeRpc<WorkflowDefinition>(target, 'workflow.template.create', { ...local, traceId: span.id })
        useAppStore.getState().addTemplate(created)
      }
      span.ok({ mode: templateId ? 'update' : 'create' })
      toast.success('Workflow saved')
    } catch (err) {
      span.fail(err, { mode: templateId ? 'update' : 'create' })
      throw err
    }
  }, [templateId, local])

  const runWorkflow = useCallback(async (inputs?: Record<string, unknown>) => {
    if (!templateId) { toast.error('Save workflow first'); return null }
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // BL-WF-02: span.id ĐÂY chính là rootTraceId của toàn bộ execution (xem §2 ở trên).
    // Browser sinh id này TRƯỚC khi có executionId từ backend — theo CR-TRACE-000 §3.3 hàng 1.
    const span = Tracers.uiWorkflowExecuteFlow.start({ templateId })
    try {
      const result = await callRuntimeRpc<{ id: string }>(target, 'workflow.execute', {
        templateId, inputs, traceId: span.id,
      })
      // Lưu rootTraceId vào execution record ngay khi biết executionId — ExecutionMonitor
      // đọc field này để hiển thị/copy cho user dùng filter TracePanel (§2.2 dưới).
      useAppStore.getState().addExecution({
        id: result.id, templateId, status: 'running', startedAt: Date.now(),
        triggeredBy: 'me', definition: local as WorkflowDefinition, rootTraceId: span.id,
      })
      // KHÔNG span.ok() ở đây — ok() chỉ đánh dấu "RPC issue thành công" (ack nhận
      // executionId), không phải "execution đã xong". Việc execution hoàn tất/fail được
      // backend tự báo qua workflow:execute span riêng (SSE), browser span này coi như
      // hoàn tất nhiệm vụ của nó (khởi tạo) ngay khi có executionId.
      span.ok({ executionId: result.id })
      toast.success('Workflow started')
      return result.id
    } catch (err) {
      span.fail(err, { templateId })
      toast.error('Failed to start workflow')
      return null
    }
  }, [templateId, local])

  return { template: local, templates, executions, addStep, removeStep, updateStep, updateTemplate, saveTemplate, runWorkflow }
}
```

**Vì sao `ui:workflow.execute` "ok" ngay khi nhận `executionId`, không đợi execution chạy xong:** đúng theo mô hình span 1-hop của CR-TRACE-000 §3.1 ("mỗi layer đo latency riêng của nó") — span này chỉ đo latency của **lời gọi RPC `workflow.execute`** (round-trip issue command), không đo toàn bộ vòng đời workflow (có thể kéo dài hàng phút, nhiều wave). Vòng đời đầy đủ được backend tự trace qua `workflow:execute` (resume cùng id) và hiển thị trong TracePanel qua SSE — browser không cần (và không nên) giữ 1 async span mở suốt thời gian chạy.

### 3.2. `useWorkflowExecution.ts` — cancelExecution + đọc `parentTraceId` từ push event

```typescript
// src/renderer/src/hooks/useWorkflowExecution.ts
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
      // (= rootTraceId của execution) trong payload, lưu lại để hiển thị. Không bắt buộc
      // cho status hoạt động đúng — chỉ phục vụ hiển thị/debug.
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
    // Field `parentTraceId` (không phải resume — theo đúng phân biệt CR-TRACE-017 §4) nhóm
    // thao tác cancel này vào cùng execution trong TracePanel, dù span này có id riêng.
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

### 3.3. `ExecutionMonitor.tsx` — hiển thị `rootTraceId` cho user copy vào TracePanel filter

```typescript
// src/renderer/src/components/workflow/ExecutionMonitor.tsx
// Thêm vào phần header, cạnh StepStatusBadge — hiển thị rootTraceId dạng monospace, click để copy.
// Lý do: TracePanel (TracePanel.tsx:236-251) có ô filter free-text tìm theo flow/id/field —
// nếu user copy đúng rootTraceId và dán vào ô filter, TracePanel hiển thị MỌI span (browser +
// backend qua SSE) cùng id này, kể cả các `workflow:stepExecute` con mang field `parentTraceId`
// trùng giá trị (TraceEventRow tìm trong `Object.values(e.fields)` — filter.ts:158-163).
{execution.rootTraceId && (
  <button
    className="text-[10px] font-mono text-muted-foreground hover:text-foreground"
    title="Copy trace ID — paste into TracePanel filter (Ctrl+Shift+T) to see all steps"
    onClick={() => navigator.clipboard.writeText(execution.rootTraceId!)}
    data-testid="root-trace-id-badge"
  >
    trace:{execution.rootTraceId}
  </button>
)}
```

## 4. Test Plan (Vitest)

| File | Test case mới |
|------|----------------|
| `src/renderer/src/hooks/__tests__/useWorkflow.test.ts` | `saveTemplate()` (create) gọi `Tracers.uiWorkflowTemplateSaveFlow.start({ mode: 'create', hasParent })` |
| | `saveTemplate()` (update) dùng `mode: 'update'`, forward `traceId: span.id` vào `workflow.template.update` |
| | `runWorkflow()` gọi `Tracers.uiWorkflowExecuteFlow.start({ templateId })`, forward `traceId: span.id` vào `workflow.execute` |
| | `runWorkflow()` lưu `rootTraceId: span.id` vào execution record qua `addExecution()` |
| | `runWorkflow()` không có `templateId` → không tạo span (return `null` sớm) |
| | RPC lỗi → `span.fail(err, { templateId })`, hàm return `null` |
| | **Assert signature fix:** `callRuntimeRpc` được gọi với `target` làm tham số đầu tiên (không phải method string) |
| `src/renderer/src/hooks/__tests__/useWorkflowExecution.test.ts` | `cancelExecution()` gọi `Tracers.uiWorkflowCancelFlow.start({ executionId, parentTraceId })` với `parentTraceId` lấy từ `execution.rootTraceId` trong store |
| | `cancelExecution()` forward `traceId: span.id` vào `workflow.cancel` params |
| | RPC `workflow.cancel` reject → `span.fail(err, { executionId })`, status KHÔNG chuyển 'cancelled' |
| `src/renderer/src/components/workflow/__tests__/ExecutionMonitor.test.tsx` | Hiển thị `data-testid="root-trace-id-badge"` khi `execution.rootTraceId` có giá trị |
| | Không hiển thị badge khi `execution.rootTraceId` undefined (execution cũ, trước khi tracing được bật) |

**Mục tiêu:** +7 test trong `useWorkflow.test.ts`, +3 test trong `useWorkflowExecution.test.ts`, +2 test trong `ExecutionMonitor.test.tsx`.

## 5. Acceptance Criteria

- [ ] `useWorkflow().saveTemplate()` tạo `ui:workflow.templateSave` span, field `mode` đúng `'create'|'update'`, forward `traceId` vào RPC tương ứng
- [ ] `useWorkflow().runWorkflow()` tạo `ui:workflow.execute` span TRƯỚC khi gọi RPC (browser sinh id đầu tiên theo CR-TRACE-000 §3.3), forward `traceId: span.id` vào `workflow.execute` params
- [ ] `WorkflowExecution.rootTraceId` được set = `span.id` ngay khi `addExecution()` chạy — không đợi backend echo lại id
- [ ] `useWorkflowExecution().cancelExecution()` tạo `ui:workflow.cancel` span với field `parentTraceId` trỏ về `execution.rootTraceId` (nhóm hiển thị, KHÔNG dùng cơ chế `resume`)
- [ ] `ExecutionMonitor` hiển thị `rootTraceId` dưới dạng có thể copy, để user dán vào ô filter TracePanel và thấy toàn bộ span (browser + backend) của 1 execution
- [ ] Bug signature tiền tồn tại (`callRuntimeRpc(method, params)` thiếu `target`) được sửa trong cả `useWorkflow.ts` (3 call site) và `useWorkflowExecution.ts` (1 call site)
- [ ] Tracer flow name dùng prefix `ui:`, không trùng `workflow:execute`/`workflow:templateCreate`/`workflow:stepExecute` phía backend
- [ ] Không tạo span FE riêng cho từng `workflow:stepExecute` — step-level tracing là trách nhiệm của backend (`StepExecutors.ts`), frontend chỉ hiển thị qua SSE + `stepStatuses`/`streamingOutput` store hiện có
