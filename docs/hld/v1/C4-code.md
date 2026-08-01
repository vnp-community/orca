# C4 — Code Level: Key Modules và Data Flows

**Level:** 4 — Code  
**Mô tả:** Các module quan trọng và data flows chi tiết  
**Cập nhật:** 2026-07-28 (thêm Platform, DB, Fleet, AgentWS, Credential, Profile+Project modules)

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
│   └── migrations/
│       ├── types.ts                 # Migration interface
│       ├── runner.ts                # MigrationRunner (sequential apply)
│       ├── index.ts                 # Migration registry
│       ├── 0001_init.ts             # projects, worktrees, agent_sessions
│       ├── 0002_sessions.ts         # terminal_scrollback_snapshots
│       ├── 0003_ssh_targets.ts      # ssh_hosts, saved_port_forwards
│       ├── 0004_automations.ts      # automations, automation_runs, notifications
│       └── 0005_auth_schema.ts      # orca_users, orca_sessions, orca_audit_log
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

| Capability | ElectronAdapter | NodeAdapter |
|-----------|-----------------|-------------|
| `app.getPath('userData')` | `electron.app.getPath()` | `~/.orca` (env override) |
| `window.show()` | `BrowserWindow.show()` | noop |
| `ipc.handle()` | `ipcMain.handle()` | WS JSON-RPC register |
| `storage.encrypt()` | `safeStorage.encryptString()` | AES-256-GCM file |
| `notification.show()` | `Notification` (Electron) | `console.log` |

### Web Server Bootstrap Flow (CR-004)

```
src/server/index.ts
    │
    ├── new NodeAdapter({ userDataPath: process.env.ORCA_USER_DATA_PATH })
    ├── bootstrapWebApp(nodeAdapter)
    │        │
    │        ├── AuthManager.init(db)
    │        ├── SessionManager.init()
    │        ├── AgentWebSocketServer.init()
    │        └── FleetHealthMonitor.start()
    ├── express app: HTTP :6769
    │        ├── POST /auth/local
    │        ├── GET  /admin/api/*  (requireAdmin)
    │        ├── GET  /health/ready
    │        └── GET  /health/metrics (Prometheus)
    └── ws server: WS :6768
             ├── /          ← browser RPC (WsSessionRouter)
             └── /agent     ← agent connections (AgentWebSocketServer)
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
    └── For each pending migration (0001 → 0005):
             ├── BEGIN TRANSACTION
             ├── migration.up(db)  ← dialect-aware DDL
             ├── INSERT orca_migrations (id, name, applied_at)
             └── COMMIT
             (ROLLBACK on error)
```

### Repository Factory (src/main/repositories/factory.ts)

```typescript
// ORCA_STORAGE_BACKEND = 'json' | 'sql' (default: 'json' for desktop)
function createRepository(env: string): IStateRepository {
  if (env === 'sql') {
    const pool = DatabaseProvider.createPool(parseDsn(process.env.ORCA_DB_URL))
    return new SqlRepository(pool)
  }
  return new JsonFileRepository(getDataFile('orca-data.json'))
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

```
orca fleet provision --project backend --concurrency 5
    │
    ├── Load fleet.yaml → FleetConfigParser.parse()
    ├── Filter servers by project
    │
    └── For each server (parallel, concurrency=5):
             ├── SSH connect (SshConnection)
             ├── FleetBootstrapService.bootstrap(server)
             │       ├── Check Node.js ≥ 22
             │       ├── Check Git ≥ 2.25
             │       ├── Check disk ≥ 5GB
             │       ├── SFTP upload relay binary
             │       ├── Verify SHA256
             │       └── Start relay daemon
             ├── FleetHealthMonitor.check(server) → initial health
             └── Update server status: 'online' | 'degraded' | 'unhealthy'
```

### Fleet Health Status Model

| Status | Condition |
|--------|-----------|
| `healthy` | SSH ✔, relay ✔, CPU<80%, RAM<85% |
| `degraded` | Relay ✔ but CPU>80% or RAM>85% |
| `unhealthy` | SSH ✔ but relay ✘ |
| `unreachable` | SSH connect timeout/fail |

### RBAC Policy Resolution (CR-006, Phase 1)

```typescript
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
               0x09 = KeepAlive (empty payload, every 30s)

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

```
DevServer config: connectionType = 'direct-websocket'
Orca: AgentWebSocketServer listening on ws://orca:6768/agent

Agent (Python/Go/TypeScript) → WebSocket connect:
    │
    ├── GET /agent (HTTP Upgrade)
    ├── Orca → { type: 'handshake-request' }
    ├── Agent → { type: 'agent.handshake',
    │              agentToken: 'raw-token',
    │              name: 'my-agent', version: '1.0.0' }
    ├── AgentTokenManager.validate(rawToken)
    │       └── SHA-256(rawToken) == storedHash ? OK : close(4001)
    ├── Orca → { type: 'handshake-ok', sessionId: 'sess-xxx' }
    │
    └── WsTransport ⇔ SshChannelMultiplexer ⇔ JSON-RPC (bidirectional)

Close codes: 4001=Unauthorized, 4002=HandshakeTimeout, 4003=VersionMismatch
```

---

## C4.6 — Integration & Credential Layer Detail

### WebCredentialStore Encryption (CR-INT-004)

```
Encryption at rest:

  masterKey = scryptSync(
    userId + ':' + ORCA_CREDENTIAL_KEY,   // input
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

| Category | Integration | Storage | Auth Method |
|----------|------------|---------|-------------|
| **A — CLI-based** | GitHub (`gh`) | Dev Server `~/.config/gh/<userId>/` | `gh auth login` (PTY) |
| **A — CLI-based** | GitLab (`glab`) | Dev Server `~/.config/glab/<userId>/` | `glab auth login` (PTY) |
| **B — HTTP API** | Bitbucket | Orca Server `WebCredentialStore` | App password |
| **B — HTTP API** | Azure DevOps | Orca Server `WebCredentialStore` | PAT token |
| **B — HTTP API** | Gitea | Orca Server `WebCredentialStore` | API token |
| **C — File token** | Linear | Orca Server `WebCredentialStore` | API key |
| **C — File token** | Jira | Orca Server `WebCredentialStore` | Basic auth token |

### Preflight Check Merge Flow (CR-INT-005, CR-GH-005)

```
RPC: preflight.check { devServerId }
    │
    ├── runLocalChecks():
    │       ├── git version (local)
    │       └── API token format (Category B+C)
    │
    ├── runRelayChecks(devServerId) via SSH:
    │       ├── GH_CONFIG_DIR=~/.config/gh/<userId>/ gh auth status
    │       ├── GLAB_CONFIG_DIR=~/.config/glab/<userId>/ glab auth status
    │       ├── node --version
    │       └── disk space check
    │
    └── mergePreflightStatuses(localResults, relayResults)
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
       ├─→ [AgentOrchestrator.start(worktreeId, agentType)]
       │         │
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
       │  node-pty.spawn(agentBinary, args, {cwd: remoteWorktreePath})
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
       │  4. Encode as QR data: { pubKey, host, port, token }
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
       │  { type: 'agent:completed', worktree: '...', summary: '...' }
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

```typescript
// relay-protocol.ts
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

Relay intercepts agent tool calls bằng cách redirect HTTP server:

```
Agent Process (e.g., Claude Code)
       │
       │  HTTP tool calls (file read, bash exec, etc.)
       │  → env CLAUDE_CODE_HOOKS_URL=http://localhost:PORT
       ▼
[Relay/agent-hook-server.ts] HTTP Interceptor
       │
       ├── File read/write → relay fs-handler (route qua WebSocket)
       ├── Bash exec → relay pty-handler (PTY subprocess)
       └── Other → pass-through to original handler
       │
       ▼
[Desktop/agent-hooks/] Receive hook events
       │
       ▼
[Renderer] Display in "Agent Activity" panel
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

### DB Schema additions (Migration 0006)

```sql
CREATE TABLE orca_company (
  id   TEXT PRIMARY KEY DEFAULT 'default',
  name TEXT NOT NULL,
  logo_url TEXT,
  profile_json TEXT DEFAULT '{}',
  created_at INTEGER,
  updated_at INTEGER
);

CREATE TABLE orca_departments (
  id         TEXT PRIMARY KEY,
  company_id TEXT NOT NULL REFERENCES orca_company(id),
  name       TEXT NOT NULL,
  team_lead_id TEXT REFERENCES orca_users(id),
  profile_json TEXT DEFAULT '{}',
  created_at INTEGER,
  updated_at INTEGER
);

-- orca_users: thêm 2 columns
ALTER TABLE orca_users ADD COLUMN department_id TEXT REFERENCES orca_departments(id);
ALTER TABLE orca_users ADD COLUMN profile_json TEXT DEFAULT '{}';
```

### RPC methods — profile.*

```typescript
// src/main/runtime/rpc/methods/profile.ts
'profile.getEffective'    // (userId) → ResolvedProfile (cached)
'profile.updateUser'      // (fields: Partial<OrcaProfile>) — personal only
'profile.getDepartment'   // (deptId) → OrcaProfile
'profile.updateDepartment'// (deptId, fields) — lead/admin
'profile.getCompany'      // () → OrcaProfile — admin
'profile.updateCompany'   // (fields) — admin only
'profile.listDepartments' // () → Department[]
'profile.createDepartment'// (name) — admin
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
├── profile-aware-agent-spawner.ts # Combine profile + project → spawn agent
│                                  # resolveProfile(userId) → agentEnv
│                                  # resolveAgentBinary(model), buildAgentArgs(trust)
│                                  # GH_CONFIG_DIR / GLAB_CONFIG_DIR injection
│                                  # relay.call('pty.spawn', {...})
│
└── project-context-injector.ts   # buildProjectContext(project, user, worktree)
                                   # Returns preamble string for agent init
```

### DB Schema additions (Migration 0007)

```sql
CREATE TABLE orca_projects (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL UNIQUE,
  description   TEXT,
  repo_url      TEXT,
  repo_path     TEXT NOT NULL,          -- path on dev server
  dev_server_id TEXT REFERENCES ssh_hosts(id),  -- THE BINDING
  default_branch TEXT DEFAULT 'main',
  tags          TEXT DEFAULT '[]',      -- JSON array
  created_by    TEXT REFERENCES orca_users(id),
  created_at    INTEGER,
  updated_at    INTEGER
);

CREATE TABLE orca_project_members (
  project_id  TEXT REFERENCES orca_projects(id) ON DELETE CASCADE,
  user_id     TEXT REFERENCES orca_users(id)    ON DELETE CASCADE,
  role        TEXT DEFAULT 'developer',  -- developer | lead | admin
  joined_at   INTEGER,
  PRIMARY KEY (project_id, user_id)
);

CREATE INDEX idx_project_members_user ON orca_project_members(user_id);
CREATE INDEX idx_projects_devserver ON orca_projects(dev_server_id);
```

### RPC methods — projects.*

```typescript
// src/main/runtime/rpc/methods/projects.ts
'projects.list'           // (userId?) → OrcaProject[] (filtered by membership)
'projects.get'            // (projectId) → OrcaProject
'projects.create'         // (input: CreateProjectInput) — lead/admin
'projects.update'         // (projectId, fields) — lead/admin
'projects.delete'         // (projectId) — admin only
'projects.updateBinding'  // (projectId, devServerId) — admin
'projects.addMember'      // (projectId, userId, role) — lead/admin
'projects.removeMember'   // (projectId, userId) — lead/admin
'projects.listMembers'    // (projectId) → ProjectMember[]
'projects.getContextPreamble' // (projectId, userId, worktreeId) → string
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
    ▼ RPC: profile.getEffective(userId)
    │   → ProfileResolver.resolve(userId) [cache hit]
    │   → { agent: { preferredModel: 'claude', trustPreset: 'standard' },
    │         shell: { envVars: {...}, pathAdditions: [...] } }
    │
    ▼ ProfileAwareAgentSpawner.spawn({
    │     relay,
    │     project: P,
    │     worktreePath: '/srv/vnp-worktrees/feature-x',
    │     profile: resolvedProfile,
    │     userId,
    │   })
    │   ├── agentEnv = {
    │   │     ...profile.shell.envVars,
    │   │     PATH: /pathAdditions + $PATH,
    │   │     GH_CONFIG_DIR: ~/.config/gh/userId/,
    │   │     ANTHROPIC_MODEL: 'claude-opus-4-5',
    │   │     ORCA_PROJECT_ID: P.id,
    │   │   }
    │   └── relay.call('pty.spawn', {
    │           cmd: 'claude',
    │           args: ['--trust', 'standard'],
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
src/main/ai-providers/
├── ai-provider-types.ts          # AIProviderAccount, TaskGrant, ProviderHealthStatus
├── provider-service.ts           # CRUD orca_ai_provider_accounts
├── provider-resolver.ts          # Resolution algorithm (priority cascade)
├── provider-health-checker.ts    # Background cron, test connection, quota tracking
├── provider-credential-relay.ts  # Encrypt + relay credentials to Dev Server
└── provider-rpc-methods.ts       # ai-providers.* RPC handlers
```

### src/main/task/ — New module (v5.0)

```
src/main/task/
├── task-types.ts                 # OrcaTask, TaskEdge, TaskGrant, TaskGrant interfaces
├── task-service.ts               # CRUD + progress calculation
├── task-dag-validator.ts         # addEdge with cycle detection, auto-block logic
├── task-graph-builder.ts         # loadTaskTree(rootId, userId) BFS + access filter
├── task-ai-planner.ts            # AI decompose + prompt generation
├── task-grant-service.ts         # hasTaskAccess(), addGrant(), revokeGrant()
├── task-agent-executor.ts        # buildTaskPreamble(), spawnAgent(), streamToActivity()
└── task-rpc-methods.ts           # tasks.* RPC handlers
```

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
│   ├── parallel-step-executor.ts # Promise.allSettled for parallel type
│   └── condition-step-executor.ts # Branch logic evaluation
└── workflow-rpc-methods.ts      # workflows.* RPC handlers
```

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

CREATE TABLE orca_step_executions (
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

-- Migration 0010: Task Graph
CREATE TABLE orca_tasks ( ... )               -- See BL-TG-01
CREATE TABLE orca_task_edges ( ... )
CREATE TABLE orca_task_grants ( ... )
CREATE TABLE orca_task_comments ( ... )
```

### Data Flow: Task → Agent → Activity Feed

```
Browser click "Run Agent" on Task T
    │
    ▼ RPC: tasks.runAgent(taskId=T, worktreeId=W?)
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
    │     initFile: preamble + task.promptTemplate
    │   })
    │   → sessionId
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

### src/main/runtime/rpc/methods/git.ts — Extended (v5.0)

```typescript
// New RPC methods added for Workspace Git UI:
'git.status'           → exec('git status --porcelain=v2 --branch', { cwd })
'git.diff'             → exec('git diff [--staged] [--] [file]', { cwd })
'git.add'              → exec('git add <files>', { cwd })
'git.restore'          → exec('git restore [--staged] <files>', { cwd })
'git.commit'           → exec('git commit -m <msg> --author=...', { cwd })
'git.push'             → execStream('git push origin <branch>', { cwd })
'git.pull'             → execStream('git pull origin <branch>', { cwd })
'git.fetch'            → exec('git fetch --all', { cwd })
'git.branch.list'      → exec('git branch -a -vv', { cwd })
'git.branch.create'    → exec('git checkout -b <name> [from]', { cwd })
'git.branch.delete'    → exec('git branch -d <name>', { cwd })
'git.merge'            → exec('git merge --no-ff <branch>', { cwd })
'git.stash'            → exec('git stash push -m <msg>', { cwd })
'git.stash.pop'        → exec('git stash pop', { cwd })
'git.log'              → exec('git log --oneline --graph --decorate -50', { cwd })

// File system:
'fs.readDir'           → relay: fs.readdir + stat per entry
'fs.readFile'          → relay: fs.readFile (max 5MB)
'fs.stat'              → relay: fs.stat
'fs.glob'              → relay: glob(pattern, { cwd, ignore })
'fs.grep'              → relay: grep -rn --include=<ext> pattern cwd (limit 30)
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

| Module | Path | Status | Depends on |
|--------|------|--------|-----------|
| ProfileResolver | `src/main/profile/ProfileResolver.ts` | ❌ TODO | DB migration 0006 |
| ProfileService | `src/main/profile/ProfileService.ts` | ❌ TODO | ProfileResolver |
| ProjectService | `src/main/project/ProjectService.ts` | ❌ TODO | DB migration 0007 |
| ProjectServerRouter | `src/main/project/ProjectServerRouter.ts` | ❌ TODO | ProjectService, DevServerManager |
| ProfileAwareAgentSpawner | `src/main/project/ProfileAwareAgentSpawner.ts` | ❌ TODO | ProfileResolver, ProjectServerRouter |
| AIProviderService | `src/main/ai-providers/AIProviderService.ts` | ❌ TODO | DB migration 0008, relay |
| ProviderResolver | `src/main/ai-providers/ProviderResolver.ts` | ❌ TODO | AIProviderService |
| DAGBuilder | `src/main/workflow/DAGBuilder.ts` | ❌ TODO | — |
| WorkflowOrchestrator | `src/main/workflow/WorkflowOrchestrator.ts` | ❌ TODO | DAGBuilder, DB 0009 |
| TemplateResolver | `src/main/workflow/TemplateResolver.ts` | ❌ TODO | WorkflowOrchestrator |
| TaskService | `src/main/task/TaskService.ts` | ❌ TODO | DB migration 0010 |
| TaskDAGValidator | `src/main/task/TaskDAGValidator.ts` | ❌ TODO | TaskService |
| TaskGrantService | `src/main/task/TaskGrantService.ts` | ❌ TODO | TaskService |
| TaskAIPlanner | `src/main/task/TaskAIPlanner.ts` | ❌ TODO | ProviderResolver, TaskService |
| TaskAgentExecutor | `src/main/task/TaskAgentExecutor.ts` | ❌ TODO | ProfileAwareAgentSpawner, TaskService |
| RelayConnectionPool | `src/main/dev-server/relay-connection-pool.ts` | ❌ TODO | DevServerRelayBridge |
| WorkspaceContext | `src/renderer/src/context/WorkspaceContext.tsx` | ❌ TODO | RelayConnectionPool (via RPC) |
| git-handler (relay) | `src/relay/git-handler.ts` | ❌ TODO | relay dispatcher |

---

## C4.11 — Dev Server Agent Module Map (v6.0 NEW)

### src/agent/ — Dev Server Agent source structure

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

### Key Data Flows (C4.11)

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

```
User clicks "Start Agent" on task
  → Gateway: resolve profile (F33 ProfileResolver)
  → Gateway: resolve provider (F35 ProviderResolver → accountId)
  → Gateway: sign context { userId, projectRoot, resolvedProfile, providerAccountId }
  → RPC 'agent.spawn' → Agent
  → ContextVerifier.verify(ctx)
  → AgentModelValidator.check(ctx.agentSettings.approvedModels, model)
  → CredentialReader.decrypt(providerAccountId) → API key
  → AgentEnvBuilder.build(ctx) → env { PATH, envVars, ANTHROPIC_API_KEY, GH_CONFIG_DIR }
  → PtyManager.create(workdir, shell)
  → spawn AI agent in PTY with env
  → AgentStateDetector monitors OSC events
  → EventBus.emit('agentStatus', { status: 'running' })
  → [wss] → Gateway → NotificationService + UI update
```

#### Flow A11.3: Workflow Step Dispatch

```
WorkflowOrchestrator (Gateway) resolves next step
  → StepServerResolver: project:vnp-blc → agentId
  → ProviderResolver: server:anthropic-default → accountId
  → Sign context { userId, projectId, projectRoot, providerAccountId }
  → RPC 'step.execute' → Agent
  → StepExecutor.dispatch(stepType, definition, ctx)
    - type='agent' → AgentStepExecutor → ProfileAwareAgentSpawner
    - type='shell' → ShellStepExecutor → execFile(command, args, { env })
    - type='action' → ActionStepExecutor → git, github ops
  → EventBus.emit('stepOutput', chunks) → Gateway stream → UI
  → StepStateStore.save(stepRunId, state)
  → On complete: EventBus.emit('stepComplete', { output })
  → [wss] → Gateway → WorkflowOrchestrator.onStepComplete(stepId, output)
  → Continue with next DAG wave
```

### Module Implementation Status (v6.0)

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

| Module | File | Change |
|--------|------|--------|
| AgentConnectionManager | `src/main/dev-server/agent-connection-manager.ts` | NEW — replaces relay-websocket-client |
| SignedContextIssuer | `src/main/dev-server/signed-context-issuer.ts` | NEW — HMAC-SHA256 context signing |
| AgentDispatcher | `src/main/dev-server/agent-dispatcher.ts` | NEW — routes RPC to correct agent |
| WorkflowOrchestrator | `src/main/workflow/WorkflowOrchestrator.ts` | MODIFY — use AgentDispatcher |
| TaskAgentExecutor | `src/main/task/TaskAgentExecutor.ts` | MODIFY — use AgentDispatcher |
| ProfileAwareAgentSpawner (gateway) | `src/main/project/ProfileAwareAgentSpawner.ts` | MODIFY — delegate to AgentDispatcher |

