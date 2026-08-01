**Level:** 3 — Components (bên trong các Containers)  
**Mô tả:** Chi tiết các components trong mỗi container quan trọng  
**Cập nhật:** 2026-07-28 (thêm C3.5 Fleet+DevServer, C3.6 Platform, C3.7 DB, C3.8 AgentWS, C3.9 Credentials, C3.10 Profile+Project)

---

## C3.1 — Main Process Components

```mermaid
C4Component
  title Components — Orca Main Process

  Container_Boundary(main, "Main Process (Node.js)") {

    Component(ipc_registry, "IPC Handler Registry", "TypeScript",
      "Đăng ký và dispatch tất cả IPC channels\ngiữa renderer và main process")

    Component(worktree_manager, "Worktree Manager", "TypeScript",
      "CRUD worktrees (git worktree add/remove)\nFan-out logic, merge strategy")

    Component(agent_orchestrator, "Agent Orchestrator", "TypeScript",
      "Spawn/stop/monitor AI agents\nTrust preset application\nSession ID management")

    Component(agent_awake_service, "Agent Awake Service", "TypeScript",
      "Parse OSC sequences từ PTY output\nDetect agent state transitions\n(idle→running→waiting→completed)")

    Component(agent_trust_presets, "Agent Trust Presets", "TypeScript",
      "Manage permission tiers per agent\nApply env vars cho security context")

    Component(terminal_manager, "Terminal Manager", "TypeScript (IPC)",
      "Bridge renderer terminal requests\ntới Daemon PTY sessions")

    Component(ssh_manager, "SSH Manager", "TypeScript",
      "SSH connection pool management\nConfig parser (~/.ssh/config)\nAuth: key, agent, password")

    Component(relay_deploy, "Relay Deploy Service", "TypeScript",
      "Upload relay binary via SFTP\nVersion check, hash verify\nPlatform detection (linux/win)")

    Component(ssh_relay_session, "SSH Relay Session", "TypeScript",
      "WebSocket connection tới relay\nRelay protocol handler\nPort forwarding setup")

    Component(git_ops, "Git Operations", "TypeScript",
      "git diff, status, commit, push\nWorktree listing, branch management\nSubmodule support")

    Component(github_client, "GitHub Client", "TypeScript",
      "REST/GraphQL API integration\nIssues, PRs, Reviews\nRate limit handling")

    Component(linear_client, "Linear Client", "TypeScript",
      "Linear API integration\nTasks, status updates\nIssue → worktree linking")

    Component(automation_engine, "Automation Engine", "TypeScript",
      "Cron scheduler\nEvent-based trigger system\nAction executor pipeline")

    Component(notification_service, "Notification Service", "TypeScript",
      "Desktop notifications\nMobile push (WebSocket bridge)\nRate limiting + batching")

    Component(persistence, "Persistence Layer", "better-sqlite3",
      "SQLite schema migrations\nRepository/worktree/session CRUD\nTransaction management")

    Component(mobile_server, "Mobile WebSocket Server", "TypeScript",
      "Accept mobile connections\nE2E encryption (TweetNaCl)\nStatus sync + dispatch routing")

    Component(text_generation, "Text Generation", "TypeScript",
      "AI-powered commit messages\nPR descriptions\nContext-aware prompts")
  }

  Rel(ipc_registry, worktree_manager, "delegates to")
  Rel(ipc_registry, agent_orchestrator, "delegates to")
  Rel(ipc_registry, terminal_manager, "delegates to")
  Rel(ipc_registry, ssh_manager, "delegates to")
  Rel(ipc_registry, git_ops, "delegates to")
  Rel(ipc_registry, github_client, "delegates to")
  Rel(ipc_registry, linear_client, "delegates to")
  Rel(ipc_registry, notification_service, "delegates to")

  Rel(agent_orchestrator, agent_awake_service, "uses")
  Rel(agent_orchestrator, agent_trust_presets, "uses")
  Rel(agent_orchestrator, persistence, "reads/writes")

  Rel(worktree_manager, git_ops, "uses")
  Rel(worktree_manager, persistence, "reads/writes")

  Rel(ssh_manager, relay_deploy, "orchestrates")
  Rel(ssh_manager, ssh_relay_session, "creates")
  Rel(ssh_relay_session, terminal_manager, "routes PTY")

  Rel(automation_engine, worktree_manager, "triggers")
  Rel(automation_engine, agent_orchestrator, "triggers")
  Rel(automation_engine, persistence, "reads config, writes runs")

  Rel(notification_service, mobile_server, "routes mobile notif")
  Rel(text_generation, github_client, "uses diff data")
```

---

## C3.2 — Daemon Components

```mermaid
C4Component
  title Components — Orca Daemon

  Container_Boundary(daemon, "Orca Daemon") {

    Component(daemon_server, "Daemon Server", "TypeScript / Unix Socket",
      "NDJSON protocol server\nClient connection management\nMessage routing")

    Component(pty_router, "PTY Router", "TypeScript",
      "Route PTY I/O giữa nhiều clients\n(GUI + CLI) cho cùng session\nMux/demux streams")

    Component(pty_adapter, "PTY Adapter", "TypeScript / node-pty",
      "Lifecycle management cho PTY processes\nResize handling, signal forwarding\nFlood control + backpressure")

    Component(session_manager, "Session Manager", "TypeScript",
      "PTY session registry\nSession ID generation\nSession state persistence\nReattach support")

    Component(history_manager, "History Manager", "TypeScript",
      "Terminal scrollback history\nIncremental restore từ snapshot\nSearch trong history")

    Component(headless_emulator, "Headless VT Emulator", "TypeScript",
      "xterm-compatible terminal emulator\n(không cần DOM)\nState serialization cho snapshots")

    Component(shell_ready, "Shell Ready Detector", "TypeScript",
      "Detect khi shell đã ready\nOSC 133 A/B/C/D parsing\nBlocking startup until ready")

    Component(daemon_health, "Daemon Health Monitor", "TypeScript",
      "Health check endpoint\nWatchdog timer\nCrash recovery và restart")
  }

  Rel(daemon_server, pty_router, "routes messages to")
  Rel(pty_router, pty_adapter, "controls")
  Rel(pty_adapter, session_manager, "registers sessions")
  Rel(session_manager, history_manager, "persists scrollback")
  Rel(history_manager, headless_emulator, "serializes state")
  Rel(pty_adapter, shell_ready, "monitors startup")
```

---

## C3.3 — Relay Components (Remote Host)

```mermaid
C4Component
  title Components — Orca Relay (trên Remote Host)

  Container_Boundary(relay_bin, "Orca Relay Binary") {

    Component(relay_server, "Relay Server", "TypeScript / WebSocket",
      "WebSocket server nhận connections từ Desktop\nProtocol handshake + auth\nSession token verification")

    Component(dispatcher, "Request Dispatcher", "TypeScript",
      "Route requests tới đúng handler\nMux multiple streams\nBackpressure management")

    Component(pty_handler_relay, "PTY Handler", "TypeScript / node-pty",
      "Tạo và quản lý PTY trên remote\nStream I/O qua WebSocket\nResize propagation")

    Component(fs_handler, "Filesystem Handler", "TypeScript",
      "File read/write/list\nFile watching via parcel-watcher\nGit-aware search (ripgrep)")

    Component(git_handler, "Git Handler", "TypeScript",
      "git diff, status, commit, push\nWorktree operations trên remote\nBranch management")

    Component(port_scanner, "Port Scanner", "TypeScript",
      "Scan localhost cho open ports mỗi 2s\nReport new ports về Desktop\nExclude system ports")

    Component(agent_hook_server, "Agent Hook Server", "TypeScript / HTTP",
      "HTTP server intercept agent hooks\nRoute tool calls qua relay\nProcess isolation")

    Component(plugin_overlay, "Plugin Overlay", "TypeScript",
      "Inject environment overlays\nAgent-specific env vars\nSecurity sandboxing")
  }

  Rel(relay_server, dispatcher, "forwards requests to")
  Rel(dispatcher, pty_handler_relay, "PTY commands")
  Rel(dispatcher, fs_handler, "FS commands")
  Rel(dispatcher, git_handler, "Git commands")
  Rel(dispatcher, port_scanner, "port events")
  Rel(dispatcher, agent_hook_server, "hook intercepts")
  Rel(pty_handler_relay, agent_hook_server, "hooks via HTTP")
  Rel(pty_handler_relay, plugin_overlay, "env injection")
```

---

## C3.4 — Renderer (UI) Components

```mermaid
C4Component
  title Components — Renderer Process (React UI)

  Container_Boundary(renderer_ui, "Renderer Process") {

    Component(app_shell, "App Shell", "React / TypeScript",
      "Top-level routing, layout\nSidebar + main content area\nModal management")

    Component(worktree_sidebar, "Worktree Sidebar", "React",
      "List worktrees với status badges\nDrag-drop reorder\nContext menu actions")

    Component(terminal_panel, "Terminal Panel", "React / xterm.js / WebGL",
      "Xterm.js renderer (WebGL accelerated)\nSplit terminal support\nScrollback display")

    Component(diff_viewer, "Diff Viewer", "React",
      "Git diff display với syntax highlight\nFile tree navigation\nInline annotation UI")

    Component(file_explorer, "File Explorer", "React",
      "Tree view của worktree files\nFile search\nContext menu")

    Component(browser_panel, "Embedded Browser", "React / Electron webview",
      "Embedded Chromium browser\nDesign mode overlay\nViewport resize controls")

    Component(github_panel, "GitHub Panel", "React",
      "Issue browser + search\nPR viewer\nWork item linking")

    Component(notifications_ui, "Notifications UI", "React",
      "Desktop notification toasts\nNotification history\nMobile pairing QR display")

    Component(automations_ui, "Automations UI", "React",
      "CRUD automation workflows\nRun history viewer\nSchedule configuration")

    Component(settings_ui, "Settings UI", "React",
      "Agent config\nSSH hosts\nKeybindings\nTheme")

    Component(store, "App State Store", "React Context / Custom Hooks",
      "Global state: worktrees, sessions,\nagent statuses, notifications\nReactive updates from IPC events")
  }

  Rel(app_shell, worktree_sidebar, "renders")
  Rel(app_shell, terminal_panel, "renders")
  Rel(app_shell, diff_viewer, "renders")
  Rel(app_shell, file_explorer, "renders")
  Rel(app_shell, browser_panel, "renders")
  Rel(app_shell, github_panel, "renders")
  Rel(app_shell, notifications_ui, "renders")
  Rel(app_shell, store, "reads state")
  Rel(store, terminal_panel, "provides session data")
  Rel(store, worktree_sidebar, "provides worktree list")
  Rel(diff_viewer, store, "reads diff data")
```

---

## C3.5 — Fleet & Dev Server Management (trong Main Process / SSH Subsystem)

```mermaid
C4Component
  title Components — Fleet & Dev Server Management

  Container_Boundary(fleet_sub, "Fleet & Dev Server Subsystem") {

    Component(fleet_config_parser, "Fleet Config Parser", "TypeScript / js-yaml",
      "Parse deploy/dev/orca-fleet.yaml\nValidate schema (Zod)\nResolve: projects[], servers[], defaults{}")

    Component(fleet_bootstrap, "Fleet Bootstrap Service", "TypeScript",
      "Orchestrate per-server setup\nCheck prerequisites: Node.js, Git, disk\nInstall: relay binary, repo clone")

    Component(fleet_remote_cmds, "Fleet Remote Commands", "TypeScript / SSH",
      "Execute remote commands qua SSH exec\nIsolated per-server connection\nCommand: node-version, git-version, relay-start")

    Component(fleet_health_monitor, "Fleet Health Monitor", "TypeScript",
      "Poll all servers every 60s\nMetrics: CPU%, RAM%, disk%, latency\nStatus: healthy/degraded/unhealthy/unreachable")

    Component(fleet_health_store, "Fleet Health Store", "TypeScript / in-memory",
      "In-memory cache của server health\nTrend tracking (last N samples)\nLookup by serverId")

    Component(fleet_status_service, "Fleet Status Service", "TypeScript",
      "Aggregate health across fleet\nWebhook alerts on status change\nPrometheus metrics endpoint")

    Component(server_grouping, "Server Grouping", "TypeScript",
      "groupSshTargetsByProject()\nSshTargetGroup, SshTargetGroupedList\nProject + tag filtering")

    Component(rbac_resolver, "RBAC Policy Resolver", "TypeScript",
      "hasPermission(userId, resource, action)\nRolePolicy lookup\nPhase 2: SSO integration")

    Component(dev_server_manager, "Dev Server Manager", "TypeScript",
      "DevServer config CRUD\nconnectionType: relay-ssh | relay-websocket | direct-websocket\nagentToken management")

    Component(relay_bridge, "DevServer Relay Bridge", "TypeScript",
      "Connect() dispatcher per connectionType\nrelay-ssh: SSH exec channel\nrelay-websocket: WsTransport to agent WS\ndirect-websocket: receive from agent WS")
  }

  Rel(fleet_config_parser, fleet_bootstrap, "parsed config")
  Rel(fleet_config_parser, server_grouping, "group by project")
  Rel(fleet_bootstrap, fleet_remote_cmds, "executes via")
  Rel(fleet_health_monitor, fleet_remote_cmds, "health poll via")
  Rel(fleet_health_monitor, fleet_health_store, "writes metrics")
  Rel(fleet_health_store, fleet_status_service, "provides data")
  Rel(fleet_status_service, rbac_resolver, "checks permission")
  Rel(dev_server_manager, relay_bridge, "creates bridge")
  Rel(relay_bridge, fleet_remote_cmds, "SSH exec (relay-ssh)")
```

---

## C3.6 — Platform Abstraction Layer (restructure_v1)

```mermaid
C4Component
  title Components — Platform Abstraction Layer

  Container_Boundary(platform_layer, "src/platform/ (IPlatformServices)") {

    Component(platform_iface, "IPlatformServices", "TypeScript Interface",
      "Root interface: IApp + IWindow + IIpcBridge\n+ ISecureStorage + ISystemInfo + INotification\nImplemented by both adapters")

    Component(electron_adapter, "ElectronAdapter", "TypeScript / Electron",
      "Wrap electron.app, BrowserWindow, ipcMain\nsafeStorage for credentials\nNativeTheme, Notification API")

    Component(node_adapter, "NodeAdapter", "TypeScript / Node.js only",
      "No Electron dependency\nNodeApp: userData=~/.orca, getPath()\nNodeWindow: noop (headless)\nNodeIpcBridge: WebSocket RPC bridge\nNodeSecureStorage: AES-256-GCM file")

    Component(ipc_bridge, "IRpcRouter", "TypeScript",
      "Route RPC method calls\nElectron: ipcMain.handle()\nNode: WebSocket JSON-RPC dispatch\nShared: method registry map")

    Component(web_entry, "Web Entry Bootstrap", "TypeScript",
      "src/server/index.ts\nBootstrap NodeAdapter\nStart HTTP :6769 + WS :6768\nMount auth, admin, agent routes")
  }

  Rel(electron_adapter, platform_iface, "implements")
  Rel(node_adapter, platform_iface, "implements")
  Rel(node_adapter, ipc_bridge, "uses NodeIpcBridge")
  Rel(electron_adapter, ipc_bridge, "uses ElectronIpcBridge")
  Rel(web_entry, node_adapter, "bootstraps with")
```

---

## C3.7 — Database Layer (sql-server CRs)

```mermaid
C4Component
  title Components — Database Abstraction Layer

  Container_Boundary(db_layer, "src/main/db/ (Multi-DB Layer)") {

    Component(db_iface, "IDatabase / IStatement", "TypeScript Interface",
      "exec(sql), prepare(sql): IStatement\nIStatement: run(), get(), all(), iterate()\nIDatabaseCapabilities: dialect, placeholderStyle")

    Component(iconn_pool, "IConnectionPool", "TypeScript Interface",
      "acquire(): Promise<IDatabase>\nrelease(conn): Promise<void>\nend(): Promise<void>\nIdleTimeout, maxConnections config")

    Component(db_provider, "DatabaseProvider Factory", "TypeScript",
      "create(config: DatabaseConfig): IDatabase\nDialect detection: sqlite|mysql|postgresql|tidb\nDSN parsing: mysql://user:pass@host/db")

    Component(sqlite_adapter, "SQLiteAdapter", "TypeScript / node:sqlite",
      "DatabaseSync wrapper\nWAL mode enabled\nPositional placeholder (?)\nCapabilities: walMode=true, dialect=sqlite")

    Component(mysql_adapter, "MySQLAdapter", "TypeScript / mysql2",
      "mysql2 async driver\nPrepared statements (?)\nPool wrapper\nCompatible: MySQL 8.x, MariaDB, TiDB")

    Component(pg_adapter, "PostgreSQLAdapter", "TypeScript / pg",
      "pg driver (node-postgres)\nNamed placeholder ($1..$N)\nSSL support\nCapabilities: returning=true, nativeJson=true")

    Component(migration_runner, "MigrationRunner", "TypeScript",
      "Sequential apply: 0001→0002→...→0005\nDialect-aware DDL (IF NOT EXISTS)\nTracking table: orca_migrations\nIdempotent re-run safe")

    Component(db_health, "DatabaseHealthMonitor", "TypeScript",
      "Ping DB every 30s\nStatus: connected|degraded|disconnected\nAuto-reconnect on failure\nMetrics: latency, errorCount")

    Component(repo_factory, "Repository Factory", "TypeScript",
      "ORCA_STORAGE_BACKEND env\njson → JsonFileRepository\nsql → SqlRepository (IConnectionPool)")
  }

  Rel(db_provider, sqlite_adapter, "creates (sqlite)")
  Rel(db_provider, mysql_adapter, "creates (mysql/tidb)")
  Rel(db_provider, pg_adapter, "creates (postgresql)")
  Rel(iconn_pool, db_iface, "pools")
  Rel(migration_runner, db_iface, "runs migrations on")
  Rel(db_health, iconn_pool, "monitors")
  Rel(repo_factory, iconn_pool, "uses (sql mode)")
```

**Migrations (0001–0005):**

| Migration | Tables tạo ra |
|-----------|----------------|
| 0001_init | `projects`, `worktrees`, `agent_sessions`, `settings` |
| 0002_sessions | `terminal_scrollback_snapshots` |
| 0003_ssh_targets | `ssh_hosts`, `saved_port_forwards` |
| 0004_automations | `automations`, `automation_runs`, `notifications`, `rate_limits` |
| 0005_auth_schema | `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies` |

---

## C3.8 — Agent WebSocket System (agent v2 CRs)

```mermaid
C4Component
  title Components — Agent WebSocket System

  Container_Boundary(agent_ws_sys, "Agent WebSocket System (src/main/dev-server/)") {

    Component(wire_protocol, "Agent Wire Protocol", "TypeScript",
      "Frame: 13-byte header + UTF-8 JSON-RPC 2.0\nTYPE[1B]: 0x01=Regular, 0x09=KeepAlive\nSEQ[4B BE], ACK[4B BE], LEN[4B BE]\nEncode/decode binary frames")

    Component(ws_transport, "WsTransport Adapter", "TypeScript / ws",
      "Implements MultiplexerTransport interface\nwrite(frame): send binary WS message\nonData: receive handler\nonClose: disconnect handler\nAdaptor between WS and SshChannelMultiplexer")

    Component(relay_ws_client, "RelayWebSocketClient", "TypeScript",
      "relay-websocket mode: Orca → Agent\nOrca connects to ws://agent:PORT/orca-relay\nBearer token auth header\nWire protocol handshake")

    Component(agent_ws_server, "AgentWebSocketServer", "TypeScript / ws",
      "direct-websocket mode: Agent → Orca\nListen on ws://orca:6768/agent path\nHandshake: agent.handshake { agentToken }\nReply: handshake-ok { sessionId }\nWire WsTransport ↔ SshChannelMultiplexer")

    Component(agent_token_mgr, "AgentTokenManager", "TypeScript",
      "Generate: crypto.randomBytes(32).hex()\nStore: SHA-256 hash per DevServer config\nValidate: SHA-256(rawToken) == storedHash\nRevoke: delete from DevServer config")

    Component(dev_relay_bridge, "DevServerRelayBridge", "TypeScript",
      "connect() dispatcher:\n- relay-ssh: SSH exec channel → SshChannelMultiplexer\n- relay-websocket: RelayWebSocketClient → WsTransport\n- direct-websocket: AgentWebSocketServer → WsTransport")
  }

  Rel(dev_relay_bridge, relay_ws_client, "relay-websocket mode")
  Rel(dev_relay_bridge, agent_ws_server, "direct-websocket mode")
  Rel(relay_ws_client, ws_transport, "uses")
  Rel(agent_ws_server, ws_transport, "uses")
  Rel(ws_transport, wire_protocol, "encode/decode frames")
  Rel(agent_ws_server, agent_token_mgr, "validates token")
```

**Wire Protocol Frame Format:**
```
┌────────────────────────────────────────────────────┐
TYPE[1] | SEQ[4 BE] | ACK[4 BE] | LEN[4 BE] | PAYLOAD[LEN]
└────────────────────────────────────────────────────┘
     = 13 bytes header total
PAYLOAD = UTF-8 JSON-RPC 2.0 (đối với Regular frames)
```

---

## C3.9 — Integration & Credential Layer (github/integration CRs)

```mermaid
C4Component
  title Components — Integration & Credential Layer

  Container_Boundary(cred_layer, "src/main/credentials/ + integrations/") {

    Component(unified_cred, "UnifiedCredentialService", "TypeScript",
      "Enum: linear|jira|github|gitlab|bitbucket|azure-devops|gitea\nRoute to correct store (WebCredentialStore / EnvToken)\nFacade for all integrations")

    Component(web_cred_store, "WebCredentialStore", "TypeScript / AES-256-GCM",
      "Per-user encrypted file store\nKey: scryptSync(userId + ORCA_CREDENTIAL_KEY)\nFile: ~/.orca/users/<userId>/<service>.enc\nIV: randomBytes(12) per write")

    Component(cred_rpc, "Credentials RPC Methods", "TypeScript",
      "credentials.set(service, token) → encrypt + store\ncredentials.revoke(service) → delete\ncredentials.status(service) → { configured, lastValidated }\ncredentials.list() → [service names only, no tokens]")

    Component(preflight_proxy, "Preflight Check Proxy", "TypeScript",
      "Category A: CLI-based integrations\nProxy preflight.check qua SSH relay\n→ Dev Server: gh/glab auth status\nmergePreflightStatuses(local, relay)")

    Component(gh_config_isolation, "GH Config Session Isolation", "TypeScript",
      "GH_CONFIG_DIR=~/.config/gh/<userId>/\nGLAB_CONFIG_DIR=~/.config/glab-cli/<userId>/\nPer-user CLI auth isolation\nInject via SSH exec env")

    Component(preflight_runner, "Preflight Runner", "TypeScript",
      "runLocalChecks(): git, disk, API token format\nrunRelayChecks(): gh/glab status, Node.js version\nmerge: relay overrides local (authoritative)")

    Component(integration_registry, "Integration Registry", "TypeScript",
      "Category B: HTTP API integrations\nBitbucket, AzureDevOps, Gitea\nInject per-user token from WebCredentialStore\nCategory C: Linear, Jira via file token")
  }

  Rel(unified_cred, web_cred_store, "Category B+C storage")
  Rel(unified_cred, integration_registry, "delegates HTTP calls")
  Rel(cred_rpc, unified_cred, "calls")
  Rel(preflight_proxy, preflight_runner, "orchestrates")
  Rel(preflight_proxy, gh_config_isolation, "inject env vars")
  Rel(preflight_runner, web_cred_store, "check token status")
```

---

## Key Design Patterns

### 1. IPC Handler Pattern (Main ↔ Renderer)

```typescript
// Main process: register handler
ipcMain.handle('worktree:create', async (event, opts) => {
  return worktreeManager.create(opts);
});

// Renderer (via contextBridge): call handler
const result = await window.orcaAPI.worktree.create(opts);
```

### 2. Agent Status Detection Pattern (OSC 133)

```
PTY Output Stream
      │
      ▼
OSC 133 Parser ──► Pattern Matching ──► Status State Machine
      │
      ▼
  IPC Event (renderer ← main)
      │
      ▼
   UI Update (worktree card badge)
      │
      ▼
   Notification (if app background)
      │
      ▼
   Mobile Push (if paired)
```

### 3. Relay Protocol (Desktop ↔ Remote)

```
Desktop (ssh_relay_session)
      │
      │  WebSocket over SSH tunnel
      ▼
Remote (relay server)
      │
      ├── pty_handler   → node-pty process
      ├── fs_handler    → filesystem ops
      ├── git_handler   → git commands
      └── port_scanner  → port detection
```

### 4. Scrollback Persistence Pattern

```
PTY Output
    │
    ▼
Headless Emulator (in-memory VT state)
    │ (on close / idle)
    ▼
@xterm/addon-serialize → compressed JSON
    │
    ▼
SQLite (terminal_scrollback_snapshots)
    │ (on reopen)
    ▼
Deserialize → Restore VT state → xterm.js renderer
```

### 5. Platform Adapter Pattern (restructure_v1)

```
              IPlatformServices
             /                 \
    ElectronAdapter          NodeAdapter
         │                       │
    electron.app             NodeApp (EventEmitter)
    BrowserWindow            NodeWindow (noop)
    ipcMain.handle()         WS RPC dispatch
    safeStorage              AES-256-GCM file
         │                       │
    Electron Desktop         Node.js Server
    (Electron runtime)       (no display)
```

### 6. Multi-DB Repository Pattern (sql-server CRs)

```
ORCA_STORAGE_BACKEND env
      │
      ├── 'json' → JsonFileRepository (orca-data.json)
      └── 'sql'  → SqlRepository
                        │
                   IConnectionPool
                        │
            ORCA_DB_URL env (DSN string)
                        │
         ┌─────────────├─────────────┐
     SQLiteAdapter   MySQLAdapter   PostgreSQLAdapter
     (file://... DSN) (mysql://...) (postgresql://...)
         TiDB = mysql2 with dialect flag
```

### 7. Agent WebSocket Connection Pattern

```
relay-websocket mode (Orca → Agent):
  Orca → HTTP Upgrade: ws://agent:PORT/orca-relay
       Header: Authorization: Bearer <agentToken>
  ► WsTransport ⇔ SshChannelMultiplexer ⇔ JSON-RPC

direct-websocket mode (Agent → Orca):
  Agent → ws://orca:6768/agent
  Orca → { type: 'handshake-request' }
  Agent → { type: 'agent.handshake', agentToken, name, version }
  Orca → { type: 'handshake-ok', sessionId }
  ► WsTransport ⇔ AgentWebSocketServer ⇔ JSON-RPC
```

### 8. Integration Preflight Proxy Pattern (github CRs)

```
Browser → preflight.check { devServerId }
               │
        Orca Server (ORCA_MULTI_USER=1)
               │
        Category A (CLI-based):
          relay.call('preflight.check')
            + GH_CONFIG_DIR=~/.config/gh/<userId>/
               │ SSH exec
               ▼
          Dev Server: gh auth status / glab auth status
               │ result
        mergePreflightStatuses(localResult, relayResult)
               │
        Category B+C (HTTP/file):
          WebCredentialStore.get(userId, service)
          → HTTP API call with per-user token
```

---

## C3.10 — Profile & Project Layer

### C3.10a — User Profile Hierarchy

```mermaid
C4Component
  title Components — User Profile Hierarchy (Web Server Mode)

  Container_Boundary(profile_layer, "Profile & Project Layer") {

    Component(company_service, "CompanyService", "TypeScript",
      "CRUD company profile\nRoot tầng 1: AI policy, security, global defaults\nAdmin only")

    Component(dept_service, "DepartmentService", "TypeScript",
      "CRUD department + department profile\nTầng 2: team AI model, fleet tags, shared env\nAdmin + Lead")

    Component(profile_resolver, "ProfileResolver", "TypeScript",
      "3-layer deep merge: company ← dept ← user\nSecurity lock: company security không bị override\narray fields: pathAdditions concatenate\nmap fields: user wins")

    Component(profile_cache, "ProfileCache", "TypeScript / Map<TTL>",
      "In-memory cache với TTL=60s\nInvalidate theo scope:\n  company update → flush all\n  dept update → flush dept members\n  user update → flush user only")

    Component(profile_rpc, "profile.* RPC methods", "TypeScript",
      "profile.getEffective(userId)\nprofile.updateUser(fields)\nprofile.updateDepartment(deptId, fields)\nprofile.updateCompany(fields) — admin only")
  }

  ContainerDb(db, "Server DB", "PostgreSQL/MySQL/SQLite",
    "orca_company.profile_json\norca_departments.profile_json\norca_users.profile_json + department_id")

  Person(admin, "Admin", "Company/Dept admin")
  Person(lead, "Lead", "Department lead")
  Person(user, "User", "Developer")

  Rel(admin, company_service, "Đặt Company defaults", "RPC: profile.updateCompany")
  Rel(admin, dept_service, "Tạo department", "RPC: profile.updateDepartment")
  Rel(lead, dept_service, "Cập nhật team settings", "RPC: profile.updateDepartment")
  Rel(user, profile_rpc, "Cập nhật personal prefs", "RPC: profile.updateUser")
  Rel(profile_rpc, profile_resolver, "Gọi khi đọc profile", "resolveProfile(userId)")
  Rel(profile_resolver, profile_cache, "Check/set cache", "Cache hit < 1ms")
  Rel(profile_resolver, db, "Load 3 layers", "SQL: users + departments + company")
  Rel(profile_cache, db, "Cache miss → DB query", "")
```

### C3.10b — Project-Dev Server Binding & Execution

```mermaid
C4Component
  title Components — Project Registry & Project-Centric Execution

  Container_Boundary(project_layer, "Project Layer") {

    Component(project_service, "ProjectService", "TypeScript",
      "CRUD projects\ngetProjectsForUser(userId): filter by membership + RBAC\nProject → DevServer binding management")

    Component(project_router, "ProjectServerRouter", "TypeScript",
      "Route worktree/agent/terminal calls\nto project.devServerId relay\nauto-detect server status before routing")

    Component(profile_spawner, "ProfileAwareAgentSpawner", "TypeScript",
      "Resolve profile → build agentEnv\nInject: shell.envVars, PATH additions\nGH_CONFIG_DIR per-user isolation\nANTHROPIC_MODEL from profile.agent.preferredModel\nagent trust preset mapping")

    Component(project_context, "ProjectContextInjector", "TypeScript",
      "Build system preamble cho agent:\n  project name, repo URL, branch\n  dev server hostname\n  developer name, team\nInject vào agent pty initFile")

    Component(project_rpc, "projects.* RPC methods", "TypeScript",
      "projects.list(userId)\nprojects.get(projectId)\nprojects.create(input)\nprojects.updateBinding(projectId, devServerId)\nprojects.addMember(projectId, userId, role)\nprojects.removeMember(projectId, userId)")
  }

  Component(relay_bridge, "DevServerRelayBridge", "TypeScript",
    "SSH relay / WS relay connection\nForwards RPC to dev server")

  ContainerDb(db, "Server DB", "",
    "orca_projects (id, dev_server_id, repo_path)\norca_project_members (project_id, user_id, role)")

  Person(lead, "Lead / Admin", "Tạo project, assign server, manage members")
  Person(dev, "Developer", "Chọn project → tạo worktree → start agent")

  Rel(lead, project_rpc, "Tạo project + binding", "RPC: projects.create/updateBinding")
  Rel(dev, project_rpc, "Xem danh sách projects", "RPC: projects.list")
  Rel(dev, project_router, "Tạo worktree cho project", "Click: New Worktree")
  Rel(project_router, project_service, "Get devServerId", "project.devServerId")
  Rel(project_router, relay_bridge, "Route relay call", "relay.call('git.worktree.add', ...)")
  Rel(project_router, profile_spawner, "Spawn agent trên dev server", "spawnAgent(project, userId)")
  Rel(profile_spawner, project_context, "Inject project preamble", "buildProjectContext()")
  Rel(profile_spawner, relay_bridge, "PTY spawn trên dev server", "relay.call('pty.spawn', {env})")
  Rel(project_service, db, "Persist", "SQL: orca_projects + orca_project_members")
```

### C3.10c — Inheritance Resolution Flow

```
Request: resolveProfile(userId='u-123')
    │
    ├── ProfileCache.get('u-123') → MISS (cold or stale)
    │
    ├── db.query: SELECT * FROM orca_users WHERE id='u-123'
    │   → { department_id: 'dept-backend', profile_json: '{"editor":{"theme":"light"}}' }
    │
    ├── db.query: SELECT * FROM orca_departments WHERE id='dept-backend'
    │   → { company_id: 'co-1', profile_json: '{"agent":{"preferredModel":"claude-opus-4-5"},"shell":{"pathAdditions":["/usr/local/go/bin"]}}' }
    │
    ├── db.query: SELECT * FROM orca_company WHERE id='co-1'
    │   → { profile_json: '{"agent":{"approvedModels":["claude-opus-4-5","codex"],"trustPreset":"standard"},"security":{"require2FA":true,"sessionTimeoutHours":8}}' }
    │
    ├── deepMerge(company, dept, user):
    │   {
    │     agent: {
    │       approvedModels: ["claude-opus-4-5","codex"],  // company only
    │       preferredModel: "claude-opus-4-5",           // dept
    │       trustPreset: "standard"                      // company (user didn't override)
    │     },
    │     editor: { theme: "light" },                    // user override
    │     shell: {
    │       pathAdditions: ["/usr/local/go/bin"],        // dept
    │       envVars: {}
    │     },
    │     security: { require2FA: true, sessionTimeoutHours: 8 }  // company LOCKED
    │   }
    │
    ├── Validate: preferredModel ∈ approvedModels ✅
    │
    ├── ProfileCache.set('u-123', result, TTL=60s)
    │
    └── return ResolvedProfile ✅

Agent spawn uses:
  ANTHROPIC_MODEL=claude-opus-4-5
  PATH=/usr/local/go/bin:$PATH
  GH_CONFIG_DIR=/home/dev/.config/gh/u-123/
  (trust args: --trust standard)
```

---

## C3.11 — AI Provider & Task Graph Layer

### C3.11a — AI Provider Account Management

```mermaid
C4Component
  title Components — AI Provider Account Management

  Container_Boundary(ai_provider_layer, "AI Provider Layer") {

    Component(provider_service, "AIProviderService", "TypeScript",
      "CRUD AIProviderAccount\nLink to dev server\nScopes: server/project/user\nDefault account management")

    Component(provider_resolver, "AIProviderResolver", "TypeScript",
      "Resolution priority:\n  user-scope > project-scope > server-default\nModel → provider detection\nFilter: status=healthy only")

    Component(provider_health, "ProviderHealthChecker", "TypeScript / cron",
      "Background check mỗi 15 phút\nTest connection qua relay → dev server\nQuota tracking: tokens_used/day\nAlert at 80% quota")

    Component(credential_relay, "ProviderCredentialRelay", "TypeScript",
      "Encrypt credentials in browser (SubtleCrypto)\nRelay encrypted blob qua SSH\nDev server: AES-256-GCM decrypt + write file\n~/.orca/ai-providers/<accountId>.enc")

    Component(provider_rpc, "ai-providers.* RPC", "TypeScript",
      "ai-providers.list(devServerId)\nai-providers.add(input) — admin/lead\nai-providers.testConnection()\nai-providers.rotateKey(accountId)\nai-providers.getUsage(accountId, date)")
  }

  ContainerDb(db, "Server DB", "",
    "orca_ai_provider_accounts\norca_provider_usage (daily tokens)")

  Component(relay, "DevServerRelayBridge", "SSH/WS",
    "Forward credential relay\nForward ai.ping / ai.complete calls")

  Rel(provider_service, db, "Persist metadata (no credentials)", "SQL")
  Rel(credential_relay, relay, "Relay encrypted credential to Dev Server", "SSH exec")
  Rel(provider_health, relay, "ai.ping per account", "RPC")
  Rel(provider_resolver, db, "Query accounts by scope + status", "SQL")
  Rel(provider_rpc, provider_service, "Delegate CRUD", "")
  Rel(provider_rpc, credential_relay, "Save/rotate credentials", "")
```

### C3.11b — Task Graph Management

```mermaid
C4Component
  title Components — Task Graph Management System

  Container_Boundary(task_layer, "Task Graph Layer") {

    Component(task_service, "TaskService", "TypeScript",
      "CRUD orca_tasks\nParent-child relationship management\nStatus/assignee/label management\nProgress calculation (recursive)")

    Component(dag_validator, "TaskDAGValidator", "TypeScript",
      "Add/remove dependency edges\nCycle detection: BFS from target\nAuto-block: dep not done → task blocked\nTopological sort for batch execution")

    Component(task_graph_builder, "TaskGraphBuilder", "TypeScript",
      "BFS subtree traversal\nAccess-filtered graph load\nCritical path calculation\nDAG layout hints for UI")

    Component(ai_planner, "TaskAIPlanner", "TypeScript",
      "Collect: task + project context + tech stack\nBuild planning prompt\nCall AI provider via relay\nParse JSON → subtask suggestions\nCritical path from estimates")

    Component(grant_service, "TaskGrantService", "TypeScript",
      "Grant resolution: owner>admin>user>team>company\nGrant inheritance: apply_tree BFS\nExpiry check\nShare link token management")

    Component(task_executor, "TaskAgentExecutor", "TypeScript",
      "Build task context preamble\nInterpolate promptTemplate: {{task.*}}\nSpawn agent via ProfileAwareAgentSpawner\nStream PTY → Task Activity Feed\nAuto-advance status on agent complete")

    Component(task_rpc, "tasks.* RPC", "TypeScript",
      "tasks.list / get / create / update / delete\ntasks.addDependency / removeDependency\ntasks.getGraph(rootId)\ntasks.aiPlan(taskId)\ntasks.runAgent(taskId)\ntasks.grant / revoke / listGrants\ntasks.shareLink(taskId)")
  }

  ContainerDb(db, "Server DB", "",
    "orca_tasks, orca_task_edges\norca_task_grants, orca_task_comments")

  Component(profile_spawner, "ProfileAwareAgentSpawner", "TypeScript",
    "Resolve profile + project → spawn agent\n(from C3.10b)")

  Rel(task_service, db, "Persist tasks", "SQL")
  Rel(dag_validator, db, "Read/write orca_task_edges", "SQL")
  Rel(grant_service, db, "Read orca_task_grants", "SQL")
  Rel(ai_planner, task_service, "Create subtasks after approval", "")
  Rel(task_executor, profile_spawner, "Delegate agent spawn", "")
  Rel(task_rpc, task_service, "")
  Rel(task_rpc, ai_planner, "")
  Rel(task_rpc, grant_service, "")
  Rel(task_rpc, task_executor, "")
```

### C3.11c — Multi-Server Workflow Orchestrator

```mermaid
C4Component
  title Components — Workflow Orchestration (Multi-Server)

  Container_Boundary(wf_layer, "Workflow Orchestration Layer") {

    Component(template_registry, "TemplateRegistry", "TypeScript",
      "CRUD workflow templates\nScopes: company/team/personal\nVisibility: private/team/company/public\nVersion management")

    Component(template_resolver, "TemplateResolver", "TypeScript",
      "Resolve inheritance chain\nApply overrides (step field patches)\nApply inject_steps (insert after/before)\nApply remove_steps\nReturn flattened WorkflowDefinition")

    Component(orchestrator, "WorkflowOrchestrator", "TypeScript",
      "Build DAG from steps + depends_on\nTopological sort → execution waves\nDispatch waves: parallel where no deps\nCollect step outputs → pass to next\nState persistence per checkpoint")

    Component(server_resolver, "WorkflowServerResolver", "TypeScript",
      "project:<id> → project.devServerId\nserver:<id> → direct\nfleet:tag:<tag> → healthy server with tag\nDefault: workflow context server")

    Component(step_executors, "Step Executors", "TypeScript",
      "AgentStepExecutor: spawn AI agent\nShellStepExecutor: relay shell.exec\nActionStepExecutor: built-in actions\nParallelStepExecutor: Promise.allSettled\nConditionStepExecutor: branch logic")

    Component(wf_rpc, "workflows.* RPC", "TypeScript",
      "workflows.listTemplates\nworkflows.create/update/delete\nworkflows.run(templateId, inputs)\nworkflows.getExecution(execId)\nworkflows.streamStepOutput(execId, stepId)\nworkflows.pause/resume/cancel")
  }

  ContainerDb(db, "Server DB", "",
    "orca_workflow_templates\norca_workflow_executions\norca_step_executions")

  Rel(orchestrator, server_resolver, "Resolve devServerId per step", "")
  Rel(orchestrator, step_executors, "Execute each step", "")
  Rel(orchestrator, db, "Persist execution state", "SQL")
  Rel(step_executors, provider_resolver, "Resolve AI provider", "BL-AIP-02")
  Rel(template_resolver, template_registry, "Load parent template", "")
  Rel(wf_rpc, orchestrator, "Start/control execution", "")
  Rel(wf_rpc, template_registry, "CRUD templates", "")
```

---

## C3.12 — Project Workspace Layer

```mermaid
C4Component
  title Components — Project Workspace (Unified IDE)

  Container_Boundary(workspace_ui, "Project Workspace UI (React)") {

    Component(project_selector, "ProjectSelector", "React",
      "Dropdown: list user's projects\nSwitch project → init WorkspaceContext\nServer status indicator")

    Component(workspace_ctx, "WorkspaceContext", "React Context",
      "Central state for active project:\n  project, relay, resolvedProfile\n  currentWorktree, gitStatus\n  activeAgentSession\n  event bus: agent.complete, git.commit")

    Component(workspace_layout, "WorkspaceLayout", "React",
      "Sidebar tabs: Explorer/Git/Agent/Workflows/Tasks/Terminal\nMain content area\nBottom terminal panel\nServer status bar")

    Component(explorer_panel, "ExplorerPanel", "React",
      "Lazy-load remote file tree (depth=1 per expand)\nGit status decorations inline\nFile viewer (read-only, syntax highlight)\nFile search: glob + grep via relay\nContext menu: copy path, git actions")

    Component(git_panel, "GitPanel", "React",
      "Status list: modified/staged/untracked\nVisual diff viewer (unified, syntax highlight)\nStage/Unstage/Discard\nCommit + AI message generation\nPush/Pull (progress stream)\nBranch Manager: list/create/switch/delete\nWorktree switcher\nPR creation (GitHub/GitLab)")

    Component(agent_panel, "AgentPanel", "React",
      "Provider display (from resolved profile)\nWorktree selector\nPrompt editor + task attach\nLive agent output stream\nRun/Stop/Save session")

    Component(relay_bridge, "RelayConnectionPool", "TypeScript",
      "Singleton per dev server\nReuse across panels\nAuto-cleanup idle > 5min\nHealthy/reconnect handling")
  }

  Container_Boundary(server_side, "Orca Web Server") {
    Component(git_rpc, "git.* RPC methods", "TypeScript",
      "git.status / diff / add / commit\ngit.push / pull (stream)\ngit.branch.* / worktree.*\ngit.merge / stash")

    Component(fs_rpc, "fs.* RPC methods", "TypeScript",
      "fs.readDir / readFile / stat\nfs.glob / grep\nfs.search (fuzzy)")
  }

  Component(relay, "Orca Relay (Dev Server)", "Node.js binary",
    "Executes git + fs commands\non actual dev server\nStreams output back")

  Rel(workspace_ctx, relay_bridge, "Connection per devServer", "")
  Rel(explorer_panel, relay_bridge, "fs.readDir / readFile / grep", "relay RPC")
  Rel(git_panel, relay_bridge, "git.* commands", "relay RPC")
  Rel(agent_panel, relay_bridge, "pty.spawn", "relay RPC")
  Rel(git_panel, relay_bridge, "ai.complete (commit msg / PR desc)", "relay RPC")
  Rel(relay_bridge, relay, "SSH tunnel → Dev Server", "relay protocol")
  Rel(git_rpc, relay, "Forward git commands", "relay.call")
  Rel(fs_rpc, relay, "Forward fs commands", "relay.call")
```

### C3.12b — Cross-panel Event Flow

```
WorkspaceContext Event Bus:

agent.complete event:
    GitPanel  ←── refresh gitStatus (immediate, no 5s wait)
    ExplorerPanel ←── refresh git decorations
    TasksPanel ←── check linked task → advance to 'review'
    NotificationBanner ←── "Agent done. 3 files changed"

git.commit event:
    TasksPanel ←── scan message for #TG-xxx → close tasks
    GitPanel ←── refresh ahead/behind count
    ExplorerPanel ←── refresh decorations (post-commit = clean)

worktree.switched event:
    GitPanel ←── reload status for new worktree path
    ExplorerPanel ←── reload file tree at new worktree path
    TerminalPanel ←── update default cwd

workflow.step.complete event:
    GitPanel ←── auto-fetch if step pushed code
    TasksPanel ←── refresh execution-linked tasks
```

### C3.12c — Remote File Explorer Data Flow

```
User expands 📁 src/ in Explorer
    │
    ▼ relay.call('fs.readDir', { path: '/srv/projects/vnp-blc/src', depth: 1 })
    │
    ├── relay → dev server: fs.readdir('/srv/projects/vnp-blc/src')
    ├── returns: [{ name: 'auth', isDir: true }, { name: 'index.ts', size: 2048 }]
    │
    ▼ overlay git status decorations:
    │   gitStatusMap = { 'src/auth/auth-manager.ts': 'M', 'src/auth/bcrypt.ts': 'A' }
    │   (pre-fetched in WorkspaceContext.gitStatus)
    │
    ▼ Render:
    │   📁 src/
    │   ├── 📁 auth/       [M] ← parent folder decorated if child modified
    │   │   ├── 📄 auth-manager.ts  [M]
    │   │   └── 📄 bcrypt-utils.ts  [A]
    │   └── 📄 index.ts

User clicks 📄 auth-manager.ts
    │
    ▼ relay.call('fs.readFile', { path: '...auth-manager.ts', encoding: 'utf-8' })
    ▼ Detect language: TypeScript (from .ts extension)
    ▼ Open in file viewer tab (Monaco read-only, syntax highlight)
```

---

## Quick Reference — Feature → Component Mapping

| Feature | Mô tả ngắn | Component(s) | ADR |
|---------|-----------|-------------|-----|
| F01 | Parallel Worktrees | C3.1 (OrcaRuntime, WorktreeManager) | — |
| F02 | Terminal Splits | C3.1 (PtyManager), C3.2 (Daemon) | — |
| F04 | AI Agent Support | C3.1 (AgentSpawner), C3.8 | — |
| F07 | SSH Worktrees | C3.3 (Relay), C3.5 (DevServerMgr) | ADR-004 |
| F22 | Web Server Mode | C3.6 (NodeAdapter, NodeIpcBridge) | ADR-001 |
| F23 | Multi-User Auth | C3.1 (AuthManager, WsSessionRouter) | ADR-003 |
| F24 | Per-User Sandbox | C3.1 (SessionManager, UserProcess) | ADR-003 |
| F25 | Admin Panel | C3.1 (Admin SPA routes) | — |
| F26 | Multi-Database | C3.7 (IConnectionPool, adapters) | ADR-002 |
| F27 | Fleet Health | C3.5 (FleetHealthMonitor) | ADR-004 |
| F28 | Dev Server Onboarding | C3.5 (DevServerProvisioner) | ADR-004 |
| F29 | Agent WebSocket | C3.8 (AgentWebSocketServer, WsTransport) | ADR-005 |
| F30 | Remote Integrations | C3.9 (WebCredentialStore, GH/GL auth) | ADR-006 |
| F31 | Fleet Provisioning | C3.5 (FleetBootstrap) | ADR-004 |
| F33 | Profile Hierarchy | C3.10a (ProfileResolver, OrcaProfile) | ADR-007 |
| F34 | Project Binding | C3.10b (ProjectService, ProjectServerRouter) | ADR-007, ADR-011 |
| F35 | AI Provider Mgmt | C3.11a (AIProviderService, ProviderResolver) | ADR-008 |
| F36 | Workflow Orchestration | C3.11c (WorkflowOrchestrator, DAGBuilder) | ADR-009 |
| F37 | Task Graph | C3.11b (TaskService, TaskGrantService, TaskAIPlanner) | ADR-010 |
| F38 | Project Workspace | C3.12 (WorkspaceContext, RelayConnectionPool) | ADR-011 |
| F39 | Remote Git UI | C3.12 (git-handler relay, GitPanel) | ADR-012 |

---

## C3.13 — Dev Server Agent Components (v6.0 NEW)

```mermaid
C4Component
  title Components — Dev Server Agent (headless, no UI)

  Container_Boundary(agent, "Dev Server Agent (Node.js binary)") {

    Component(rpc_server, "Agent RPC Server", "TypeScript / WebSocket",
      "JSON-RPC 2.0 over WebSocket\\nHandshake + capability advertisement\\nContext verification (HMAC-SHA256)\\nMethod router + event emitter")

    Component(context_verifier, "Context Verifier", "TypeScript",
      "Verify signed RpcExecutionContext\\nHMAC-SHA256 signature check\\nExpiry validation (30s TTL)\\nPath traversal prevention")

    Component(pty_manager, "PTY Manager", "TypeScript / node-pty",
      "Create/resize/write/kill PTY sessions\\nPer-userId session registry\\nOutput streaming via event.stream\\nSession state persistence (local SQLite)")

    Component(agent_spawner, "Profile-Aware Agent Spawner", "TypeScript",
      "Spawn AI agents (Claude, Codex, Gemini...)\\nInject resolvedProfile as env vars\\nValidate approvedModels whitelist\\nOSC state detection (idle→running→complete)\\nUsage tracking per accountId")

    Component(worktree_engine, "Worktree Engine", "TypeScript",
      "git worktree add/remove/list\\nFan-out: create N worktrees + spawn agents\\nOrphaned worktree recovery\\nLocal SQLite: worktree metadata")

    Component(git_engine, "Git Engine", "TypeScript",
      "git status, diff, add, commit, push, log\\nStream progress events\\nPer-user GH_CONFIG_DIR isolation\\nPR creation via gh CLI\\nCommit author injection from ctx")

    Component(fs_engine, "File System Engine", "TypeScript",
      "readDir, readFile, writeFile (under projectRoot)\\nfs.watch → event.fsChange\\nripgrep search\\nPath traversal enforcement via SecureFs")

    Component(ai_cred_store, "AI Provider Credential Store", "TypeScript",
      "AES-256-GCM encrypted files (.enc)\\nKey: scrypt(ORCA_AI_CREDENTIAL_KEY + accountId)\\nWrite, delete, test, list operations\\nNever exposes plaintext over network")

    Component(step_executor, "Workflow Step Executor", "TypeScript",
      "Execute workflow steps dispatched from Gateway\\nStep types: agent, shell, action\\nShell: execFile() (no injection), timeout, whitelist\\nAgent: delegates to agent_spawner\\nStream output via event.stepOutput")

    Component(health_reporter, "Health Reporter", "TypeScript",
      "Collect CPU%, RAM, disk, network latency\\nEmit event.health every 60s\\nExpose /health diagnostic endpoint (localhost:6790)\\nReport active PTYs, agents, worktrees")

    Component(local_db, "Local SQLite", "better-sqlite3",
      "Agent-local state (no sharing with Gateway DB)\\nTables: agent_worktrees, agent_sessions,\\n  agent_task_runs, agent_audit_log\\nPersist state across restarts")

    Component(event_bus, "Event Bus", "TypeScript / EventEmitter",
      "Internal pub/sub for all agent events\\nFan-out to: RPC Server (stream to Gateway)\\nBuffer events when Gateway disconnected")

    Component(reconnect_manager, "Reconnect Manager", "TypeScript",
      "Monitor Gateway WS connection health\\nExponential backoff reconnect (5s→60s max)\\nBuffer events during disconnection (max 1000)\\nSync state on reconnect (reconnect.sync)")
  }

  Rel(rpc_server, context_verifier, "verifies context for every call")
  Rel(rpc_server, pty_manager, "pty.* methods")
  Rel(rpc_server, agent_spawner, "agent.* methods")
  Rel(rpc_server, worktree_engine, "worktree.* methods")
  Rel(rpc_server, git_engine, "git.* methods")
  Rel(rpc_server, fs_engine, "fs.* methods")
  Rel(rpc_server, ai_cred_store, "aiProvider.* methods")
  Rel(rpc_server, step_executor, "step.* methods")
  Rel(rpc_server, health_reporter, "health.* methods")

  Rel(agent_spawner, ai_cred_store, "load credentials for spawn")
  Rel(agent_spawner, pty_manager, "spawn into PTY")
  Rel(step_executor, agent_spawner, "agent step type")
  Rel(step_executor, git_engine, "action step: git ops")

  Rel(pty_manager, event_bus, "emit PTY output events")
  Rel(agent_spawner, event_bus, "emit agent status events")
  Rel(git_engine, event_bus, "emit git change events")
  Rel(fs_engine, event_bus, "emit fs watch events")
  Rel(step_executor, event_bus, "emit step output/complete events")
  Rel(health_reporter, event_bus, "emit health events")

  Rel(event_bus, rpc_server, "stream events to Gateway")
  Rel(rpc_server, reconnect_manager, "connection state management")

  Rel(pty_manager, local_db, "persist session state")
  Rel(worktree_engine, local_db, "persist worktree metadata")
  Rel(step_executor, local_db, "persist step execution state")
  Rel(rpc_server, local_db, "write audit log")
```

### Agent Startup Sequence

```
orca-agent start
  │
  ├─ 1. Load config (/etc/orca-agent/config.yaml)
  ├─ 2. Init local SQLite (run migrations)
  ├─ 3. Start Health Reporter
  ├─ 4. Start RPC Server (internal only)
  ├─ 5. Connect to Gateway (outbound WS)
  ├─ 6. Perform handshake (capabilities advertisement)
  ├─ 7. Start emitting health events every 60s
  └─ 8. Ready to receive RPC calls
```

### Agent Isolation Model (without fork())

Dev Server Agent không fork() per user như Gateway (F24). Isolation được enforce qua:

| Mechanism | How |
|-----------|-----|
| PTY ownership | `ptyId` bound to `userId`, router checks ownership |
| File path | `SecureFs.validatePath()` checks `projectRoot` + `allowedRoots` |
| Git author | Injected from `ctx.userEmail`, cannot be overridden |
| AI env | `GH_CONFIG_DIR` per-userId (isolated GitHub CLI config) |
| Shell commands | `execFile()` (no shell), `disallowedCommands` whitelist |
| Audit | All RPC calls logged with `userId + outcome` |

---

## Feature → Component Map (v6.0 updated)

| Feature | Gateway Component | Agent Component |
|---------|------------------|----------------|
| F01 Parallel Worktrees | ProjectService (quota check, RBAC) | C3.13 WorktreeEngine |
| F02 Terminal Splits | WsSessionRouter (routing) | C3.13 PtyManager |
| F03 Mobile Companion | NotificationService (routing) | C3.13 EventBus |
| F04 AI Agent Support | ProfileResolver + ProviderResolver | C3.13 ProfileAwareAgentSpawner |
| F06 GitHub/Linear | WebCredentialStore (tokens) | C3.13 GitEngine (gh CLI) |
| F07 SSH Worktrees | DevServerManager (fleet) | C3.13 (SSH outbound tunneling) |
| F09 Orca CLI | AuthManager (validate) | C3.13 RpcServer (execute) |
| F11 Notifications | NotificationService | C3.13 EventBus (agent events) |
| F12 File Explorer | — (proxy) | C3.13 FsEngine |
| F13 Text Search | — (proxy) | C3.13 FsEngine (ripgrep) |
| F14 Automations | — (legacy, deprecated → F36) | C3.13 StepExecutor |
| F17 AI Vault | — (proxy) | C3.13 LocalSQLite (session storage) |
| F18 Ephemeral VM | Scheduling + quota | C3.13 StepExecutor + docker ops |
| F25 Admin Panel | Admin SPA routes (C3.1) | C3.13 HealthReporter |
| F27 Fleet Health | FleetHealthMonitor (aggregate) | C3.13 HealthReporter |
| F28 Dev Server Onboard | DevServerProvisioner | C3.13 first-connect bootstrap |
| F29 Agent WebSocket | WS multiplexer (C3.8) | C3.13 RpcServer |
| F30 Remote Integrations | WebCredentialStore | C3.13 GitEngine |
| F31 Fleet Provisioning | DevServerManager | C3.13 ReconnectManager |
| F33 Profile Hierarchy | C3.10a ProfileResolver | C3.13 ContextVerifier (receive) |
| F34 Project Binding | C3.10b ProjectService | C3.13 ContextVerifier (enforce) |
| F35 AI Provider Mgmt | C3.11a AIProviderService (meta) | C3.13 AiCredStore |
| F36 Workflow Orchestration | C3.11c WorkflowOrchestrator (DAG, dispatch) | C3.13 StepExecutor |
| F37 Task Graph | C3.11b TaskService (grant, plan) | C3.13 ProfileAwareAgentSpawner |
| F38 Project Workspace | C3.12 WorkspaceContext | C3.13 FsEngine + GitEngine |
| F39 Remote Git UI | — (proxy) | C3.13 GitEngine |

---

## Component Dependency Graph (v6.0)

```
═══════════════ GATEWAY (Control Plane) ═══════════════

C3.6 Platform ──────────────────────────────────────────┐
                                                         ↓
C3.7 Database ←── C3.1 Main Process ←── C3.10 Profile/Project
                         ↑                     ↓
C3.5 Fleet ──────────────┼──────────── C3.11 AI Provider + Task + Workflow
                         ↓                     ↓
                  C3.8 Agent WS         C3.12 Project Workspace
                  (WS multiplexer)
                         ↓
         AgentConnectionManager ←─ Signed Context Issuer
                         │
                   [wss:// outbound]
                         ↓

═══════════════ DEV SERVER AGENT (Data Plane) ═══════════════

C3.13 RpcServer ←── C3.13 ContextVerifier
        │
        ├── C3.13 PtyManager ──────── C3.13 ProfileAwareAgentSpawner
        │                                          ↓
        ├── C3.13 WorktreeEngine ──── C3.13 AiCredStore
        │
        ├── C3.13 GitEngine
        ├── C3.13 FsEngine
        ├── C3.13 StepExecutor ────── C3.13 ProfileAwareAgentSpawner
        │                                          │
        └── C3.13 HealthReporter      C3.13 LocalSQLite
                         ↑
              All → C3.13 EventBus → C3.13 RpcServer (stream to Gateway)
```

Thứ tự khởi động Agent (`orca-agent start`):
1. Load config + validate
2. `LocalSQLite` → init + run migrations
3. `HealthReporter` → start metrics collection
4. `PtyManager`, `WorktreeEngine`, `GitEngine`, `FsEngine`, `AiCredStore` → init
5. `StepExecutor` → register step type handlers
6. `RpcServer` → bind to internal port
7. `ReconnectManager` → connect to Gateway wss://
8. Handshake → capability advertisement
9. Ready
