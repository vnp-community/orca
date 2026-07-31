# SOL-FE-V5-02: Project Workspace Shell

**TDD Ref:** [TDD-FE-12](../../../tdd/12-project-workspace-ui.md)  
**Feature:** F34, F38 | **ADR:** ADR-011 | **HLD:** C3.12, C4.10  
**Status:** ✅ DONE — Implemented via TASK-V5-02, TASK-V5-03  
**Additive-only:** ✅ Không sửa App.tsx, main.tsx

---

## 1. Files Cần Tạo

| File | Loại | Mô tả |
|------|------|-------|
| `src/renderer/src/context/WorkspaceContext.tsx` | React Context | Project-scoped state + event bus |
| `src/renderer/src/store/slices/workspace.ts` | Zustand Slice | projects, fileTree, gitStatus, activeProject |
| `src/renderer/src/components/workspace/WorkspaceLayout.tsx` | Component | 3-panel resizable layout |
| `src/renderer/src/components/project/ProjectSwitcher.tsx` | Component | Dropdown project picker |
| `src/renderer/src/components/project/ProjectSettings.tsx` | Component | Settings dialog |
| `src/renderer/src/components/project/MemberManager.tsx` | Component | Member CRUD |
| `src/renderer/src/components/workspace/WorkspaceTabBar.tsx` | Component | Git/Tasks/Workflows/Agent tabs |
| `src/renderer/src/components/workspace/WorkspaceStatusBar.tsx` | Component | Status bar + panel toggles |
| `src/renderer/src/components/workspace/NoProjectSelected.tsx` | Component | Empty state |
| `src/renderer/src/components/workspace/WorkspaceSkeletonLoader.tsx` | Component | Loading skeleton |
| `src/renderer/src/components/workspace/OfflineBanner.tsx` | Component | Offline warning banner |
| `src/renderer/src/hooks/useWorkspace.ts` | Hook | WorkspaceContext consumer |

---

## 2. WorkspaceContext — Thiết kế

```typescript
// src/renderer/src/context/WorkspaceContext.tsx

export type WorkspaceContextValue = {
  // State
  project:              OrcaProject | null
  devServer:            DevServer | null
  isOffline:            boolean
  isInitializing:       boolean
  gitStatus:            GitStatus | null
  worktrees:            Worktree[]
  currentWorktree:      Worktree | null
  fileTree:             FileTreeNode | null
  resolvedProfile:      ResolvedProfile | null
  activeAgentSessionId: string | null

  // Actions
  switchProject(projectId: string): Promise<void>
  setCurrentWorktree(wt: Worktree): void
  refreshGitStatus(): Promise<void>
  refreshFileTree(dirPath?: string): Promise<void>

  // Micro event bus
  emit(event: string, payload?: unknown): void
  on(event: string, handler: (payload: unknown) => void): () => void
}

// Events:
// 'agent.complete'    — agent finished a task
// 'files.changed'     — files changed on disk
// 'git.committed'     — after commit (refresh status)
// 'project.switched'  — after switchProject

export const WorkspaceContext = createContext<WorkspaceContextValue>(null!)

export function WorkspaceProvider({ children }: { children: ReactNode }) {
  const [project, setProject] = useState<OrcaProject | null>(null)
  const [isOffline, setIsOffline] = useState(false)
  const [isInitializing, setIsInitializing] = useState(false)
  const handlers = useRef<Map<string, Set<Function>>>(new Map())

  const switchProject = useCallback(async (projectId: string) => {
    setIsInitializing(true)
    try {
      const [proj, worktrees, gitStatus, fileTree, resolvedProfile] = await Promise.all([
        rpc('project.get', { projectId }),
        rpc('worktree.list', { projectId }),
        rpc('git.status', { projectId }).catch(() => null),
        rpc('workspace.refreshFileTree', { projectId, dirPath: '.' }).catch(() => null),
        rpc('profile.getResolved', {}).catch(() => null),
      ])
      setProject(proj as OrcaProject)
      // ... set other state
      setIsOffline(false)
      emit('project.switched', { projectId })
    } catch (err: any) {
      if (err.code === 'DEV_SERVER_UNREACHABLE') setIsOffline(true)
      else throw err
    } finally {
      setIsInitializing(false)
    }
  }, [])

  // Micro event bus
  const emit = useCallback((event: string, payload?: unknown) => {
    handlers.current.get(event)?.forEach(h => h(payload))
  }, [])

  const on = useCallback((event: string, handler: Function) => {
    if (!handlers.current.has(event)) handlers.current.set(event, new Set())
    handlers.current.get(event)!.add(handler)
    return () => handlers.current.get(event)?.delete(handler)  // cleanup
  }, [])

  return (
    <WorkspaceContext.Provider value={{ project, isOffline, isInitializing,
      switchProject, emit, on, /* ... */ }}>
      {children}
    </WorkspaceContext.Provider>
  )
}

// Hook:
export function useWorkspace() {
  return useContext(WorkspaceContext)
}
```

---

## 3. Workspace Slice (Zustand)

```typescript
// src/renderer/src/store/slices/workspace.ts

export type WorkspaceSliceState = {
  projects:      OrcaProject[]
  activeProject: OrcaProject | null
}

export type WorkspaceSliceActions = {
  setProjects(projects: OrcaProject[]): void
  setActiveProject(project: OrcaProject | null): void
  addProject(project: OrcaProject): void
  removeProject(projectId: string): void
}
```

> **Lý do tách biệt WorkspaceContext và WorkspaceSlice:**
> - WorkspaceContext: ephemeral data (fileTree, gitStatus, isOffline) — project-scoped, reset khi switch project
> - WorkspaceSlice: persistent data (project list) — global, không reset

---

## 4. WorkspaceLayout Integration

```typescript
// WorkspaceLayout mount point:
// App.tsx KHÔNG thay đổi — WorkspaceLayout là một tab/view trong App hiện tại
// Tạm thời: accessible via Settings → Workspace hoặc new top-level route

// OPTION A (Recommended): Thêm WorkspaceLayout như một Panel trong App.tsx thông qua
// window.api hook (additive):
window.api.onOpenWorkspace = (projectId: string) => {
  // Update store.activeProject → App.tsx hiển thị WorkspaceLayout
}

// OPTION B: Mount riêng như Admin SPA (/workspace/) — cần thêm entry HTML
```

**Đề xuất:** Option A — additive, không tạo entry point mới.

---

## 5. Files Cần Sửa (Additive)

| File | Thay đổi |
|------|---------|
| `src/renderer/src/store/index.ts` | Register `createWorkspaceSlice` |
| `src/renderer/src/web/main-web-bootstrap.tsx` | Wrap với `WorkspaceProvider` (sau auth check) |

---

## 6. RPC Methods

| Method | Params | Return |
|--------|--------|--------|
| `project.list` | `{}` | `OrcaProject[]` |
| `project.get` | `{ projectId }` | `OrcaProject` |
| `project.create` | `{ name, repoPath, devServerId }` | `OrcaProject` |
| `project.update` | `{ projectId, ...patch }` | `OrcaProject` |
| `project.delete` | `{ projectId }` | `void` |
| `project.listMembers` | `{ projectId }` | `ProjectMember[]` |
| `project.addMember` | `{ projectId, userId, role }` | `ProjectMember` |
| `project.removeMember` | `{ projectId, userId }` | `void` |
| `workspace.refreshFileTree` | `{ projectId, dirPath? }` | `FileTreeNode` |

---

## 7. Test Plan

```
src/renderer/src/context/__tests__/WorkspaceContext.test.tsx  (6 tests)
├── switchProject loads project data and sets state
├── switchProject: DEV_SERVER_UNREACHABLE → isOffline=true
├── refreshGitStatus updates gitStatus
├── emit + on: handler receives event
├── on returns cleanup function that removes handler
└── agent.complete event triggers listeners

src/renderer/src/components/project/__tests__/
├── ProjectSwitcher.test.tsx      (5 tests)
│   ├── renders current project name
│   ├── opens dropdown with project list
│   ├── calls switchProject on item select
│   ├── shows loading spinner during initialization
│   └── filters by search text
└── WorkspaceLayout.test.tsx      (6 tests)
    ├── renders NoProjectSelected when no project
    ├── renders WorkspaceSkeletonLoader when initializing
    ├── renders OfflineBanner when isOffline
    ├── shows GitPanel when git tab active
    ├── shows TaskGraphPanel when tasks tab active
    └── toggles terminal panel visibility

src/renderer/src/components/workspace/__tests__/
├── OfflineBanner.test.tsx        (3 tests)
├── WorkspaceSkeletonLoader.test.tsx (2 tests)
└── ProjectSettings.test.tsx      (5 tests)
```

**Target:** ≥ 27 tests

---

## 8. Dependency Graph

```
WorkspaceProvider (TDD-FE-12)
  ← consumed by: TDD-FE-15 (TaskGraph)
  ← consumed by: TDD-FE-16 (GitPanel)
  ← consumed by: TDD-FE-17 (ExplorerPanel)
```

**Implement TRƯỚC khi implement FE-15, 16, 17.**
