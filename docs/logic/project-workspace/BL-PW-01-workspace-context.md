# BL-PW-01 — Project Workspace Context

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-PW-01 |
| **Tên** | Project Workspace Context |
| **Domain** | Project Workspace |
| **Actor** | Developer, Lead |
| **Priority** | P0 |

---

## Mô tả

Khi user chọn project, hệ thống khởi tạo **WorkspaceContext** — trạng thái trung tâm của toàn bộ Project Workspace. Context này được dùng bởi tất cả panels (Explorer, Git, Agent, Workflow, Tasks, Terminal).

---

## WorkspaceContext

```typescript
interface WorkspaceContext {
  // Project info
  project: OrcaProject          // id, name, repoPath, devServerId
  devServer: SshHost            // host, port, status
  
  // Connection
  relay: DevServerRelayBridge   // active SSH relay to dev server
  isConnected: boolean
  connectionError?: string
  
  // Current state
  currentWorktree: Worktree | null   // selected worktree
  availableWorktrees: Worktree[]
  currentBranch: string
  
  // Resolved profile (for agent env injection)
  resolvedProfile: ResolvedProfile
  
  // Cache (TTL 30s)
  gitStatus?: GitStatus
  fileTree?: FileTreeNode        // lazy-loaded
  
  // Active sessions
  activeAgentSessionId?: string
  activeWorkflowExecutionIds: string[]
}
```

---

## Luồng: Switch Project

```
User → chọn project "vnp-blc-backend"
    │
    ├── Validate permission:
    │   ProjectGrantService.hasAccess(userId, projectId, 'view') → OK
    │
    ├── Teardown previous workspace (if any):
    │   - Close terminal sessions (với warning nếu có session đang chạy)
    │   - Stop git status poll
    │   - Keep relay connection nếu same dev server
    │
    ├── Load project + server:
    │   project = ProjectService.get(projectId)
    │   server  = SshHostService.get(project.devServerId)
    │
    ├── Establish/reuse relay connection:
    │   relay = RelayConnectionPool.get(project.devServerId)
    │   IF !relay OR relay.status !== 'connected':
    │     relay = await DevServerRelayBridge.connect(server)
    │
    ├── Check server health:
    │   healthStatus = FleetHealthMonitor.getCached(server.id)
    │   IF status === 'unreachable':
    │     offlineMode = true  // read-only, use cached data
    │
    ├── Load workspace data (parallel):
    │   [gitStatus, worktrees, fileTreeRoot, activeWorkflows] = await Promise.all([
    │     relay.call('git.status', { cwd: project.repoPath }),
    │     relay.call('git.worktree.list', { repoPath: project.repoPath }),
    │     relay.call('fs.readDir', { path: project.repoPath, depth: 2 }),
    │     WorkflowService.getActiveExecutions(projectId),
    │   ])
    │
    ├── Resolve profile:
    │   resolvedProfile = ProfileResolver.resolve(userId)
    │
    ├── Build WorkspaceContext → set in React Context
    │
    ├── Start background polls:
    │   - git status: mỗi 5s (khi Git tab active hoặc agent running)
    │   - server health: mỗi 30s
    │
    └── Render workspace UI
```

---

## Relay Connection Pool

```typescript
// Singleton per dev server — reuse connections across projects on same server
class RelayConnectionPool {
  private static connections = new Map<string, DevServerRelayBridge>()

  static get(devServerId: string): DevServerRelayBridge | null {
    return this.connections.get(devServerId) ?? null
  }

  static async getOrConnect(devServerId: string, server: SshHost): Promise<DevServerRelayBridge> {
    const existing = this.connections.get(devServerId)
    if (existing?.isAlive()) return existing

    const relay = await DevServerRelayBridge.connect(server)
    this.connections.set(devServerId, relay)
    return relay
  }

  // Auto-cleanup: disconnect if no workspace using server for > 5min
  static release(devServerId: string) { ... }
}
```

---

## Offline Mode

```
IF server unreachable:
    ├── Show banner: "⚠️ dev-alpha.internal is unreachable. Some features disabled."
    ├── File Explorer: show cached file tree (read-only)
    ├── Git tab: show last known status (read-only)
    ├── Agent tab: disable "Run Agent" button
    ├── Terminal: show "Cannot connect to server"
    └── Retry button: attempt reconnect
```

---

## Tiêu chí chấp nhận

- [ ] WorkspaceContext initialized khi switch project
- [ ] RelayConnectionPool: reuse connections, cleanup idle
- [ ] Parallel data load on switch: gitStatus + worktrees + fileTree + workflows
- [ ] Offline mode: banner + read-only fallback
- [ ] Git status poll: 5s interval khi tab active
- [ ] Profile resolve on workspace init
- [ ] Teardown previous workspace on switch (warn if agent running)
