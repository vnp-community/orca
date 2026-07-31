# TASK-042: WorkspaceService + WorkspaceContext React

**Phase:** 7 — Workspace + Remote Git  
**Solution ref:** [SOL-V5-006](../solutions/SOL-V5-006-project-workspace.md) §3, §4  
**Prerequisite:** TASK-041, TASK-035 (TaskService wired)  
**Status:** ✅ DONE

---

## Files cần tạo

### `src/main/workspace/WorkspaceService.ts`

Constructor:
```typescript
constructor(
  private readonly router: ProjectServerRouter,
  private readonly profileResolver: ProfileResolver,
  private readonly taskService: TaskService,
  private readonly workflowOrchestrator: WorkflowOrchestrator,
  private readonly relayPool: RelayConnectionPool
) {}
```

**Public API:**
- `initWorkspace(projectId, userId)` → `WorkspaceInitResult`
  - Parallel fetch: gitStatus, worktrees, fileTree, pendingTasks
  - gitStatus: `relay.call('git.exec', { args: ['status', '--porcelain=v2', '--branch'] })`
  - worktrees: `relay.call('git.exec', { args: ['worktree', 'list', '--porcelain'] })`
  - fileTree: `relay.call('fs.readDir', { path: project.repoPath, depth: 2 })`
  - pendingTasks: `taskService.list({ projectId, status: ['todo', 'in_progress', 'blocked'] })`
  - All relay calls: catch → return empty (offline tolerant)
- `teardownWorkspace(projectId)` → `void` — calls `relayPool.release()`
- `refreshFileTree(projectId, userId, path?)` → `FileTreeNode[]`
- `refreshGitStatus(projectId, userId, worktreePath)` → `GitStatus`

**Helper methods (private):**
- `parseGitStatus(stdout)` → `GitStatus`
- `parseWorktreeList(stdout)` → `GitWorktree[]`

### `src/renderer/src/context/WorkspaceContext.tsx`

React context (TDD-19 §4):
```typescript
interface WorkspaceState {
  currentProjectId: string | null
  project: OrcaProject | null
  devServer: PersistedDevServer | null
  gitStatus: GitStatus | null
  worktrees: GitWorktree[]
  fileTree: FileTreeNode[]
  resolvedProfile: ResolvedProfile | null
  pendingTasks: OrcaTask[]
  isLoading: boolean
  isOffline: boolean
}

interface WorkspaceActions {
  switchProject(projectId: string): Promise<void>
  refreshGitStatus(worktreePath: string): Promise<void>
  refreshFileTree(path?: string): Promise<void>
  emit(event: WorkspaceEvent): void
  on(type: string, handler: (event: WorkspaceEvent) => void): () => void  // returns unsubscribe
}
```

### `src/renderer/src/hooks/useWorkspace.ts`

```typescript
export { useWorkspace } from '../context/WorkspaceContext'
```

---

## Acceptance Criteria

- [x] `WorkspaceService` export với 4 public methods
- [x] `initWorkspace` parallel fetches + offline tolerant
- [x] `teardownWorkspace` calls `relayPool.release()`
- [x] `WorkspaceContext` React context
- [x] `useWorkspace` hook export
- [x] Không TypeScript errors
