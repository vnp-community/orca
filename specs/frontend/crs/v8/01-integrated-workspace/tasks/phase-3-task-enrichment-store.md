# Phase 3 — Task Enrichment Store

**Phase:** 3/6  
**Priority:** P2  
**Effort:** ~1 ngày  
**Depends on:** Phase 2 (use-agent-tool-capabilities.ts với PIS_TOOL_NAMES)  
**Unlocks:** Phase 4  
**Source solutions:** FE-IW-SOL-002

---

## Mục tiêu Phase

Tạo Zustand slice và hook cho task enrichment (auto-fetch CRS context + feature context khi task có `kg_context`). Đây là state layer foundation cho Phase 4 UI.

---

## T11 — Extend `task-types.ts`

```
Action:    EDIT
File:      src/renderer/src/types/task-types.ts
Solution:  FE-IW-SOL-002 §SOL-2A, FE-IW-SOL-003 §SOL-3A
```

**Thêm các types sau vào file (append hoặc merge vào existing types):**

```typescript
// ─── KG Context (CR-ORCA-002) ────────────────────────────────────────────────

// Mirror của Go TaskKGContext struct (SOL-ORCA-002 §2.2)
// snake_case để match JSON API từ Go json:"field_name" tags
export interface TaskKGContext {
  crs_id?: string       // Go: CRSID "CRS-045"
  feature_ref?: string  // Go: FeatureRef "F27"
  service?: string      // Go: Service "ai-service"
  repo_target?: string  // Go: RepoTarget "vnp-workspace" (dùng cho convention verify)
}

// EnrichedTask enrichment cache per-task (mirror SOL-ORCA-002 §2.5)
// crsContext và featureContext là raw JSON (json.RawMessage equivalent)
export interface TaskEnrichment {
  taskId: string
  crsContext?: unknown    // pis_query_crs result
  featureContext?: unknown // pis_query_feature_context result
  fetchedAt: number       // epoch ms — cache TTL 5 phút
  loading: boolean
  error?: string
}

// ─── Task Rejection (CR-ORCA-003) ────────────────────────────────────────────

// Mirror JSON của TaskRejectedResponse struct (SOL-ORCA-003 §2.5)
// field names khớp JSON: task_id, status, reason, action_required
export interface TaskRejection {
  task_id: string           // backend: TaskID
  status: 'rejected'        // always "rejected"
  reason: string            // "convention check failed: 1 files missing '// implements: CRS-045', 0 invalid"
  action_required: string   // "Add '// implements: CRS-045' comment to each changed file"
}

// TaskStatus — mở rộng với 'rejected' (SOL-ORCA-003 quality gate)
export type TaskStatus =
  | 'pending'
  | 'in-progress'
  | 'done'
  | 'rejected'     // quality gate fail từ CompletionHook
  | 'cancelled'

// ─── Task entity extension ────────────────────────────────────────────────────

// Thêm vào existing Task interface (nếu chưa có):
// kg_context: snake_case để match API response JSON
// rejection: present khi status='rejected'
//
// export interface Task {
//   ...existing fields...
//   status: TaskStatus
//   kg_context?: TaskKGContext
//   rejection?: TaskRejection
// }
```

**Chú ý AI executor:** Nếu file đã có `Task` interface, chỉ thêm `kg_context?: TaskKGContext` và `rejection?: TaskRejection` vào đó. Không duplicate interface.

**Verify:** Types export đúng. `TaskKGContext` có 4 snake_case fields. `TaskRejection` có 4 fields.

---

## T12 — Tạo `task-enrichment.ts` Zustand slice

```
Action:    NEW
File:      src/renderer/src/store/slices/task-enrichment.ts
Solution:  FE-IW-SOL-002 §SOL-2B, FE-IW-SOL-003 §SOL-3D
```

**Code cần tạo:**

```typescript
// src/renderer/src/store/slices/task-enrichment.ts
// Zustand slice: cache KG enrichment per task (CR-ORCA-002, CR-ORCA-003)
// Tách slice riêng vì enrichment là derived data với lifecycle riêng

import type { StateCreator } from 'zustand'
import type { AppState } from '../index'
import type { TaskEnrichment } from '../../types/task-types'

export interface TaskEnrichmentSlice {
  taskEnrichments: Record<string, TaskEnrichment>

  // Loading state khi fetch bắt đầu
  setTaskEnrichmentLoading: (taskId: string) => void

  // Lưu kết quả fetch (best-effort — cả hai có thể undefined nếu fetch fail)
  setTaskEnrichment: (
    taskId: string,
    crsContext?: unknown,
    featureContext?: unknown
  ) => void

  // Lưu error khi cả hai fetch fail
  setTaskEnrichmentError: (taskId: string, error: string) => void

  // CR-ORCA-003: cập nhật CRS status trong crsContext sau quality gate
  // Gọi từ onCRSStatusUpdated IPC event (PISTool.UpdateCRSStatus → "done")
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
          ...(s.taskEnrichments[taskId] ?? { taskId, fetchedAt: 0, loading: false }),
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
          ...(s.taskEnrichments[taskId] ?? { taskId, fetchedAt: 0, loading: false }),
          loading: false,
          error,
        },
      },
    })),

  // Best-effort CRS status patch (SOL-ORCA-003 §2.4)
  // Chỉ patch field 'status' trong crsContext raw JSON — không replace whole object
  updateEnrichmentCRSStatus: (taskId, newStatus) =>
    set((s) => {
      const entry = s.taskEnrichments[taskId]
      if (!entry?.crsContext) return s
      const updated = {
        ...(entry.crsContext as Record<string, unknown>),
        status: newStatus,
      }
      return {
        taskEnrichments: {
          ...s.taskEnrichments,
          [taskId]: { ...entry, crsContext: updated },
        },
      }
    }),
})
```

**Verify:** Slice export `TaskEnrichmentSlice`. 4 actions đúng signature.

---

## T12b — Đăng ký slice vào store index

```
Action:    EDIT
File:      src/renderer/src/store/index.ts
Solution:  FE-IW-SOL-002 §SOL-2B
```

**Thêm vào store:**

```typescript
// Thêm import:
import { createTaskEnrichmentSlice, type TaskEnrichmentSlice } from './slices/task-enrichment'

// Thêm TaskEnrichmentSlice vào AppState type:
// export type AppState = ... & TaskEnrichmentSlice & ...

// Thêm vào create() call:
// ...createTaskEnrichmentSlice(...a),
```

**Chú ý AI executor:** Chỉ thêm import và spread `...createTaskEnrichmentSlice(...a)` vào `create()`. Không sửa logic khác.

**Verify:** Store compile. `useAppStore().taskEnrichments` accessible.

---

## T13 — Tạo `use-task-enrichment.ts`

```
Action:    NEW
File:      src/renderer/src/hooks/use-task-enrichment.ts
Solution:  FE-IW-SOL-002 §SOL-2C
```

**Code cần tạo:**

```typescript
// src/renderer/src/hooks/use-task-enrichment.ts
// Hook auto-fetch KG context khi task có kg_context (CR-ORCA-002)
// Pattern: concurrent fetch CRS + feature, best-effort, 5min cache

import { useEffect } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import type { Task } from '../types/task-types'

// 5 phút cache — consistent với backend best-effort approach (SOL-ORCA-002 §5)
const KG_CACHE_TTL_MS = 5 * 60 * 1000

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
    // Skip nếu không có KG refs (backend: CRSID == "" → skip enrichment)
    if (!kgCtx?.crs_id && !kgCtx?.feature_ref) return

    // Cache: skip nếu đã fetch < 5 phút
    if (entry?.fetchedAt && Date.now() - entry.fetchedAt < KG_CACHE_TTL_MS) return

    const target = environmentId
      ? { kind: 'environment' as const, environmentId }
      : { kind: 'local' as const }

    setTaskEnrichmentLoading(task.id)

    // Concurrent fetch — mirror backend concurrent approach (SOL-ORCA-002 §5 risk)
    // Promise.allSettled: tiếp tục dù 1 trong 2 fail
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
    ])
      .then(([crsResult, featureResult]) => {
        // Best-effort: luôn gọi setTaskEnrichment dù có failure
        setTaskEnrichment(
          task.id,
          crsResult.status === 'fulfilled' ? crsResult.value : undefined,
          featureResult.status === 'fulfilled' ? featureResult.value : undefined
        )
      })
      .catch((err) => {
        setTaskEnrichmentError(task.id, String(err))
      })
  }, [task.id, kgCtx?.crs_id, kgCtx?.feature_ref, environmentId])

  return { entry, loading: entry?.loading ?? false }
}
```

**Verify:** Hook không crash khi `task.kg_context = undefined`. Cache 5 phút hoạt động.

---

## T13b — Tạo `task-selectors.ts`

```
Action:    NEW hoặc EDIT
File:      src/renderer/src/store/selectors/task-selectors.ts
Solution:  FE-IW-SOL-003 §SOL-3E
```

**Code cần thêm:**

```typescript
// src/renderer/src/store/selectors/task-selectors.ts
// Selectors cho task store (CR-ORCA-003: batch update sau onCRSStatusUpdated)

import type { AppState } from '../index'

// Tìm taskIds có kg_context.crs_id = crsId
// Dùng trong onCRSStatusUpdated handler để batch update KG badge
export const selectTaskIdsByCRSId = (state: AppState, crsId: string): string[] => {
  // Assume tasks tồn tại trong state (adapt theo actual store shape)
  const tasks = Object.values((state as Record<string, unknown>).tasks ?? {}) as Array<{
    id: string
    kg_context?: { crs_id?: string }
  }>
  return tasks
    .filter((task) => task.kg_context?.crs_id === crsId)
    .map((task) => task.id)
}
```

**Chú ý AI executor:** Adapt `state.tasks` access theo actual store shape trong project. Nếu tasks lưu trong slice khác, điều chỉnh path.

**Verify:** Selector return `string[]`. TypeScript clean.

---

## Checklist Phase 3

- [ ] T11: `task-types.ts` có `TaskKGContext` (4 snake_case fields), `TaskEnrichment`, `TaskRejection`, `TaskStatus`
- [ ] T12: `task-enrichment.ts` slice có 4 actions đúng signature
- [ ] T12b: `store/index.ts` đăng ký `createTaskEnrichmentSlice`
- [ ] T13: `use-task-enrichment.ts` — skip khi no kg_context, cache 5min, concurrent fetch
- [ ] T13b: `task-selectors.ts` — `selectTaskIdsByCRSId` return string[]
- [ ] TypeScript compile: `npx tsc --noEmit` — 0 errors từ Phase 3 files
