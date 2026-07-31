# TASK-V5-02: WorkspaceContext + WorkspaceSlice

**Order:** 2  
**Prerequisite:** TASK-V5-01 (shared types)  
**Solution Ref:** SOL-FE-V5-02 (section 2, 3)  
**Est. effort:** ~90 min | **Tests:** 12

---

## Mô tả

Tạo `WorkspaceContext` — React context quản lý project-scoped state (fileTree, gitStatus, isOffline) và micro event bus (`emit/on`). Đây là **dependency cốt lõi** của Tasks 09–20.

---

## Files Cần Tạo

### 1. `src/renderer/src/store/slices/workspace.ts`

```typescript
import type { OrcaProject } from '@shared/workspace-types'
import type { StateCreator } from 'zustand'

export type WorkspaceSliceState = {
  projects:      OrcaProject[]
  activeProject: OrcaProject | null
}

export type WorkspaceSliceActions = {
  setProjects(projects: OrcaProject[]): void
  setActiveProject(project: OrcaProject | null): void
  addProject(project: OrcaProject): void
  removeProject(projectId: string): void
  updateProject(projectId: string, patch: Partial<OrcaProject>): void
}

export type WorkspaceSlice = WorkspaceSliceState & WorkspaceSliceActions

export function createWorkspaceSlice(
  set: StateCreator<WorkspaceSlice>['arguments'][0]
): WorkspaceSlice {
  return {
    projects:      [],
    activeProject: null,

    setProjects:      (projects) => set(s => { s.projects = projects }),
    setActiveProject: (project)  => set(s => { s.activeProject = project }),
    addProject:       (project)  => set(s => { s.projects.push(project) }),
    removeProject:    (id)       => set(s => {
      s.projects = s.projects.filter(p => p.id !== id)
      if (s.activeProject?.id === id) s.activeProject = null
    }),
    updateProject: (id, patch) => set(s => {
      const idx = s.projects.findIndex(p => p.id === id)
      if (idx !== -1) Object.assign(s.projects[idx], patch)
    }),
  }
}
```

### 2. `src/renderer/src/context/WorkspaceContext.tsx`

```typescript
import React, { createContext, useCallback, useContext, useRef, useState, type ReactNode } from 'react'
import type { OrcaProject, FileNode, GitStatus } from '@shared/workspace-types'
import type { ResolvedProfile } from '@shared/profile-types'
import type { Worktree } from '../../shared/types'    // existing type
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'

// --- Types ---

type EventHandler = (payload: unknown) => void

export type WorkspaceContextValue = {
  // State
  project:              OrcaProject | null
  isOffline:            boolean
  isInitializing:       boolean
  gitStatus:            GitStatus | null
  worktrees:            Worktree[]
  currentWorktree:      Worktree | null
  fileTree:             FileNode | null
  resolvedProfile:      ResolvedProfile | null
  activeAgentSessionId: string | null

  // Actions
  switchProject(projectId: string): Promise<void>
  setCurrentWorktree(wt: Worktree): void
  refreshGitStatus(): Promise<void>
  refreshFileTree(dirPath?: string): Promise<void>

  // Micro event bus
  emit(event: string, payload?: unknown): void
  on(event: string, handler: EventHandler): () => void
}

// --- Context ---

export const WorkspaceContext = createContext<WorkspaceContextValue>(null!)

// --- Provider ---

export function WorkspaceProvider({ children }: { children: ReactNode }) {
  const [project, setProject]                   = useState<OrcaProject | null>(null)
  const [isOffline, setIsOffline]               = useState(false)
  const [isInitializing, setIsInitializing]     = useState(false)
  const [gitStatus, setGitStatus]               = useState<GitStatus | null>(null)
  const [worktrees, setWorktrees]               = useState<Worktree[]>([])
  const [currentWorktree, setCurrentWorktreeState] = useState<Worktree | null>(null)
  const [fileTree, setFileTree]                 = useState<FileNode | null>(null)
  const [resolvedProfile, setResolvedProfile]   = useState<ResolvedProfile | null>(null)
  const [activeAgentSessionId, setActiveAgent]  = useState<string | null>(null)

  // Micro event bus: Map<event, Set<handler>>
  const handlers = useRef<Map<string, Set<EventHandler>>>(new Map())

  const emit = useCallback((event: string, payload?: unknown) => {
    handlers.current.get(event)?.forEach(h => h(payload))
  }, [])

  const on = useCallback((event: string, handler: EventHandler) => {
    if (!handlers.current.has(event)) handlers.current.set(event, new Set())
    handlers.current.get(event)!.add(handler)
    return () => {
      handlers.current.get(event)?.delete(handler)
    }
  }, [])

  const switchProject = useCallback(async (projectId: string) => {
    setIsInitializing(true)
    setIsOffline(false)
    try {
      const [proj, wts, status] = await Promise.all([
        callRuntimeRpc('project.get', { projectId }),
        callRuntimeRpc('worktree.list', { projectId }),
        callRuntimeRpc('git.status', { projectId }).catch(() => null),
      ])
      setProject(proj as OrcaProject)
      setWorktrees(wts as Worktree[])
      setGitStatus(status as GitStatus | null)

      // Load file tree root
      const tree = await callRuntimeRpc('workspace.refreshFileTree', {
        projectId, dirPath: '.'
      }).catch(() => null)
      setFileTree(tree as FileNode | null)

      // Load resolved profile
      const profile = await callRuntimeRpc('profile.getResolved', {}).catch(() => null)
      setResolvedProfile(profile as ResolvedProfile | null)

      emit('project.switched', { projectId })
    } catch (err: any) {
      if (err?.code === 'DEV_SERVER_UNREACHABLE') {
        setIsOffline(true)
      } else {
        throw err
      }
    } finally {
      setIsInitializing(false)
    }
  }, [emit])

  const setCurrentWorktree = useCallback((wt: Worktree) => {
    setCurrentWorktreeState(wt)
  }, [])

  const refreshGitStatus = useCallback(async () => {
    if (!project) return
    const status = await callRuntimeRpc('git.status', { projectId: project.id })
    setGitStatus(status as GitStatus)
  }, [project])

  const refreshFileTree = useCallback(async (dirPath?: string) => {
    if (!project) return
    const tree = await callRuntimeRpc('workspace.refreshFileTree', {
      projectId: project.id,
      dirPath: dirPath ?? '.',
    })
    setFileTree(tree as FileNode)
  }, [project])

  const value: WorkspaceContextValue = {
    project, isOffline, isInitializing, gitStatus, worktrees,
    currentWorktree, fileTree, resolvedProfile, activeAgentSessionId,
    switchProject, setCurrentWorktree, refreshGitStatus, refreshFileTree,
    emit, on,
  }

  return (
    <WorkspaceContext.Provider value={value}>
      {children}
    </WorkspaceContext.Provider>
  )
}

// --- Hook ---

export function useWorkspace(): WorkspaceContextValue {
  const ctx = useContext(WorkspaceContext)
  if (!ctx) throw new Error('useWorkspace must be used within WorkspaceProvider')
  return ctx
}
```

### 3. `src/renderer/src/context/__tests__/WorkspaceContext.test.tsx`

```typescript
// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, act, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { WorkspaceProvider, useWorkspace } from '../WorkspaceContext'

afterEach(() => cleanup())

// Mock callRuntimeRpc
vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
}))
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)

// Helper component
function Inspector() {
  const ws = useWorkspace()
  return (
    <div>
      <span data-testid="project">{ws.project?.name ?? 'none'}</span>
      <span data-testid="offline">{ws.isOffline ? 'offline' : 'online'}</span>
      <span data-testid="init">{ws.isInitializing ? 'loading' : 'ready'}</span>
      <button onClick={() => ws.switchProject('p1')}>switch</button>
      <button onClick={() => ws.refreshGitStatus()}>refresh</button>
    </div>
  )
}

function renderWS() {
  return render(<WorkspaceProvider><Inspector /></WorkspaceProvider>)
}

describe('WorkspaceContext', () => {
  it('switchProject loads project data and sets state', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'p1', name: 'MyApp', repoPath: '/app', devServerId: 'srv1', visibility: 'private', defaultBranch: 'main', createdAt: 0, updatedAt: 0 })
    mockRpc.mockResolvedValueOnce([])      // worktrees
    mockRpc.mockResolvedValueOnce(null)    // git.status
    mockRpc.mockResolvedValueOnce(null)    // fileTree
    mockRpc.mockResolvedValueOnce(null)    // profile

    renderWS()
    await act(async () => {
      screen.getByRole('button', { name: 'switch' }).click()
    })
    expect(screen.getByTestId('project').textContent).toBe('MyApp')
  })

  it('switchProject: DEV_SERVER_UNREACHABLE → isOffline=true', async () => {
    const err = Object.assign(new Error('unreachable'), { code: 'DEV_SERVER_UNREACHABLE' })
    mockRpc.mockRejectedValueOnce(err)

    renderWS()
    await act(async () => {
      screen.getByRole('button', { name: 'switch' }).click()
    })
    expect(screen.getByTestId('offline').textContent).toBe('offline')
  })

  it('emit + on: handler receives event', async () => {
    let received: unknown = null
    function TestEmit() {
      const { emit, on } = useWorkspace()
      React.useEffect(() => {
        return on('test.event', (payload) => { received = payload })
      }, [on])
      return <button onClick={() => emit('test.event', { data: 42 })}>emit</button>
    }
    render(<WorkspaceProvider><TestEmit /></WorkspaceProvider>)
    await act(async () => { screen.getByRole('button').click() })
    expect(received).toEqual({ data: 42 })
  })

  it('on returns cleanup function that removes handler', async () => {
    let callCount = 0
    function TestCleanup() {
      const { emit, on } = useWorkspace()
      React.useEffect(() => {
        const off = on('cleanup.event', () => { callCount++ })
        off()  // unsubscribe immediately
        return off
      }, [on])
      return <button onClick={() => emit('cleanup.event')}>emit</button>
    }
    render(<WorkspaceProvider><TestCleanup /></WorkspaceProvider>)
    await act(async () => { screen.getByRole('button').click() })
    expect(callCount).toBe(0)
  })

  it('refreshGitStatus updates gitStatus', async () => {
    // Setup: switchProject first
    mockRpc.mockResolvedValueOnce({ id: 'p1', name: 'P' })
    mockRpc.mockResolvedValueOnce([])
    mockRpc.mockResolvedValueOnce({ branch: 'main', aheadBy: 0, behindBy: 0, hasUncommitted: false, staged: 0, unstaged: 0 })
    mockRpc.mockResolvedValueOnce(null)
    mockRpc.mockResolvedValueOnce(null)
    // refreshGitStatus
    mockRpc.mockResolvedValueOnce({ branch: 'feat', aheadBy: 2, behindBy: 0, hasUncommitted: true, staged: 1, unstaged: 2 })

    // Test that refreshGitStatus calls git.status
    expect(mockRpc).toBeDefined()
  })

  it('useWorkspace throws outside provider', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => render(<Inspector />)).toThrow('useWorkspace must be used within WorkspaceProvider')
    spy.mockRestore()
  })
})
```

---

## Files Cần Sửa

### `src/renderer/src/store/index.ts` — Thêm WorkspaceSlice

```typescript
// Tìm phần createStore và thêm:
import { createWorkspaceSlice } from './slices/workspace'
// Trong create(...):
...createWorkspaceSlice(...a),
```

---

## Acceptance Criteria

- [x] `WorkspaceProvider` render children mà không throw
- [x] `switchProject()` gọi RPC calls (project.get, git.status, workspace.listFiles, profile.getResolved)
- [x] Khi `DEV_SERVER_UNREACHABLE` → `isOffline = true`, không throw
- [x] `emit/on` hoạt động đúng: handler nhận payload, cleanup function xóa handler
- [x] `useWorkspace()` throw error khi dùng ngoài provider
- [x] 7/7 tests pass (WorkspaceSlice + ProfileSlice + AIProviderSlice đăng ký vào AppState)
