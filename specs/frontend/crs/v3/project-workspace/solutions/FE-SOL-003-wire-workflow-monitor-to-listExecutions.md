# FE-SOL-003: Nối `WorkflowMonitor` (tab Workflows) vào `workflow.listExecutions`

> **✅ ĐÃ IMPLEMENT (2026-09-06)** — đúng như kế hoạch dưới, không lệch. `WorkflowMonitor.test.tsx`
> mới (5 test case: fetch đúng params, empty/error/list state, click-to-expand + quay lại danh
> sách). `gitnexus detect_changes`: risk **low**, 0 execution flow bị ảnh hưởng.

## CR Reference
- **CR:** [CR-PW-003](../../../../../../docs/crs/v3/project-workspace/CR-PW-003-workflows-tab-wiring.md)
- **Mức độ:** 🟡 P1
- **Impact analysis (gitnexus):** `WorkflowMonitor` (`components/workflow/WorkflowMonitor.tsx`) —
  risk LOW, 0 caller trực tiếp ngoài `WorkspaceLayout.tsx` (lazy-loaded).
- **Pattern tham chiếu:** `TaskGraphPanel`/`useTasks.ts` (RPC-driven, nhận `projectId` prop, cùng
  parent `WorkspaceLayout.tsx`) — solution này theo đúng pattern đó, không phát minh pattern mới.

---

## Root Cause

`WorkflowMonitor.tsx` là stub tĩnh, 0 RPC call. Backend `workflow.listExecutions` (filter theo
`projectId`) đã tồn tại, hoạt động, có test — 0 caller ở frontend (grep xác nhận). Store
`workflow.ts` slice đã có `executions: WorkflowExecution[]` + `addExecution`/`updateExecutionStatus`
nhưng thiếu 1 setter bulk-replace cho kết quả list ban đầu.

## Giải pháp

### Bước 1 — Thêm `setExecutions` vào `WorkflowSlice` (mirror `setTemplates` đã có)

**File:** `frontend/src/renderer/src/store/slices/workflow.ts` (MODIFY)

```typescript
setExecutions(executions: WorkflowExecution[]): void
// ...
setExecutions: (executions) => set(() => ({ executions })),
```

(Giữ nguyên convention "return object, không mutate" đã ghi chú sẵn trong file — bug class
BUG-FE-TASKGRAPH-SETTINGS.)

### Bước 2 — Viết lại `WorkflowMonitor.tsx` — list + click-to-expand vào `ExecutionMonitor` có sẵn

**File:** `frontend/src/renderer/src/components/workflow/WorkflowMonitor.tsx` (REWRITE)

```tsx
// WorkflowMonitor.tsx — lists this project's workflow executions (CR-PW-003 fix; was a static
// stub). Reuses ExecutionMonitor for the detail view instead of re-building step/wave tracking.
import { useCallback, useEffect, useState } from 'react'
import { ExecutionMonitor } from './ExecutionMonitor'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import type { WorkflowExecution, WorkflowExecutionStatus } from '@shared/workflow-types'

// Why not StepStatusBadge: it only maps StepStatus (pending/running/completed/failed/skipped),
// not WorkflowExecutionStatus (which also has 'cancelled') — reusing it here would destructure
// undefined and crash on a cancelled execution (see ExecutionMonitor.tsx's own `as any` cast of
// the same mismatch). Small inline map avoids repeating that bug in new code.
const STATUS_LABEL: Record<WorkflowExecutionStatus, string> = {
  pending: 'Pending',
  running: 'Running',
  completed: 'Completed',
  failed: 'Failed',
  cancelled: 'Cancelled',
}

export function WorkflowMonitor({ projectId }: { projectId: string }) {
  const executions = useAppStore((s) => s.executions)
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState(false)
  const [selectedExecutionId, setSelectedExecutionId] = useState<string | null>(null)

  const load = useCallback(async () => {
    setIsLoading(true)
    setLoadError(false)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const result = await callRuntimeRpc<WorkflowExecution[]>(target, 'workflow.listExecutions', {
        projectId,
      })
      useAppStore.getState().setExecutions(result)
    } catch {
      setLoadError(true)
    } finally {
      setIsLoading(false)
    }
  }, [projectId])

  useEffect(() => {
    void load()
  }, [load])

  if (selectedExecutionId) {
    return (
      <div className="workflow-monitor" data-testid="workflow-monitor">
        <button
          className="text-xs text-muted-foreground hover:text-foreground px-4 pt-3"
          onClick={() => setSelectedExecutionId(null)}
          data-testid="workflow-back-to-list"
        >
          &larr; Back to executions
        </button>
        <ExecutionMonitor executionId={selectedExecutionId} />
      </div>
    )
  }

  return (
    <div className="workflow-monitor p-4 space-y-2" data-testid="workflow-monitor">
      {isLoading ? (
        <div className="text-xs text-muted-foreground" data-testid="workflow-loading">Loading executions…</div>
      ) : loadError ? (
        <div className="text-xs text-destructive" data-testid="workflow-load-error">
          Failed to load workflow executions.
        </div>
      ) : executions.length === 0 ? (
        <div className="text-xs text-muted-foreground" data-testid="workflow-empty">
          No workflow executions for this project yet.
        </div>
      ) : (
        executions.map((execution) => (
          <button
            key={execution.id}
            className="execution-row w-full text-left border rounded p-2 hover:bg-accent flex items-center justify-between"
            onClick={() => setSelectedExecutionId(execution.id)}
            data-testid={`execution-row-${execution.id}`}
          >
            <span className="text-sm font-medium">{execution.definition?.name ?? execution.id}</span>
            <span className="text-xs text-muted-foreground">
              {STATUS_LABEL[execution.status]} &middot; {execution.triggeredBy}
            </span>
          </button>
        ))
      )}
    </div>
  )
}
```

### Bước 3 — Truyền `projectId` từ `WorkspaceLayout.tsx`

**File:** `frontend/src/renderer/src/components/workspace/WorkspaceLayout.tsx` (MODIFY)

```tsx
{activeTab === 'workflows' && <WorkflowMonitor projectId={project.id} />}
```

## Files cần sửa

| File | Action |
|------|--------|
| `frontend/src/renderer/src/store/slices/workflow.ts` | MODIFY — thêm `setExecutions` |
| `frontend/src/renderer/src/components/workflow/WorkflowMonitor.tsx` | REWRITE |
| `frontend/src/renderer/src/components/workspace/WorkspaceLayout.tsx` | MODIFY — truyền `projectId` |
| `frontend/src/renderer/src/components/workflow/__tests__/WorkflowMonitor.test.tsx` | CREATE |

## Task breakdown

- [FE-TASK-004](../tasks/FE-TASK-004-workflow-slice-set-executions.md)
- [FE-TASK-005](../tasks/FE-TASK-005-wire-workflow-monitor-rpc.md)

## Verification

```bash
cd frontend && npx vitest run src/renderer/src/components/workflow/__tests__/WorkflowMonitor.test.tsx
cd frontend && npx tsc --noEmit -p .
```

## Không làm ở solution này

- `useWorkflowExecution.ts`/`ExecutionMonitor.tsx`'s phụ thuộc `window.api.on` (Electron-only bridge,
  nghi vấn không hoạt động ở Web mode) — follow-up riêng, xem "Không thuộc phạm vi" trong
  CR-PW-003.
- `ExecutionMonitor.tsx`'s `StepStatusBadge` không xử lý status `'cancelled'` — bug có sẵn, không
  do solution này gây ra, không sửa ở đây để giữ phạm vi hẹp.
- Trigger workflow mới (`workflow.execute`) từ UI này — CR chỉ yêu cầu hiển thị dữ liệu đã có, tab
  vẫn read-only (giữ nguyên khả năng Cancel đã có sẵn trong `ExecutionMonitor`).
