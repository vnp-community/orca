# TDD-19: Project Workspace

**Document:** TDD-19 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** Project Workspace — RelayConnectionPool, WorkspaceContext, FileExplorer
**Feature:** F38
**ADR:** ADR-011
**HLD Ref:** C3.12, C4.10
**Source files (to create):**
- `src/main/dev-server/relay-connection-pool.ts`
- `src/main/workspace/WorkspaceService.ts`
- `src/renderer/src/context/WorkspaceContext.tsx`
- `src/renderer/src/hooks/useWorkspace.ts`
- `src/main/runtime/rpc/methods/workspace.ts`

> **Status: ❌ TODO** — v5.0 proposed; ADR-011: pool + context + micro-emitter

---

## 1. Mục tiêu

Khi user chọn project, toàn bộ workspace panels (Explorer, Git, Agent, Terminal, Tasks, Workflows) chia sẻ:
- **Một relay connection** đến dev server (reuse, không tạo mới)
- **Trạng thái chung**: project, git status, worktrees, active agent session
- **Event bus**: panels notify nhau qua micro-emitter
- **Offline mode**: cached state, disable write ops

---

## 2. RelayConnectionPool (Server-side)

```typescript
// src/main/dev-server/relay-connection-pool.ts

export class RelayConnectionPool {
  private static readonly connections = new Map<string, DevServerRelayBridge>()
  private static readonly refCounts = new Map<string, number>()
  private static readonly idleTimers = new Map<string, ReturnType<typeof setTimeout>>()

  private static readonly IDLE_CLEANUP_MS = 5 * 60 * 1000  // 5 min idle timeout

  static async getOrConnect(
    devServerId: string,
    server: PersistedDevServer
  ): Promise<DevServerRelayBridge> {
    // Cancel any pending cleanup timer
    const timer = this.idleTimers.get(devServerId)
    if (timer) {
      clearTimeout(timer)
      this.idleTimers.delete(devServerId)
    }

    const existing = this.connections.get(devServerId)
    if (existing?.isAlive()) {
      this.refCounts.set(devServerId, (this.refCounts.get(devServerId) ?? 0) + 1)
      return existing
    }

    // Remove stale connection
    this.connections.delete(devServerId)

    const relay = await DevServerRelayBridge.connect(server)
    this.connections.set(devServerId, relay)
    this.refCounts.set(devServerId, 1)
    return relay
  }

  static release(devServerId: string): void {
    const count = Math.max(0, (this.refCounts.get(devServerId) ?? 0) - 1)
    this.refCounts.set(devServerId, count)

    if (count === 0) {
      // Schedule cleanup after idle timeout
      const timer = setTimeout(() => {
        this.connections.get(devServerId)?.disconnect()
        this.connections.delete(devServerId)
        this.refCounts.delete(devServerId)
        this.idleTimers.delete(devServerId)
      }, this.IDLE_CLEANUP_MS)
      this.idleTimers.set(devServerId, timer)
    }
  }

  static async disconnectAll(): Promise<void> {
    for (const [, relay] of this.connections) {
      await relay.disconnect()
    }
    this.connections.clear()
    this.refCounts.clear()
  }

  static getStatus(): Record<string, { refCount: number; alive: boolean }> {
    return Object.fromEntries(
      [...this.connections.entries()].map(([id, relay]) => [
        id,
        { refCount: this.refCounts.get(id) ?? 0, alive: relay.isAlive() }
      ])
    )
  }
}
```

---

## 3. WorkspaceService (Server-side RPC handler)

```typescript
// src/main/workspace/WorkspaceService.ts

export interface WorkspaceInitResult {
  project: OrcaProject
  devServer: PersistedDevServer
  gitStatus: GitStatus
  worktrees: GitWorktree[]
  fileTree: FileTreeNode[]         // depth-2 directory listing
  resolvedProfile: ResolvedProfile
  activeWorkflows: WorkflowExecution[]
  pendingTasks: OrcaTask[]
}

export class WorkspaceService {
  constructor(
    private readonly router: ProjectServerRouter,
    private readonly profileResolver: ProfileResolver,
    private readonly taskService: TaskService
  ) {}

  /**
   * Initialize workspace for a project.
   * Parallel-fetches all needed data for workspace panels.
   */
  async initWorkspace(projectId: string, userId: string): Promise<WorkspaceInitResult> {
    const context = await this.router.getProjectContext(projectId, userId, this.profileResolver)
    const { project, devServer, resolvedProfile } = context
    const relay = await this.router.getRelayForProject(projectId, userId)

    // Parallel initialization — fail-fast on critical, graceful on optional
    const [gitStatus, worktrees, fileTree, activeWorkflows, pendingTasks] = await Promise.all([
      relay.call('git.exec', { cwd: project.repoPath, args: ['status', '--porcelain=v2', '--branch'] })
        .then(r => parseGitStatus((r as any).stdout))
        .catch(() => ({ branch: project.defaultBranch, ahead: 0, behind: 0, files: [] })),

      relay.call('git.exec', { cwd: project.repoPath, args: ['worktree', 'list', '--porcelain'] })
        .then(r => parseWorktreeList((r as any).stdout))
        .catch(() => []),

      relay.call('fs.readDir', { path: project.repoPath, depth: 2 })
        .then(r => (r as any).entries as FileTreeNode[])
        .catch(() => []),

      this.loadActiveWorkflows(projectId),

      this.taskService.list({ projectId, status: ['todo', 'in_progress', 'blocked'] }),
    ])

    return { project, devServer, gitStatus, worktrees, fileTree, resolvedProfile, activeWorkflows, pendingTasks }
  }

  async teardownWorkspace(projectId: string): Promise<void> {
    const project = await this.router.getProject(projectId)
    if (project) {
      RelayConnectionPool.release(project.devServerId)
    }
  }

  async refreshFileTree(projectId: string, userId: string, path?: string): Promise<FileTreeNode[]> {
    const relay = await this.router.getRelayForProject(projectId, userId)
    const project = await this.router.getProject(projectId)!
    const result = await relay.call('fs.readDir', { path: path ?? project.repoPath, depth: 1 })
    return (result as any).entries
  }

  async refreshGitStatus(projectId: string, userId: string, worktreePath: string): Promise<GitStatus> {
    const relay = await this.router.getRelayForProject(projectId, userId)
    const result = await relay.call('git.exec', {
      cwd: worktreePath,
      args: ['status', '--porcelain=v2', '--branch'],
    })
    return parseGitStatus((result as any).stdout)
  }

  private async loadActiveWorkflows(projectId: string): Promise<WorkflowExecution[]> {
    // Load running or recent executions for this project
    return [] // TODO: query orca_workflow_executions
  }
}
```

---

## 4. WorkspaceContext (Client-side React)

```typescript
// src/renderer/src/context/WorkspaceContext.tsx

type WorkspaceEvent =
  | { type: 'agent.complete'; filesChanged: number; sessionId: string }
  | { type: 'git.commit'; hash: string; message: string; branch: string }
  | { type: 'git.push'; branch: string; remote: string }
  | { type: 'worktree.switched'; path: string; branch: string }
  | { type: 'workflow.step.complete'; stepId: string; executionId: string }
  | { type: 'task.statusChanged'; taskId: string; newStatus: string }
  | { type: 'files.changed'; paths: string[] }

type EventHandler = (event: WorkspaceEvent) => void

interface WorkspaceContextValue {
  // State
  project: OrcaProject | null
  devServer: PersistedDevServer | null
  isConnected: boolean
  isOffline: boolean
  gitStatus: GitStatus | null
  worktrees: GitWorktree[]
  currentWorktree: GitWorktree | null
  fileTree: FileTreeNode[]
  resolvedProfile: ResolvedProfile | null
  activeAgentSessionId: string | null
  isInitializing: boolean

  // Actions
  switchProject(projectId: string): Promise<void>
  setCurrentWorktree(wt: GitWorktree): void
  refreshGitStatus(): Promise<void>
  refreshFileTree(path?: string): Promise<void>
  setActiveAgentSession(id: string | null): void

  // Event bus
  emit(event: WorkspaceEvent): void
  on(eventType: WorkspaceEvent['type'], handler: EventHandler): () => void  // returns unsub
}

const WorkspaceContext = createContext<WorkspaceContextValue>(null!)

export function WorkspaceProvider({ children }: { children: React.ReactNode }) {
  const [project, setProject] = useState<OrcaProject | null>(null)
  const [isOffline, setIsOffline] = useState(false)
  const [gitStatus, setGitStatus] = useState<GitStatus | null>(null)
  const [worktrees, setWorktrees] = useState<GitWorktree[]>([])
  const [currentWorktree, setCurrentWorktree] = useState<GitWorktree | null>(null)
  const [fileTree, setFileTree] = useState<FileTreeNode[]>([])
  const [resolvedProfile, setResolvedProfile] = useState<ResolvedProfile | null>(null)
  const [activeAgentSessionId, setActiveAgentSession] = useState<string | null>(null)
  const [isInitializing, setIsInitializing] = useState(false)

  // Micro event bus
  const eventHandlers = useRef(new Map<string, Set<EventHandler>>())

  const emit = useCallback((event: WorkspaceEvent) => {
    const handlers = eventHandlers.current.get(event.type)
    handlers?.forEach(h => h(event))
  }, [])

  const on = useCallback((eventType: WorkspaceEvent['type'], handler: EventHandler) => {
    if (!eventHandlers.current.has(eventType)) {
      eventHandlers.current.set(eventType, new Set())
    }
    eventHandlers.current.get(eventType)!.add(handler)
    return () => eventHandlers.current.get(eventType)?.delete(handler)
  }, [])

  const switchProject = useCallback(async (projectId: string) => {
    setIsInitializing(true)
    try {
      // Teardown previous
      if (project) {
        await rpc.call('workspace.teardown', { projectId: project.id })
      }

      const result = await rpc.call('workspace.init', { projectId }) as WorkspaceInitResult
      setProject(result.project)
      setGitStatus(result.gitStatus)
      setWorktrees(result.worktrees)
      setCurrentWorktree(result.worktrees.find(w => w.isMain) ?? result.worktrees[0] ?? null)
      setFileTree(result.fileTree)
      setResolvedProfile(result.resolvedProfile)
      setIsOffline(false)
    } catch (err: any) {
      if (err.code === 'DEV_SERVER_UNREACHABLE') setIsOffline(true)
      throw err
    } finally {
      setIsInitializing(false)
    }
  }, [project])

  const refreshGitStatus = useCallback(async () => {
    if (!project || !currentWorktree) return
    const status = await rpc.call('workspace.refreshGitStatus', {
      projectId: project.id,
      worktreePath: currentWorktree.path,
    }) as GitStatus
    setGitStatus(status)
  }, [project, currentWorktree])

  const refreshFileTree = useCallback(async (path?: string) => {
    if (!project) return
    const entries = await rpc.call('workspace.refreshFileTree', {
      projectId: project.id, path
    }) as FileTreeNode[]
    setFileTree(entries)
  }, [project])

  // Auto-refresh git status on agent complete
  useEffect(() => {
    return on('agent.complete', () => refreshGitStatus())
  }, [on, refreshGitStatus])

  const value: WorkspaceContextValue = {
    project, devServer: null, isConnected: !isOffline, isOffline,
    gitStatus, worktrees, currentWorktree, fileTree, resolvedProfile,
    activeAgentSessionId, isInitializing,
    switchProject, setCurrentWorktree, refreshGitStatus, refreshFileTree,
    setActiveAgentSession, emit, on
  }

  return <WorkspaceContext.Provider value={value}>{children}</WorkspaceContext.Provider>
}

export const useWorkspace = () => useContext(WorkspaceContext)
```

---

## 5. RPC Methods

```typescript
// namespace: 'workspace'

'workspace.init'              // (project member) → WorkspaceInitResult
'workspace.teardown'          // (any auth) → void — release relay connection
'workspace.refreshGitStatus'  // (member) → GitStatus
'workspace.refreshFileTree'   // (member) → FileTreeNode[]
'workspace.poolStatus'        // (admin) → { devServerId → { refCount, alive } }
```

---

## 6. Cross-Panel Event Patterns

```typescript
// ExplorerPanel:
const { on } = useWorkspace()
useEffect(() => on('agent.complete', () => refreshFileTree()), [on, refreshFileTree])
useEffect(() => on('files.changed', ({ paths }) => refreshFileTree()), [on])

// GitPanel:
useEffect(() => on('agent.complete', () => refreshGitStatus()), [on, refreshGitStatus])
useEffect(() => on('git.commit', () => refreshGitStatus()), [on, refreshGitStatus])

// TasksPanel:
useEffect(() => on('git.commit', ({ message }) => checkTaskRefs(message)), [on])
useEffect(() => on('agent.complete', ({ sessionId }) => checkLinkedTask(sessionId)), [on])
```

---

## 7. Test Coverage

```
src/main/dev-server/__tests__/
├── relay-connection-pool.test.ts
│   ├── getOrConnect: reuse existing alive connection
│   ├── getOrConnect: reconnect if dead
│   ├── release: ref count decrements; cleanup after idle timeout
│   ├── release: multiple users same server → cleanup after all released
│   └── disconnectAll: all connections closed
src/main/workspace/__tests__/
├── WorkspaceService.test.ts
│   ├── initWorkspace: parallel init, all data populated
│   ├── initWorkspace: git status fails → returns empty (offline tolerant)
│   └── refreshGitStatus: relay called with correct worktree path
src/renderer/src/context/__tests__/
├── WorkspaceContext.test.tsx
│   ├── switchProject: data loaded, offline on error
│   ├── emit + on: handler called with correct event
│   ├── on: returns unsubscribe function
│   └── auto-refresh: agent.complete triggers refreshGitStatus
```

**Target:** ≥ 30 tests
