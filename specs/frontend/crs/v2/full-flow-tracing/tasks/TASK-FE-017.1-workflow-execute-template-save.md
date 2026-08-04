# TASK-FE-017.1: Mở rộng `WorkflowExecution.rootTraceId` + instrument `saveTemplate()`/`runWorkflow()`

**Phase:** 3
**SOL Ref:** [SOL-FE-TRACE-017 §2, §2.1, §3.1](../solutions/SOL-FE-TRACE-017-workflow-orchestration.md)
**CR Ref:** [CR-TRACE-017](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001 — tracer `uiWorkflowTemplateSaveFlow`/`uiWorkflowExecuteFlow` đã đăng ký) + external TASK-BE-000
**Status:** ✅ Done (2026-08-04) — Implemented as specced. Drift: shared type file is `src/shared/workflow-types.ts` (matches doc; a second, unrelated duplicate `src/renderer/src/types/workflow-types.ts` exists but is not imported by `useWorkflow.ts`/`useWorkflowExecution.ts`/`store/slices/workflow.ts`, all of which use the `@shared/workflow-types` alias — left untouched, out of scope). Dropped the `hasParent` field from the `saveTemplate` start-span sample (not required by Acceptance Criteria). `pnpm tsc --noEmit` clean; `useWorkflow.test.ts` 9/9 passing (rewrote existing 3 tests for new `callRuntimeRpc(target, method, params)` signature + added tracer/rootTraceId/error-path tests).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "useWorkflow"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "useWorkflow", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

Ngoài ra, task này thêm field `rootTraceId` (additive) vào type `WorkflowExecution` trong `src/shared/workflow-types.ts` — có thể chạy thêm `codegraph explore "WorkflowExecution"` để xem toàn bộ nơi type này được dùng trước khi thêm field.

## Mô tả

**Mô hình `parentTraceId`:** CR-TRACE-017 §4 (backend) định nghĩa `workflow:execute` là span cha (1 per execution, id = `rootTraceId`), mỗi step chạy `workflow:stepExecute` là span con độc lập mang field nghiệp vụ `parentTraceId: rootTraceId` (KHÔNG phải cơ chế `resume` — field tuỳ ý dùng để nhóm hiển thị). Frontend cần: (1) tạo `rootTraceId` ở browser TRƯỚC khi có `executionId` từ backend, (2) lưu vào `WorkflowExecution` trong store, (3) hiển thị cho user copy (task riêng — TASK-FE-017.3).

**Bug tiền tồn tại (signature mismatch):** cả 3 lời gọi `callRuntimeRpc` trong `useWorkflow.ts` thiếu tham số `target` bắt buộc đầu — task này sửa đúng luôn theo pattern `getActiveRuntimeTarget()` đã dùng nhất quán ở `useTask.ts`/`useTasks.ts`/`useAIProviders.ts`.

## File: `src/shared/workflow-types.ts` [MODIFY, additive]

```typescript
export type WorkflowExecution = {
  id:           string
  templateId:   string
  status:       WorkflowExecutionStatus
  startedAt:    number
  endedAt?:     number
  triggeredBy:  string
  definition:   WorkflowDefinition
  /** Span id của `ui:workflow.execute` (FE) == `workflow:execute` (BE, nếu resume đúng).
   *  Dùng để filter TracePanel theo toàn bộ execution. */
  rootTraceId?: string
}
```

## File: `src/renderer/src/hooks/useWorkflow.ts` [MODIFY]

```typescript
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
    // BL-WF-01: field `mode` phân biệt create/update.
    const span = Tracers.uiWorkflowTemplateSaveFlow.start({ mode: templateId ? 'update' : 'create', hasParent: !!local.templateId })
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
    // BL-WF-02: span.id CHÍNH LÀ rootTraceId của toàn bộ execution. Browser sinh
    // id này TRƯỚC khi có executionId từ backend.
    const span = Tracers.uiWorkflowExecuteFlow.start({ templateId })
    try {
      const result = await callRuntimeRpc<{ id: string }>(target, 'workflow.execute', { templateId, inputs, traceId: span.id })
      // Lưu rootTraceId vào execution record ngay khi biết executionId.
      useAppStore.getState().addExecution({
        id: result.id, templateId, status: 'running', startedAt: Date.now(),
        triggeredBy: 'me', definition: local as WorkflowDefinition, rootTraceId: span.id,
      })
      // KHÔNG span.ok() ở đây tới khi execution xong — ok() chỉ đánh dấu "RPC issue
      // thành công" (ack nhận executionId), không phải "execution đã xong". Vòng đời
      // đầy đủ do backend tự trace qua workflow:execute (resume cùng id).
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

**Vì sao `ui:workflow.execute` "ok" ngay khi nhận `executionId`, không đợi execution chạy xong:** span chỉ đo latency của lời gọi RPC `workflow.execute` (round-trip issue command), không đo toàn bộ vòng đời workflow (có thể kéo dài hàng phút). Browser không giữ 1 async span mở suốt thời gian chạy.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/hooks/__tests__/useWorkflow.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `useWorkflow().saveTemplate()` tạo `ui:workflow.templateSave` span, field `mode` đúng `'create'|'update'`, forward `traceId` vào RPC tương ứng
- [ ] `useWorkflow().runWorkflow()` tạo `ui:workflow.execute` span TRƯỚC khi gọi RPC, forward `traceId: span.id` vào `workflow.execute` params
- [ ] `WorkflowExecution.rootTraceId` được set = `span.id` ngay khi `addExecution()` chạy — không đợi backend echo lại id
- [ ] `runWorkflow()` không có `templateId` → không tạo span (return `null` sớm)
- [ ] Bug signature tiền tồn tại được sửa trong `useWorkflow.ts` (3 call site: `workflow.template.update`, `workflow.template.create`, `workflow.execute`) — assert `callRuntimeRpc` gọi với `target` làm tham số đầu tiên
- [ ] Tracer flow name dùng prefix `ui:`, không trùng `workflow:execute`/`workflow:templateCreate` phía backend
- [ ] Test suite đạt ≥ 7 test case mới theo Test Plan SOL-FE-TRACE-017 §4 (create/update mode, forward traceId, lưu rootTraceId, không có templateId → null, lỗi → fail + null, signature fix)
