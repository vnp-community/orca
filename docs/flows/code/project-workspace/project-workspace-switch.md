# Project Workspace Switch Flow — F34 Project-Dev Server Binding & F38 Project Workspace

> **Scope**: Luồng user chọn Project → kết nối Dev Server → khởi tạo đầy đủ Workspace context
>
> **Key files**:
> - [`src/main/project/project-service.ts`](../../src/main/project/project-service.ts) — ProjectService
> - [`src/main/project/project-server-router.ts`](../../src/main/project/project-server-router.ts) — ProjectServerRouter
> - [`src/main/dev-server/relay-connection-pool.ts`](../../src/main/dev-server/relay-connection-pool.ts) — RelayConnectionPool
> - [`src/renderer/src/context/WorkspaceContext.tsx`](../../src/renderer/src/context/WorkspaceContext.tsx) — WorkspaceContext
> - [`src/renderer/src/components/workspace/ProjectSelector.tsx`](../../src/renderer/src/components/workspace/ProjectSelector.tsx) — ProjectSelector
> - **Features**: [F34 Project Binding](../features/F34-project-dev-server-binding.md), [F38 Project Workspace](../features/F38-project-workspace.md)
> - **Business Logic**: [BL-PRF-03](../logic/profile/BL-PRF-03-project-server-assignment.md), [BL-PW-01](../logic/project-workspace/BL-PW-01-workspace-context.md)

---

## 1. Tổng quan Data Flow

```
User                    Browser UI              Orca Server                 Dev Server
  │                         │                       │                           │
  │ Click "vnp-blc"          │                       │                           │
  │─────────────────────────►│                       │                           │
  │                          │ switchProject('proj')  │                           │
  │                          │──────────────────────►│                           │
  │                          │                       │ projects.get(proj)         │
  │                          │                       │ → { devServerId, repoPath }│
  │                          │                       │                           │
  │                          │                       │ RelayConnectionPool        │
  │                          │                       │ .getOrConnect(devServerId) │
  │                          │                       │──────────────────────────►│
  │                          │                       │◄──────────────────────────│ relay ready
  │                          │                       │                           │
  │                          │                       │ Promise.all [4 parallel]:  │
  │                          │                       │ ─ git.status(repoPath)     │
  │                          │                       │ ─ git.worktree.list()      │
  │                          │                       │ ─ fs.readDir(repoPath,2)   │
  │                          │                       │ ─ workflow.getActive()      │
  │                          │                       │◄──────────────────────────│ all results
  │                          │                       │                           │
  │                          │ WorkspaceContext ready │                           │
  │                          │◄──────────────────────│                           │
  │◄─────────────────────────│                       │                           │
  │ UI renders               │                       │                           │
  │ ExplorerPanel + GitPanel │                       │                           │
```

---

## 2. WorkspaceContext.switchProject() — Chi tiết

```typescript
// src/renderer/src/context/WorkspaceContext.tsx

async function switchProject(projectId: string): Promise<void> {
  setIsLoading(true)

  // Step 1: Fetch project metadata
  const project = await rpc('projects.get', { projectId })
  // → { id, name, devServerId, repoPath, defaultBranch, ... }

  // Step 2: Check dev server availability
  const serverStatus = await rpc('fleet.getStatus', { devServerId: project.devServerId })
  if (serverStatus.status === 'offline') {
    setIsOffline(true)
    setProject(project)
    return  // Partial state: project known, but offline
  }

  // Step 3: Ensure relay connection
  await rpc('relay.getOrConnect', { devServerId: project.devServerId })
  // → DevServerRelayBridge: SSH connect + relay deploy nếu chưa có

  // Step 4: Resolve profile
  const resolvedProfile = await rpc('profile.getEffective')
  // → ResolvedProfile (cached 60s)

  // Step 5: Parallel init (critical path optimization)
  const [gitStatus, worktrees, fileTree, activeWorkflows] = await Promise.all([
    rpc('git.status', { cwd: project.repoPath }),
    rpc('git.worktree.list', { repoPath: project.repoPath }),
    rpc('fs.readDir', { path: project.repoPath, depth: 2 }),
    rpc('workflow.getActiveExecutions', { projectId }),
  ])

  // Step 6: Set WorkspaceContext state
  setProject(project)
  setDevServer({ id: project.devServerId, status: serverStatus })
  setResolvedProfile(resolvedProfile)
  setGitStatus(gitStatus)
  setAvailableWorktrees(worktrees)
  setFileTree(fileTree)
  setActiveWorkflows(activeWorkflows)
  setIsConnected(true)
  setIsLoading(false)

  // Step 7: Start polling (5s interval)
  startGitStatusPoll(project.repoPath, 5000)

  // Step 8: Emit event
  emit({ type: 'project.switched', projectId, devServerId: project.devServerId })
}
```

---

## 3. RelayConnectionPool — Connection Reuse

```typescript
// src/main/dev-server/relay-connection-pool.ts

class RelayConnectionPool {
  // Shared pool: devServerId → DevServerRelayBridge
  private pool = new Map<string, DevServerRelayBridge>()

  async getOrConnect(devServerId: string): Promise<DevServerRelayBridge> {
    const existing = this.pool.get(devServerId)

    if (existing && existing.state === 'ready') {
      // ✅ Reuse: không tạo SSH connection mới
      return existing
    }

    if (existing && existing.state === 'reconnecting') {
      // ⏳ Wait for reconnect to complete
      return existing.waitReady()
    }

    // 🆕 Create new bridge
    const target = await SshConnectionStore.get(devServerId)
    const conn = await SshConnection.connect(target)
    const bridge = new DevServerRelayBridge(devServerId, conn)
    await bridge.establish()  // deploy relay + registerProviders

    this.pool.set(devServerId, bridge)

    // Cleanup khi bridge disposed
    bridge.onDispose(() => this.pool.delete(devServerId))

    return bridge
  }

  // Được gọi khi health monitor detect server offline
  async evict(devServerId: string): Promise<void> {
    const bridge = this.pool.get(devServerId)
    if (bridge) {
      await bridge.dispose()
      this.pool.delete(devServerId)
    }
  }
}
```

---

## 4. Project → Dev Server Binding

### 4.1 Khi Admin tạo Project

```typescript
// RPC: projects.create
{
  name: 'vnp-blc-backend',
  repoPath: '/srv/repos/vnp-blc',
  devServerId: 'svr-01',         // ← THE BINDING
  defaultBranch: 'main',
  description: 'VNP Blockchain backend',
}

// → INSERT orca_projects (id, name, repo_path, dev_server_id, ...)
// → Dev Server ID được lưu trong DB — không thể thay đổi thường xuyên
```

### 4.2 ProjectServerRouter.routeAgentSpawn

```typescript
// src/main/project/project-server-router.ts

async routeAgentSpawn(projectId: string, userId: string, prompt: string): Promise<void> {
  const project = await ProjectService.get(projectId)

  // Verify server available
  const serverStatus = FleetHealthMonitor.getCached(project.devServerId)
  if (!serverStatus || serverStatus.status !== 'healthy') {
    throw new Error(`Dev server ${project.devServerId} is not healthy`)
  }

  // Check user permission (project membership)
  const member = await ProjectService.getMember(projectId, userId)
  if (!member) throw new ForbiddenError('Not a project member')

  // Spawn agent on the project's dev server
  const relay = await RelayConnectionPool.getOrConnect(project.devServerId)
  return ProfileAwareAgentSpawner.spawn({
    userId,
    projectId,
    devServerId: project.devServerId,
    relay,
    worktreePath: project.repoPath,  // hoặc selected worktree
    userPrompt: prompt,
  })
}
```

---

## 5. DB Schema (Migration 0007)

```sql
CREATE TABLE orca_projects (
  id             TEXT PRIMARY KEY,
  name           TEXT NOT NULL UNIQUE,
  description    TEXT,
  repo_url       TEXT,
  repo_path      TEXT NOT NULL,             -- path trên Dev Server
  dev_server_id  TEXT REFERENCES ssh_hosts(id),  -- THE BINDING
  default_branch TEXT DEFAULT 'main',
  tags           TEXT DEFAULT '[]',          -- JSON array
  created_by     TEXT REFERENCES orca_users(id),
  created_at     INTEGER,
  updated_at     INTEGER
);
CREATE INDEX idx_projects_devserver ON orca_projects(dev_server_id);
CREATE INDEX idx_projects_name ON orca_projects(name);

CREATE TABLE orca_project_members (
  project_id  TEXT REFERENCES orca_projects(id) ON DELETE CASCADE,
  user_id     TEXT REFERENCES orca_users(id) ON DELETE CASCADE,
  role        TEXT DEFAULT 'developer',       -- developer | lead | admin
  joined_at   INTEGER,
  PRIMARY KEY (project_id, user_id)
);
CREATE INDEX idx_project_members_user ON orca_project_members(user_id);
```

---

## 6. RPC Methods — projects.*

```typescript
'projects.list'           // (userId?) → OrcaProject[] filtered by membership + RBAC
'projects.get'            // (projectId) → OrcaProject
'projects.create'         // (input) — lead/admin only
'projects.update'         // (projectId, fields) — lead/admin
'projects.delete'         // (projectId) — admin only
'projects.updateBinding'  // (projectId, devServerId) — admin: rebind to different server
'projects.addMember'      // (projectId, userId, role) — lead/admin
'projects.removeMember'   // (projectId, userId) — lead/admin
'projects.listMembers'    // (projectId) → ProjectMember[]

// Workspace init methods (gọi trong switchProject)
'relay.getOrConnect'      // (devServerId) → void (establish if needed)
'fleet.getStatus'         // (devServerId) → ServerStatus
'workflow.getActiveExecutions' // (projectId) → WorkflowExecution[]
```

---

## 7. WorkspaceContext Interface

```typescript
// src/renderer/src/context/WorkspaceContext.tsx

interface WorkspaceContextValue {
  // Project
  project: OrcaProject | null
  devServer: SshHost | null
  isConnected: boolean
  isOffline: boolean
  isLoading: boolean

  // Connection
  relay: DevServerRelayBridge | null   // via RPC proxy

  // Worktree
  currentWorktree: Worktree | null
  availableWorktrees: Worktree[]
  setCurrentWorktree: (wt: Worktree) => void

  // Git
  gitStatus: GitStatus | null
  refreshGitStatus: () => Promise<void>

  // File system
  fileTree: FileTreeNode | null

  // Profile
  resolvedProfile: ResolvedProfile | null

  // Workflow
  activeWorkflows: WorkflowExecution[]

  // Agent
  activeAgentSessionId: string | null

  // Event bus
  emit: (event: WorkspaceEvent) => void
  on: (event: string, handler: Function) => () => void  // returns unsubscribe fn

  // Actions
  switchProject: (projectId: string) => Promise<void>
}
```

---

## 8. Server Offline Handling

```
switchProject('proj-abc')
    │
    ├── fleet.getStatus('svr-01') → { status: 'offline' }
    │
    ├── setIsOffline(true)
    │   setProject(project)  ← biết project nhưng không kết nối được
    │
    └── UI shows:
        ┌──────────────────────────────────────────────┐
        │  ⚠️ Dev Server Offline                        │
        │  vnp-blc-backend (172.20.2.31)               │
        │  Last seen: 5 minutes ago                    │
        │                                              │
        │  [Retry]  [Use different server]             │
        └──────────────────────────────────────────────┘

    Khi [Retry]:
        → fleet.getStatus() → if 'healthy' → switchProject() lại
        → hoặc ProjectService.updateBinding(newDevServerId) (admin action)
```

---

## 9. Cross-References

| Resource | Mô tả |
|---|---|
| [relay-management.md](./relay-management.md) | SSH relay lifecycle |
| [profile-resolution.md](./profile-resolution.md) | Profile inject sau switch |
| [remote-git-ui.md](./remote-git-ui.md) | Git operations sau khi workspace ready |
| [task-agent-execution.md](./task-agent-execution.md) | Agent spawn trong project context |
| **HLD C1 Flow 9** | Project Workspace Switch |
| **HLD C4.8** | Project & Project-Centric Execution |
| **HLD C4.10** | Project Workspace Module Map |
| **F34 Project Binding** | Feature spec |
| **F38 Project Workspace** | Feature spec |
| **BL-PRF-03** | Project-server assignment |
| **BL-PW-01** | Workspace context business logic |
