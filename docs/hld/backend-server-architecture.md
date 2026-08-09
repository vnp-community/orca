# Orca Backend Server — Vai trò, Chức năng & Kết nối

**Nguồn:** Trích xuất từ HLD v1 (C1, C2, C3, C4)  
**Cập nhật:** 2026-08-03 (thêm §7.1 — Dev Server provider registry qua IPC & `devServer:proxyNotification`)

---

## 1. Orca Backend là gì?

Orca có **hai runtime modes** — cùng một codebase, hai adapter khác nhau:

| Mode | Runtime | Entry point | Dùng khi |
|------|---------|-------------|----------|
| **Electron Desktop** | Electron main process | `src/main/index.ts` | Single-user, developer workstation |
| **Orca Web Server** | Node.js server | `src/server/index.ts` | Multi-user, team deployment, CI/CD |

Cả hai dùng chung **Platform Abstraction Layer** (`IPlatformServices`) để swap adapter mà không đổi business logic.

---

## 2. Vai trò của Orca Backend Server

| Vai trò | Mô tả |
|---------|-------|
| **Control Plane** | Điều phối toàn bộ hệ thống — không tự thực thi code, chỉ orchestrate |
| **Auth & Session Gateway** | Xác thực users, quản lý session cookie, RBAC |
| **RPC Router** | Route tất cả JSON-RPC calls từ browser/CLI đến đúng service/relay |
| **Fleet Manager** | Quản lý fleet Dev Servers, health monitor, provisioning |
| **Profile Resolver** | Deep-merge Company ← Dept ← User profile, cache TTL 60s |
| **Project Registry** | Binding project → dev server, manage membership + RBAC |
| **AI Provider Manager** | Manage provider accounts metadata, relay credentials (không lưu plaintext) |
| **Workflow Orchestrator** | Build DAG, topological dispatch steps đến đúng dev server |
| **Task Graph** | Manage task DAG, AI decomposition, spawn agents per task |
| **Admin Panel Host** | Serve SPA `/admin` cho Users CRUD, Sessions, Audit Log |
| **Agent WebSocket Hub** | Accept agent connections (direct-websocket mode) |

---

## 3. Kiến trúc nội bộ — Platform Abstraction Layer

```
          IPlatformServices (interface)
         /                             \
ElectronAdapter                    NodeAdapter
  electron.app                       NodeApp (~/.orca)
  BrowserWindow                      NodeWindow (noop)
  ipcMain.handle()           →       WS JSON-RPC dispatch
  safeStorage (OS keychain)          AES-256-GCM file
  Notification API                   console.log
       │                                   │
  Electron Desktop                  Orca Web Server
  (src/main/index.ts)               (src/server/index.ts)
```

---

## 4. Web Server Bootstrap Flow

```
src/server/index.ts
    │
    ├── new NodeAdapter({ userDataPath: ~/.orca })
    ├── bootstrapWebApp(nodeAdapter)
    │        ├── AuthManager.init(db)
    │        ├── SessionManager.init()
    │        ├── AgentWebSocketServer.init()
    │        └── FleetHealthMonitor.start()
    │
    ├── Express HTTP :6769
    │        ├── POST /auth/local           ← đăng nhập
    │        ├── GET  /admin/api/*          ← Admin SPA (requireAdmin guard)
    │        ├── GET  /health/ready         ← liveness probe
    │        └── GET  /health/metrics       ← Prometheus endpoint
    │
    └── WebSocket Server :6768
             ├── /           ← Browser RPC (WsSessionRouter, per-user process)
             └── /agent      ← Agent connections (AgentWebSocketServer)
```

---

## 5. Domain Services bên trong Backend

| Domain | Module | Chức năng chính |
|--------|--------|----------------|
| **Auth** | `AuthManager` / `auth-router.ts` | bcrypt 12 rounds + HTTP-only cookie + session TTL |
| **Session** | `SessionManager` + `fork()` | Mỗi user = Node.js process riêng, Unix socket `~/.orca/users/<userId>/orca.sock` |
| **Fleet** | `FleetHealthMonitor` | Poll dev servers mỗi 60s — CPU/RAM/disk/latency |
| **Profile** | `ProfileResolver` | 3-layer merge (Company ← Dept ← User), cache 60s |
| **Project** | `ProjectService` + `ProjectServerRouter` | Project → devServerId binding, auto-route requests |
| **AI Provider** | `AIProviderService` + `ProviderCredentialRelay` | Metadata CRUD, credential relay qua SSH |
| **Workflow** | `WorkflowOrchestrator` + `DAGBuilder` | Template inheritance, wave-based execution |
| **Task Graph** | `TaskService` + `TaskAgentExecutor` | DAG validation, AI planning, grant resolution |
| **Credentials** | `WebCredentialStore` | AES-256-GCM per-user credential files |
| **DB** | `IConnectionPool` + adapters | SQLite / MySQL / PostgreSQL / TiDB |
| **RBAC** | `hasPermission()` | Role → resource → action policy table |

---

## 6. Kết nối với các thành phần khác

### 6.1 Browser / Frontend → Orca Backend

```
Browser
  │
  ├── HTTPS :6769
  │     ├── POST /auth/local  → AuthManager (bcrypt verify) → set orca_session cookie
  │     ├── GET  /admin/api/* → requireAdmin → Admin CRUD handlers
  │     └── GET  /health/*    → health endpoints
  │
  └── WebSocket :6768
        │ (WS upgrade + cookie auth)
        ├── WsSessionRouter → per-user Unix socket
        │         └── User process (fork) → JSON-RPC dispatch
        └── /agent path → AgentWebSocketServer (direct-websocket mode)

Protocol: JSON-RPC 2.0 over binary WebSocket frames (13-byte header)
Auth: HTTP-only cookie `orca_session`
```

### 6.2 Orca Backend → Dev Server (3 modes)

```
Mode 1: relay-ssh
  Backend → ssh2 library → SSH exec channel → SshChannelMultiplexer → JSON-RPC

Mode 2: relay-websocket (Orca outbound)
  Backend → HTTP Upgrade ws://agent:PORT/orca-relay
           Header: Authorization: Bearer <agentToken>
           → WsTransport → SshChannelMultiplexer → JSON-RPC

Mode 3: direct-websocket (Agent inbound)
  Dev Server Agent → wss://backend:6768/agent
           Handshake: { agentToken } → { sessionId }
           → AgentWebSocketServer → WsTransport → JSON-RPC
```

### 6.3 Orca Backend → AI Agent CLIs (PTY)

```
AgentOrchestrator / ProfileAwareAgentSpawner
    │
    └── node-pty.spawn(agentBinary, args, { cwd, env })
              ├── claude --trust standard
              ├── codex --model gpt-4o
              ├── gemini --model gemini-2.0
              └── custom agent binary
              │
              PTY stdin/stdout ← OSC escape sequences parsing (AgentAwakeService)
              State: idle → running → waiting → completed
```

> **Lưu ý:** Khi spawn agent trên **local machine**, Backend gọi trực tiếp `node-pty`.  
> Khi spawn agent trên **Dev Server**, Backend relay lệnh `pty.spawn` qua SSH/WS đến Dev Server Agent.

### 6.4 Orca Backend → Git Platforms (REST/GraphQL)

```
GitHub Client  → HTTPS REST/GraphQL (issues, PRs, reviews, rate limit handling)
GitLab Client  → HTTPS REST
Linear Client  → HTTPS REST
Jira Client    → HTTPS Basic Auth (token từ WebCredentialStore)
Bitbucket      → HTTPS App Password
Azure DevOps   → HTTPS PAT token

Auth: per-user token từ WebCredentialStore (AES-256-GCM)
Preflight proxy: Category A (gh/glab CLI) → relay đến Dev Server để check
```

### 6.5 Orca Backend → Database

```
ORCA_STORAGE_BACKEND env
  │
  ├── 'json' → JsonFileRepository (orca-data.json)  ← Desktop mode
  └── 'sql'  → SqlRepository
                    │
               ORCA_DB_URL (DSN)
                    │
       ┌────────────┼────────────┐
  SQLiteAdapter  MySQLAdapter  PostgreSQLAdapter
  (file://...)   (mysql://...)  (postgresql://...)
  [TiDB = mysql2 + dialect flag]

Migration runner: 0001 → 0010 (sequential, idempotent)
Health monitor: ping DB mỗi 30s, auto-reconnect
```

### 6.6 Orca Backend → Mobile App

```
Orca Backend
    │
    ├── QR pairing: { pubKey, host, port, token }
    │         → TweetNaCl key exchange → shared secret
    │
    └── WebSocket (E2E encrypted TweetNaCl box cipher)
              ├── Push: { type: 'agent:completed', summary }
              └── Receive: { type: 'dispatch', prompt }
                         → inject vào PTY stdin
```

### 6.7 Orca Backend → Orca CLI

```
Orca CLI (orca worktree/agent/serve)
    │
    └── Unix Socket → Orca Daemon (NDJSON protocol)
              ├── Headless mode: chạy không có GUI
              └── Reattach: khôi phục PTY sessions
```

---

## 7. Per-User Session Isolation (Web Server Mode)

```
User đăng nhập → POST /auth/local → set cookie
    │
    ▼ WebSocket connect → WsSessionRouter
    │   → validate cookie → userId
    │   → SessionManager.getOrCreate(userId)
    │
    ▼ Session process (fork())
    │   Process: ~/.orca/users/<userId>/orca.sock
    │   Timeout: idle 4h → kill
    │   Max respawns: 3
    │
    └── Mọi RPC call trong session đều isolated trong process riêng
```

### 7.1 Dev Server Provider Registry qua IPC (2026-08)

Provider registry (`IFilesystemProvider`/`IGitProvider`/`IPtyProvider`, dùng chung với SSH Targets — xem [dev-server-architecture.md](./dev-server-architecture.md) §15) là **transport-agnostic đối với process nào giữ connection thật**: connection WebSocket outbound của một Dev Server luôn sống trong **process cha (Gateway)**, không phải trong per-user child process — nhưng mọi user (ở mọi child process) đều phải gọi được provider của Dev Server đó, và giờ còn phải **nhận được** notification agent chủ động push (`pty.data`, `pty.exit`, `fs.changed` — xem dev-server-architecture.md §15.3).

`GatewayDevServerManagerProxy` (chạy trong mỗi child process) forward mọi RPC call của provider qua IPC (`process.send`) về `SessionManager` ở process cha, và ngược lại nhận 2 loại broadcast từ `SessionManager`:

| IPC message type | Nguồn phát | Mục đích |
|-------------------|-----------|----------|
| `devServer:event` | `devServerManager.on('devServer:added' \| 'removed' \| 'statusChanged')` | Đồng bộ trạng thái Dev Server (connect/disconnect) tới mọi child process |
| `devServer:proxyNotification` *(2026-08, mới)* | `devServerManager.on('devServer:notification')` | Relay notification agent chủ động push (`pty.data`/`pty.exit`/`fs.changed`) tới mọi child process; mỗi child tự lọc theo `devServerId` nó đang quan tâm |

`devServer:proxyNotification` chạy **song song** với `devServer:event` đã có từ trước — cùng cơ chế broadcast (`SessionManager` lặp `this.processes` và `proc.process.send(...)`), khác payload và mục đích.

---

## 8. Communication Matrix đầy đủ

| From | To | Protocol | Port/Path | Format |
|------|----|----------|-----------|--------|
| Browser | Orca HTTP | HTTPS | `:6769` | REST JSON |
| Browser | Orca WS | WebSocket | `:6768/` | JSON-RPC (binary frames) |
| Dev Agent | Orca WS | WebSocket | `:6768/agent` | Binary + JSON-RPC |
| Orca | Dev Server | SSH exec | `:22` | relay protocol |
| Orca | Dev Server | WebSocket | `agent:PORT/orca-relay` | Binary + JSON-RPC |
| Orca | AI Agent CLIs | PTY stdin/stdout | — | Text + OSC sequences |
| Orca | GitHub/GitLab | HTTPS | `:443` | REST/GraphQL JSON |
| Orca | Linear/Jira | HTTPS | `:443` | REST JSON |
| Orca | Database | SQL | DSN | IConnectionPool queries |
| Orca | Mobile | WebSocket | dynamic | TweetNaCl encrypted JSON |
| CLI | Daemon | Unix Socket | `~/.orca/daemon.sock` | NDJSON |
| Daemon | AI Agents | PTY | — | Text |
| Gateway (parent) | User child process | IPC (`fork()` channel) | — | `devServer:event` / `devServer:proxyNotification` (2026-08, xem §7.1) |

---

## 9. RPC Method Namespaces (JSON-RPC 2.0)

| Namespace | Số methods | Ví dụ |
|-----------|-----------|-------|
| `profile.*` | 8 | `getEffective`, `updateUser`, `updateCompany` |
| `projects.*` | 9 | `list`, `create`, `updateBinding`, `addMember` |
| `ai-providers.*` | 5 | `list`, `add`, `testConnection`, `rotateKey` |
| `workflows.*` | 7 | `run`, `pause`, `resume`, `streamStepOutput` |
| `tasks.*` | 11 | `create`, `addDependency`, `aiPlan`, `runAgent`, `grant` |
| `credentials.*` | 4 | `set`, `revoke`, `status`, `list` |
| `preflight.*` | 1 | `check` (local + relay merge) |
| `git.*` | ~10 | `status`, `diff`, `commit`, `push`, `branch.*` |
| `fs.*` | ~5 | `readDir`, `readFile`, `glob`, `grep` |
| `pty.*` | ~5 | `spawn`, `write`, `resize`, `kill` |

---

## 10. DB Schema Overview (Migrations 0001–0010)

| Migration | Tables |
|-----------|--------|
| 0001 `init` | `projects`, `worktrees`, `agent_sessions`, `settings` |
| 0002 `sessions` | `terminal_scrollback_snapshots` |
| 0003 `ssh_targets` | `ssh_hosts`, `saved_port_forwards` |
| 0004 `orca_app_tables` | `orca_projects` (legacy, tab/state cho desktop/single-user mode — KHÔNG liên quan Project↔DevServer binding), `orca_repos`, `orca_ssh_targets`, `orca_global_settings` |
| 0005 `auth_schema` | `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies` |
| 0006 `profile` | `orca_company`, `orca_departments` + ALTER `orca_users` |
| 0007 `project` | `orca_v5_projects`, `orca_v5_project_members` — dùng tiền tố `v5` để tránh đụng độ với bảng `orca_projects` legacy của migration 0004 (xem comment `0007_projects.ts:5-9`). Đây là bảng project thật cho tính năng Project-DevServer binding (F34/TDD-15). |
| 0008 `ai_providers` | `orca_ai_provider_accounts`, `orca_provider_usage` |
| 0009 `workflow` | `orca_workflow_templates`, `orca_workflow_executions`, `orca_step_executions` |
| 0010 `task_graph` | `orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments` |

> ⚠️ **Lưu ý naming:** `orca_projects` (0004) và `orca_v5_projects` (0007) là **hai bảng khác nhau, cùng tồn tại song song trong DB**, không phải hai phiên bản của cùng một bảng. `orca_projects` lưu state/tab đơn giản (desktop/single-user mode, không có `dev_server_id`); `orca_v5_projects` là entity project đầy đủ gắn với dev server (server mode, F34). Khi viết SQL hoặc query trực tiếp, LUÔN xác nhận đang thao tác đúng bảng theo mục đích. Xem `BUG-BE-HLD-016` để biết bối cảnh đầy đủ và kế hoạch dọn dẹp dài hạn.

---

## 11. Sơ đồ tổng quan kết nối

```
                    ┌──────────────────────────────────────┐
                    │        ORCA BACKEND SERVER            │
                    │     (Control Plane / Gateway)         │
                    │                                       │
  Browser ─HTTPS──→│ :6769  AuthManager + Admin SPA        │
  Browser ─WS──────│ :6768  WsSessionRouter (per-user fork)│
  Dev Agent ─WS────│        AgentWebSocketServer            │
                    │                                       │
                    │  ProfileResolver   (cache 60s)        │
                    │  ProjectService    (binding)           │
                    │  AIProviderSvc     (metadata)          │
                    │  WorkflowOrch      (DAG dispatch)      │
                    │  TaskService       (grant/plan/exec)   │
                    │  FleetMonitor      (60s poll)          │
                    │  WebCredentialStore (AES-256-GCM)      │
                    │  IConnectionPool   (DB abstraction)    │
                    └───────────────┬──────────────────────┘
                                    │
             ┌──────────────────────┼──────────────────┐
             ↓                      ↓                   ↓
     Dev Server(s)           AI Agent CLIs       External APIs
   (SSH / WS relay)         (local PTY spawn)    (GitHub/GitLab)
   git, fs, pty, shell      Claude/Codex          REST/GraphQL
   AiCredStore              Gemini/custom          HTTPS :443
   StepExecutor             OSC state detection
```
