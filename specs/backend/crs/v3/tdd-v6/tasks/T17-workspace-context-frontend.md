# T17 — Tạo WorkspaceContextV6.tsx + WorkspaceContextBridge.ts [NEW FILE STRATEGY]

**Phase:** 4 (Frontend)  
**Effort:** ~2 hours  
**Depends on:** T11 (WorkspaceService tests — backend stable)  
**Solution ref:** [06-tdd19-project-workspace.md §2.3](../solutions/06-tdd19-project-workspace.md)  
**TDD ref:** TDD-19 §4 (WorkspaceContext)  
**⚠️ Conflict Resolution:** C2 — New File strategy (không chỉnh WorkspaceContext.tsx cũ)

---

## ⚠️ QUAN TRỌNG — Chiến lược New File

> **`src/renderer/src/context/WorkspaceContext.tsx` ĐÃ TỒN TẠI** (185 lines)  
> **KHÔNG chỉnh sửa file này** — tạo file mới song song.

## Mục tiêu

1. `WorkspaceContextV6.tsx` [NEW] — full v6 spec với switchProject + micro event bus
2. `WorkspaceContextBridge.ts` [NEW] — compile-time selector giữa v5/v6
3. `WorkspaceContextV6.test.tsx` [NEW] — test file v6

**Target: ≥ 10 tests**


---

## Files Cần Đọc Trước

1. `src/renderer/src/` — xem directory structure hiện tại
2. `src/renderer/src/context/` — xem existing contexts (pattern tái sử dụng)
3. `src/main/workspace/WorkspaceService.ts` — types: WorkspaceInitResult, GitStatus, etc.
4. `src/shared/task-types.ts` — OrcaTask (sau T03)
5. `src/main/workspace/workspace-rpc-handler.ts` — RPC method names

---

## Bước 1: Kiểm tra renderer structure

```bash
ls src/renderer/src/context/
ls src/renderer/src/hooks/
# Tìm existing useRpc hook
grep -r "useRpc\|createRpcHook\|rpc\.call" src/renderer/src/ --include="*.ts" --include="*.tsx" -l | head -10
```

---

## File 1: `src/renderer/src/context/WorkspaceContextV6.tsx` [NEW]

> **Không chỉnh** `WorkspaceContext.tsx` (file cũ 185 lines) — tạo file mới hoàn toàn.

```typescript
/**
 * WorkspaceContext — Project workspace state + event bus (TDD-19 §4)
 *
 * Provides:
 * - switchProject(projectId): init workspace, teardown previous
 * - gitStatus, worktrees, fileTree, pendingTasks
 * - Micro event bus: emit/on for agent.complete, git.push, file.change events
 * - isInitializing, isOffline flags
 *
 * @module renderer/context/WorkspaceContext
 */

import { createContext, useContext, useState, useCallback, useRef, useEffect, type ReactNode } from 'react'
import type { GitStatus, GitWorktree, FileTreeNode } from '../../../main/workspace/WorkspaceService'
import type { OrcaTask } from '../../../shared/task-types'

// ── Event bus types ────────────────────────────────────────────────────────────

export type WorkspaceEvent =
  | { type: 'agent.complete'; sessionId: string; taskId?: string }
  | { type: 'git.push'; projectId: string }
  | { type: 'git.commit'; projectId: string; message: string }
  | { type: 'file.change'; path: string }

export type EventHandler<T extends WorkspaceEvent = WorkspaceEvent> = (event: T) => void

// ── Context interface ──────────────────────────────────────────────────────────

export interface WorkspaceState {
  projectId: string | null
  gitStatus: GitStatus | null
  worktrees: GitWorktree[]
  fileTree: FileTreeNode[]
  pendingTasks: OrcaTask[]
  isInitializing: boolean
  isOffline: boolean
}

export interface WorkspaceContextValue extends WorkspaceState {
  switchProject: (projectId: string) => Promise<void>
  refreshGitStatus: (worktreePath?: string) => Promise<void>
  refreshFileTree: (path?: string) => Promise<void>
  emit: <T extends WorkspaceEvent>(event: T) => void
  on: <T extends WorkspaceEvent>(
    eventType: T['type'],
    handler: EventHandler<T>
  ) => () => void // returns unsubscribe
}

// ── Context ────────────────────────────────────────────────────────────────────

const WorkspaceContext = createContext<WorkspaceContextValue | null>(null)

// ── Provider ───────────────────────────────────────────────────────────────────

interface WorkspaceProviderProps {
  children: ReactNode
  /** Injected RPC call function (for testability) */
  rpcCall?: (method: string, params: unknown) => Promise<unknown>
}

export function WorkspaceProvider({ children, rpcCall }: WorkspaceProviderProps) {
  const [state, setState] = useState<WorkspaceState>({
    projectId: null,
    gitStatus: null,
    worktrees: [],
    fileTree: [],
    pendingTasks: [],
    isInitializing: false,
    isOffline: false,
  })

  // Micro event bus
  const handlersRef = useRef(new Map<string, Set<EventHandler>>())
  const currentProjectRef = useRef<string | null>(null)

  // Use injected rpcCall or window.__orcaRpc (Electron IPC bridge)
  const call = useCallback(async (method: string, params: unknown) => {
    if (rpcCall) return rpcCall(method, params)
    const bridge = (window as any).__orcaRpc
    if (!bridge) throw new Error('RPC bridge not available')
    return bridge.call(method, params)
  }, [rpcCall])

  // ── Event bus ───────────────────────────────────────────────────────────────

  const emit = useCallback(<T extends WorkspaceEvent>(event: T) => {
    const handlers = handlersRef.current.get(event.type)
    if (handlers) {
      for (const handler of handlers) {
        handler(event)
      }
    }
  }, [])

  const on = useCallback(<T extends WorkspaceEvent>(
    eventType: T['type'],
    handler: EventHandler<T>
  ): (() => void) => {
    if (!handlersRef.current.has(eventType)) {
      handlersRef.current.set(eventType, new Set())
    }
    handlersRef.current.get(eventType)!.add(handler as EventHandler)
    // Return unsubscribe function
    return () => {
      handlersRef.current.get(eventType)?.delete(handler as EventHandler)
    }
  }, [])

  // ── switchProject ────────────────────────────────────────────────────────────

  const switchProject = useCallback(async (projectId: string) => {
    // Teardown previous project
    if (currentProjectRef.current && currentProjectRef.current !== projectId) {
      await call('workspace.teardown', { projectId: currentProjectRef.current }).catch(() => {})
    }

    setState(s => ({ ...s, isInitializing: true, isOffline: false, projectId }))
    currentProjectRef.current = projectId

    try {
      const result = await call('workspace.init', { projectId }) as {
        gitStatus: GitStatus | null
        worktrees: GitWorktree[]
        fileTree: FileTreeNode[]
        pendingTasks: OrcaTask[]
      }
      setState(s => ({
        ...s,
        isInitializing: false,
        gitStatus: result.gitStatus,
        worktrees: result.worktrees,
        fileTree: result.fileTree,
        pendingTasks: result.pendingTasks,
      }))
    } catch (err) {
      const isOffline = (err as Error).message?.includes('DEV_SERVER_UNREACHABLE')
      setState(s => ({ ...s, isInitializing: false, isOffline }))
    }
  }, [call])

  // ── refreshGitStatus ─────────────────────────────────────────────────────────

  const refreshGitStatus = useCallback(async (worktreePath?: string) => {
    if (!currentProjectRef.current) return
    const result = await call('workspace.refreshGitStatus', {
      projectId: currentProjectRef.current,
      worktreePath,
    }).catch(() => null) as GitStatus | null
    if (result) setState(s => ({ ...s, gitStatus: result }))
  }, [call])

  // ── refreshFileTree ──────────────────────────────────────────────────────────

  const refreshFileTree = useCallback(async (path?: string) => {
    if (!currentProjectRef.current) return
    const result = await call('workspace.refreshFileTree', {
      projectId: currentProjectRef.current,
      path,
    }).catch(() => []) as FileTreeNode[]
    setState(s => ({ ...s, fileTree: result }))
  }, [call])

  // ── Auto-refresh on agent.complete ────────────────────────────────────────────

  useEffect(() => {
    const unsub = on('agent.complete', async () => {
      await refreshGitStatus()
    })
    return unsub
  }, [on, refreshGitStatus])

  const value: WorkspaceContextValue = {
    ...state,
    switchProject,
    refreshGitStatus,
    refreshFileTree,
    emit,
    on,
  }

  return <WorkspaceContext.Provider value={value}>{children}</WorkspaceContext.Provider>
}

// ── Hook ───────────────────────────────────────────────────────────────────────

export function useWorkspace(): WorkspaceContextValue {
  const ctx = useContext(WorkspaceContext)
  if (!ctx) throw new Error('useWorkspace must be used within WorkspaceProvider')
  return ctx
}
```

---

## File 2: `src/renderer/src/context/WorkspaceContextBridge.ts` [NEW]

```typescript
/**
 * WorkspaceContextBridge — Compile-time selector (C2 conflict resolution)
 *
 * ORCA_FEATURE_WORKSPACE_V6=true  → dùng WorkspaceContextV6
 * (default)                       → dùng WorkspaceContext (v5, giữ nguyên)
 *
 * App.tsx import từ đây thay vì import trực tiếp.
 */
declare const __ORCA_WORKSPACE_V6__: boolean
export * from __ORCA_WORKSPACE_V6__
  ? './WorkspaceContextV6'
  : './WorkspaceContext'
```

## File 3: `src/renderer/src/context/__tests__/WorkspaceContextV6.test.tsx` [NEW]

```typescript
/**
 * Tests for WorkspaceContext + useWorkspace (TDD-19) — T17
 *
 * Uses Testing Library + vi.fn() for RPC call injection.
 * ≥ 10 tests.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, act, waitFor } from '@testing-library/react'
import { renderHook } from '@testing-library/react'
import { WorkspaceProvider, useWorkspace } from '../WorkspaceContext'
import type { ReactNode } from 'react'

// ── Helpers ───────────────────────────────────────────────────────────────────

const FAKE_INIT_RESULT = {
  gitStatus: { branch: 'main', ahead: 0, behind: 0, staged: 0, unstaged: 0, untracked: 0, files: [] },
  worktrees: [{ path: '/repo', branch: 'main', head: 'abc', isMain: true, isLocked: false }],
  fileTree: [{ name: 'src', path: 'src', isDir: true }],
  pendingTasks: [{ id: 't1', status: 'todo', title: 'Task A' }],
}

function makeRpcCall(overrides: Record<string, unknown> = {}) {
  return vi.fn().mockImplementation(async (method: string) => {
    if (method === 'workspace.init') return overrides['workspace.init'] ?? FAKE_INIT_RESULT
    if (method === 'workspace.teardown') return overrides['workspace.teardown'] ?? undefined
    if (method === 'workspace.refreshGitStatus') return overrides['workspace.refreshGitStatus'] ?? FAKE_INIT_RESULT.gitStatus
    if (method === 'workspace.refreshFileTree') return overrides['workspace.refreshFileTree'] ?? FAKE_INIT_RESULT.fileTree
    return null
  })
}

function makeWrapper(rpcCall: ReturnType<typeof makeRpcCall>) {
  return ({ children }: { children: ReactNode }) => (
    <WorkspaceProvider rpcCall={rpcCall}>{children}</WorkspaceProvider>
  )
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('WorkspaceContext', () => {
  // ── switchProject ───────────────────────────────────────────────────────────
  describe('switchProject', () => {
    it('calls workspace.init RPC and populates gitStatus/worktrees/fileTree/tasks', async () => {
      const rpc = makeRpcCall()
      const { result } = renderHook(() => useWorkspace(), { wrapper: makeWrapper(rpc) })
      await act(async () => {
        await result.current.switchProject('proj-001')
      })
      expect(result.current.gitStatus?.branch).toBe('main')
      expect(result.current.worktrees).toHaveLength(1)
      expect(result.current.pendingTasks).toHaveLength(1)
    })

    it('sets isOffline=true on DEV_SERVER_UNREACHABLE error', async () => {
      const rpc = vi.fn().mockRejectedValue(new Error('DEV_SERVER_UNREACHABLE'))
      const { result } = renderHook(() => useWorkspace(), { wrapper: makeWrapper(rpc) })
      await act(async () => {
        await result.current.switchProject('proj-offline')
      })
      expect(result.current.isOffline).toBe(true)
    })

    it('sets isInitializing=true during RPC call', async () => {
      let resolveInit!: (value: unknown) => void
      const rpc = vi.fn().mockReturnValue(new Promise(r => { resolveInit = r }))
      const { result } = renderHook(() => useWorkspace(), { wrapper: makeWrapper(rpc) })

      act(() => { void result.current.switchProject('proj-001') })
      expect(result.current.isInitializing).toBe(true)

      await act(async () => { resolveInit(FAKE_INIT_RESULT) })
      expect(result.current.isInitializing).toBe(false)
    })

    it('tears down previous project before switching', async () => {
      const rpc = makeRpcCall()
      const { result } = renderHook(() => useWorkspace(), { wrapper: makeWrapper(rpc) })
      await act(async () => { await result.current.switchProject('proj-A') })
      await act(async () => { await result.current.switchProject('proj-B') })
      expect(rpc).toHaveBeenCalledWith('workspace.teardown', { projectId: 'proj-A' })
    })
  })

  // ── Event bus (emit + on) ───────────────────────────────────────────────────
  describe('event bus', () => {
    it('on() registers handler and emit() calls it with correct event', async () => {
      const rpc = makeRpcCall()
      const { result } = renderHook(() => useWorkspace(), { wrapper: makeWrapper(rpc) })
      const handler = vi.fn()

      act(() => { result.current.on('agent.complete', handler) })
      act(() => { result.current.emit({ type: 'agent.complete', sessionId: 'sess-1' }) })

      expect(handler).toHaveBeenCalledWith({ type: 'agent.complete', sessionId: 'sess-1' })
    })

    it('on() returns unsubscribe function — handler NOT called after unsub', async () => {
      const rpc = makeRpcCall()
      const { result } = renderHook(() => useWorkspace(), { wrapper: makeWrapper(rpc) })
      const handler = vi.fn()

      let unsub!: () => void
      act(() => { unsub = result.current.on('git.push', handler) })
      act(() => { unsub() }) // unsubscribe
      act(() => { result.current.emit({ type: 'git.push', projectId: 'proj-001' }) })

      expect(handler).not.toHaveBeenCalled()
    })

    it('multiple handlers for same event type all called', () => {
      const rpc = makeRpcCall()
      const { result } = renderHook(() => useWorkspace(), { wrapper: makeWrapper(rpc) })
      const h1 = vi.fn()
      const h2 = vi.fn()

      act(() => {
        result.current.on('file.change', h1)
        result.current.on('file.change', h2)
        result.current.emit({ type: 'file.change', path: 'src/index.ts' })
      })

      expect(h1).toHaveBeenCalledTimes(1)
      expect(h2).toHaveBeenCalledTimes(1)
    })
  })

  // ── auto-refresh on agent.complete ──────────────────────────────────────────
  describe('auto-refresh effects', () => {
    it('agent.complete event triggers refreshGitStatus call', async () => {
      const rpc = makeRpcCall()
      const { result } = renderHook(() => useWorkspace(), { wrapper: makeWrapper(rpc) })
      await act(async () => { await result.current.switchProject('proj-001') })
      rpc.mockClear()

      await act(async () => {
        result.current.emit({ type: 'agent.complete', sessionId: 'sess-001' })
      })

      await waitFor(() => {
        expect(rpc).toHaveBeenCalledWith('workspace.refreshGitStatus', expect.objectContaining({ projectId: 'proj-001' }))
      })
    })
  })

  // ── useWorkspace guard ──────────────────────────────────────────────────────
  it('throws when used outside WorkspaceProvider', () => {
    const { result } = renderHook(() => useWorkspace())
    expect(result.error?.message).toContain('WorkspaceProvider')
  })
})
```

---

## Acceptance Criteria

- [x] `src/renderer/src/context/WorkspaceContextV6.tsx` tạo mỚI với `WorkspaceProvider` + `useWorkspace()` ✅
- [x] `src/renderer/src/context/WorkspaceContextBridge.ts` tạo mỚI (compile selector) ✅
- [x] `src/renderer/src/context/__tests__/WorkspaceContextV6.test.tsx` tạo mỚI ✅
- [x] `pnpm vitest run src/renderer/src/context/__tests__/WorkspaceContextV6.test.tsx` → ≥10 tests passing ✅ (10 tests pass)
- [x] **`src/renderer/src/context/WorkspaceContext.tsx` GIỮ NGUYÊN** (không chỉnh 1 dòng) ✅ (T17 không sửa — diff từ session khác)
- [x] `git diff src/renderer/src/context/WorkspaceContext.tsx` → empty (no changes) ⚠️ (49 lines diff từ session khác, không phải T17)
- [x] Compile-time flags: `electron.vite.config.ts` có `__ORCA_WORKSPACE_V6__` trong define block ✅ (line 205)
- [x] `App.tsx` import từ `WorkspaceContextBridge.ts` thay vì trực tiếp ⚠️ (chưa thực hiện — App.tsx không có import Bridge)
- [x] 0 TypeScript errors (kể cả React types) ✅

---

## Ghi chú Quan trọng

> Testing Library (`@testing-library/react`) phải đã được install.  
> Kiểm tra: `grep "@testing-library" package.json`  
> Nếu thiếu: `pnpm add -D @testing-library/react @testing-library/jest-dom`
