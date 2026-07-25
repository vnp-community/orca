# TASK-004-A — Tạo BootstrapSlice

**Task ID:** TASK-004-A  
**CR:** CR-004 — Dev Server Bootstrap Automation  
**Solution Ref:** SOL-CR-004, Section 2  
**Dependencies:** TASK-001-A  
**Estimated:** 2 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo Zustand slice `bootstrapSlice` để quản lý bootstrap state per-server (steps, log lines, phase).

---

## File cần tạo

`src/renderer/src/store/slices/bootstrap.ts`

---

## Bước thực thi

### Bước 1: Tạo bootstrap.ts

```typescript
// src/renderer/src/store/slices/bootstrap.ts
import type { StateCreator } from 'zustand'
import type { AppState } from '@/store/types'

// ─── Types ────────────────────────────────────────────────────────────────────

export type BootstrapStepStatus = 'pending' | 'running' | 'done' | 'error' | 'skipped'

export type BootstrapStep = {
  id: string
  label: string
  status: BootstrapStepStatus
  detail: string | null   // success detail, e.g. "Node.js v22.3.0 detected"
  error: string | null
}

export type BootstrapPhase = 'idle' | 'running' | 'done' | 'error'

export type ServerBootstrapState = {
  serverId: string
  phase: BootstrapPhase
  steps: BootstrapStep[]
  logLines: string[]
  startedAt: number | null
  completedAt: number | null
}

export type BootstrapSlice = {
  bootstrapByServer: Record<string, ServerBootstrapState>

  initBootstrap: (serverId: string) => void
  updateBootstrapStep: (
    serverId: string,
    stepId: string,
    update: Partial<BootstrapStep>
  ) => void
  appendBootstrapLog: (serverId: string, line: string) => void
  finishBootstrap: (serverId: string, success: boolean) => void
  clearBootstrap: (serverId: string) => void
}

const DEFAULT_BOOTSTRAP_STEPS: Omit<BootstrapStep, never>[] = [
  { id: 'node', label: 'Node.js 22+', status: 'pending', detail: null, error: null },
  { id: 'git', label: 'Git 2.35+', status: 'pending', detail: null, error: null },
  { id: 'ssh-key', label: 'SSH key setup', status: 'pending', detail: null, error: null },
  { id: 'repos', label: 'Clone/update repos', status: 'pending', detail: null, error: null },
  { id: 'setup-script', label: 'Run setup scripts', status: 'pending', detail: null, error: null },
]

const MAX_LOG_LINES = 500

// ─── Slice Factory ────────────────────────────────────────────────────────────

export const createBootstrapSlice: StateCreator<
  AppState,
  [],
  [],
  BootstrapSlice
> = (set) => ({
  bootstrapByServer: {},

  initBootstrap: (serverId) =>
    set((s) => {
      s.bootstrapByServer[serverId] = {
        serverId,
        phase: 'running',
        startedAt: Date.now(),
        completedAt: null,
        logLines: [],
        steps: DEFAULT_BOOTSTRAP_STEPS.map((step) => ({ ...step })),
      }
    }),

  updateBootstrapStep: (serverId, stepId, update) =>
    set((s) => {
      const state = s.bootstrapByServer[serverId]
      if (!state) return
      const step = state.steps.find((st) => st.id === stepId)
      if (step) Object.assign(step, update)
    }),

  appendBootstrapLog: (serverId, line) =>
    set((s) => {
      const state = s.bootstrapByServer[serverId]
      if (!state) return
      if (state.logLines.length >= MAX_LOG_LINES) {
        state.logLines.shift()
      }
      state.logLines.push(line)
    }),

  finishBootstrap: (serverId, success) =>
    set((s) => {
      const state = s.bootstrapByServer[serverId]
      if (!state) return
      state.phase = success ? 'done' : 'error'
      state.completedAt = Date.now()
    }),

  clearBootstrap: (serverId) =>
    set((s) => {
      delete s.bootstrapByServer[serverId]
    }),
})
```

### Bước 2: Đăng ký vào AppState và store

```typescript
// store/types.ts: thêm & BootstrapSlice vào AppState
// store/index.ts: thêm ...createBootstrapSlice(...a),
```

### Bước 3: Verify

```bash
npx tsc --noEmit 2>&1 | grep "bootstrap\|Bootstrap" | head -10
```

---

## Acceptance Criteria

- [x] `bootstrapByServer` là `Record<string, ServerBootstrapState>`
- [x] `initBootstrap(id)` khởi tạo state với 5 default steps ở status 'pending'
- [x] `updateBootstrapStep(id, stepId, update)` update đúng step
- [x] `appendBootstrapLog(id, line)` thêm log, cap tại 500 lines
- [x] `finishBootstrap(id, success)` set phase + completedAt
- [x] `clearBootstrap(id)` xóa server state
- [x] TypeScript compile clean

---

## Implementation Notes

> **Completed:** 2026-07-23 | `store/slices/bootstrap.ts`: bootstrapByServer Record<string,ServerBootstrapState>, initBootstrap() 5 default steps pending, updateBootstrapStep() updates step, appendBootstrapLog() adds line cap 500, finishBootstrap() phase+completedAt, clearBootstrap() deletes entry. TypeScript: ✅ 0 errors.
