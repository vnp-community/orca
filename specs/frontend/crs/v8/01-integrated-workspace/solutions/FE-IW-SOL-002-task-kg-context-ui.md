# FE-IW-SOL-002 — Frontend: Task KG Context Display

**Solution ID:** FE-IW-SOL-002  
**CR Reference:** CR-ORCA-002 (task-proto-kg-context)  
**Backend Solution:** SOL-ORCA-002  
**TDD Reference:** TDD-FE-02 State Management | TDD-FE-05 UI Components  
**Status:** ✅ Done — implemented 2026-10-19  
**Priority:** P2  
**Effort:** 2–3 ngày  
**Depends On:** FE-IW-SOL-001 (pis tools hook), SOL-ORCA-001 backend

---

## Alignment với Backend Solution

| Backend detail (SOL-ORCA-002) | Frontend implication |
|-------------------------------|----------------------|
| `TaskKGContext` struct: `CRSID`, `FeatureRef`, `Service`, `RepoTarget` (Go PascalCase) | Frontend type mirror với camelCase: `crsId`, `featureRef`, `service`, `repoTarget` |
| API fields JSON: `crs_id`, `feature_ref`, `service`, `repo_target` (snake_case) | Form inputs submit snake_case JSON — phải match API |
| `EnrichedTask` có `CRSContext` và `FeatureContext` (`json.RawMessage`) | Frontend store raw JSON per-task — không transform |
| `BuildSystemPrompt()` — `// implements: CRS-XXX` comment instruction | UI chỉ display, không tự generate prompt |
| DB: `kg_context` JSONB column, index trên `crs_id` | Frontend có thể query tasks theo crs_id (future) |
| Best-effort KG fetch: fail → log warning, task vẫn assign | Frontend handle undefined `crsContext`/`featureContext` gracefully |
| Concurrent fetch: `pis_query_crs` + `pis_query_feature_context` parallel | Frontend fetch cũng parallel (`Promise.allSettled`) |
| `TaskKGContext.CRSID == ""` → skip enrichment | Frontend skip fetch khi không có crsId |

---

## Giải pháp

### SOL-2A: Task KG Context Type

```typescript
// src/renderer/src/types/task-types.ts (extend)

// Mirror của Go TaskKGContext struct (SOL-ORCA-002 §2.2)
// JSON field names khớp API: crs_id, feature_ref, service, repo_target
export interface TaskKGContext {
  crs_id?: string       // backend: CRSID "CRS-045"
  feature_ref?: string  // backend: FeatureRef "F27"
  service?: string      // backend: Service "ai-service"
  repo_target?: string  // backend: RepoTarget "vnp-workspace"
}

// EnrichedTask mirror (SOL-ORCA-002 §2.5) — frontend caches per-task
export interface TaskEnrichment {
  taskId: string
  crsContext?: unknown   // raw JSON từ pis_query_crs (json.RawMessage)
  featureContext?: unknown // raw JSON từ pis_query_feature_context
  fetchedAt: number
  loading: boolean
  error?: string
}

// Extend existing Task type
export interface Task {
  // ... existing fields ...
  kg_context?: TaskKGContext  // snake_case để match API response
}
```

**Tại sao giữ snake_case cho `kg_context`**: API trả về `kg_context` (JSON snake_case từ Go `json:"kg_context"`). Tránh transform thêm một lớp.

---

### SOL-2B: Zustand Slice — `task-enrichment`

```typescript
// src/renderer/src/store/slices/task-enrichment.ts

import type { StateCreator } from 'zustand'
import type { AppState } from '../index'
import type { TaskEnrichment } from '../../types/task-types'

export interface TaskEnrichmentSlice {
  taskEnrichments: Record<string, TaskEnrichment>
  setTaskEnrichmentLoading: (taskId: string) => void
  setTaskEnrichment: (taskId: string, crsContext?: unknown, featureContext?: unknown) => void
  setTaskEnrichmentError: (taskId: string, error: string) => void
  // CR-ORCA-003: cập nhật CRS status sau quality gate
  updateEnrichmentCRSStatus: (taskId: string, newStatus: string) => void
}

export const createTaskEnrichmentSlice: StateCreator<
  AppState,
  [],
  [],
  TaskEnrichmentSlice
> = (set) => ({
  taskEnrichments: {},

  setTaskEnrichmentLoading: (taskId) =>
    set((s) => ({
      taskEnrichments: {
        ...s.taskEnrichments,
        [taskId]: {
          ...(s.taskEnrichments[taskId] ?? { taskId, fetchedAt: 0 }),
          loading: true,
        },
      },
    })),

  setTaskEnrichment: (taskId, crsContext, featureContext) =>
    set((s) => ({
      taskEnrichments: {
        ...s.taskEnrichments,
        [taskId]: {
          taskId,
          crsContext,
          featureContext,
          fetchedAt: Date.now(),
          loading: false,
        },
      },
    })),

  setTaskEnrichmentError: (taskId, error) =>
    set((s) => ({
      taskEnrichments: {
        ...s.taskEnrichments,
        [taskId]: {
          ...(s.taskEnrichments[taskId] ?? { taskId, fetchedAt: 0 }),
          loading: false,
          error,
        },
      },
    })),

  // Best-effort CRS status update (SOL-ORCA-003 — sau quality gate)
  updateEnrichmentCRSStatus: (taskId, newStatus) =>
    set((s) => {
      const entry = s.taskEnrichments[taskId]
      if (!entry?.crsContext) return s
      // crsContext là raw JSON — patch status field
      const updated = { ...(entry.crsContext as Record<string, unknown>), status: newStatus }
      return {
        taskEnrichments: {
          ...s.taskEnrichments,
          [taskId]: { ...entry, crsContext: updated },
        },
      }
    }),
})
```

---

### SOL-2C: `useTaskEnrichment` Hook

```typescript
// src/renderer/src/hooks/use-task-enrichment.ts

import { useEffect } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import type { Task } from '../types/task-types'

const KG_CACHE_TTL_MS = 5 * 60 * 1000 // 5 phút — consistent với backend best-effort

export function useTaskEnrichment(task: Task, environmentId?: string) {
  const {
    taskEnrichments,
    setTaskEnrichmentLoading,
    setTaskEnrichment,
    setTaskEnrichmentError,
  } = useAppStore()

  const entry = taskEnrichments[task.id]
  const kgCtx = task.kg_context

  useEffect(() => {
    // Skip nếu không có KG refs (backend: CRSID == "" → skip)
    if (!kgCtx?.crs_id && !kgCtx?.feature_ref) return
    // Cache: skip nếu đã fetch < 5 phút
    if (entry?.fetchedAt && Date.now() - entry.fetchedAt < KG_CACHE_TTL_MS) return

    const target = environmentId
      ? { kind: 'environment' as const, environmentId }
      : { kind: 'local' as const }

    setTaskEnrichmentLoading(task.id)

    // Concurrent fetch — mirror backend concurrent approach (SOL-ORCA-002 §5 risk: add timeout)
    Promise.allSettled([
      kgCtx.crs_id
        ? callRuntimeRpc(target, 'agent.tool_call', {
            tool: 'pis_query_crs',
            input: { crs_id: kgCtx.crs_id },
          })
        : Promise.resolve(undefined),
      kgCtx.feature_ref
        ? callRuntimeRpc(target, 'agent.tool_call', {
            tool: 'pis_query_feature_context',
            input: { feature_id: kgCtx.feature_ref },
          })
        : Promise.resolve(undefined),
    ]).then(([crsResult, featureResult]) => {
      // Best-effort: settled → luôn gọi setTaskEnrichment dù có failure
      setTaskEnrichment(
        task.id,
        crsResult.status === 'fulfilled' ? crsResult.value : undefined,
        featureResult.status === 'fulfilled' ? featureResult.value : undefined
      )
    }).catch((err) => {
      setTaskEnrichmentError(task.id, String(err))
    })
  }, [task.id, kgCtx?.crs_id, kgCtx?.feature_ref, environmentId])

  return { entry, loading: entry?.loading ?? false }
}
```

---

### SOL-2D: Task Detail — KG Context Panel

```typescript
// src/renderer/src/components/tasks/task-kg-context-panel.tsx

import type { FC } from 'react'
import type { Task } from '../../types/task-types'
import { useTaskEnrichment } from '../../hooks/use-task-enrichment'

interface Props {
  task: Task
  environmentId?: string
}

// CRS status values từ backend (SOL-ORCA-002 §2.3 + PATCH body validation)
type CRSStatus = 'pending' | 'in-progress' | 'done' | 'cancelled'

export const TaskKGContextPanel: FC<Props> = ({ task, environmentId }) => {
  const { entry, loading } = useTaskEnrichment(task, environmentId)

  // Ẩn panel khi không có KG context (backend: skip enrichment nếu CRSID == "")
  if (!task.kg_context?.crs_id && !task.kg_context?.feature_ref) return null

  const kgCtx = task.kg_context
  const crsNode = entry?.crsContext as Record<string, unknown> | undefined
  const featureNode = entry?.featureContext as Record<string, unknown> | undefined

  return (
    <section
      className="task-kg-context-panel"
      aria-label="Knowledge Graph Context"
      id={`task-kg-context-${task.id}`}
    >
      <h4 className="task-kg-context-panel__title">KG Context</h4>

      {loading && (
        <div className="task-kg-context-panel__loading" aria-live="polite">
          Loading KG context…
        </div>
      )}

      {kgCtx.crs_id && (
        <div className="task-kg-context-panel__crs">
          <span className="kg-badge kg-badge--crs">CRS</span>
          <code>{kgCtx.crs_id}</code>
          {crsNode?.status && (
            <KGStatusBadge status={crsNode.status as CRSStatus} />
          )}
          {crsNode?.layer && (
            <span className="kg-meta">Layer: {String(crsNode.layer)}</span>
          )}
          {crsNode?.feature_ref && (
            <span className="kg-meta">Feature: {String(crsNode.feature_ref)}</span>
          )}
        </div>
      )}

      {kgCtx.feature_ref && (
        <div className="task-kg-context-panel__feature">
          <span className="kg-badge kg-badge--feature">Feature</span>
          <code>{kgCtx.feature_ref}</code>
          {featureNode?.description && (
            <span className="kg-meta">{String(featureNode.description)}</span>
          )}
        </div>
      )}

      {kgCtx.service && (
        <div className="task-kg-context-panel__meta">
          <span className="kg-meta-label">Service:</span>
          <code>{kgCtx.service}</code>
        </div>
      )}

      {kgCtx.repo_target && (
        <div className="task-kg-context-panel__meta">
          <span className="kg-meta-label">Repo:</span>
          <code>{kgCtx.repo_target}</code>
        </div>
      )}

      {/* System prompt convention hint — mirrors BuildSystemPrompt() (SOL-ORCA-002 §2.6) */}
      {kgCtx.crs_id && (
        <div className="task-kg-context-panel__convention-hint">
          <span className="kg-meta-label">Convention:</span>
          <code>// implements: {kgCtx.crs_id}</code>
        </div>
      )}
    </section>
  )
}
```

---

### SOL-2E: Task Form — KG Fields (snake_case API match)

```typescript
// src/renderer/src/components/tasks/task-form-kg-fields.tsx
// Fields submit as snake_case JSON để match API (SOL-ORCA-002 §2.4)

import type { FC } from 'react'
import type { TaskKGContext } from '../../types/task-types'

interface Props {
  value: TaskKGContext
  onChange: (ctx: TaskKGContext) => void
}

export const TaskFormKGFields: FC<Props> = ({ value, onChange }) => (
  <fieldset className="task-form-kg-fields" id="task-form-kg-fields">
    <legend>KG Context (Optional)</legend>

    <div className="form-field">
      <label htmlFor="task-crs-id">CRS ID</label>
      <input
        id="task-crs-id"
        type="text"
        placeholder="CRS-045"
        value={value.crs_id ?? ''}
        onChange={(e) => onChange({ ...value, crs_id: e.target.value || undefined })}
      />
      <span className="form-field__hint">PIKB CRS implementing this task tracks</span>
    </div>

    <div className="form-field">
      <label htmlFor="task-feature-ref">Feature Ref</label>
      <input
        id="task-feature-ref"
        type="text"
        placeholder="F27"
        value={value.feature_ref ?? ''}
        onChange={(e) => onChange({ ...value, feature_ref: e.target.value || undefined })}
      />
    </div>

    <div className="form-field">
      <label htmlFor="task-service">Target Service</label>
      <input
        id="task-service"
        type="text"
        placeholder="ai-service"
        value={value.service ?? ''}
        onChange={(e) => onChange({ ...value, service: e.target.value || undefined })}
      />
    </div>

    <div className="form-field">
      <label htmlFor="task-repo-target">Repo Target</label>
      <input
        id="task-repo-target"
        type="text"
        placeholder="vnp-workspace"
        value={value.repo_target ?? ''}
        onChange={(e) => onChange({ ...value, repo_target: e.target.value || undefined })}
      />
      <span className="form-field__hint">
        Required for convention verify (// implements: CRS-XXX check)
      </span>
    </div>
  </fieldset>
)
```

---

## Files cần tạo/sửa

```
src/renderer/src/
├── types/
│   └── task-types.ts                           ← EDIT: TaskKGContext, TaskEnrichment, Task.kg_context
├── store/
│   ├── index.ts                                ← EDIT: add createTaskEnrichmentSlice
│   └── slices/
│       └── task-enrichment.ts                  ← NEW (bao gồm updateEnrichmentCRSStatus cho SOL-003)
├── hooks/
│   └── use-task-enrichment.ts                  ← NEW
└── components/
    └── tasks/
        ├── task-kg-context-panel.tsx            ← NEW
        └── task-form-kg-fields.tsx              ← NEW
```

---

## Tests

```typescript
// types/__tests__/task-types.test.ts
describe('TaskKGContext shape', () => {
  it('snake_case fields match API JSON: crs_id, feature_ref, service, repo_target')
})

// store/slices/__tests__/task-enrichment.test.ts
describe('TaskEnrichmentSlice', () => {
  it('setTaskEnrichmentLoading: loading=true, fetchedAt giữ nguyên')
  it('setTaskEnrichment: lưu crsContext và featureContext, loading=false')
  it('setTaskEnrichmentError: loading=false, error set')
  it('updateEnrichmentCRSStatus: patch status trong crsContext raw JSON')
})

// hooks/__tests__/use-task-enrichment.test.ts
describe('useTaskEnrichment', () => {
  it('không fetch khi kg_context undefined (backend: CRSID == "" → skip)')
  it('không fetch khi crs_id và feature_ref đều absent')
  it('skip fetch khi cache < 5 phút')
  it('gọi pis_query_crs khi crs_id present')
  it('gọi pis_query_feature_context khi feature_ref present')
  it('concurrent fetch: gọi cả hai khi cả crs_id và feature_ref có mặt')
  it('best-effort: setTaskEnrichment gọi dù pis_query_crs fail')
})

// components/tasks/__tests__/task-kg-context-panel.test.tsx
describe('TaskKGContextPanel', () => {
  it('render null khi kg_context absent (no crs_id AND no feature_ref)')
  it('hiển thị CRS ID và status badge từ crsContext.status')
  it('hiển thị convention hint: // implements: CRS-045')
  it('hiển thị repo_target khi có')
  it('loading state với aria-live=polite')
})

// components/tasks/__tests__/task-form-kg-fields.test.tsx
describe('TaskFormKGFields', () => {
  it('4 inputs có unique IDs')
  it('onChange truyền snake_case object')
  it('empty string → undefined (không gửi null lên API)')
})
```

---

## Acceptance Criteria (Frontend)

| AC | Mô tả | Khớp với Backend AC |
|----|-------|---------------------|
| AC-F1 | `TaskKGContext` type có 4 snake_case fields khớp API JSON | SOL-ORCA-002 §2.4 CreateTaskRequest |
| AC-F2 | Panel hiển thị CRS status badge từ raw crsContext JSON | SOL-ORCA-002 §2.5 EnrichedTask.CRSContext |
| AC-F3 | Convention hint `// implements: CRS-XXX` hiển thị | SOL-ORCA-002 §2.6 BuildSystemPrompt |
| AC-F4 | Panel ẩn khi `crs_id` và `feature_ref` đều absent | SOL-ORCA-002 §2.5 — skip khi CRSID=="" |
| AC-F5 | Concurrent fetch CRS + Feature (`Promise.allSettled`) | SOL-ORCA-002 risk: concurrent fetch |
| AC-F6 | Best-effort: panel không crash khi 1 trong 2 fetch fail | SOL-ORCA-002 §2.5 best-effort pattern |
| AC-F7 | `repo_target` field hiển thị (dùng cho quality gate SOL-003) | SOL-ORCA-002 §2.2 RepoTarget field |
