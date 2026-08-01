# Runtime — Orca Server

> **Scope**: OrcaRuntimeService — lõi xử lý mọi operations của Orca
> **Key files**:
> - [`src/main/runtime/orca-runtime.ts`](../../src/main/runtime/orca-runtime.ts) — OrcaRuntimeService (~7000 LOC)
> - [`src/main/runtime/runtime-rpc.ts`](../../src/main/runtime/runtime-rpc.ts) — OrcaRuntimeRpcServer — transport + auth
> - [`src/main/server-bootstrap.ts`](../../src/main/server-bootstrap.ts) — khởi động trình tự
> - [`src/main/index.ts`](../../src/main/index.ts) — entry point server mode

---

## 1. Tổng quan

`OrcaRuntimeService` là singleton service chạy trong Orca Server process, chứa toàn bộ business logic: quản lý worktree, PTY terminals, SSH connections, agent orchestration, git operations. `OrcaRuntimeRpcServer` là transport layer bọc bên ngoài, xử lý auth và routing JSON-RPC đến đúng method của runtime.

```
                    ┌─────────────────────────────────────────┐
                    │         OrcaRuntimeRpcServer             │
                    │  (transport + auth + admission control)  │
                    │                                          │
                    │  WS :6768  ←─ E2EEChannel (per conn)   │
                    │  Unix sock ←─ direct authToken          │
                    │                                          │
                    │  handleRequest():                        │
                    │    parseAndAuthenticate()                │
                    │    route method → runtime.METHOD()       │
                    └──────────────────┬──────────────────────┘
                                       │ RPC calls
                    ┌──────────────────▼──────────────────────┐
                    │         OrcaRuntimeService               │
                    │  (business logic, stateful)              │
                    │                                          │
                    │  ┌──────────┐ ┌────────┐ ┌──────────┐  │
                    │  │ SSH Mgr  │ │ PTY Mgr│ │ Git Ops  │  │
                    │  └──────────┘ └────────┘ └──────────┘  │
                    │  ┌──────────┐ ┌────────┐ ┌──────────┐  │
                    │  │Agent Orch│ │Worktree│ │ Projects │  │
                    │  └──────────┘ └────────┘ └──────────┘  │
                    └──────────────────────────────────────────┘
```

---

## 2. Startup Sequence

### 2.1 Server Mode (Node.js, không có Electron)

```typescript
// src/main/index.ts — startOrcaServer()

// 1. Parse CLI args (--port, --http-port, --domain, --user-data-path, ...)
// 2. ServerBootstrap.initialize()
//    a. mkdir userDataPath
//    b. loadOrCreateE2EEKeypair()     → orca-e2ee-keypair.json
//    c. initializeSqliteStore()       → orca-server.db
//    d. initializeJsonStateRepo()     → orca-data.json
//    e. startDatabaseHealthMonitor()
//    f. startDaemonProcess()          → PTY daemon binary (daemon-entry.js)
//    g. createOrcaRuntimeService()    → OrcaRuntimeService instance
//    h. createOrcaRuntimeRpcServer()  → OrcaRuntimeRpcServer instance
// 3. rpcServer.startListening()
//    a. Bind WS server trên :6768
//    b. Start HTTP server trên :6769 (web SPA + /health/*)
//    c. sweepOrphanedRuntimeSockets() → cleanup stale .sock files
//    d. Print: "✅ Ready! Press Ctrl+C to stop."
// 4. createPairingOffer() → print pairingUrl (nếu --json: output JSON)
```

### 2.2 E2EE Keypair Bootstrap

```typescript
// Chạy 1 lần khi server khởi động
this.e2eeKeypair = loadOrCreateE2EEKeypair(this.userDataPath)
// → Nếu chưa có: generateKeyPair() → write orca-e2ee-keypair.json (chmod 600)
// → Nếu có: load từ file → reuse (server identity persistent qua restart)
```

### 2.3 PTY Daemon

```bash
# daemon-entry.js — subprocess riêng của Orca Server
# Xử lý PTY (pseudo-terminal) để không block main process
# Error khi thiếu file:
Error: Cannot find module '/opt/out/main/daemon-entry.js'
# → Terminal features không available nhưng server vẫn chạy
```

---

## 3. OrcaRuntimeService — Modules chính

### 3.1 Runtime ID

```typescript
private readonly runtimeId = randomUUID()
// UUID v4, mới mỗi lần start
// Dùng để: track ownership của SSH targets, PTY handles, database records
```

### 3.2 Project & Worktree Management

```typescript
// Projects là root containers cho worktrees
type Project = {
  id: string, name: string, hostSetup: ProjectHostSetup, ...
}

// Worktrees là git worktrees (branches) trong project
type Worktree = {
  id: string, projectId: string, path: string, branch: string, ...
}

// Key RPCs:
'project.list'          → Project[]
'project.create'        → Project
'worktree.list'         → Worktree[]
'worktree.create'       → CreateWorktreeResult
'worktree.delete'       → RemoveWorktreeResult
```

### 3.3 PTY (Terminal) Management

```typescript
// Mỗi terminal = 1 PTY session
type PtyHandle = {
  pid: number, ptyId: string, connectionId: string,
  cols: number, rows: number, ...
}

// Key RPCs:
'terminal.create'      → { ptyId }
'terminal.input'       → void      // gửi keystrokes
'terminal.resize'      → void      // thay đổi terminal size
'terminal.subscribe'   → stream    // subscribe output stream
'terminal.kill'        → void
```

### 3.4 SSH Connection Management

```typescript
// Kết nối đến remote servers (dev machines)
// SshConnectionStore quản lý tất cả SshTarget và SshConnection

// Key RPCs:
'ssh.addTarget'         → SshTarget
'ssh.removeTarget'      → void
'ssh.connect'           → void
'ssh.disconnect'        → void
'ssh.listTargets'       → SshTarget[]
'ssh.getConnectionState' → SshConnectionState
```

### 3.5 Agent Orchestration

```typescript
// AI agents: Claude, Codex, Gemini CLI
// Mỗi agent run = 1 orchestration task với PTY session riêng

// Key RPCs:
'orchestration.create'  → OrchestrationTask
'orchestration.run'     → stream (agent output)
'orchestration.stop'    → void
'orchestration.list'    → OrchestrationTask[]
```

### 3.6 Git Operations

```typescript
// Git operations được chạy trong worktree path
// Key RPCs:
'git.status'           → GitStatus
'git.listWorktrees'    → GitWorktreeInfo[]
'git.commit'           → CommitResult
'git.push'             → PushResult
'git.clone'            → CloneResult (long-running)
```

---

## 4. OrcaRuntimeRpcServer — Transport Layer

### 4.1 Dual Transport

```typescript
export class OrcaRuntimeRpcServer {
  // Transport 1: WebSocket (external clients — web, mobile, Electron remote)
  private wsTransport: WebSocketTransport | null = null

  // Transport 2: Unix socket (local Electron IPC, CLI tools)
  // Path: /data/orca/o-{runtimeId}-{hash}.sock

  // Auth token: 48-hex random, ephemeral (per process)
  private readonly authToken = randomBytes(24).toString('hex')

  // E2EE channels: 1 per WS connection
  private e2eeChannels = new Map<WebSocket, E2EEChannel>()

  // Connection IDs: random hex per WS connection (for binary stream routing)
  private wsConnectionIds = new Map<WebSocket, string>()
}
```

### 4.2 Request Routing

```typescript
// handleRequest(rawRequest, reply, context)
// 1. Authenticate: authToken (runtime | scoped)
// 2. Classify method (slow vs fast) → admission control
// 3. Route to runtime.METHOD(args)
// 4. Return result or stream

// Slow methods (gated by counter):
//   git.clone, orchestration.run, ...
// Fast methods (bypass):
//   terminal.input, runtime.status, ssh.listTargets, ...
```

### 4.3 Subscribe/Unsubscribe Pattern

Nhiều RPCs dùng event subscription:
```
Client gửi: { method: 'terminal.subscribe', args: { ptyId } }
Server gửi stream: { event: 'terminal.output', data: '<bytes>' } (nhiều messages)
Client gửi: { method: 'terminal.unsubscribe', args: { ptyId } }
```

List các subscriptions:
- `accounts.subscribe` / `unsubscribe`
- `terminal.subscribe` / `unsubscribe`
- `session.tabs.subscribe` / `subscribeAll` / `unsubscribe` / `unsubscribeAll`
- `runtime.clientEvents.subscribe` / `unsubscribe`
- `notifications.subscribe` / `unsubscribe`

### 4.4 Binary Stream (PTY output)

PTY output được gửi qua binary WebSocket frames để tránh JSON encoding overhead:

```typescript
// Binary frame format: [streamId: 4 bytes] + [payload]
type TerminalStreamFrame = {
  streamId: number
  data:     Uint8Array
}
// Routing: binaryStreamHandlers.get(connectionId)?.get(streamId)?.(frame)
```

---

## 5. Client Connection Lifecycle

```
WS connect
    │
    ▼
E2EEChannel: awaiting_hello
    │ e2ee_hello
    ▼
E2EEChannel: awaiting_auth
    │ e2ee_auth (deviceToken)
    │ validateToken() → OK
    ▼
E2EEChannel: ready
    │ wsTransport.setClientId(ws, deviceToken)
    │ runtime.onClientConnected(deviceToken) [nếu cần]
    │
    │ ── RPC exchange ──
    │ { method, authToken, args } → decrypt → route → encrypt reply
    │
    │ ── subscribe streams ──
    │ terminal.subscribe → PTY output stream (encrypted binary frames)
    │
WS close / error
    │ e2eeChannels.delete(ws)
    │ runtime.onClientDisconnected(clientId)
    │   → cleanup PTY sessions của client này
    │   → cancel pending requests
    │   → release held resources
```

### onClientDisconnected

```typescript
// src/main/runtime/orca-runtime.ts:7917
onClientDisconnected(clientId: string): void {
  // Resize terminals (remove client's size contribution)
  for (const handle of this.ptyHandles.values()) {
    if (handle.connectionId === clientId) {
      this.resizeForClient(handle, clientId, null)
    }
  }
  // Cancel any pending requests for this client
  // Unsubscribe all subscriptions
  // ...
}
```

---

## 6. Database & State

### 6.1 SQLite (`orca-server.db`)

Lưu persistent state:
```sql
-- Orchestration tasks (agent runs)
CREATE TABLE orchestration_tasks (
  id TEXT PRIMARY KEY, runtimeId TEXT,
  tabId TEXT, status TEXT, ...
)

-- Worktrees metadata
-- Projects
-- SSH targets (fleet)
```

### 6.2 JSON State (`orca-data.json`)

```typescript
// PersistedUIState: UI preferences, window state
// GlobalSettings: user settings
// Loaded khi start, saved on change
```

### 6.3 In-Memory State

```typescript
// OrcaRuntimeService — không persist:
private ptyHandles:    Map<string, PtyHandle>
private sshConnections: Map<string, SshConnection>  // managed by SshConnectionStore
private subscriptions:  Map<string, Set<string>>     // clientId → subscribed ptyIds
private pendingRequests: Map<string, Deferred>       // requestId → awaiting response
```

---

## 7. HTTP Server (:6769)

```typescript
// src/main/http-server.ts (hoặc inline trong runtime-rpc.ts)

// Static files: web SPA
GET /          → out/web/index.html
GET /assets/*  → out/web/assets/

// Health endpoints
GET /health           → { status, uptime, database }
GET /health/ready     → { status: 'healthy' | 'unhealthy', ... }
GET /health/metrics   → { connections, memory, ... }
```

---

## 8. Keepalive & Timeout

```typescript
// Keepalive: prevent idle WS từ bị close bởi load balancer / nginx
// Mỗi 10s nếu có pending dispatch, gửi keepalive
const KEEPALIVE_INTERVAL_MS = 10_000

// Rpc timeout: nếu long-running RPC không respond sau X giây:
// → Emit { _keepalive: true } trên WS để giữ connection
```

---

## 9. Runtime ID và Ownership

```typescript
const runtimeId = randomUUID()  // fresh mỗi lần start

// Dùng để:
// 1. Validate database records → chỉ process records của runtime hiện tại
if (record.runtimeId !== this.runtimeId) return // stale record từ crash trước

// 2. SSH target ownership
export function getRuntimeOwnedSshTargetId(runtimeId: string): string {
  return `runtime:${runtimeId}`  // on-demand SSH target cho worktree-remote
}

// 3. PTY socket paths: o-{hash}.sock
sweepOrphanedRuntimeSockets(userDataPath, pid)
// → cleanup .sock files từ crashed processes
```

---

## 10. RPC Method Categories

| Category | Prefix | Số lượng approx | Slow? |
|---------|--------|----------------|-------|
| Runtime status | `runtime.*` | ~5 | No |
| Session tabs | `session.tabs.*` | ~8 | No |
| Terminals (PTY) | `terminal.*` | ~10 | Mixed |
| SSH targets | `ssh.*` | ~8 | Mixed |
| Projects | `project.*` | ~10 | No |
| Worktrees | `worktree.*` | ~15 | Mixed |
| Git operations | `git.*` | ~20 | Mixed |
| Agent/Orchestration | `orchestration.*` | ~8 | Yes |
| Mobile pairing | `mobile.*` | ~5 | No |
| AI accounts | `accounts.*` | ~10 | No |
| Browser | `browser.*` | ~5 | Mixed |
| Notifications | `notifications.*` | ~3 | No |
| Settings | `settings.*` | ~5 | No |
| **[v5.0] Profile** | `profile.*` | ~8 | No |
| **[v5.0] Projects** | `projects.*` | ~9 | No |
| **[v5.0] AI Providers** | `aiProvider.*` | ~7 | Mixed |
| **[v5.0] Workflow** | `workflow.*` | ~10 | Mixed |
| **[v5.0] Tasks** | `tasks.*` | ~12 | Mixed |
| **[v5.0] Relay Pool** | `relay.*` | ~3 | Mixed |
| **[v5.0] Fleet** | `fleet.*` | ~5 | No |

---

## 11. Multi-Database Bootstrap (v5.0)

Web Server Mode hỗ trợ nhiều database backends thông qua `IConnectionPool`:

```typescript
// src/server/index.ts (bootstrapWebApp)

// 1. Parse DSN từ env
const dsn = parseDsn(process.env.ORCA_DB_URL ?? 'file:./orca-server.db')
// Examples:
//   'file:./data.db'                          → SQLite (default)
//   'mysql://user:pass@localhost:3306/orca'   → MySQL
//   'postgresql://user:pass@host:5432/orca'   → PostgreSQL
//   'mysql://user:pass@host/orca?dialect=tidb' → TiDB

// 2. Create connection pool
const pool = DatabaseProvider.createPool(dsn)
// → SQLiteAdapter | MySQLAdapter | PostgreSQLAdapter | TiDBAdapter

// 3. Run migrations (0001 → 0010)
await MigrationRunner.run(pool)
// → Idempotent: skip already-applied migrations
// → Each migration trong transaction: BEGIN → up(db) → INSERT orca_migrations → COMMIT

// 4. Create repositories
const stateRepo = new SqlRepository(pool)  // hoặc JsonFileRepository (Electron mode)
```

### Migration Summary

| Migration | Tables | Features |
|---|---|---|
| 0001 initial | orca_worktrees, orca_sessions | F01 Worktrees, F02 Terminals |
| 0002 automations | orca_automations | F14 Automations |
| 0003 sessions | orca_workspace_sessions | F22/23 Web Server |
| 0004 app_tables | orca_dev_servers, orca_users | F23/24/25/27 |
| 0005 auth | orca_audit_log, bcrypt auth | F23/25 Admin |
| **0006** | orca_company, orca_departments | **F33 Profile Hierarchy** |
| **0007** | orca_projects, orca_project_members | **F34 Project Binding** |
| **0008** | orca_ai_provider_accounts, orca_provider_usage | **F35 AI Providers** |
| **0009** | orca_workflow_templates, orca_workflow_executions, orca_step_executions | **F36 Workflows** |
| **0010** | orca_tasks, orca_task_edges, orca_task_grants, orca_task_comments | **F37 Task Graph** |

---

## 12. WorkspaceContext Integration (v5.0)

Sau khi user chọn project, `WorkspaceContext` giữ state của toàn bộ workspace:

```typescript
// WorkspaceContext state flow:
1. switchProject(projectId)
   → projects.get(projectId)
   → relay.getOrConnect(devServerId)    // RelayConnectionPool
   → profile.getEffective()            // ProfileResolver (cached 60s)
   → Promise.all([git.status, git.worktree.list, fs.readDir, workflow.getActiveExecutions])
   → Set state: project, devServer, gitStatus, worktrees, fileTree, resolvedProfile

2. Polling:
   git.status every 5s → update GitPanel badges
   fleet.getStatus every 30s → update ServerStatusBar

3. Events (WorkspaceEvent):
   'agent.complete'     → refresh git.status + file decorations
   'git.commit'         → refresh log, ahead/behind
   'git.push'           → update PR status
   'workflow.step.done' → update workflow panel
   'project.switched'   → full workspace reinit
```

Xem chi tiết: [project-workspace-switch.md](./project-workspace-switch.md)

---

## 13. v6.0 Dev Server Agent RPC

Dev Server Agent (v6.0) expose JSON-RPC 2.0 methods:

```
PTY/Terminal:
  pty.spawn   → { ptyId, sessionId }  (creates PTY process)
  pty.write   → void                   (send keystrokes)
  pty.resize  → void
  pty.kill    → void
  pty.list    → PtySession[]           (per userId from ctx)

Git (new):
  git.status        → GitStatus (porcelain v2)
  git.diff          → string (unified diff)
  git.add           → void
  git.restore       → void
  git.commit        → CommitResult
  git.push          → stream (progress lines)
  git.pull          → stream
  git.branch.list   → BranchInfo[]
  git.branch.create → void
  git.branch.switch → void
  git.log           → CommitInfo[]
  git.createPR      → { prUrl }
  git.generateCommitMessage → { message }

File System:
  fs.readDir  → FileTreeNode (depth=2)
  fs.readFile → { content }  (max 5MB)
  fs.stat     → FileStat
  fs.glob     → string[]
  fs.grep     → SearchResult[]

Worktree:
  worktree.add    → { path }
  worktree.list   → WorktreeInfo[]
  worktree.remove → void
  worktree.fanout → { taskIds } (fan-out to N worktrees + spawn agents)

AI Credentials:
  ai.provider.writeCredential → void (decrypt + write .enc file)
  ai.provider.readCredential  → { apiKey }  (decrypt + return)
  ai.provider.healthCheck     → { status }
  ai.provider.deleteCredential → void

Health:
  health.metrics → { cpu, ram, disk, latency }
  health.status  → { status: 'healthy' | 'degraded' | 'offline' }
```

---

## 14. Cross-References (v5/v6)

| Resource | Mô tả |
|---|---|
| [multi-user-session.md](./multi-user-session.md) | Per-user sandbox (WsSessionRouter) |
| [profile-resolution.md](./profile-resolution.md) | ProfileResolver integration |
| [project-workspace-switch.md](./project-workspace-switch.md) | WorkspaceContext + RelayConnectionPool |
| [relay-management.md](./relay-management.md) | SSH relay lifecycle |
| [agent-connection-modes.md](./agent-connection-modes.md) | Agent WS + v6 HMAC context |
| **HLD C4.1** | Main module structure |
| **HLD C4.2** | Web Server bootstrap (bootstrapWebApp) |
| **HLD C4.3** | Multi-DB layer (IConnectionPool, MigrationRunner) |
| **HLD C4.11** | Dev Server Agent module map (v6.0) |
