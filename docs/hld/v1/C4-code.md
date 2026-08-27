# C4 — Code Level: Key Modules và Data Flows

**Level:** 4 — Code  
**Mô tả:** Các module quan trọng và data flows chi tiết  
**Cập nhật:** 2026-07-28 (thêm Platform, DB, Fleet, AgentWS, Credential, Profile+Project modules)  
**Cập nhật 2026-08-14:** Đối chiếu với 6 audit `audit/backend/*` và `audit/agent/*` (2026-08-08, GitNexus/CodeGraph, trích dẫn `file:line`) — đã sửa các mục sai lệch nghiêm trọng (migrations, RPC namespaces, wire-protocol chi tiết, credential model) và đánh dấu rõ nội dung chưa triển khai.

> ⚠️ **Đọc trước khi dùng tài liệu này:** Repo thật là monorepo tách package (`backend/src/`, `agent/src/`, `frontend/src/renderer/`, `desktop/src/main/`) — không phải cây `src/` đơn nhất mà phần lớn sơ đồ dưới đây dùng làm ký hiệu tắt (quy ước kế thừa từ HLD gốc, chưa cập nhật theo lần tách package). Khi một mục dưới đây đã được đối chiếu với audit, đường dẫn thật (`backend/src/...`, `agent/src/...`) được ghi kèm. Quy ước trạng thái dùng trong tài liệu: ✅ đã triển khai đúng, ⚠️ triển khai một phần/khác chi tiết, ❌/🚧 chưa triển khai (proposal). Đặc biệt: **§C4.11 "Dev Server Agent Module Map (v6.0 NEW)" mô tả kiến trúc đề xuất trong `docs/adrs/v2/ADR-017` — package `src/agent/` phân lớp A0-A4 đó CHƯA TỪNG được tạo ra trong code.** Agent thật (`agent/src/relay/*`, flat, không phân lớp) được mô tả trong §C4.11a.

---

## C4.1 — Module Map (Main Process)

### src/main/ — Cấu trúc module theo domain

```
src/main/
│
├── index.ts                         # Entry point (Electron main)
│
├── ipc/                             # IPC Handler Registration
│   ├── app.ts                       # App lifecycle handlers
│   ├── pty.ts                       # Terminal/PTY handlers
│   ├── filesystem.ts                # File operation handlers
│   ├── git.ts                       # Git operation handlers
│   ├── github.ts                    # GitHub API handlers
│   ├── ssh.ts                       # SSH operation handlers
│   └── automations.ts              # Automation handlers
│
├── # Agent Subsystem
├── agent-awake-service.ts           # OSC-based agent state detection
├── agent-trust-presets.ts           # Permission tier management
├── agent-hooks/                     # Agent hook interception
│   └── hooks.ts                     # Hook handler registration
│
├── # Worktree Subsystem
├── worktrees.ts                     # Worktree CRUD
├── worktree-logic.ts                # Business logic
├── worktree-removal-safety.ts       # Safety checks before delete
├── worktree-remote.ts               # SSH worktree support
├── local-worktree-filesystem.ts     # Local FS operations
│
├── # SSH Subsystem
├── ssh/
│   ├── ssh-connection.ts            # Core SSH connection
│   ├── ssh-relay-session.ts         # Relay session
│   ├── ssh-relay-deploy.ts          # Binary deployment
│   ├── ssh-port-forward.ts          # Port forwarding
│   ├── ssh-port-scanner.ts          # Port detection
│   ├── fleet-remote-commands.ts     # [NEW] SSH fleet commands
│   ├── fleet-bootstrap-service.ts   # [NEW] Bootstrap automation
│   ├── fleet-health-monitor.ts      # [NEW] Health poll (60s)
│   ├── fleet-health-store.ts        # [NEW] In-memory health cache
│   └── fleet-status-service.ts      # [NEW] Aggregation + webhooks
│
├── # Dev Server / Agent WebSocket Subsystem [NEW]
├── dev-server/
│   ├── dev-server-manager.ts        # DevServer CRUD + connectionType
│   ├── dev-server-relay-bridge.ts   # connect() dispatcher
│   ├── agent-wire-protocol.ts       # 13-byte frame encode/decode
│   ├── ws-transport.ts              # WsTransport ⇔ MultiplexerTransport
│   ├── relay-websocket-client.ts    # relay-websocket: Orca → agent WS
│   └── agent-ws-server.ts           # direct-websocket: Agent → Orca
│
├── # Database Layer [NEW]
├── db/
│   ├── types.ts                     # IDatabase, IStatement, IDatabaseCapabilities
│   ├── provider.ts                  # DatabaseProvider factory
│   ├── config.ts                    # DatabaseConfig Zod schema
│   ├── dsn-parser.ts                # DSN string → DatabaseConfig
│   ├── config-loader.ts             # ENV → DatabaseConfig
│   ├── pool.ts                      # IConnectionPool interface
│   ├── generic-pool.ts              # GenericConnectionPool impl
│   ├── health.ts                    # DatabaseHealth types
│   ├── health-monitor.ts            # Periodic DB health check (30s)
│   ├── sqlite/
│   │   ├── sqlite-adapter.ts        # SQLiteDatabase (node:sqlite)
│   │   └── sqlite-pool.ts           # SQLiteConnectionPool
│   ├── mysql/
│   │   └── mysql-adapter.ts         # MySQLDatabase (mysql2)
│   ├── postgresql/
│   │   └── pg-adapter.ts            # PostgreSQLDatabase (pg)
│   ├── tidb/
│   │   └── tidb-adapter.ts          # TiDB (mysql2 + dialect flag)
│   └── migrations/                  # 🔧 13 migrations thật (0001→0013), không dừng ở 0005 —
│       │                            #   xem `audit/backend/backend-vs-design-review.md` §2.7
│       ├── types.ts                 # Migration interface
│       ├── runner.ts                # MigrationRunner (sequential apply)
│       ├── index.ts                 # Migration registry
│       ├── 0001_*.ts                # settings, projects, repos, ssh_targets
│       ├── 0002_*.ts                # automations
│       ├── 0003_*.ts                # workspace_sessions
│       ├── 0004_*.ts                # orca_projects, orca_repos, orca_ssh_targets, orca_global_settings
│       ├── 0005_auth_schema.ts      # orca_users, orca_sessions, orca_audit_log, orca_access_policies
│       ├── 0006_*.ts                # orca_companies (số nhiều), orca_departments, orca_user_profiles
│       ├── 0007_*.ts                # orca_v5_projects, orca_v5_project_members
│       │                            #   (đổi tên — orca_projects/orca_project_members đã bị 0004 chiếm)
│       ├── 0008_*.ts                # orca_ai_provider_accounts, orca_provider_usage
│       ├── 0009_*.ts                # orca_workflow_templates, orca_workflow_executions,
│       │                            #   orca_workflow_step_executions
│       ├── 0010_*.ts                # orca_tasks, orca_task_edges, orca_task_grants,
│       │                            #   orca_task_comments, orca_team_members
│       └── 0011–0013_*.ts           # terminal_sessions, port_forwards, push_subscriptions,
│                                     #   workflow trace correlation (root_trace_id, dùng bởi F40) —
│                                     #   breakdown chính xác theo từng số migration chưa được audit
│                                     #   xác nhận chi tiết, chỉ xác nhận nhóm bảng (§2.7)
│
├── # Repository Layer [NEW]
├── repositories/
│   ├── types.ts                     # IStateRepository interface
│   ├── json-file-repository.ts      # Existing JSON file storage
│   ├── sql-repository.ts            # SQL-based storage
│   └── factory.ts                   # Repository factory (env-based select)
│
├── # Credential & Integration Layer [NEW]
├── credentials/
│   └── web-credential-store.ts      # WebCredentialStore (AES-256-GCM)
├── runtime/rpc/methods/
│   └── credentials.ts               # 4 RPC: set/revoke/status/list
│
├── # Git Subsystem
├── git/
│   └── (git operations)
│
├── # Platform Integrations
├── github/                          # GitHub API client + preflight proxy
├── gitlab/                          # GitLab API client + preflight proxy
├── linear/                          # Linear API client
├── jira/                            # Jira API client
├── azure-devops/                    # Azure DevOps integration
├── bitbucket/                       # Bitbucket (API token)
│
├── # AI Agent Integrations
├── claude/                          # Claude Code specific
├── claude-accounts/                 # Multi-account management
├── codex/                           # OpenAI Codex
├── opencode/                        # OpenCode
├── gemini/                          # Gemini CLI
├── cursor/                          # Cursor integration
│
├── # Automation Engine
├── automations/
│   ├── service.ts                   # Core automation service
│   ├── external-manager.ts          # External automation dispatch
│   ├── headless-dispatch.ts         # Headless execution
│   └── precheck-runner.ts           # Pre-flight checks
│
├── # Persistence (legacy)
├── persistence.ts                   # SQLite schema + queries (Electron mode)
│
├── # Mobile Communication
├── ipc/mobile.ts                    # Mobile WebSocket handlers
│
└── daemon/                          # Daemon subsystem
    ├── daemon-init.ts               # Initialization
    ├── daemon-server.ts             # Unix socket server
    ├── daemon-pty-adapter.ts        # PTY management
    ├── daemon-pty-router.ts         # I/O routing
    └── session.ts                   # Session management

# Platform Abstraction Layer [NEW — restructure_v1]
src/platform/
├── index.ts                         # Re-export all interfaces
├── types.ts                         # IPlatformServices root type
├── app-interface.ts                 # IApp (lifecycle, paths)
├── window-interface.ts              # IWindow (show, hide, bounds)
├── ipc-interface.ts                 # IIpcBridge (handle, invoke)
├── storage-interface.ts             # ISecureStorage (encrypt/decrypt)
├── system-interface.ts              # ISystemInfo (platform, version)
├── notification-interface.ts        # INotification (show, request)
└── adapters/
    ├── electron/
    │   └── index.ts                 # ElectronAdapter (wraps Electron APIs)
    └── node/
        ├── index.ts                 # NodeAdapter (pure Node.js)
        ├── app.ts                   # NodeApp: userData=~/.orca
        ├── window.ts                # NodeWindow: noop (headless)
        ├── ipc.ts                   # NodeIpcBridge: WS JSON-RPC
        ├── storage.ts               # NodeSecureStorage: AES-256-GCM
        ├── system.ts                # NodeSystemInfo: os.platform()
        ├── notification.ts          # NodeNotification: log-only
        └── __tests__/

# Shared Types [extended]
src/shared/
├── fleet-config-parser.ts           # [NEW] YAML FleetConfig parser
├── rbac-types.ts                    # [NEW] RolePolicy, hasPermission()
├── ssh-types.ts                     # [EXTENDED] SshTargetGroup
└── integration-credential-errors.ts # [EXTENDED] IntegrationCredentialService enum

# Web Server Entry [NEW — restructure_v1 CR-004]
src/server/
└── index.ts                         # bootstrapWebApp(NodeAdapter)
                                     # HTTP :6769 + WS :6768
                                     # Routes: auth, admin, agent, health
```

---

## C4.2 — Platform Abstraction Layer Detail

### IPlatformServices (src/platform/types.ts)

```typescript
export interface IPlatformServices {
  app: IApp            // lifecycle, getPath(), getVersion()
  window: IWindowManager // show, hide, bounds, always-on-top
  ipc: IIpcBridge      // handle(channel, handler) | invoke(channel, args)
  storage: ISecureStorage // encryptString(key, val) | decryptString(key)
  system: ISystemInfo  // platform(), arch(), freeMemory()
  notification: INotification // show(title, body, opts)
}
```

### NodeAdapter vs ElectronAdapter

❌ **`ElectronAdapter` không tồn tại trong code** (`audit/backend/backend-vs-design-review.md` §2.1). Comment tại `backend/src/platform/types.ts:5` mô tả nó là aspirational — `desktop/src/main/index.ts` import thẳng package `electron`, không qua interface trừu tượng nào. Platform Abstraction Layer chỉ đối xứng ở nhánh Node (server); nhánh Electron Desktop dùng SDK gốc trực tiếp. Bảng dưới đây mô tả **ý định thiết kế**, không phải trạng thái code hiện tại của cột "ElectronAdapter":

| Capability | ElectronAdapter (🚧 proposed, chưa tồn tại) | NodeAdapter (✅ có thật) |
|-----------|-----------------|-------------|
| `app.getPath('userData')` | `electron.app.getPath()` | `~/.orca` (env override) |
| `window.show()` | `BrowserWindow.show()` | noop |
| `ipc.handle()` | `ipcMain.handle()` | WS JSON-RPC register |
| `storage.encrypt()` | `safeStorage.encryptString()` | AES-256-GCM file (`NodeSecureStorage`) |
| `notification.show()` | `Notification` (Electron) | `console.log` |

### Web Server Bootstrap Flow (CR-004)

⚠️ Tên hàm/method trong sơ đồ gốc không khớp code thật — đã sửa theo `audit/backend/backend-vs-design-review.md` §2.2. Điểm quan trọng nhất: **Agent WS thật gắn vào `httpPort` (6769), không phải `rpcPort` (6768)** — comment trong chính code (`backend/src/main/dev-server/agent-ws-server.ts:5-6`) xác nhận, nhưng message lỗi runtime ở cùng file (dòng 103) vẫn tự mâu thuẫn và ghi sai `6768`.

```
backend/src/server/index.ts
    │
    ├── new NodeAdapter({ userDataPath: process.env.ORCA_USER_DATA_PATH })
    ├── initializeOrcaServices(nodeAdapter)     # ⚠️ tên thật, không phải bootstrapWebApp()
    │        │                                   #   (bootstrapWebApp là hàm phía frontend/renderer, khác tầng)
    │        ├── new AuthManager(db, auditLogger)        # ⚠️ constructor, không có static .init()
    │        ├── SessionManager (khởi tạo khi ORCA_MULTI_USER=1)
    │        ├── new AgentWebSocketServer() rồi .attach(httpServer)  # ⚠️ không có .init()
    │        └── FleetHealthStore                # ❌ FleetHealthMonitor.start() không tồn tại dạng đó —
    │                                             #   chỉ có in-memory store, không phải monitor loop
    ├── express/http app: HTTP :6769
    │        ├── POST /auth/local
    │        ├── GET  /admin/api/*  (requireAdmin)
    │        ├── GET  /health/ready
    │        └── GET  /health/metrics (Prometheus)
    └── ws server: WS :6768 (single-user) — khi ORCA_MULTI_USER=1, browser WS cũng chạy qua :6769
             ├── /          ← browser RPC (WsSessionRouter, chỉ multi-user mode)
             └── /agent     ← 🔧 agent connections thật nằm trên httpServer :6769, KHÔNG phải :6768
                               (AgentWebSocketServer.attach(httpServer))
```

---

## C4.3 — Database Layer Detail

### IDatabase Interface (src/main/db/types.ts)

```typescript
export interface IDatabase {
  exec(sql: string): void | Promise<void>
  prepare(sql: string): IStatement | Promise<IStatement>
  close(): void | Promise<void>
  readonly capabilities: IDatabaseCapabilities
  // Transactions
  transaction<T>(fn: (db: IDatabase) => T): T | Promise<T>
}

export interface IDatabaseCapabilities {
  dialect: 'sqlite' | 'mysql' | 'postgresql' | 'tidb'
  walMode: boolean        // SQLite only
  returning: boolean      // PostgreSQL
  nativeJson: boolean     // PostgreSQL
  placeholderStyle: 'positional' | 'named' | 'both'
}
```

### DSN Format → Adapter Selection

| DSN Pattern | Adapter | Driver |
|-------------|---------|--------|
| `file:./data.db` ár `sqlite://` | SQLiteAdapter | `node:sqlite` (built-in) |
| `mysql://user:pass@host:3306/db` | MySQLAdapter | `mysql2` |
| `mysql://user:pass@host/db?dialect=tidb` | TiDBAdapter | `mysql2` + dialect flag |
| `postgresql://user:pass@host:5432/db` | PostgreSQLAdapter | `pg` |
| `mariadb://...` | MySQLAdapter | `mysql2` (MySQL-compat) |

### Migration Runner Flow

```
MigrationRunner.run(db)
    │
    ├── CREATE TABLE IF NOT EXISTS orca_migrations (id, name, applied_at)
    ├── SELECT applied migrations
    │
    └── For each pending migration (0001 → 0013):   # 🔧 13 migrations thật, không dừng ở 0005 — §2.7
             ├── BEGIN TRANSACTION
             ├── migration.up(db)  ← dialect-aware DDL
             ├── INSERT orca_migrations (id, name, applied_at)
             └── COMMIT
             (ROLLBACK on error)
```

### Repository Factory (backend/src/main/repositories/factory.ts) — 🔧 sửa theo §2.7

```typescript
// ❌ `ORCA_STORAGE_BACKEND` không tồn tại trong code. Lựa chọn json/sql thực tế dựa vào
// việc loadDatabaseConfig() trả về null hay không (backend/src/main/server-bootstrap.ts:219-266).
// Tên class thật: JsonFileStateRepository / SqlStateRepository (không phải JsonFileRepository/SqlRepository).
function createRepository(): IStateRepository {
  const dbConfig = loadDatabaseConfig()
  if (dbConfig) {
    const pool = DatabaseProvider.createPool(dbConfig)
    return new SqlStateRepository(pool)
  }
  return new JsonFileStateRepository(getDataFile('store.json'))   // ⚠️ file thật là store.json, không phải orca-data.json
}
```

---

## C4.4 — Fleet Management Detail

### Fleet Config YAML Structure (deploy/dev/orca-fleet.yaml)

```yaml
defaults:
  relayGracePeriodSec: 30
  nodeVersion: "22"

projects:
  - name: backend
    tags: [production, api]
  - name: frontend
    tags: [staging]

servers:
  - hostname: dev1.example.com
    user: ubuntu
    identityFile: ~/.ssh/dev_key
    project: backend
    tags: [primary]
    port: 22
```

### Fleet Provisioning Flow (CR-003, CR-004)

🚧 **Phần lớn sơ đồ này chưa triển khai** (`audit/backend/backend-vs-design-review.md` §5.10/F31): CLI `orca fleet provision --project ... --concurrency N --dry-run` **hoàn toàn không tồn tại** (không có `backend/src/cli/`, grep "fleet provision" 0 kết quả). `FleetBootstrapService` (class) không tồn tại — hàm thật là `bootstrapServer()` (đơn lẻ, không tách preflight/bootstrap), và **thiếu 2/7 bước**: disk-space check (≥5GB) và verify SHA256 của relay binary — cả hai chưa có code.

```
orca fleet provision --project backend --concurrency 5     # ❌ CLI này chưa tồn tại
    │
    ├── Load fleet.yaml → FleetConfigParser.parse()        # ✅ có thật, đúng logic
    ├── Filter servers by project                          # ✅ groupSshTargetsByProject()
    │
    └── For each server (parallel, concurrency=5):          # ❌ concurrency/dry-run chưa implement
             ├── SSH connect (SshConnection)
             ├── bootstrapServer(server)                    # ⚠️ hàm đơn, không phải FleetBootstrapService.bootstrap()
             │       ├── Check Node.js ≥ 22
             │       ├── Check Git ≥ 2.25
             │       ├── Check disk ≥ 5GB                   # ❌ chưa có code
             │       ├── SFTP upload relay binary
             │       ├── Verify SHA256                      # ❌ chưa có code
             │       └── Start relay daemon                 # (nằm ở cơ chế riêng, không tích hợp vào bootstrapServer)
             ├── FleetHealthMonitor.check(server) → initial health
             └── Update server status: 'online' | 'degraded' | 'unhealthy'
```

### Fleet Health Status Model

❌ **Model CPU/RAM dưới đây chưa triển khai** (`audit/backend/backend-vs-design-review.md` §2.6/§5.6/F27, xác nhận 2 lần độc lập): `FleetHealthMonitor.runHealthCheck()` chỉ đọc `SshConnectionStatus` đã có sẵn — **không hề exec lệnh đo CPU/RAM/disk, không ping đo latency**. Field `pingLatencyMs` tồn tại trong type `HealthRecord` nhưng không bao giờ được ghi giá trị (dead field). Khái niệm CPU/RAM/disk hoàn toàn không tồn tại trong bất kỳ type nào hay ở `/health/metrics` Prometheus.

| Status | Condition (🚧 thiết kế đề xuất — thực tế chỉ dựa trên SSH connection status) |
|--------|-----------|
| `healthy` | SSH ✔, relay ✔, CPU<80%, RAM<85% |
| `degraded` | Relay ✔ but CPU>80% or RAM>85% |
| `unhealthy` | SSH ✔ but relay ✘ |
| `unreachable` | SSH connect timeout/fail |

### RBAC Policy Resolution (CR-006, Phase 1)

❌ **`hasPermission()` như mô tả dưới đây không tồn tại** (`audit/backend/backend-vs-design-review.md` §2.6/§5.11/F32 — bug bảo mật nghiêm trọng nhất toàn bộ audit). Thực tế có **2 cơ chế permission tách biệt, không liên kết**: `resolveUserPermissions()` (`backend/src/shared/rbac-types.ts:73-119`, merge allowlist server/project + agentTrust — khác hẳn "Role→resource→action") và `TaskGrantService.resolvePermission()` (RBAC riêng cho task graph, BFS ancestor). Nghiêm trọng hơn: `requireAdmin(ctx)` trong `backend/src/main/profile/profile-rpc-handler.ts:282-293` **chỉ kiểm tra đã login, không kiểm tra `role==='admin'`** — bất kỳ user nào cũng gọi được các RPC set chính sách bảo mật công ty.

```typescript
// 🚧 Đề xuất — CHƯA triển khai; code thật không có bảng POLICY_TABLE thống nhất role×resource×action
// src/shared/rbac-types.ts
interface RolePolicy { role: string; resource: string; action: string }

function hasPermission(
  userId: string,
  resource: 'ssh_host' | 'fleet' | 'worktree',
  action: 'read' | 'write' | 'admin'
): boolean {
  const userRole = getUserRole(userId) // 'developer' | 'lead' | 'admin'
  return POLICY_TABLE[userRole][resource]?.includes(action) ?? false
}
// Phase 2: SSO integration (OIDC/SAML) for enterprise login
```

---

## C4.5 — Agent WebSocket Detail

### Wire Protocol Frame Structure (CR-AG-001)

```
Ocra Frame (binary WebSocket message):

 Byte  0     : TYPE (uint8)
               0x01 = Regular (JSON-RPC payload)
               0x09 = KeepAlive (empty payload, every 5s, timeout 20s)   # ⚠️ sửa "30s" → 5s/20s thật
               #    (`agent/src/shared/agent-wire-protocol.ts:21-22`, `agent/src/main/ssh/relay-protocol.ts:24-25`;
               #     audit/agent/connection-wire-protocol-vs-design-review.md §2.1 — không có logic "3 lần miss")

 Bytes 1–4  : SEQ  (uint32 big-endian)  — monotonically increasing
 Bytes 5–8  : ACK  (uint32 big-endian)  — highest received SEQ from peer
 Bytes 9–12 : LEN  (uint32 big-endian)  — payload length in bytes
 Bytes 13+  : PAYLOAD (UTF-8 JSON-RPC 2.0)

Total header: 13 bytes
```

### relay-websocket Mode Flow (CR-AG-003)

```
DevServer config: connectionType = 'relay-websocket'
                  wsUrl = 'ws://agent-host:6799/orca-relay'
                  agentToken = 'sha256-hash-of-raw-token'

DevServerRelayBridge.connect()
    │
    └── RelayWebSocketClient.connect(wsUrl)
             │
             ├── HTTP Upgrade: GET /orca-relay
             ├── Header: Authorization: Bearer <rawAgentToken>
             ├── Agent validates token → accept WS upgrade
             │
             └── WsTransport wraps WebSocket
                      │
                      └── SshChannelMultiplexer.setTransport(wsTransport)
                               │
                               └── JSON-RPC methods: preflight.check,
                                              pty.spawn, fs.read, git.exec...
```

### direct-websocket Mode Flow (CR-AG-004)

⚠️ **Port và close codes dưới đây sai** so với code thật (`audit/agent/connection-wire-protocol-vs-design-review.md` §2.3-2.4, xác nhận chéo với backend audit §2.2/§2.4): `AgentWebSocketServer` gắn vào **httpPort (mặc định 6769)**, không phải `rpcPort` (6768) — comment trong chính `backend/src/main/dev-server/agent-ws-server.ts:5-6` xác nhận đúng port thật, nhưng message lỗi runtime cùng file (dòng 103) vẫn tự mâu thuẫn và ghi `6768`. Close code khi handshake thất bại **không phải** custom code `4001/4002/4003` — code thật dùng WS chuẩn `1008` (token sai) hoặc `1005` (timeout mặc định), kèm JSON-RPC error frame `code: -33101 AuthFailed` trước khi đóng.

```
DevServer config: connectionType = 'direct-websocket'
Orca: AgentWebSocketServer listening on ws://orca:6769/agent   # 🔧 6769, không phải 6768

Agent (Python/Go/TypeScript) → WebSocket connect:
    │
    ├── GET /agent (HTTP Upgrade)
    ├── Agent → { method: 'agent.handshake', id: 1,           # ⚠️ agent tự gửi trước, không có
    │              params: { capabilities, agentToken,          #   bước Orca → 'handshake-request' riêng
    │              name: 'my-agent', version: '1.0.0' } }
    ├── Backend validate token (SHA-256 hash so khớp trong pendingSlots — không phải AgentTokenManager,
    │       class đó chỉ tồn tại phía agent cho self-renewal, xem §C4.11a)
    │       └── hash khớp ? OK : ws.close(1008, ...) + JSON-RPC error -33101 AuthFailed
    ├── Orca → JSON-RPC response { result: { ok: true } }        # ⚠️ không có message type riêng 'handshake-ok'
    │
    └── WsTransport ⇔ SshChannelMultiplexer ⇔ JSON-RPC (bidirectional)

Close codes thật: 1008 (Policy Violation — token sai), 1005/1011 (timeout/server error) — KHÔNG dùng 4001-4004.
Token: `agt-<devServerId>-<timestamp>` do agent tự yêu cầu qua `POST /api/agent-token`
       (auth bằng ORCA_AGENT_API_SECRET) — không phải admin UI + DB, xem §C4.11a.
```

---

## C4.6 — Integration & Credential Layer Detail

### WebCredentialStore Encryption (CR-INT-004)

⚠️ Biến môi trường sai tên: doc/CR-INT-004 ghi `ORCA_CREDENTIAL_KEY`, biến thật trong code là **`ORCA_SERVER_SECRET`** (`backend/src/main/credentials/index.ts:11,16`; `ORCA_CREDENTIAL_KEY` không xuất hiện ở bất kỳ đâu — `audit/backend/backend-vs-design-review.md` §5.9/F30). `WebCredentialStore` phiên bản V2 thật cũng khác chi tiết dưới đây: `salt` ngẫu nhiên 32-byte mỗi lần ghi (không derive từ `userId`), `iv=16 byte` (không phải 12) — có cơ chế migrate V1→V2 doc không nhắc.

```
Encryption at rest:

  masterKey = scryptSync(
    userId + ':' + ORCA_SERVER_SECRET,    // 🔧 ORCA_SERVER_SECRET, không phải ORCA_CREDENTIAL_KEY
    userId,                                 // salt
    { N: 16384, r: 8, p: 1, keylen: 32 } // params
  )

  iv = randomBytes(12)  // 12 bytes for GCM
  { ciphertext, authTag } = AES-256-GCM.encrypt(plaintext, masterKey, iv)

  stored = { iv: hex(iv), ct: hex(ciphertext), tag: hex(authTag) }
  file = ~/.orca/users/<userId>/<service>.enc
  chmod 0600
```

### Integration Categories (CR-INT-000)

✅ Phân loại Category A/B/C đúng danh sách provider (`audit/backend/backend-vs-design-review.md` §5.9/F30 — "Jira/Bitbucket/Azure/Gitea/Linear gọi trực tiếp từ Backend LÀ đúng thiết kế, chỉ Category A mới cần relay"). Nhưng **Category A tự mâu thuẫn với chính thiết kế của nó trong code thật**: xem cảnh báo bên dưới bảng.

| Category | Integration | Storage | Auth Method |
|----------|------------|---------|-------------|
| **A — CLI-based** | GitHub (`gh`) | Dev Server `~/.config/gh/<userId>/` (🚧 chỉ đúng cho implementation phía Agent — xem cảnh báo) | `gh auth login` (PTY) |
| **A — CLI-based** | GitLab (`glab`) | Dev Server `~/.config/glab/<userId>/` (🚧 tương tự) | `glab auth login` (PTY) |
| **B — HTTP API** | Bitbucket | Orca Server `WebCredentialStore` | App password |
| **B — HTTP API** | Azure DevOps | Orca Server `WebCredentialStore` | PAT token |
| **B — HTTP API** | Gitea | Orca Server `WebCredentialStore` | API token |
| **C — File token** | Linear | Orca Server `WebCredentialStore` | API key |
| **C — File token** | Jira | Orca Server `WebCredentialStore` | Basic auth token |

> ❌ **Phát hiện quan trọng nhất về Category A (`audit/backend/backend-vs-design-review.md` §2.12b/§5.9, `audit/agent/git-ssh-external-api-vs-design-review.md`):** Backend **tự thực thi** `gh`/`glab` trực tiếp trong process của mình qua `ghExecFileAsync` → `child_process.execFile` (`backend/src/main/github/*`, `gitlab/*`), **không relay** tới Dev Server Agent, và **không set `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR`** ở bất kỳ đâu trong `backend/src/main` — vi phạm trực tiếp nguyên tắc "Auth never through Gateway" mà bảng trên mô tả. Implementation đúng thiết kế **đã tồn tại ở phía Agent** (`agent/src/relay/agent-git-handler.ts` + `agent/src/relay/external-api-connector.ts`, per-user `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` đúng path bảng trên) nhưng **không có caller nào từ Backend** — 100% dead code từ góc nhìn Backend Web Server. Thêm nữa, RPC dispatch thật trên Agent đăng ký `github.pr.create`/`github.pr.merge`/`gitlab.mr.create`/... (namespace `github.*`/`gitlab.*`), không phải `git.pr.create`/`git.mr.*` phẳng như một số phần tài liệu khác mô tả — và **`git.pr.create`/`github.pr.create` là 2 implementation trùng lặp cho cùng chức năng "tạo PR"** (`agent-git-handler.ts` vs `external-api-connector.ts`), chỉ bản thứ hai có idempotency-check.

### Preflight Check Merge Flow (CR-INT-005, CR-GH-005)

❌ **`mergePreflightStatuses` không tồn tại trong code** dù acceptance criteria đánh dấu hoàn thành (`audit/backend/backend-vs-design-review.md` §5.9/F30). `relay.call('pty.spawn', {command:'gh', args, env:{}, ...})` thật truyền `env` **rỗng** — không có `userId`/`GH_CONFIG_DIR` — nên cơ chế per-user isolation ở 2 luồng relay hẹp còn lại (`*.startAuthLogin`, `preflight.check` khi có `devServerId`) cũng không hoạt động.

```
RPC: preflight.check { devServerId }
    │
    ├── runLocalChecks():
    │       ├── git version (local)
    │       └── API token format (Category B+C)
    │
    ├── runRelayChecks(devServerId) via SSH:
    │       ├── GH_CONFIG_DIR=~/.config/gh/<userId>/ gh auth status   # 🚧 env không thực sự được truyền — xem trên
    │       ├── GLAB_CONFIG_DIR=~/.config/glab/<userId>/ glab auth status
    │       ├── node --version
    │       └── disk space check
    │
    └── mergePreflightStatuses(localResults, relayResults)          # ❌ hàm này không tồn tại trong code
             │
             └── relay results OVERRIDE local (relay is authoritative for CLI)
                  fallback to local-only if SSH fails (+ 'relay-connectivity' warning)
```

---

### Flow A: Tạo Worktree và Khởi động Agent

```
User clicks "New Worktree + Start Agent"
       │
       ▼
[Renderer] window.orcaAPI.worktree.create({baseRef, agentType})
       │  contextBridge
       ▼
[Preload] ipcRenderer.invoke('worktree:create', opts)
       │  Electron IPC
       ▼
[Main/ipc/worktrees.ts] handler registered in index.ts
       │
       ├─→ [WorktreesIPC.create()]
       │         │
       │         ├─→ Validate: check git repo, disk space
       │         ├─→ git worktree add <path> <base-ref>
       │         ├─→ persistence.createWorktree({id, path, branch})
       │         └─→ emit 'worktree:created' event
       │
       ├─→ [AgentOrchestrator.start(worktreeId, agentType)]   # ❌ không có symbol tên AgentOrchestrator trong code
       │         │                                             #   (audit/backend §2.11) — bản gần nhất là
       │         │                                             #   ProfileAwareAgentSpawner, nhưng spawn() của nó
       │         │                                             #   LUÔN đi qua relay.call('agent.exec', ...), không
       │         │                                             #   có nhánh local node-pty.spawn cho "local machine"
       │         ├─→ Load AgentConfig (binary, startupCommand, trustPreset)
       │         ├─→ Apply trust preset env vars
       │         ├─→ daemon.spawnPTY({cwd: worktreePath, cmd: agentBin})
       │         ├─→ Register AgentAwakeService listener
       │         └─→ emit 'agent:started' event
       │
       ▼
[Main → Renderer] ipcMain.emit('worktree:created', worktreeData)
       │
       ▼
[Renderer/store] Update worktrees state
       │
       ▼
[UI] Sidebar shows new worktree card
     Terminal panel opens with agent PTY
```

---

### Flow B: SSH Remote Connection → Relay Deploy → Agent Start

```
User adds SSH host and starts remote agent
       │
       ▼
[Main/ssh/ssh-config-parser.ts] Parse ~/.ssh/config
       │ resolve: HostName, Port, User, IdentityFile, ProxyJump
       ▼
[Main/ssh/ssh-auth-resolution.ts] Try auth methods:
       │  1. SSH key (IdentityFile)
       │  2. SSH agent forwarding
       │  3. Password prompt → renderer dialog
       ▼
[Main/ssh/ssh-connection.ts] Establish SSH connection
       │  - Open control channel
       │  - Setup keepalive
       ▼
[Main/ssh/ssh-relay-deploy.ts] Deploy relay:
       │  1. Check relay version: ssh exec "orca-relay --version"
       │  2. Detect remote platform: uname -m + uname -s
       │  3. SFTP upload binary → ~/.local/bin/orca-relay
       │  4. chmod +x, verify SHA256
       ▼
[Main/ssh/ssh-relay-session.ts] Start relay session:
       │  1. ssh exec: "orca-relay --listen 7777 --token <session-token>"
       │  2. Open WebSocket over SSH tunnel: localhost:7777
       │  3. Protocol handshake (relay-handshake.ts)
       │  4. Session established
       ▼
[Relay/pty-handler.ts] Spawn remote PTY:
       │  ⚠️ `pty.spawn` thật (`agent/src/relay/pty-handler.ts`) luôn spawn một SHELL,
       │  không spawn agentBinary trực tiếp — agent CLI được gõ/paste vào shell đã chạy
       │  qua `commandDelivery`. RPC method spawn thẳng agent binary bằng node-pty là
       │  `agent.spawn` (khác RPC, `agent/src/relay/agent-spawner.ts:387-392`).
       │  (audit/agent/pty-ai-cli-vs-design-review.md §2.6)
       │  node-pty.spawn(shell, shellArgs, {cwd: remoteWorktreePath})   # 🔧 sửa: shell, không phải agentBinary
       ▼
[Main] PTY I/O streams bridged:
       │  Desktop keyboard → SSH WebSocket → Relay PTY → Agent stdin
       │  Agent stdout → Relay → SSH WebSocket → Desktop terminal
       ▼
[Main/ssh/ssh-port-scanner.ts] Background port scanning:
       │  Relay scans localhost every 2s
       │  New port detected → WebSocket event → Desktop → SSH tunnel
       └→ Notification: "Port 3001 → remote:3000 [Open]"
```

---

### Flow C: Mobile Pairing và Remote Dispatch

```
[Desktop] Settings → Mobile → "Show QR Code"
       │
       ▼
[Main/ipc/mobile.ts] Generate pairing payload:
       │  1. Generate ephemeral keypair (TweetNaCl)
       │  2. Generate one-time token (random 32 bytes)
       │  3. Get local IP + WebSocket port
       │  4. Encode as QR data: { pubKey, host, port, token }    # ⚠️ tên field khác code thật —
       │     Schema thật (`PairingOfferSchema`, `backend/src/shared/pairing.ts:6-18`):
       │     { v, endpoint, deviceToken, publicKeyB64, scope? } — host/port gộp vào `endpoint`,
       │     token→deviceToken, pubKey→publicKeyB64 (audit/backend/backend-vs-design-review.md §2.13)
       ▼
[Renderer] Display QR code image
       │
       ▼ [Mobile scans QR]
       │
[Mobile] Decode QR → send pairing request:
       │  POST ws://<host>:<port> { mobilePubKey, token }
       ▼
[Main/mobile-server] Verify token (one-time, expire 5min)
       │  1. Exchange public keys
       │  2. Derive shared secret: TweetNaCl.box(desktopPriv, mobilePub)
       │  3. Invalidate token
       ▼
[Main/mobile-server] WebSocket connection established
       │  All subsequent messages encrypted with shared secret
       ▼
[Agent status change detected by AgentAwakeService]
       │
       ▼
[Main/notification-service] Format notification
       │  { type: 'agent:completed', worktree: '...', summary: '...' }   # ⚠️ literal string 'agent:completed'/'dispatch'
       │  không xác nhận được trong code đã khảo sát — có NotificationEvent + cơ chế inject input vào
       │  PTY nói chung qua Session.write(), nhưng chưa xác nhận đúng tên message như trên
       │  (audit/backend/backend-vs-design-review.md §2.13)
       ▼
[Main/mobile-server] Encrypt + send via WebSocket
       │
       ▼
[Mobile] Decrypt → show push notification
       │
       ▼ [Sam taps notification → opens app → types prompt]
       │
[Mobile] Encrypt dispatch: { prompt: "Continue with tests" }
       │
       ▼
[Main/mobile-server] Decrypt → validate → find target agent
       │
       ▼
[Daemon/pty-adapter] Inject prompt into agent PTY stdin
       │
       ▼
[Agent] Receives prompt → begins processing
       │
       ▼
[Status update] Running → sends back to mobile
```

---

### Flow D: Automation Cron Execution

```
[Main/automations/service.ts] Cron scheduler tick (every 30s)
       │
       ▼
Query: SELECT automations WHERE next_run <= NOW() AND enabled = 1
       │
       ▼
FOR each triggered automation:
       │
       ├─→ Create run record: { status: 'running', started_at }
       │
       ├─→ Execute actions sequentially:
       │      │
       │      ├── create_worktree → WorktreeManager.create()
       │      ├── run_agent      → AgentOrchestrator.start()
       │      ├── wait           → AgentAwakeService.waitForCompletion()
       │      ├── commit         → GitOps.commit()
       │      ├── create_pr      → GitHubClient.createPR()
       │      ├── notify         → NotificationService.send()
       │      └── cleanup        → WorktreeManager.cleanup()
       │
       ├─→ Update run record: { status: 'completed'/'failed' }
       │
       └─→ Calculate next_run from cron expression
```

---

## C4.3 — SQLite Schema (Simplified)

> ⚠️ **Lưu ý phạm vi:** Schema dưới đây mô tả tầng persistence cũ, đơn-file (`persistence.ts`, mục "Persistence (legacy)" trong §C4.1) dùng cho Electron desktop mode single-user — nó **không phải** cùng hệ thống với 13 migration DB (`backend/src/main/db/migrations/0001-0013`) mô tả ở §C4.3 "Database Layer Detail" phía trên và §2.7 của `audit/backend/backend-vs-design-review.md`. Hai tầng này song song tồn tại cho 2 chế độ chạy khác nhau (Electron desktop vs Web Server); tên bảng ở đây (`projects`, `worktrees`, `agent_sessions`...) không khớp và không nên đối chiếu trực tiếp với tên bảng `orca_*` của tầng migration Web Server.

```sql
-- Core entities
CREATE TABLE projects (
  id TEXT PRIMARY KEY,
  path TEXT NOT NULL UNIQUE,
  name TEXT,
  remote_url TEXT,
  icon_href TEXT,
  created_at INTEGER
);

CREATE TABLE worktrees (
  id TEXT PRIMARY KEY,
  project_id TEXT REFERENCES projects(id),
  path TEXT NOT NULL,
  branch TEXT,
  base_ref TEXT,
  status TEXT,           -- 'active' | 'removing' | 'error'
  agent_type TEXT,
  agent_session_id TEXT, -- for resume
  created_at INTEGER,
  updated_at INTEGER
);

CREATE TABLE agent_sessions (
  id TEXT PRIMARY KEY,
  worktree_id TEXT REFERENCES worktrees(id),
  agent_type TEXT,
  status TEXT,           -- 'idle'|'running'|'waiting'|'completed'|'error'|'stopped'
  started_at INTEGER,
  ended_at INTEGER,
  resume_flag TEXT       -- CLI flag for resume (--resume, --session-file)
);

CREATE TABLE terminal_scrollback_snapshots (
  worktree_id TEXT REFERENCES worktrees(id),
  serialized BLOB,       -- gzip compressed xterm serialization
  cursor_x INTEGER,
  cursor_y INTEGER,
  snapshot_at INTEGER,
  PRIMARY KEY (worktree_id)
);

CREATE TABLE ssh_hosts (
  id TEXT PRIMARY KEY,
  alias TEXT NOT NULL,
  hostname TEXT NOT NULL,
  port INTEGER DEFAULT 22,
  username TEXT,
  identity_file TEXT,
  proxy_jump TEXT,
  relay_version TEXT,    -- deployed relay version
  last_connected_at INTEGER
);

CREATE TABLE automations (
  id TEXT PRIMARY KEY,
  project_id TEXT REFERENCES projects(id),
  name TEXT NOT NULL,
  trigger_type TEXT,     -- 'cron' | 'manual' | 'event'
  trigger_cron TEXT,
  trigger_event TEXT,
  actions_json TEXT,     -- JSON array of actions
  enabled INTEGER DEFAULT 1,
  next_run INTEGER,
  created_at INTEGER
);

CREATE TABLE automation_runs (
  id TEXT PRIMARY KEY,
  automation_id TEXT REFERENCES automations(id),
  status TEXT,           -- 'running'|'completed'|'failed'|'timeout'
  started_at INTEGER,
  ended_at INTEGER,
  log_json TEXT          -- execution log per action
);

CREATE TABLE notifications (
  id TEXT PRIMARY KEY,
  type TEXT,
  title TEXT,
  body TEXT,
  worktree_id TEXT,
  read INTEGER DEFAULT 0,
  created_at INTEGER
);

CREATE TABLE rate_limits (
  agent_type TEXT,
  account_id TEXT,
  reset_at INTEGER,
  PRIMARY KEY (agent_type, account_id)
);

CREATE TABLE settings (
  key TEXT PRIMARY KEY,
  value TEXT
);
```

---

## C4.4 — IPC Channel Catalog

| Channel | Direction | Handler File | Mô tả |
|---------|-----------|-------------|-------|
| `worktree:create` | R→M | ipc/worktrees | Tạo worktree |
| `worktree:list` | R→M | ipc/worktrees | List worktrees |
| `worktree:remove` | R→M | ipc/worktrees | Xóa worktree |
| `worktree:compare` | R→M | ipc/worktrees | Compare multiple |
| `agent:start` | R→M | ipc/app | Khởi động agent |
| `agent:stop` | R→M | ipc/app | Dừng agent |
| `agent:status` | R→M | ipc/app | Get status |
| `pty:create` | R→M | ipc/pty | Tạo PTY session |
| `pty:input` | R→M | ipc/pty | Send input |
| `pty:resize` | R→M | ipc/pty | Resize PTY |
| `pty:data` | M→R | ipc/pty | PTY output |
| `git:diff` | R→M | ipc/git | Get diff |
| `git:status` | R→M | ipc/git | Get status |
| `git:commit` | R→M | ipc/git | Commit |
| `github:issues:list` | R→M | ipc/github | List issues |
| `github:pr:create` | R→M | ipc/github | Create PR |
| `ssh:connect` | R→M | ipc/ssh | Connect SSH host |
| `ssh:disconnect` | R→M | ipc/ssh | Disconnect |
| `automation:create` | R→M | ipc/automations | Create automation |
| `automation:run` | R→M | ipc/automations | Trigger manually |
| `notification:list` | R→M | ipc/notifications | Get history |
| `worktree:created` | M→R | event | New worktree |
| `agent:status:changed` | M→R | event | Status update |
| `port:detected` | M→R | event | Remote port found |
| `notification:new` | M→R | event | New notification |

---

## C4.5 — Relay Protocol (WebSocket Binary Frames)

> ⚠️ **Lưu ý:** wire format thật đã có ở §C4.5 "Agent WebSocket Detail" phía trên — khung 13-byte header `[TYPE u8][SEQ u32BE][ACK u32BE][LEN u32BE][PAYLOAD JSON-RPC]` (`agent/src/main/ssh/relay-protocol.ts:14`, `agent/src/shared/agent-wire-protocol.ts`), khớp đúng với `audit/agent/connection-wire-protocol-vs-design-review.md` §2.1. Interface `RelayFrame` với field `type: 'pty:data' | 'fs:read' | ...` dưới đây là minh hoạ khái niệm ở tầng logic (loại message được gửi qua payload JSON-RPC), **không phải struct thật trên wire** — không có field `type` dạng chuỗi này trong khung nhị phân thật, và method dispatch thật dùng JSON-RPC 2.0 (`method`/`params`/`id`) bên trong PAYLOAD, không phải discriminated union như minh hoạ.

```typescript
// relay-protocol.ts (minh hoạ khái niệm — không phải struct thật, xem cảnh báo trên)
interface RelayFrame {
  type: 'pty:data' | 'pty:resize' | 'pty:close'
      | 'fs:read' | 'fs:write' | 'fs:list' | 'fs:watch'
      | 'git:exec' | 'git:diff' | 'git:status'
      | 'port:detected' | 'port:closed'
      | 'hook:request' | 'hook:response'
      | 'ping' | 'pong';
  id: string;          // request correlation ID
  sessionId: string;   // PTY/stream session
  payload: Uint8Array; // encoded payload (msgpack or JSON)
}
```

---

## C4.6 — Agent Hook Interception

⚠️ **Cơ chế mô tả dưới đây (HTTP interceptor redirect tool calls) không khớp code thật.** Cơ chế thật (`audit/agent/pty-ai-cli-vs-design-review.md` §2.7, `audit/agent/credential-fswatch-telemetry-vs-design-review.md`): agent CLI (Claude/Codex/...) tự **POST JSON tới một HTTP loopback server nội bộ** khi hook lifecycle event xảy ra (`PreToolUse`, `PostToolUse`, `Stop`...), không phải "intercept tool calls" qua redirect. Có 2 tầng: `AgentHookServer` (`agent/src/main/agent-hooks/server.ts`, local) và `RelayAgentHookServer` (`agent/src/relay/agent-hook-server.ts`, phía relay/SSH remote host) — forward qua JSON-RPC **notification** `agent.hook` (không có `id`, không xuất hiện trong bảng RPC request/response).

```
Agent Process (e.g., Claude Code)
       │
       │  Hook lifecycle event (PreToolUse/PostToolUse/Stop) → managed hook script
       │  POST http://127.0.0.1:<ORCA_AGENT_HOOK_PORT>/hook/<agent>
       │  Header: X-Orca-Agent-Hook-Token: <ORCA_AGENT_HOOK_TOKEN>
       ▼
[agent/src/relay/agent-hook-server.ts | agent/src/main/agent-hooks/server.ts] HTTP receiver
       │  ✅ khớp: kiểm tra token header đúng thiết kế
       │  ❌ KHÔNG intercept file-read/bash-exec — chỉ nhận structured status JSON
       │  forward qua JSON-RPC notification 'agent.hook' (AGENT_HOOK_NOTIFICATION_METHOD)
       ▼
[Backend/Desktop] AgentHookServer.applyNormalizedStatus() — dedupe/anti-flicker phức tạp
       │
       ▼
[Renderer] Display in "Agent Activity" panel

⚠️ Cài đặt hook (install/installRemote/remove per-agent HookService) chỉ tồn tại ở
backend/src/main/, desktop/src/main/, frontend/src/main/ — KHÔNG có bản nào trong agent/src/main/;
agent/ chỉ đóng vai trò "bên nhận" (listener/server), không phải "bên cài đặt".
```

---

## C4.7 — Profile System Module Map

### src/main/profile/ — New module (v5.0)

```
src/main/profile/
├── profile-types.ts              # OrcaProfile interface (6 sections)
│                                 # ResolvedProfile + ResolvedProfileWithMeta
│
├── profile-resolver.ts           # resolveProfile(userId): ResolvedProfile
│                                 # deepMergeProfiles(company, dept, user)
│                                 # Security lock, approvedModels validation
│                                 # Source attribution (_sources metadata)
│
├── profile-cache.ts              # In-memory Map<userId, {value, expiresAt}>
│                                 # TTL = 60s
│                                 # invalidateByCompany(), invalidateByDept(deptId)
│                                 # invalidateByUser(userId)
│
├── profile-service.ts            # updateCompanyProfile(json) — admin only
│                                 # updateDepartmentProfile(deptId, json)
│                                 # updateUserProfile(userId, json)
│                                 # getEffectiveProfile(userId) → resolved
│
├── company-service.ts            # CRUD orca_company table
│                                 # ensureCompanyExists() — singleton
│
└── department-service.ts         # CRUD orca_departments
                                  # getDepartmentsByCompany()
                                  # getDepartmentsForUser(userId)
```

### src/shared/profile-types.ts

```typescript
interface OrcaProfile {
  agent?: {
    preferredModel?: string
    trustPreset?: 'minimal' | 'standard' | 'permissive'
    maxTokensPerSession?: number
    autoApproveFileRead?: boolean
    approvedModels?: string[]      // Company-level only
  }
  editor?: {
    theme?: 'dark' | 'light' | 'system'
    fontSize?: number
    fontFamily?: string
    tabSize?: number
    keybindings?: 'vscode' | 'vim' | 'emacs'
    wordWrap?: boolean
  }
  shell?: {
    defaultShell?: string
    pathAdditions?: string[]       // concatenated across tiers
    envVars?: Record<string, string>   // user override dept override company
    startupCommands?: string[]
  }
  integrations?: {
    githubOrg?: string
    linearWorkspace?: string
    defaultReviewer?: string
    prTemplate?: string
  }
  fleet?: {
    allowedServerTags?: string[]
    defaultConnectionType?: string
    sshKeyPath?: string
  }
  security?: {                     // Company-level ONLY
    require2FA?: boolean
    sessionTimeoutHours?: number
    allowedIpRanges?: string[]
    auditAllActions?: boolean
  }
}

interface ResolvedProfile extends OrcaProfile {
  _sources: Record<string, 'company' | 'department' | 'user'>
}
```

### DB Schema additions (Migration 0006) — 🔧 tên bảng sửa theo §2.7

```sql
-- ⚠️ Tên bảng thật khác đề xuất: orca_companies (số nhiều, không phải orca_company), và code thực tế
-- có thêm bảng orca_user_profiles không nằm trong thiết kế gốc.
CREATE TABLE orca_companies (
  id   TEXT PRIMARY KEY DEFAULT 'default',
  name TEXT NOT NULL,
  logo_url TEXT,
  profile_json TEXT DEFAULT '{}',
  created_at INTEGER,
  updated_at INTEGER
);

CREATE TABLE orca_departments (
  id         TEXT PRIMARY KEY,
  company_id TEXT NOT NULL REFERENCES orca_companies(id),
  name       TEXT NOT NULL,
  team_lead_id TEXT REFERENCES orca_users(id),
  profile_json TEXT DEFAULT '{}',
  created_at INTEGER,
  updated_at INTEGER
);

-- orca_user_profiles: bảng thêm ngoài thiết kế gốc (không phải ALTER TABLE orca_users)
CREATE TABLE orca_user_profiles ( ... );  -- xem audit/backend §2.7 để xác nhận cột chi tiết
```

### RPC methods — profile.*

⚠️ **8 method liệt kê dưới đây undercount** — namespace thật có **10 method** (`audit/backend/backend-vs-design-review.md` §2.10). Sai lệch cụ thể nhất: `profile.getEffective` **không tồn tại**, tên thật là `profile.getResolved`.

```typescript
// backend/src/main/runtime/rpc/methods/profile.ts
'profile.getResolved'     // 🔧 tên thật — KHÔNG phải 'profile.getEffective' — (userId) → ResolvedProfile (cached)
'profile.updateUser'      // (fields: Partial<OrcaProfile>) — personal only
'profile.getDepartment'   // (deptId) → OrcaProfile
'profile.updateDepartment'// (deptId, fields) — lead/admin
'profile.getCompany'      // () → OrcaProfile — admin
'profile.updateCompany'   // (fields) — admin only  ❌ requireAdmin(ctx) là stub, không check role thật — §F32
'profile.listDepartments' // () → Department[]
'profile.createDepartment'// (name) — admin
// + 2 method khác chưa liệt kê ở đây (namespace thật có 10 method — xem audit §2.10)
```

---

## C4.8 — Project & Project-Centric Execution Module Map

### src/main/project/ — New module (v5.0)

```
src/main/project/
├── project-types.ts              # OrcaProject, ProjectMember interfaces
│
├── project-service.ts            # CRUD orca_projects
│                                 # getProjectsForUser(userId): filter membership + RBAC
│                                 # updateDevServerBinding(projectId, devServerId)
│                                 # addMember(projectId, userId, role)
│                                 # removeMember(projectId, userId)
│
├── project-server-router.ts      # Route actions to project.devServerId
│                                 # routeWorktreeCreate(projectId, branchInput)
│                                 # routeTerminalOpen(projectId)
│                                 # routeAgentSpawn(projectId, userId, userPrompt)
│                                 # checkServerAvailability(devServerId): ServerStatus
│
├── ProfileAwareAgentSpawner.ts    # ⚠️ tên file thật PascalCase, tại backend/src/main/project/
│                                  # spawn() LUÔN đi qua relay.call('agent.exec', ...) — KHÔNG có
│                                  # nhánh node-pty.spawn trực tiếp cho local machine, và KHÔNG
│                                  # gọi relay.call('pty.spawn', ...) như dòng dưới từng ghi
│                                  # (audit/backend §2.11; audit/agent/pty-ai-cli-vs-design-review.md §2.3)
│                                  # GH_CONFIG_DIR / GLAB_CONFIG_DIR injection — ✅ có, nhưng agent/
│                                  # side (SubAgentSpawner.buildAgentEnv) chỉ làm đúng phần này;
│                                  # PATH-extension từ pathAdditions và ANTHROPIC_MODEL từ
│                                  # preferredModel KHÔNG được set ở tầng agent/ — nếu có, phải đến
│                                  # từ backend qua extraEnv passthrough (không xác nhận được)
│
└── project-context-injector.ts   # buildProjectContext(project, user, worktree)
                                   # Returns preamble string for agent init
```

### DB Schema additions (Migration 0007) — 🔧 tên bảng sửa theo §2.7

⚠️ **Tên bảng thật: `orca_v5_projects` / `orca_v5_project_members`**, không phải `orca_projects`/`orca_project_members` — đổi tên vì migration 0004 đã chiếm tên `orca_projects` trước đó (comment trong code giải thích lý do đổi tên tránh đụng độ).

```sql
CREATE TABLE orca_v5_projects (            -- 🔧 orca_v5_projects, không phải orca_projects
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL UNIQUE,
  description   TEXT,
  repo_url      TEXT,
  repo_path     TEXT NOT NULL,          -- path on dev server
  dev_server_id TEXT REFERENCES ssh_hosts(id),  -- THE BINDING — ❌ không thể rebind sau khi tạo (F34)
  default_branch TEXT DEFAULT 'main',
  tags          TEXT DEFAULT '[]',      -- JSON array
  created_by    TEXT REFERENCES orca_users(id),
  created_at    INTEGER,
  updated_at    INTEGER
);

CREATE TABLE orca_v5_project_members (     -- 🔧 orca_v5_project_members, không phải orca_project_members
  project_id  TEXT REFERENCES orca_v5_projects(id) ON DELETE CASCADE,
  user_id     TEXT REFERENCES orca_users(id)    ON DELETE CASCADE,
  role        TEXT DEFAULT 'developer',  -- developer | lead | admin — ProjectRole trùng tên với org-level role, khác domain
  joined_at   INTEGER,
  PRIMARY KEY (project_id, user_id)
);

CREATE INDEX idx_project_members_user ON orca_v5_project_members(user_id);
CREATE INDEX idx_projects_devserver ON orca_v5_projects(dev_server_id);
```

### RPC methods — project.* (🔧 namespace số ít, không phải projects.* — §2.10)

```typescript
// backend/src/main/runtime/rpc/methods/project.ts
'project.list'            // (userId?) → OrcaProject[] (filtered by membership)
'project.get'             // (projectId) → OrcaProject
'project.create'          // (input: CreateProjectInput) — ❌ không giới hạn lead/admin trong code (F34)
'project.update'          // (projectId, fields) — lead/admin
'project.delete'          // (projectId) — admin only
// ❌ 'projects.updateBinding' KHÔNG tồn tại — devServerId không nằm trong schema update(),
//    không có method bindDevServer/rebindDevServer nào (F34): không thể đổi dev server sau khi tạo project
'project.addMember'       // (projectId, userId, role) — lead/admin
'project.removeMember'    // (projectId, userId) — lead/admin
'project.listMembers'     // (projectId) → ProjectMember[]
'project.getContextPreamble' // (projectId, userId, worktreeId) → string
// namespace thật có 10 method (doc gốc liệt kê 9) — xem audit §2.10 cho danh sách đầy đủ
// ⚠️ requireOwnerOrAdmin chỉ check role!=='owner', KHÔNG check admin — dead code trong tên hàm (F34)
```

### Project-centric Routing Data Flow

```
User → click "New Worktree" for project P
    │
    ▼ RPC: projects.get(P) → { devServerId: 'server-alpha', repoPath: '/srv/vnp' }
    │
    ▼ ProjectServerRouter.checkServerAvailability('server-alpha')
    │   → status: 'healthy'  (via FleetHealthMonitor cache)
    │
    ▼ relay = DevServerManager.getRelay('server-alpha')
    │   → existing SSH relay connection (or establish new)
    │
    ▼ relay.call('git.worktree.add', {
    │     basePath: '/srv/vnp',
    │     branch: 'feature/x',
    │     worktreePath: '/srv/vnp-worktrees/feature-x'
    │   })
    │
    ▼ RPC: profile.getResolved(userId)          # 🔧 tên thật, không phải profile.getEffective
    │   → ProfileResolver.resolve(userId) [cache hit]
    │   → { agent: { preferredModel: 'claude', trustPreset: 'standard' },
    │         shell: { envVars: {...}, pathAdditions: [...] } }
    │
    ▼ ProfileAwareAgentSpawner.spawn({          # backend/src/main/project/ProfileAwareAgentSpawner.ts
    │     relay,
    │     project: P,
    │     worktreePath: '/srv/vnp-worktrees/feature-x',
    │     profile: resolvedProfile,
    │     userId,
    │   })
    │   ├── agentEnv = {                        # 🚧 build ở tầng backend — agent/'s SubAgentSpawner
    │   │     ...profile.shell.envVars,          #    KHÔNG tự đọc profile (agent-side là dead code cho
    │   │     PATH: /pathAdditions + $PATH,      #    profile injection, xem §C4.11a) — nếu các giá trị
    │   │     GH_CONFIG_DIR: ~/.config/gh/userId/, #  này tới được agent, phải qua extraEnv passthrough
    │   │     ANTHROPIC_MODEL: 'claude-opus-4-5', #  từ backend, KHÔNG xác nhận được trong audit
    │   │     ORCA_PROJECT_ID: P.id,
    │   │   }
    │   └── relay.call('agent.exec', {           # 🔧 method thật là agent.exec (generic passthrough),
    │           binary: 'claude',                #    KHÔNG phải 'pty.spawn' — pty.spawn spawns a shell,
    │           args: ['--trust', 'standard'],    #    không spawn agent binary trực tiếp (§C4.10, §2.11)
    │           cwd: '/srv/vnp-worktrees/feature-x',
    │           env: agentEnv,
    │         })
    │
    ▼ PTY stream → WebSocket → Browser terminal ✅
```

---

## C4.9 — AI Provider & Task Graph Module Map

### src/main/ai-providers/ — New module (v5.0)

```
backend/src/main/ai-providers/       # (kể cả tên module — cấu trúc thư mục chưa xác nhận 1:1 qua audit)
├── ai-provider-types.ts          # AIProviderAccount, TaskGrant, ProviderHealthStatus
├── AIProviderService.ts          # ✅ CRUD orca_ai_provider_accounts — tên PascalCase thật
├── ProviderResolver.ts           # ✅ Resolution algorithm — ⚠️ priority đảo ngược: doc project→user,
│                                 #    code thật user-scope → project-scope (§5.14/F35)
├── ProviderHealthChecker.ts      # ✅ Background cron 15 phút, test connection, quota tracking
├── provider-credential-relay.ts  # ❌ class ProviderCredentialRelay KHÔNG tồn tại — logic nằm trong
│                                 #    AIProviderService.writeCredentialToDevServer() (§2.6)
└── provider-rpc-methods.ts       # aiProvider.* RPC handlers — namespace camelCase, xem bên dưới
```

❌ **Thiếu 2 tính năng được coi là "đã có" (§5.14/F35):** key rotation (grace period 30s, status `'rotating'`, audit log) hoàn toàn không tồn tại — update key hiện ghi đè trực tiếp qua `writeCredential`; quota-80%-alert không tồn tại (chỉ phát hiện SAU khi vượt quota).

### src/main/task/ — New module (v5.0)

```
src/main/task/
├── task-types.ts                 # OrcaTask, TaskEdge, TaskGrant, TaskGrant interfaces
├── task-service.ts               # CRUD + progress calculation
├── task-dag-validator.ts         # addEdge with cycle detection, auto-block logic
├── task-graph-builder.ts         # loadTaskTree(rootId, userId) BFS + access filter
├── task-ai-planner.ts            # AI decompose + prompt generation
├── task-grant-service.ts         # hasTaskAccess(), addGrant(), revokeGrant() — ✅ 5-level (view<comment<
│                                 #   edit<execute<manage) + BFS ancestor đúng thuật toán thiết kế
├── task-agent-executor.ts        # buildTaskPreamble(), spawnAgent(), streamToActivity()
└── task-rpc-methods.ts           # task.* RPC handlers (🔧 số ít, không phải tasks.*)
```

⚠️ Namespace thật `task.*` có **18 method** (doc gốc liệt kê 11) — `addDependency`→thực tế `addEdge`; `aiPlan`→tách thành `aiDecompose`+`aiApply`; `runAgent`→`execute` (§2.10). Cycle detection đã implement đầy đủ (`TaskDAGValidator`, DFS/BFS có index). Progress tracking **không rollup** từ subtask thật — chỉ bảng tĩnh map status→% (§5.16/F37).

### src/main/workflow/ — New module (v5.0)

```
src/main/workflow/
├── workflow-types.ts             # WorkflowDefinition, StepDef, WorkflowExecution
├── template-registry.ts         # CRUD orca_workflow_templates
├── template-resolver.ts         # resolveTemplate() — merge inheritance chain
├── workflow-orchestrator.ts     # start(), resume(), pause(), buildDAG()
├── dag-builder.ts               # steps → DAG, topological sort, wave planning
├── server-resolver.ts           # project:/server:/fleet:tag: → devServerId
├── workflow-provider-resolver.ts # Delegate to AIProviderResolver with workflow ctx
├── execution-store.ts           # Persist/load WorkflowExecution state
├── executors/
│   ├── agent-step-executor.ts   # Spawn AI agent on dev server
│   ├── shell-step-executor.ts   # relay.call('shell.exec', ...)
│   ├── action-step-executor.ts  # Built-in actions (github.createPR, ...)
│   ├── parallel-step-executor.ts # ⚠️ không phải executor riêng — parallel chỉ đạt ngầm định qua wave execution
│   └── condition-step-executor.ts # Branch logic evaluation
└── workflow-rpc-methods.ts      # workflow.* RPC handlers (🔧 số ít, không phải workflows.*)
```

⚠️ **Namespace thật `workflow.*` có 7 method nhưng tên khác gần hết** (§2.10): không có `run`/`pause`/`resume`/`streamStepOutput` — thực tế chỉ `execute`/`cancel`, không có pause/resume hay streaming riêng. ❌ **Pause/Resume hoàn toàn không tồn tại** — `WorkflowStatus` không có `'paused'`; `resumeRunningExecutions()` chỉ là crash-recovery khi server restart, không phải user-triggered qua UI (§5.15/F36). ❌ **Provider selection theo từng step: 0% code** dù đây là tính năng chính minh hoạ trong F36 — `WorkflowStepConfig` là bag opaque, không import `AIProviderService`/`ProviderResolver`. ⚠️ Dispatch đa server chỉ hỗ trợ `project:<id>`; `server:<devServerId>` ném lỗi "not yet implemented"; `fleet:tag:<tag>` hoàn toàn không xử lý.

### DB Migrations (v5.0 batch)

```sql
-- Migration 0008: AI Provider Accounts
CREATE TABLE orca_ai_provider_accounts ( ... )  -- See BL-AIP-03
CREATE TABLE orca_provider_usage ( ... )

-- Migration 0009: Workflow System
CREATE TABLE orca_workflow_templates (
  id           TEXT PRIMARY KEY,
  scope        TEXT,           -- company|team|personal
  team_id      TEXT,
  owner_id     TEXT REFERENCES orca_users(id),
  name         TEXT NOT NULL,
  description  TEXT,
  template_yaml TEXT NOT NULL,
  parent_template_id TEXT,    -- inheritance
  visibility   TEXT DEFAULT 'private',
  version      TEXT DEFAULT '1.0',
  tags         TEXT DEFAULT '[]',
  usage_count  INTEGER DEFAULT 0,
  rating       REAL,
  share_token  TEXT,
  created_at   INTEGER,
  updated_at   INTEGER
);

CREATE TABLE orca_workflow_executions (
  id            TEXT PRIMARY KEY,
  template_id   TEXT REFERENCES orca_workflow_templates(id),
  user_id       TEXT REFERENCES orca_users(id),
  status        TEXT DEFAULT 'queued',
  inputs_json   TEXT DEFAULT '{}',
  outputs_json  TEXT DEFAULT '{}',
  started_at    INTEGER,
  completed_at  INTEGER,
  error         TEXT
);

CREATE TABLE orca_workflow_step_executions (  -- 🔧 tên thật có thêm tiền tố "workflow_" (§2.7)
  id              TEXT PRIMARY KEY,
  execution_id    TEXT REFERENCES orca_workflow_executions(id),
  step_id         TEXT NOT NULL,
  status          TEXT DEFAULT 'pending',
  dev_server_id   TEXT,
  provider_id     TEXT,
  output_json     TEXT,
  log_stream_id   TEXT,
  started_at      INTEGER,
  completed_at    INTEGER,
  error           TEXT
);

-- Migration 0010: Task Graph — ✅ khớp + thêm bảng orca_team_members ngoài thiết kế gốc (§2.7)
CREATE TABLE orca_tasks ( ... )               -- See BL-TG-01
CREATE TABLE orca_task_edges ( ... )
CREATE TABLE orca_task_grants ( ... )
CREATE TABLE orca_task_comments ( ... )
CREATE TABLE orca_team_members ( ... )        -- 🔧 thêm ngoài thiết kế gốc (§2.7)

-- Migration 0011–0013: 🔧 hoàn toàn thiếu trong thiết kế gốc — bổ sung theo §2.7
--   0011–0013 thêm: terminal_sessions, port_forwards, push_subscriptions, và cơ chế
--   trace-correlation-qua-restart cho Workflow (root_trace_id) — dùng bởi F40 Full-Flow-Tracing
--   để tái tạo parent span sau khi Orca Server restart (WorkflowOrchestrator.ts:110,120,249-252,359-360).
--   Đây là ví dụ hiếm hoi code triển khai VƯỢT mô tả thiết kế theo đúng tinh thần (§5.19/F40, ~95% khớp).
```

### Data Flow: Task → Agent → Activity Feed

```
Browser click "Run Agent" on Task T
    │
    ▼ RPC: task.execute(taskId=T, worktreeId=W?)     # 🔧 task.execute, không phải tasks.runAgent (§2.10)
    │
    ├── TaskGrantService.hasTaskAccess(userId, T, 'execute') → OK
    │
    ├── TaskService.get(T)
    │   → { title, description, promptTemplate, aiContext, projectId }
    │
    ├── ProjectService.get(projectId)
    │   → { devServerId, repoPath }
    │
    ├── FleetHealthMonitor.check(devServerId)
    │   → { status: 'healthy' }
    │
    ├── AIProviderResolver.resolve({
    │     devServerId, projectId, userId,
    │     model: resolvedProfile.agent.preferredModel
    │   })
    │   → AIProviderAccount { id, model, ... }
    │
    ├── TaskAgentExecutor.buildPreamble(task, project, user, deps)
    │   → "# Task Context\nTask: Implement bcrypt...\n..."
    │
    ├── ProfileAwareAgentSpawner.spawn({
    │     relay, project, worktreePath,
    │     profile: resolvedProfile,
    │     userId,
    │     extraEnv: { ORCA_TASK_ID: T.id },
    │     initFile: preamble + task.promptTemplate     # ❌ `initFile`/systemPreamble không tồn tại trong
    │   })                                              #    agent/ (audit/agent/pty-ai-cli-vs-design-review.md
    │   → sessionId                                     #    §2.6) — preamble injection thật (nếu có) phải xảy ra
    │                                                    #    ở tầng backend trước khi gọi relay.call('agent.exec', ...)
    │
    ├── UPDATE orca_tasks SET agent_session_id=sessionId
    │
    ├── PTY output → WebSocket broadcast
    │   → Task Activity Feed (browser)
    │   → Workflow Step log (if task linked to workflow)
    │
    └── Agent complete event
        → UPDATE task status='review', actual_hours=elapsed
        → Check parent progress update
        → Notify assignee + reporter (WebSocket push)
```

---

## C4.10 — Project Workspace Module Map

### src/renderer/src/components/workspace/ — New (v5.0)

```
src/renderer/src/components/workspace/
├── WorkspaceLayout.tsx           # Top-level: sidebar + main area + bottom panel
├── ProjectSelector.tsx           # Project dropdown with server status indicator
├── ServerStatusBar.tsx           # Online/degraded/offline banner + retry
│
├── ExplorerPanel.tsx             # Main file tree container
├── FileTreeNode.tsx              # Single node: icon + name + git badge
├── RemoteFileViewer.tsx          # Read-only file tab (Monaco, syntax highlight)
├── FileSearchPanel.tsx           # Glob + grep search results
│
├── GitPanel.tsx                  # Git tab: status + diff + commit + sync + branches
├── DiffViewer.tsx                # Unified diff with syntax highlighting
├── CommitForm.tsx                # Message input + AI generate + commit/push buttons
├── BranchManager.tsx             # Branch list: local/remote + create/switch/delete
├── WorktreeSwitcher.tsx          # Worktree dropdown + new worktree
├── GitLog.tsx                    # Last 50 commits + branch graph (ASCII)
├── PullRequestForm.tsx           # Title + body (AI) + reviewers + base branch
├── ConflictPanel.tsx             # List conflict files + AI resolve
│
├── AgentPanel.tsx                # Agent control: provider + prompt + output
└── WorkspaceTerminal.tsx         # Bottom panel PTY sessions
```

### src/renderer/src/context/WorkspaceContext.tsx — New (v5.0)

```typescript
interface WorkspaceContextValue {
  // Project
  project: OrcaProject | null
  devServer: SshHost | null
  isConnected: boolean
  isOffline: boolean

  // Connection
  relay: DevServerRelayBridge | null

  // Worktree
  currentWorktree: Worktree | null
  availableWorktrees: Worktree[]
  setCurrentWorktree: (wt: Worktree) => void

  // Git
  gitStatus: GitStatus | null
  refreshGitStatus: () => Promise<void>

  // Profile
  resolvedProfile: ResolvedProfile | null

  // Agent
  activeAgentSessionId: string | null
  setActiveAgentSession: (id: string | null) => void

  // Event bus
  emit: (event: WorkspaceEvent) => void
  on: (event: string, handler: Function) => () => void

  // Actions
  switchProject: (projectId: string) => Promise<void>
}

type WorkspaceEvent =
  | { type: 'agent.complete'; filesChanged: number }
  | { type: 'git.commit'; hash: string; message: string }
  | { type: 'git.push'; branch: string }
  | { type: 'worktree.switched'; path: string; branch: string }
  | { type: 'workflow.step.complete'; stepId: string; executionId: string }
```

### backend/src/main/runtime/rpc/methods/git.ts — Extended (v5.0)

⚠️ **Namespace phẳng thật không có sub-namespace lồng `git.branch.*`** (`audit/backend/backend-vs-design-review.md` §2.10): code thật đặt tên phẳng (vd. `git.branchCompare`) thay vì `git.branch.list`/`git.branch.create` như dưới đây. Số lượng method thật cũng lớn hơn nhiều: doc liệt kê ~10-15, thực tế `git.*` có **~35 method**. `fs.*`/`pty.*` dưới đây là **giao thức nội bộ backend→dev-agent**, không phải API mà frontend gọi trực tiếp — API client thật là `files.*` (28 method) và `terminal.*` (30 method); tài liệu gốc nhầm lớp giao thức.

```typescript
// Method names dưới đây minh hoạ Ý ĐỊNH — xem cảnh báo trên về namespace phẳng thật + số lượng thật
'git.status'           → exec('git status --porcelain=v2 --branch', { cwd })
'git.diff'             → exec('git diff [--staged] [--] [file]', { cwd })
'git.add'              → exec('git add <files>', { cwd })
'git.restore'          → exec('git restore [--staged] <files>', { cwd })
'git.commit'           → exec('git commit -m <msg> --author=...', { cwd })
'git.push'             → execStream('git push origin <branch>', { cwd })
'git.pull'             → execStream('git pull origin <branch>', { cwd })
'git.fetch'            → exec('git fetch --all', { cwd })
'git.branchCompare'    → 🔧 tên thật phẳng — KHÔNG có 'git.branch.list'/'git.branch.create' lồng namespace
'git.merge'            → exec('git merge --no-ff <branch>', { cwd })
'git.stash'            → exec('git stash push -m <msg>', { cwd })
'git.stash.pop'        → exec('git stash pop', { cwd })
'git.log'              → exec('git log --oneline --graph --decorate -50', { cwd })
// ⚠️ DevServerGitProvider ném lỗi "not supported for Dev Server hosts yet" cho getHistory (git log),
// getStagedCommitContext (AI commit message), getBranchCompare/getCommitCompare/getBranchDiff/
// getCommitDiff, getSubmoduleStatus, checkIgnoredPaths — nhiều thao tác này KHÔNG hoạt động khi
// repo nằm trên Dev Server (audit/backend §5.18/F39)

// File system (giao thức internal backend↔dev-agent; API client thật là files.*, xem trên):
'fs.readDir'           → relay: fs.readdir + stat per entry
'fs.readFile'          → relay: fs.readFile (max 5MB)
'fs.stat'              → relay: fs.stat
'fs.glob'              → relay: glob(pattern, { cwd, ignore })
'fs.grep'              → relay: grep -rn --include=<ext> pattern cwd (limit 30)   # ⚠️ doc gốc ghi 'fs.search', tên thật 'fs.grep'
```

### Key Data Flow: Project Switch → Full Workspace Init

```
User selects project "vnp-blc-backend"
    │
    ▼ WorkspaceContext.switchProject('proj-abc')
    │
    ├── ProjectService.get('proj-abc')
    │   → { devServerId: 'svr-01', repoPath: '/srv/vnp', ... }
    │
    ├── RelayConnectionPool.getOrConnect('svr-01')
    │   → DevServerRelayBridge (SSH established)
    │
    ├── FleetHealthMonitor.getCached('svr-01')
    │   → { status: 'healthy', latencyMs: 12 }
    │
    ├── ProfileResolver.resolve(userId) [cache]
    │   → ResolvedProfile { agent, editor, shell, ... }
    │
    ├── Promise.all([
    │   relay.call('git.status', { cwd: '/srv/vnp' }),
    │   relay.call('git.worktree.list', { repoPath: '/srv/vnp' }),
    │   relay.call('fs.readDir', { path: '/srv/vnp', depth: 2 }),
    │   WorkflowService.getActiveExecutions('proj-abc'),
    │ ])
    │
    ├── WorkspaceContext state set:
    │   { project, devServer, relay, resolvedProfile,
    │     gitStatus, availableWorktrees, fileTree }
    │
    ├── Start git status poll (5s interval)
    │
    └── UI renders: Explorer (file tree) + Git (status)
        Agent tab (provider from profile) ready
```

### Key Data Flow: Remote Git Push with Progress

```
User: [Push] in Git tab
    │
    ▼ RPC: git.push({ cwd: worktreePath, remote: 'origin', branch })
    │
    ├── Orca Server: relay.callStream('git.push', { cwd, remote, branch })
    │
    ├── Dev Server relay: spawn git push, capture stdout/stderr
    │   → stream lines back via WebSocket:
    │   { type: 'git.push.progress', line: 'Counting objects: 5, done.' }
    │   { type: 'git.push.progress', line: 'Writing objects: 100%...' }
    │   { type: 'git.push.done', success: true }
    │
    ├── UI: render each progress line in push panel
    │
    └── On done:
        refresh gitStatus (ahead/behind reset)
        WorkspaceContext emit('git.push', { branch })
        → if PR already open → refresh PR status
```

---

## Quick Reference — Feature → C4 Module Mapping

| Feature | C4 Module(s) | Key classes/files | ADR |
|---------|-------------|------------------|-----|
| F22 Web Server | C4.2 | `server-bootstrap.ts`, `NodeAdapter` | ADR-001 |
| F23 Multi-User Auth | C4.1, C4.2 | `AuthManager`, `SessionManager` | ADR-003 |
| F24 Per-User Sandbox | C4.1 | `SessionManager.fork()`, `WsSessionRouter` | ADR-003 |
| F26 Multi-Database | C4.3 | `IConnectionPool`, `MigrationRunner`, adapters | ADR-002 |
| F27 Fleet Health | C4.4 | `FleetHealthMonitor`, `FleetHealthStore` | ADR-004 |
| F28 Dev Server Onboarding | C4.4 | `DevServerProvisioner`, `SftpUpload` | ADR-004 |
| F29 Agent WebSocket | C4.5 | `AgentWebSocketServer`, `WsTransport`, `relay/protocol.ts` | ADR-005 |
| F30 Remote Integrations | C4.6 | `WebCredentialStore`, `GH_CONFIG_DIR` isolation | ADR-006 |
| F33 Profile Hierarchy | C4.7 | `ProfileResolver`, `deepMerge()`, `OrcaProfile` | ADR-007 |
| F34 Project Binding | C4.8 | `ProjectService`, `ProjectServerRouter`, `ProfileAwareAgentSpawner` | ADR-007, ADR-011 |
| F35 AI Provider | C4.9 | `AIProviderService`, `ProviderResolver`, relay: `ai.provider.*` | ADR-008 |
| F36 Workflow | C4.9 | `WorkflowOrchestrator`, `DAGBuilder`, `TemplateResolver` | ADR-009 |
| F37 Task Graph | C4.9 | `TaskService`, `TaskDAGValidator`, `TaskGrantService`, `TaskAIPlanner` | ADR-010 |
| F38 Project Workspace | C4.10 | `RelayConnectionPool`, `WorkspaceContext`, `ExplorerPanel` | ADR-011 |
| F39 Remote Git UI | C4.10 | `git-handler.ts` (relay), `GitPanel`, `DiffViewer`, `CommitForm` | ADR-012 |

---

## Module Dependency Rules (Layering)

```
Forbidden imports (enforced by oxlint / eslint-boundaries):

❌ src/relay/* → src/main/*          (relay is standalone binary)
❌ src/main/db/* → src/main/auth/*   (DB layer must not know about auth)
❌ src/platform/* → src/main/*       (platform interface must stay pure)
❌ src/renderer/* → src/main/*       (renderer → RPC only, no direct import)

Allowed:
✅ src/main/* → src/platform/* (via getPlatform())
✅ src/main/* → src/main/db/* (via IConnectionPool)
✅ src/main/* → src/relay/* (via relay.call() RPC)
✅ src/renderer/* → src/shared/* (shared types)
```

---

## v5.0 New Modules Checklist

> 🔧 **Bảng này đã lỗi thời so với code thật — sửa lại theo `audit/backend/backend-vs-design-review.md` §2.6/§5.x (2026-08-08).** Toàn bộ danh sách gốc đánh dấu `❌ TODO`, nhưng đối chiếu trực tiếp với code cho thấy **hầu hết các module này đã được triển khai** (chỉ khác tên file/class/chi tiết hành vi so với thiết kế — không phải "chưa làm"). Giữ nguyên cột "Depends on" gốc; cột Status/ghi chú đã cập nhật theo bằng chứng `file:line` từ audit.

| Module | Path (đường dẫn thật ở `backend/src/`) | Status | Ghi chú |
|--------|------|--------|-----------|
| ProfileResolver | `main/profile/ProfileResolver.ts` | ✅ Done | 3-layer merge + cache TTL 60s đúng thiết kế |
| ProfileService | `main/profile/profile-rpc-handler.ts` (RPC handler, không phải class `ProfileService`) | ✅ Done | ⚠️ `requireAdmin(ctx)` là stub — không check role thật (bug bảo mật, F32) |
| ProjectService | `main/project/ProjectService.ts` | ✅ Done | Bảng `orca_v5_projects` (không phải `orca_projects`, §2.7) |
| ProjectServerRouter | `main/project/ProjectServerRouter.ts` | ✅ Done | Đúng luồng routing theo `devServerId` |
| ProfileAwareAgentSpawner | `main/project/ProfileAwareAgentSpawner.ts` | ✅ Done | `spawn()` LUÔN qua `relay.call('agent.exec', ...)` — không có nhánh local node-pty |
| AIProviderService | `main/ai-providers/AIProviderService.ts` | ✅ Done (⚠️ một phần) | `ProviderCredentialRelay` (class riêng) KHÔNG tồn tại — logic gộp trong `AIProviderService.writeCredentialToDevServer()` |
| ProviderResolver | `main/ai-providers/ProviderResolver.ts` | ✅ Done | Priority cascade đảo ngược so với thiết kế (user→project, không phải project→user) |
| DAGBuilder | `main/workflow/` (wave-based, Kahn's algorithm) | ✅ Done | Cycle detection đúng chuẩn |
| WorkflowOrchestrator | `main/workflow/WorkflowOrchestrator.ts` | ✅ Done | ❌ Pause/Resume không tồn tại; provider-selection theo step 0% code (F36) |
| TemplateResolver | `main/workflow/` (`TemplateResolver`, `MAX_INHERIT_DEPTH=5`) | ✅ Done | Đúng thiết kế |
| TaskService | `main/task/TaskService.ts` | ✅ Done | Namespace RPC thật `task.*` (18 method, không phải `tasks.*` 11 method) |
| TaskDAGValidator | `main/task/TaskDAGValidator.ts` | ✅ Done | Cycle detection DFS/BFS đầy đủ, không phải TODO |
| TaskGrantService | `main/task/TaskGrantService.ts` | ✅ Done | 5-level permission + BFS ancestor đúng thuật toán |
| TaskAIPlanner | `main/task/TaskAIPlanner.ts` | ✅ Done (⚠️ một phần) | Thiếu dependencies/criticalPath giữa subtask khi AI decompose |
| TaskAgentExecutor | `main/task/TaskAgentExecutor.ts` | ✅ Done | Progress tracking không rollup từ subtask thật |
| RelayConnectionPool | không xác nhận được tên class này — tương đương gần nhất là `DevServerRelayBridge`/`dev-server-manager.ts` | ⚠️ Không xác nhận | Audit chưa xác nhận tên class `RelayConnectionPool` tồn tại 1:1 |
| WorkspaceContext | `frontend/src/renderer/src/context/WorkspaceContext.tsx` (đường dẫn package đã tách) | ✅ Done (⚠️ có bug) | Tab "Agent" trong `WorkspaceLayout` không render nội dung (dead tab, F38) |
| git-handler (relay) | `agent/src/relay/git-handler.ts` (`GitHandler` — package `agent/`, không phải `src/relay/`) | ✅ Done | 35+ RPC methods; có 3 lớp thực thi git/gh song song không thống nhất (xem §C4.11a) |

---

## C4.11 — Dev Server Agent Module Map (v6.0 NEW)

> 🚧 **Toàn bộ §C4.11 mô tả kiến trúc "v6.0" đề xuất trong `docs/adrs/v2/ADR-017-dev-server-agent-layer-model.md` (và ADR-013/014/015/018/019) — CHƯA ĐƯỢC TRIỂN KHAI.** Chính ADR-017 tự ghi `❌ src/agent/ package chưa tồn tại` (dòng 221), và các ADR liên quan tự đánh dấu `❌ Chưa implement (v6.0 proposed)`. Xác nhận bằng grep trực tiếp trên code (`audit/agent/connection-wire-protocol-vs-design-review.md` §2.7, `audit/agent/rpc-dispatch-lifecycle-vs-design-review.md` §0, §2.10): **không có `ContextVerifier`, `SignedExecutionContext`, field `_ctx`, `ReconnectManager`, hay bất kỳ file nào dưới `src/agent/{rpc,pty,worktree,git,fs,execution,storage,reporting}/` ở bất kỳ đâu trong repo.** Package agent thật (`agent/src/relay/*`) có cấu trúc hoàn toàn khác — flat, không phân lớp A0-A4 — mô tả đầy đủ ở **§C4.11a** ngay sau bảng "Module Implementation Status" bên dưới. Đọc §C4.11 dưới đây như một **roadmap/proposal**, không phải tài liệu tham chiếu cho code hiện tại.

### src/agent/ — Dev Server Agent source structure (🚧 PROPOSED, chưa implement — xem banner trên)

```
src/agent/
│
├── index.ts                         # Agent entry point (start command)
├── config.ts                        # Config loader (config.yaml + env)
│
├── # RPC & Protocol Layer
├── rpc/
│   ├── agent-rpc-server.ts          # JSON-RPC 2.0 over WebSocket
│   ├── context-verifier.ts          # HMAC-SHA256 signed context verification
│   ├── method-router.ts             # Dispatch RPC methods to handlers
│   ├── event-emitter.ts             # Stream events back to Gateway
│   └── reconnect-manager.ts        # Exponential backoff reconnect
│
├── # PTY & Terminal
├── pty/
│   ├── pty-manager.ts               # PTY lifecycle (create/resize/write/kill)
│   ├── pty-session-store.ts         # Per-userId session registry
│   ├── pty-output-streamer.ts       # Chunk PTY output → event.stream
│   └── pty-state-persistence.ts    # SQLite: save/restore PTY state
│
├── # AI Agent Spawning
├── agent-spawn/
│   ├── profile-aware-agent-spawner.ts  # Main spawner (profile inject)
│   ├── agent-env-builder.ts            # Build env from resolvedProfile
│   ├── agent-model-validator.ts        # Check approvedModels whitelist
│   ├── agent-state-detector.ts         # OSC parsing: idle/running/waiting
│   ├── agent-usage-tracker.ts          # Track tokens per accountId
│   └── agent-session-resume.ts         # Session ID for --resume
│
├── # Worktree
├── worktree/
│   ├── worktree-engine.ts           # git worktree add/remove/list
│   ├── worktree-fanout.ts           # Fan-out: N worktrees + spawn agents
│   ├── worktree-recovery.ts         # Detect + recover orphaned worktrees
│   └── worktree-store.ts            # SQLite: worktree metadata
│
├── # Git Operations
├── git/
│   ├── git-engine.ts                # Core: status, diff, add, commit, push
│   ├── git-stream.ts                # Stream git output → events
│   ├── git-author-injector.ts       # Inject ctx.userEmail as git author
│   ├── git-user-isolation.ts        # Per-userId GH_CONFIG_DIR
│   ├── git-pr-creator.ts            # gh CLI: PR creation
│   └── git-commit-message-ai.ts    # LLM commit message generation
│
├── # File System
├── fs/
│   ├── fs-engine.ts                 # readDir, readFile, writeFile
│   ├── secure-fs.ts                 # SecureFs: path validation + traversal prevention
│   ├── fs-watcher.ts                # chokidar watch → event.fsChange
│   └── fs-search.ts                 # ripgrep wrapper → event stream
│
├── # AI Provider Credentials
├── ai-providers/
│   ├── credential-store.ts          # AES-256-GCM .enc file management
│   ├── credential-writer.ts         # Write encrypted credential from relay
│   ├── credential-reader.ts         # Decrypt credential for agent spawn
│   ├── provider-health-checker.ts   # Test connection to provider API
│   └── provider-key-deriver.ts     # scrypt(ORCA_AI_CREDENTIAL_KEY + accountId)
│
├── # Workflow Step Execution
├── step-executor/
│   ├── step-executor.ts             # Main dispatcher per step type
│   ├── agent-step-executor.ts       # type='agent': spawn AI agent
│   ├── shell-step-executor.ts       # type='shell': execFile (no injection)
│   ├── action-step-executor.ts      # type='action': git, github ops
│   ├── step-output-streamer.ts      # event.stepOutput chunks
│   └── step-state-store.ts          # SQLite: execution state persistence
│
├── # Ephemeral VM (F18)
├── ephemeral-vm/
│   ├── vm-runtime.ts                # VM lifecycle (create, run, destroy)
│   ├── vm-recipe-executor.ts        # YAML recipe runner
│   └── vm-docker-adapter.ts         # Docker/OCI container runtime
│
├── # Health & Observability
├── health/
│   ├── health-reporter.ts           # Collect + emit metrics every 60s
│   ├── metrics-collector.ts         # CPU, RAM, disk, network, latency
│   ├── diagnostic-server.ts         # localhost:6790 /health endpoint
│   └── audit-logger.ts             # Append-only local audit log
│
├── # Local Storage
├── db/
│   ├── agent-db.ts                  # better-sqlite3 init + migrations
│   ├── migrations/
│   │   ├── A001_init.ts             # agent_worktrees, agent_sessions
│   │   ├── A002_tasks.ts            # agent_task_runs, agent_step_runs
│   │   └── A003_audit.ts            # agent_audit_log
│   └── repositories/
│       ├── worktree-repo.ts
│       ├── session-repo.ts
│       └── task-run-repo.ts
│
└── # Event Bus
    └── event-bus.ts                 # Internal EventEmitter + buffer logic
```

---

## C4.11a — Real Dev Server Agent Structure (`agent/src/`) — as implemented today

✅ Đây là cấu trúc **thật** của package `agent/` (isolated copy, split from monorepo — `agent/package.json:4`), đối chiếu trực tiếp qua 6 audit `audit/agent/*.md` (2026-08-08). Entry point build thật là `agent/src/relay/agent-entry.ts` → `out/agent.js` (WS transport, modes `direct-websocket`/`relay-websocket`); `agent/src/relay/relay.ts` là daemon khác (SSH-exec transport, deploy qua SCP) — **không có build pipeline xác nhận được** trong `agent/build.mjs` hiện tại (chỉ build `agent-entry.ts`).

```
agent/src/
│
├── relay/                              # ✅ Package chính — flat, KHÔNG phân lớp A0-A4 như §C4.11 đề xuất
│   ├── agent-entry.ts                  # Entry point WS agent.js — modes: direct-websocket / relay-websocket
│   ├── agent-config.ts                 # AgentConnectionMode, agentPort mặc định 6799, ORCA_URL
│   ├── agent-connection-direct.ts      # direct-websocket: reconnect backoff [1s,2s,5s,15s,30s] (khác ADR-019's 1-2-4-8-16-30s)
│   ├── agent-connection-relay.ts       # relay-websocket: agent là WS server tại path /orca-relay
│   ├── agent-session.ts                # Handshake (agent.handshake tự gửi, KHÔNG có nonce/HMAC), wire framing
│   ├── agent-wire.ts                   # encodeDataFrame/encodeKeepaliveFrame (13-byte header)
│   ├── agent-token-manager.ts          # AgentTokenManager — self-renewal 80% TTL, pre-fetch — KHÔNG phải
│   │                                   #   admin-UI+DB token model; token dạng agt-<devServerId>-<timestamp>
│   ├── agent-rpc-dispatch.ts           # 🔑 RPC DISPATCH ENTRY POINT THẬT — route() dòng 259-855, ~40 method
│   │                                   #   (xem bảng namespace thật ở §C4.11b ngay dưới)
│   ├── agent-spawner.ts                # SubAgentSpawner — agent.spawn/kill/sendInput, PTY_REGISTRY,
│   │                                   #   buildAgentEnv() (KHÔNG đọc OrcaProfile — profile injection
│   │                                   #   agent-side là dead code), resolveAgentSpec() chỉ 5 model family
│   │                                   #   (claude/codex/gemini/opencode/ollama, KHÔNG phải 30+ agent của F04)
│   ├── agent-exec-handler.ts           # ⚠️ chứa handleAgentExec() — ❌ DEAD CODE, không case nào gọi tới nó;
│   │                                   #   bản SỐNG thật là inline case 'agent.exec' trong agent-rpc-dispatch.ts:594-624
│   ├── agent-credential-store.ts       # ✅ SỐNG — ~/.orca/credentials/<accountId>.enc, AES-256-GCM,
│   │                                   #   salt ngẫu nhiên/write (không phải salt=accountId như thiết kế)
│   ├── ai-provider-handler.ts          # ❌ DEAD CODE — 0 caller (GitNexus impactedCount:0); path đúng thiết kế
│   │                                   #   (~/.orca/ai-providers/) nhưng không ai gọi, và tự claim sai về mã hoá at-rest
│   ├── ai-complete-handler.ts          # ai.complete — resolveApiKey() CHỈ đọc process.env, KHÔNG lookup credential store
│   ├── git-handler.ts                  # GitHandler — git engine chính, git.exec/execStream, 35+ method,
│   │                                   #   dùng GitCapabilityCache đúng pattern AGENTS.md (điểm sáng nhất)
│   ├── agent-git-handler.ts            # handleGitPrCreate → case 'git.pr.create' — ⚠️ TRÙNG LẶP với dưới
│   ├── external-api-connector.ts       # handleGitHubPrCreate → case 'github.pr.create' — có idempotency-check
│   │                                   #   ⚠️ 2 implementation PR-create khác nhau cùng tồn tại (git.pr.create
│   │                                   #   vs github.pr.create) — giữ 1, deprecate cái kia
│   ├── pty-handler.ts                  # pty.create/attach/write/resize/destroy/scrollback/sendSignal —
│   │                                   #   LUÔN spawn SHELL, không spawn agent binary trực tiếp
│   ├── fs-handler.ts                   # fs.* dispatch (SSH-exec/relay.ts transport) → RelayFilesystemWatchRegistry
│   │                                   #   → cluster @parcel/watcher (crash-isolation, quarantine) — KHÔNG nhắc trong HLD
│   ├── fs-agent-extensions.ts          # fs.watch cho WS/agent.js transport — dùng node:fs built-in, KHÔNG @parcel/watcher
│   ├── agent-hook-server.ts            # RelayAgentHookServer — nhận POST hook JSON, forward notification 'agent.hook'
│   ├── context.ts                      # RelayContext.registerRoot() RỖNG CÓ CHỦ ĐÍCH — FS allowlist đã bị
│   │                                   #   gỡ bỏ chủ động (xem docs/relay-fs-allowlist-removal.md); KHÔNG có
│   │                                   #   ContextVerifier/_ctx nào ở đây — trust boundary dồn hết lên renderer/SSH-user
│   └── relay-command-env.ts            # buildRelayGitEnv() — KHÔNG đi qua main/git/runner.ts (xem dưới)
│
├── main/
│   ├── ssh/                            # Phía relay/multiplexer CHẠY TRÊN remote host — KHÔNG phải phía
│   │   ├── relay-protocol.ts           #   thiết lập kết nối SSH (auth/config/reconnect nằm ở backend|desktop)
│   │   │                               #   HEADER_LENGTH=13, MessageType.Regular=1/KeepAlive=9 — khớp thiết kế
│   │   ├── ssh-channel-multiplexer.ts  #   Keepalive 5s / timeout 20s (không phải 30s/90s)
│   │   ├── ssh-remote-platform.ts
│   │   ├── ssh-filesystem-stream-reader.ts
│   │   └── ssh-git-response-stream-reader.ts
│   ├── git/
│   │   ├── runner.ts                   # ghExecFileAsync/glabExecFileAsync — circuit breaker + retry + WSL
│   │   │                               #   routing tinh vi — ⚠️ CHỈ 1 caller trong toàn agent/ (commit-message
│   │   │                               #   generator); GitHandler/agent-git-handler/external-api-connector
│   │   │                               #   ĐỀU BYPASS module này — bảo vệ rate-limit không áp dụng cho PR/MR thật
│   │   └── gh-rate-limit-breaker.ts    # Circuit breaker cho `gh` CLI — chỉ dùng bởi runner.ts (xem trên)
│   ├── agent-hooks/
│   │   ├── server.ts                   # AgentHookServer — hook receiver chính, dedupe/anti-flicker phức tạp
│   │   └── migration-unsupported-pty-state.ts  # ⚠️ tên gây hiểu nhầm — KHÔNG liên quan DB/version migration,
│   │                                            #   chỉ là event bus nhỏ theo dõi PTY không hỗ trợ hook-status
│   ├── profile/OrcaProfile.ts          # ❌ type-only, DEAD CODE trong agent/ — không hàm runtime nào đọc nó
│   ├── codex-cli/command.ts            # ⚠️ tên gây hiểu nhầm — KHÔNG phải logic riêng Codex, là resolver
│   │                                   #   PATH/version-manager dùng chung cho claude+codex (vi phạm AGENTS.md
│   │                                   #   "File and Module Naming")
│   ├── providers/                      # ⚠️ TRÙNG TÊN, nội dung khác hẳn backend|desktop/src/main/providers/
│   │                                   #   (Gateway-side IPtyProvider/IFilesystemProvider/IGitProvider registry
│   │                                   #   của §15 KHÔNG có bản trong agent/) — đây là Windows foreground-process
│   │                                   #   detection cho agent status recognition
│   ├── observability/{tracer,redactor,instrumentation}.ts   # ❌ DEAD CODE, KHÔNG có thiết kế — code tự trích dẫn
│   │                                                         #   "telemetry-error-tracking.md" — file này không tồn tại
│   ├── telemetry/*                     # ❌ DEAD CODE — initTelemetry 0 caller trong agent/; import electron's
│   │                                   #   `app` — leftover copy từ desktop/, không hợp lý cho Dev Server headless
│   ├── diagnostics/main-thread-churn-probe.ts   # ❌ DEAD CODE — không caller nào kể cả entry point
│   ├── ipc/                             # Cluster parcel-watcher (runtime-watcher-process-pool,
│   │                                   #   parcel-watcher-process-supervisor, filesystem-watcher-event-batch)
│   │                                   #   — chỉ phục vụ relay.ts/SSH-exec transport, KHÔNG dùng bởi agent.js/WS
│   └── {wsl,git-bash,pwsh,win32-utils}.ts   # Cross-platform shims — đúng vai trò nhưng CÔ LẬP khỏi GitHandler
│                                              #   (chỉ dùng bởi runner.ts, vốn cũng gần như dead — xem trên)
│
└── shared/
    ├── agent-wire-protocol.ts          # Frame constants, AGENT_MIN_VERSION (hằng số CHẾT — không đâu so sánh),
    │                                   #   AgentErrorCodes (JSON-RPC -32xxx chuẩn + agent-specific -33001..-33101)
    ├── git-capability-cache.ts          # GitCapabilityCache — pattern AGENTS.md ✅, dùng bởi GitHandler
    ├── agent-hook-relay.ts             # AGENT_HOOK_NOTIFICATION_METHOD = 'agent.hook'
    ├── agent-status-types.ts           # AgentStatusState: working/blocked/waiting/done (4-state, hook-based)
    ├── agent-title-core.ts             # AgentStatus: working/permission/idle (3-state, OSC-title fallback)
    ├── types.ts                        # TuiAgent union — 32 agent (terminal-injection roster, khác 5-model AGENT_SPECS)
    └── tui-agent-config.ts             # TUI_AGENT_CONFIG — launch = gõ lệnh vào shell PTY, không spawn binary trực tiếp
```

**3 state machine "agent status" song song, không hợp nhất** (`AgentLifecycleState` 6-state PTY-process ở `agent-spawner.ts:46`, `AgentStatusState` 4-state hook-based, `AgentStatus` 3-state OSC-title) — không cái nào khớp state machine 5-state mà §C4.11/HLD §11.3 mô tả như một model thống nhất duy nhất.

**Xác thực kết nối agent thật (thay cho §C4.5's mô tả cũ hơn):** bearer-token tĩnh dạng `agt-<devServerId>-<timestamp>` (`generateAgentToken()`, `agent/src/shared/agent-wire-protocol.ts:89-91`), agent tự gia hạn qua `POST /api/agent-token` (auth bằng `ORCA_AGENT_API_SECRET`), hash SHA-256 trước khi so khớp phía Backend — **không có** `SignedExecutionContext`/HMAC per-request/`ContextVerifier` nào như §C4.11 mô tả.

---

## C4.11b — RPC Method Surface thật (`agent/src/relay/agent-rpc-dispatch.ts:259-855`)

Toàn bộ `case` thật trong `route()` (~40 method), theo `audit/agent/rpc-dispatch-lifecycle-vs-design-review.md` §2.2:

```
tools/list, tools/call,                                              # MCP layer — không tài liệu hoá ở đâu khác
git.exec, git.execStream, git.pr.create, git.worktree.list/add/remove,
fs.readDir, fs.readFile, fs.grep, fs.stat, fs.glob, fs.writeFile, fs.mkdir, fs.rmdir, fs.watch, fs.unwatch,
ai.provider.writeCredential, ai.provider.readCredential, ai.provider.healthCheck, ai.provider.deleteCredential,
preflight.check,
github.pr.create, github.pr.merge, github.issue.list, github.issue.create,
gitlab.mr.create, gitlab.pipeline.status,
agent.spawn, agent.kill, agent.sendInput, agent.exec,
ai.complete, shell.eval,
pty.create, pty.attach, pty.write, pty.resize, pty.destroy, pty.scrollback, pty.sendSignal
```

Khác biệt hệ thống so với mọi bảng RPC method trong §C4.5/§C4.9/§C4.11 phía trên: `git.exec`/`git.execStream` là **generic passthrough** (Orca gửi cả câu lệnh git, agent chỉ exec) thay vì method chi tiết theo từng operation; namespace credential là `ai.provider.*` (dot-separated), không phải `aiProvider.*` camelCase liền; **hoàn toàn không có `step.*`/`health.*`** (CR-DS-003 coi đây là nền tảng cho F36/F27 nhưng chưa từng implement); có sẵn lớp giao thức MCP (`tools/list`/`tools/call`) không được nhắc tới ở bất kỳ tài liệu HLD/ADR/CR nào khác.

---

### Key Data Flows (C4.11)

> 🚧 Toàn bộ 3 flow dưới đây (A11.1-A11.3) mô tả kiến trúc "v6.0" đề xuất — **chưa có dòng code nào trong `agent/src` triển khai `ContextVerifier`, `EventBus` class, `StepExecutor`, hay `AgentConnectionManager`/`SignedContextIssuer` phía Gateway.** Đối chiếu thật: xem `agent-rpc-dispatch.ts` (§C4.11a/§C4.11b) — request/response là JSON-RPC 2.0 thường không có `_ctx`; event là JSON-RPC notification gửi trực tiếp qua `ws.send`/`dispatcher.notify`, không qua class `EventBus` riêng.

#### Flow A11.1: PTY Session via Gateway

```
UI (browser tab)
  → WS :6768 → Gateway WsSessionRouter
  → AgentConnectionManager.dispatch(agentId, 'pty.create', signedCtx)
  → [wss] → Agent RpcServer
  → ContextVerifier.verify(signedCtx) [HMAC check]
  → PtyManager.create(params, ctx) [create PTY, bind userId]
  → Return { ptyId } → Gateway → UI

UI types in terminal
  → RPC 'pty.write' { ptyId, data }
  → Agent PtyManager.write(ptyId, data, ctx)
  → PTY stdin → output to stdout
  → PtyOutputStreamer → EventBus → event.stream { ptyId, data }
  → [wss] → Gateway EventFanout → UI (xterm.js)
```

#### Flow A11.2: AI Agent Spawn with Profile

🚧 Toàn bộ flow này là đề xuất. Thật ra `agent.spawn` (`agent/src/relay/agent-spawner.ts`, có thật, xem §C4.11a) nhận `{ taskId, userId, modelId, accountId, cwd, resumeId, worktreePath, branchName }` — **không có `ContextVerifier.verify()`, không có `AgentModelValidator`/approvedModels check nào trước khi spawn** (`buildAgentEnv()` không check gì cả — lỗ hổng bảo mật thật, `audit/agent/rpc-dispatch-lifecycle-vs-design-review.md` §2.3). `cols`/`rows` PTY luôn hardcode `220×50`, không nhận từ caller. Response không mang `ptyId` đồng bộ qua request `id` gốc (dispatcher dùng `void handleAgentSpawn(...)` fire-and-forget) — `ptyId` chỉ về qua notification `agent.output`/`agent.exited` sau đó.

```
User clicks "Start Agent" on task
  → Gateway: resolve profile (F33 ProfileResolver)
  → Gateway: resolve provider (F35 ProviderResolver → accountId)
  → Gateway: sign context { userId, projectRoot, resolvedProfile, providerAccountId }   # ❌ không tồn tại
  → RPC 'agent.spawn' → Agent
  → ContextVerifier.verify(ctx)                                    # ❌ không tồn tại — không verify gì cả
  → AgentModelValidator.check(ctx.agentSettings.approvedModels, model)  # ❌ không tồn tại
  → CredentialReader.decrypt(providerAccountId) → API key           # ⚠️ tên khác: readDecryptedKey() trong
  │                                                                  #   agent-credential-store.ts; luồng spawn
  │                                                                  #   thường nhận thẳng resolvedApiKey PLAINTEXT
  │                                                                  #   từ Gateway qua params — mâu thuẫn với lời
  │                                                                  #   hứa "Gateway không thấy plaintext" (§C4.6)
  → AgentEnvBuilder.build(ctx) → env { PATH, envVars, ANTHROPIC_API_KEY, GH_CONFIG_DIR }  # ⚠️ tên thật buildAgentEnv();
  │                                                                  #   KHÔNG set PATH/pathAdditions hay ANTHROPIC_MODEL
  → PtyManager.create(workdir, shell)                                # ⚠️ tên thật PTY_REGISTRY trong agent-spawner.ts
  → spawn AI agent in PTY with env                                   # cols/rows hardcode 220×50
  → AgentStateDetector monitors OSC events                           # ⚠️ cơ chế chính thật là hook-JSON POST
  │                                                                   #   (agent.hook notification), OSC chỉ fallback
  → EventBus.emit('agentStatus', { status: 'running' })               # ❌ không có class EventBus — notification
  │                                                                   #   JSON-RPC trực tiếp qua ws.send
  → [wss] → Gateway → NotificationService + UI update
```

#### Flow A11.3: Workflow Step Dispatch

❌ **`step.execute`/`step.*` hoàn toàn không tồn tại trong `agent/src`** (grep 0 kết quả, `audit/agent/rpc-dispatch-lifecycle-vs-design-review.md` §2.2/§2.10) — không có `StepExecutor` class nào phía agent. Đây là toàn bộ nhóm RPC method còn thiếu mà CR-DS-003 coi là nền tảng cho F36 Workflow, khớp với phát hiện ở §5.15/F36 rằng pause/resume và provider-selection theo step chưa triển khai ở tầng backend.

```
WorkflowOrchestrator (Gateway) resolves next step
  → StepServerResolver: project:vnp-blc → agentId
  → ProviderResolver: server:anthropic-default → accountId
  → Sign context { userId, projectId, projectRoot, providerAccountId }   # ❌ không tồn tại
  → RPC 'step.execute' → Agent                                           # ❌ method không tồn tại trong agent/
  → StepExecutor.dispatch(stepType, definition, ctx)                     # ❌ class không tồn tại
    - type='agent' → AgentStepExecutor → ProfileAwareAgentSpawner
    - type='shell' → ShellStepExecutor → execFile(command, args, { env })
    - type='action' → ActionStepExecutor → git, github ops
  → EventBus.emit('stepOutput', chunks) → Gateway stream → UI            # ❌ không streaming step output thật (F36)
  → StepStateStore.save(stepRunId, state)                                # ❌ không tồn tại phía agent
  → On complete: EventBus.emit('stepComplete', { output })
  → [wss] → Gateway → WorkflowOrchestrator.onStepComplete(stepId, output)
  → Continue with next DAG wave
```

### Module Implementation Status (v6.0)

✅ **Bảng dưới đây (khác với bảng "v5.0 New Modules Checklist" ở trên) ĐÚNG như hiện trạng** — đã xác nhận qua audit rằng tất cả các module này thật sự chưa tồn tại (`❌ TODO` là chính xác), không cần sửa. Xem §C4.11a cho module tương đương thật sự đang chạy trong `agent/src/relay/*`.

| Module | File | Status | Priority |
|--------|------|--------|---------|
| AgentRpcServer | `src/agent/rpc/agent-rpc-server.ts` | ❌ TODO | P0 |
| ContextVerifier | `src/agent/rpc/context-verifier.ts` | ❌ TODO | P0 |
| ReconnectManager | `src/agent/rpc/reconnect-manager.ts` | ❌ TODO | P0 |
| PtyManager (agent) | `src/agent/pty/pty-manager.ts` | ❌ TODO | P0 |
| ProfileAwareAgentSpawner | `src/agent/agent-spawn/profile-aware-agent-spawner.ts` | ❌ TODO | P0 |
| AgentEnvBuilder | `src/agent/agent-spawn/agent-env-builder.ts` | ❌ TODO | P0 |
| WorktreeEngine | `src/agent/worktree/worktree-engine.ts` | ❌ TODO | P0 |
| GitEngine | `src/agent/git/git-engine.ts` | ❌ TODO | P0 |
| SecureFs | `src/agent/fs/secure-fs.ts` | ❌ TODO | P0 |
| FsEngine | `src/agent/fs/fs-engine.ts` | ❌ TODO | P0 |
| CredentialStore | `src/agent/ai-providers/credential-store.ts` | ❌ TODO | P0 |
| StepExecutor | `src/agent/step-executor/step-executor.ts` | ❌ TODO | P1 |
| ShellStepExecutor | `src/agent/step-executor/shell-step-executor.ts` | ❌ TODO | P1 |
| HealthReporter | `src/agent/health/health-reporter.ts` | ❌ TODO | P1 |
| AgentDb | `src/agent/db/agent-db.ts` | ❌ TODO | P0 |
| EventBus | `src/agent/event-bus.ts` | ❌ TODO | P0 |

### Gateway Changes (v6.0)

🚧 Bảng dưới đây cũng là đề xuất chưa triển khai — `AgentConnectionManager`/`SignedContextIssuer`/`AgentDispatcher` không tồn tại trong `backend/src/main`; `relay-websocket-client.ts`/`DevServerRelayBridge` (§C4.5) vẫn là cơ chế đang chạy thật.

| Module | File | Change |
|--------|------|--------|
| AgentConnectionManager | `src/main/dev-server/agent-connection-manager.ts` | NEW — replaces relay-websocket-client |
| SignedContextIssuer | `src/main/dev-server/signed-context-issuer.ts` | NEW — HMAC-SHA256 context signing |
| AgentDispatcher | `src/main/dev-server/agent-dispatcher.ts` | NEW — routes RPC to correct agent |
| WorkflowOrchestrator | `src/main/workflow/WorkflowOrchestrator.ts` | MODIFY — use AgentDispatcher |
| TaskAgentExecutor | `src/main/task/TaskAgentExecutor.ts` | MODIFY — use AgentDispatcher |
| ProfileAwareAgentSpawner (gateway) | `src/main/project/ProfileAwareAgentSpawner.ts` | MODIFY — delegate to AgentDispatcher |

