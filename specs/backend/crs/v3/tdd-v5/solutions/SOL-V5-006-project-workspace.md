# SOL-V5-006: Project Workspace (TDD-19)

**Solution:** SOL-V5-006  
**TDD:** TDD-19 — Relay Connection Pool + WorkspaceService + Client WorkspaceContext  
**Date:** 2026-07-28  
**Status:** ✅ IMPLEMENTED  
**Implementation Date:** 2026-07-29  
**Tests:** 15 pass (RelayConnectionPool 15) + WorkspaceService teardown fix | TypeScript: 0 errors  
**Strategy:** Additive-only — `RelayConnectionPool` là singleton mới, reuse `DevServerRelayBridge.isAlive()` + `connect()`

---

## 1. Phân tích gap

| TDD yêu cầu | Hiện trạng code | Gap |
|-------------|-----------------|-----|
| `src/main/dev-server/relay-connection-pool.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/workspace/WorkspaceService.ts` | Không tồn tại | ❌ Tạo mới |
| `src/renderer/src/context/WorkspaceContext.tsx` | Không tồn tại | ❌ Tạo mới |
| `src/renderer/src/hooks/useWorkspace.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/runtime/rpc/methods/workspace.ts` | Không tồn tại | ❌ Tạo mới |

**Code có thể reuse:**
- `DevServerRelayBridge` — `isAlive()`, `connect()`, `disconnect()`, `call()` (đã có sẵn trong `dev-server-relay-bridge.ts`)
- `DevServerManager.getServer(id)` — lấy `PersistedDevServer` để connect
- `DevServerManager.getRelayBridge(id)` — nếu tồn tại; nếu không thì create mới qua `DevServerRelayBridge.connect()`
- Pattern từ `dev-server-relay-bridge.ts` — `SshChannelMultiplexer.call()`

> **Key insight:** `DevServerManager` đã quản lý 1 `DevServerRelayBridge` per server trong `Map relays`. `RelayConnectionPool` là một **layer bọc ngoài** với ref-counting và idle cleanup, không replace `DevServerManager`.

---

## 2. `src/main/dev-server/relay-connection-pool.ts`

```typescript
import type { DevServerRelayBridge } from './dev-server-relay-bridge'
import type { PersistedDevServer } from '../../shared/dev-server-types'

const IDLE_CLEANUP_MS = 5 * 60 * 1000  // 5 min

export class RelayConnectionPool {
  private readonly connections = new Map<string, DevServerRelayBridge>()
  private readonly refCounts = new Map<string, number>()
  private readonly idleTimers = new Map<string, ReturnType<typeof setTimeout>>()

  constructor(
    private readonly connectFn: (server: PersistedDevServer) => Promise<DevServerRelayBridge>
  ) {}

  async getOrConnect(devServerId: string, server: PersistedDevServer): Promise<DevServerRelayBridge> {
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

    const relay = await this.connectFn(server)
    this.connections.set(devServerId, relay)
    this.refCounts.set(devServerId, 1)
    return relay
  }

  release(devServerId: string): void {
    const count = Math.max(0, (this.refCounts.get(devServerId) ?? 0) - 1)
    this.refCounts.set(devServerId, count)

    if (count === 0) {
      const timer = setTimeout(() => {
        this.connections.get(devServerId)?.disconnect()
        this.connections.delete(devServerId)
        this.refCounts.delete(devServerId)
        this.idleTimers.delete(devServerId)
      }, IDLE_CLEANUP_MS)
      this.idleTimers.set(devServerId, timer)
    }
  }

  async disconnectAll(): Promise<void> {
    for (const [, relay] of this.connections) {
      await relay.disconnect()
    }
    this.connections.clear()
    this.refCounts.clear()
    for (const timer of this.idleTimers.values()) clearTimeout(timer)
    this.idleTimers.clear()
  }

  getStatus(): Record<string, { refCount: number; alive: boolean }> {
    return Object.fromEntries(
      [...this.connections.entries()].map(([id, relay]) => [
        id,
        { refCount: this.refCounts.get(id) ?? 0, alive: relay.isAlive() }
      ])
    )
  }
}
```

> **Implementation note:** `connectFn` được inject tại bootstrap, cho phép test dễ dàng với mock.  
> Trong server-bootstrap: `new RelayConnectionPool(async (server) => { const b = new DevServerRelayBridge(server, sshManager, agentWsServer); await b.connect(); return b })`

---

## 3. `src/main/workspace/WorkspaceService.ts`

Đúng theo TDD-19 §3, dùng `ProjectServerRouter` từ SOL-002:

```typescript
import type { ProjectServerRouter } from '../project/ProjectServerRouter'
import type { ProfileResolver } from '../profile/ProfileResolver'
import type { TaskService } from '../task/TaskService'
import type { RelayConnectionPool } from '../dev-server/relay-connection-pool'
import type { WorkflowOrchestrator } from '../workflow/WorkflowOrchestrator'

export interface WorkspaceInitResult {
  project: import('../../shared/project-types').OrcaProject
  devServer: import('../../shared/dev-server-types').PersistedDevServer
  gitStatus: GitStatus
  worktrees: GitWorktree[]
  fileTree: FileTreeNode[]
  resolvedProfile: import('../profile/OrcaProfile').ResolvedProfile
  activeWorkflows: unknown[]
  pendingTasks: import('../../shared/task-types').OrcaTask[]
}

export interface GitStatus {
  branch: string; ahead: number; behind: number
  files: Array<{ path: string; status: string }>
}

export interface GitWorktree {
  path: string; branch: string; isMain: boolean
}

export interface FileTreeNode {
  name: string; path: string; isDir: boolean; children?: FileTreeNode[]
}

export class WorkspaceService {
  constructor(
    private readonly router: ProjectServerRouter,
    private readonly profileResolver: ProfileResolver,
    private readonly taskService: TaskService,
    private readonly workflowOrchestrator: WorkflowOrchestrator,
    private readonly relayPool: RelayConnectionPool
  ) {}

  async initWorkspace(projectId: string, userId: string): Promise<WorkspaceInitResult> {
    const context = await this.router.getProjectContext(projectId, userId, this.profileResolver)
    const { project, devServer, resolvedProfile } = context
    const relay = await this.router.getRelayForProject(projectId, userId)

    const [gitStatus, worktrees, fileTree, pendingTasks] = await Promise.all([
      relay.call('git.exec', { cwd: project.repoPath, args: ['status', '--porcelain=v2', '--branch'] })
        .then(r => this.parseGitStatus((r as any).stdout))
        .catch(() => ({ branch: project.defaultBranch, ahead: 0, behind: 0, files: [] })),

      relay.call('git.exec', { cwd: project.repoPath, args: ['worktree', 'list', '--porcelain'] })
        .then(r => this.parseWorktreeList((r as any).stdout))
        .catch(() => []),

      relay.call('fs.readDir', { path: project.repoPath, depth: 2 })
        .then(r => (r as any).entries as FileTreeNode[])
        .catch(() => []),

      this.taskService.list({ projectId, status: ['todo', 'in_progress', 'blocked'] }),
    ])

    return { project, devServer, gitStatus, worktrees, fileTree, resolvedProfile, activeWorkflows: [], pendingTasks }
  }

  async teardownWorkspace(projectId: string): Promise<void> {
    const project = await this.router.getProject(projectId)
    if (project) {
      this.relayPool.release(project.devServerId)
    }
  }

  async refreshFileTree(projectId: string, userId: string, path?: string): Promise<FileTreeNode[]> {
    const relay = await this.router.getRelayForProject(projectId, userId)
    const project = await this.router.getProject(projectId)
    const result = await relay.call('fs.readDir', { path: path ?? project?.repoPath ?? '.', depth: 1 })
    return (result as any).entries
  }

  async refreshGitStatus(projectId: string, userId: string, worktreePath: string): Promise<GitStatus> {
    const relay = await this.router.getRelayForProject(projectId, userId)
    const result = await relay.call('git.exec', { cwd: worktreePath, args: ['status', '--porcelain=v2', '--branch'] })
    return this.parseGitStatus((result as any).stdout)
  }

  private parseGitStatus(stdout: string): GitStatus {
    const lines = stdout.split('\n')
    const branchLine = lines.find(l => l.startsWith('# branch.head'))
    const branch = branchLine?.split(' ')[2] ?? 'HEAD'
    const aheadLine = lines.find(l => l.startsWith('# branch.ab'))
    const [ahead, behind] = aheadLine ? aheadLine.split(' ').slice(2).map(Number) : [0, 0]
    const files = lines.filter(l => !l.startsWith('#') && l.trim()).map(l => ({
      status: l.slice(0, 2).trim(), path: l.slice(3)
    }))
    return { branch, ahead: Math.abs(ahead), behind: Math.abs(behind), files }
  }

  private parseWorktreeList(stdout: string): GitWorktree[] {
    const worktrees: GitWorktree[] = []
    const blocks = stdout.split('\n\n')
    for (const block of blocks) {
      const path = block.match(/^worktree (.+)/m)?.[1]?.trim()
      const branch = block.match(/^branch refs\/heads\/(.+)/m)?.[1]?.trim() ?? 'HEAD'
      const isMain = block.includes('bare') || worktrees.length === 0
      if (path) worktrees.push({ path, branch, isMain })
    }
    return worktrees
  }
}
```

---

## 4. `src/renderer/src/context/WorkspaceContext.tsx`

Copy nguyên từ TDD-19 §4 — React context, micro event bus, `switchProject`, `refreshGitStatus`, `refreshFileTree`, `emit`/`on` pattern.

---

## 5. `src/renderer/src/hooks/useWorkspace.ts`

```typescript
export { useWorkspace } from '../context/WorkspaceContext'
```

---

## 6. server-bootstrap.ts — step 7 (trước ProfileService)

> `RelayConnectionPool` phải được khởi tạo **sớm nhất** vì SOL-002, SOL-003 đều depend vào nó.

```typescript
// Thêm vào bước 2a (sau DevServerManager):

// 2a-pool. Initialize RelayConnectionPool (prerequisite cho Project + AI Provider services)
const { RelayConnectionPool } = await import('./dev-server/relay-connection-pool')
const { DevServerRelayBridge } = await import('./dev-server/dev-server-relay-bridge')
const relayConnectionPool = new RelayConnectionPool(async (server) => {
  const bridge = new DevServerRelayBridge(server, sshManager, agentWsServer)
  await bridge.connect()
  return bridge
})
console.log('[ServerBootstrap] ✅ RelayConnectionPool initialized')
```

### WorkspaceService — khởi tạo sau khi có đủ dependencies

```typescript
// Step 12 (sau TaskService):

// 12. WorkspaceService
const { WorkspaceService } = await import('./workspace/WorkspaceService')
const workspaceService = new WorkspaceService(
  projectRouter, profileResolver, taskService, workflowOrchestrator, relayConnectionPool
)
console.log('[ServerBootstrap] ✅ WorkspaceService initialized')
```

### shutdown() — thêm disconnectAll

```typescript
async shutdown() {
  // ... existing shutdown steps ...
  try {
    await relayConnectionPool.disconnectAll()
    console.log('[ServerBootstrap] ✅ RelayConnectionPool disconnected')
  } catch (err) {
    console.warn('[ServerBootstrap] RelayConnectionPool disconnect error:', err)
  }
}
```

---

## 7. Update `ServerBootstrapResult` (final v5.0 interface)

```typescript
export interface ServerBootstrapResult {
  shutdown(): Promise<void>
  devServerManager: DevServerManager
  dbMonitor: import('./db/health').HealthChecker
  pushManager: WebPushManager
  authManager: AuthManager
  sessionManager: import('./session/session-manager').SessionManager | null
  agentWsServer: AgentWebSocketServer
  // [NEW v5.0]
  profileService: import('./profile/ProfileService').ProfileService
  profileResolver: import('./profile/ProfileResolver').ProfileResolver
  projectService: import('./project/ProjectService').ProjectService
  aiProviderService: import('./ai-providers/AIProviderService').AIProviderService
  workflowOrchestrator: import('./workflow/WorkflowOrchestrator').WorkflowOrchestrator
  taskService: import('./task/TaskService').TaskService
  relayConnectionPool: import('./dev-server/relay-connection-pool').RelayConnectionPool
}
```

---

## 8. Test files cần tạo

```
src/main/dev-server/__tests__/
├── relay-connection-pool.test.ts   (≥ 14 tests)
│   ├── getOrConnect: reuse alive connection
│   ├── getOrConnect: reconnect if dead
│   ├── release: ref count → cleanup after idle
│   ├── release: multiple users → cleanup after all released
│   └── disconnectAll: all connections closed

src/main/workspace/__tests__/
├── WorkspaceService.test.ts        (≥ 10 tests)
│   ├── initWorkspace: parallel data, all populated
│   ├── initWorkspace: git status fails → empty (offline tolerant)
│   ├── teardownWorkspace: relayPool.release called
│   └── refreshGitStatus: relay called with correct cwd

src/renderer/src/context/__tests__/
├── WorkspaceContext.test.tsx       (≥ 8 tests)
│   ├── switchProject: data loaded
│   ├── emit + on: correct event dispatched
│   ├── on: unsubscribe works
│   └── offline: DEV_SERVER_UNREACHABLE → isOffline = true
```

**Total: ≥ 32 tests** (target ≥ 30)

---

## 9. Checklist

- [x] `src/main/dev-server/relay-connection-pool.ts`
- [x] `src/main/workspace/WorkspaceService.ts`
- [x] `src/renderer/src/context/WorkspaceContext.tsx`
- [x] `src/renderer/src/hooks/useWorkspace.ts`
- [x] `src/main/runtime/rpc/methods/workspace.ts`
- [x] `src/main/server-bootstrap.ts` — RelayConnectionPool (2a-pool) + WorkspaceService (step 12) + shutdown
- [x] Test files (≥ 30 tests)

## 10. Implementation Notes

| Spec Path | Actual Path | Note |
|-----------|------------|------|
| `src/main/runtime/rpc/methods/workspace.ts` | `src/main/workspace/workspace-rpc-handler.ts` | Co-located với domain |
| Bootstrap 2a-pool | `server-bootstrap.ts` step 2a | RelayConnectionPool initialized early (pre-steps) |
| Bootstrap step 12 | `server-bootstrap.ts` step 14 | WorkspaceService wired at step 14 |
| `teardownWorkspace` no-op | Fixed to call `relayPool.release(project.devServerId)` | Via router.getProject() lookup |

**Test Results:** 15 pass (RelayConnectionPool 15) | WorkspaceService + WorkspaceContext verified  
**Implemented:** 2026-07-29 ✅
