**Level:** 3 — Components (bên trong các Containers)  
**Mô tả:** Chi tiết các components trong mỗi container quan trọng  
**Cập nhật:** 2026-07-28 (thêm C3.5 Fleet+DevServer, C3.6 Platform, C3.7 DB, C3.8 AgentWS, C3.9 Credentials, C3.10 Profile+Project)  
**Correction pass:** 2026-08-14 — đối chiếu với 6 audit `file:line` (`audit/backend/backend-vs-design-review.md`, `audit/agent/*.md`) để sửa port/namespace/tên hàm sai và tách rõ phần đã triển khai khỏi tầm nhìn "Dev Server Agent v6.0" (ADR-017/018, `docs/adrs/v2/`) chưa có trong code.

**Quy ước trạng thái dùng trong tài liệu này:** ✅ khớp code · ⚠️ khớp một phần / sai chi tiết · ❌ không tồn tại trong code (bug hoặc tài liệu sai) · 🚧 Proposed, chưa implement (tầm nhìn kiến trúc, không phải mô tả hệ thống hiện tại).

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
      "⚠️ Không có symbol tên này trong code. Spawn/stop/monitor thật nằm rải ở\nProfileAwareAgentSpawner (backend, luôn relay qua agent.exec)\nvà SubAgentSpawner (agent/relay/agent-spawner.ts, Dev Server tier)")

    Component(agent_awake_service, "Agent Awake Service", "TypeScript",
      "⚠️ Thật ra chỉ là power-save-blocker (desktop/src/main/agent-awake-service.ts)\nkhông tự parse OSC — nhận AgentAwakeStatus.state từ nơi khác.\nKhông có state machine 4-state (idle→running→waiting→completed) thống nhất nào\ntrong code — xem 3 state machine rời rạc ở Design Pattern #2 bên dưới")

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
      "git diff, status, commit, push\nWorktree operations trên remote\nBranch management\n⚠️ Có 3 bộ thực thi git/gh song song không chia sẻ code trong agent/:\nGitHandler (git-handler.ts, engine chính), agent-git-handler.ts,\nexternal-api-connector.ts — cả 2 cái sau đều đăng ký case 'git.pr.create'\nvà 'github.pr.create' cho cùng chức năng tạo PR, không hợp nhất")

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
      "⚠️ Không có class `FleetBootstrapService` — hàm thật là bootstrapServer()\nOrchestrate per-server setup\n❌ Thiếu 2/7 bước: disk-space check (≥5GB), verify SHA256 relay binary")

    Component(fleet_remote_cmds, "Fleet Remote Commands", "TypeScript / SSH",
      "Execute remote commands qua SSH exec\nIsolated per-server connection\nCommand: node-version, git-version, relay-start")

    Component(fleet_health_monitor, "Fleet Health Monitor", "TypeScript",
      "Poll all servers every 60s (đúng)\n❌ KHÔNG thu thập CPU%/RAM%/disk%/latency — khái niệm này\nkhông tồn tại trong data model; monitor chỉ đọc SshConnectionStatus\nđã có sẵn. `pingLatencyMs` là dead field, không bao giờ được ghi")

    Component(fleet_health_store, "Fleet Health Store", "TypeScript / in-memory",
      "In-memory cache của server health\nTrend tracking (last N samples)\nLookup by serverId")

    Component(fleet_status_service, "Fleet Status Service", "TypeScript",
      "Aggregate health across fleet\nWebhook alerts on status change\nPrometheus metrics endpoint")

    Component(server_grouping, "Server Grouping", "TypeScript",
      "groupSshTargetsByProject()\nSshTargetGroup, SshTargetGroupedList\nProject + tag filtering")

    Component(rbac_resolver, "RBAC Policy Resolver", "TypeScript",
      "❌ `hasPermission(userId, resource, action)` không tồn tại trong code.\nThực tế RBAC bị phân mảnh 4 cơ chế không liên kết:\nresolveUserPermissions() (fleet, allowlist union), requireAdmin (HTTP route,\nđúng), requireAdmin (RPC handler — STUB không check role, xem C3.10a),\nrequireOwnerOrAdmin (project — không check admin, dead code trong tên hàm)")

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
      "❌ Không tồn tại trong code. `desktop/src/main/index.ts` import thẳng\npackage `electron` thật, không qua interface trừu tượng nào — lời hứa\n\"swap adapter không đổi business logic\" chưa hiện thực ở nhánh Electron.\nCác file platform/stubs/electron-*-stub.ts làm chiều NGƯỢC lại\n(giả lập Electron API cho NodeAdapter/web), không phải bản thân adapter này")

    Component(node_adapter, "NodeAdapter", "TypeScript / Node.js only",
      "No Electron dependency\nNodeApp: userData=~/.orca, getPath()\nNodeWindow: noop (headless)\nNodeIpcBridge: WebSocket RPC bridge\nNodeSecureStorage: AES-256-GCM file")

    Component(ipc_bridge, "IRpcRouter", "TypeScript",
      "Route RPC method calls\nElectron: ipcMain.handle()\nNode: WebSocket JSON-RPC dispatch\nShared: method registry map")

    Component(web_entry, "Web Entry Bootstrap", "TypeScript",
      "backend/src/server/index.ts (hàm thật: initializeOrcaServices(),\nkhông phải bootstrapWebApp — tên đó thuộc frontend web bootstrap)\nStart RPC/WS :6768 (browser, single-user) + HTTP :6769\n⚠️ Agent WS (/agent) và WS multi-user (ORCA_MULTI_USER=1) đều\ngắn vào httpServer :6769, KHÔNG phải :6768 — xem C3.8")
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
      "⚠️ Sequential apply: 0001→0002→...→0013 (13 migration thật, không\ndừng ở 0005 — xem bảng cập nhật bên dưới)\nDialect-aware DDL (IF NOT EXISTS)\nTracking table: orca_migrations\nIdempotent re-run safe")

    Component(db_health, "DatabaseHealthMonitor", "TypeScript",
      "Ping DB every 30s (đúng)\nStatus: connected|degraded|disconnected\n❌ Auto-reconnect on failure — chưa thấy code hiện thực,\nchỉ emit trạng thái unhealthy/degraded\nMetrics: latency, errorCount")

    Component(repo_factory, "Repository Factory", "TypeScript",
      "❌ `ORCA_STORAGE_BACKEND` env không tồn tại trong code — lựa chọn\njson/sql thực tế dựa vào loadDatabaseConfig() trả null hay không\n⚠️ Tên class thật: JsonFileStateRepository / SqlStateRepository\n(không phải JsonFileRepository/SqlRepository)")
  }

  Rel(db_provider, sqlite_adapter, "creates (sqlite)")
  Rel(db_provider, mysql_adapter, "creates (mysql/tidb)")
  Rel(db_provider, pg_adapter, "creates (postgresql)")
  Rel(iconn_pool, db_iface, "pools")
  Rel(migration_runner, db_iface, "runs migrations on")
  Rel(db_health, iconn_pool, "monitors")
  Rel(repo_factory, iconn_pool, "uses (sql mode)")
```

**Migrations (0001–0013) — bảng thật, khác đáng kể so với bản gốc tài liệu này (❌ sai lệch nghiêm trọng nhất được audit ghi nhận, `audit/backend/backend-vs-design-review.md` §2.7):**

| Migration | Tables tạo ra (thực tế trong code) | Ghi chú |
|-----------|----------------|---------|
| 0001 | `settings`, `projects`, `repos`, `ssh_targets` | ❌ khác hoàn toàn bản cũ của tài liệu (`projects/worktrees/agent_sessions/settings`) |
| 0002 | `automations` | ❌ khác hoàn toàn (`terminal_scrollback_snapshots`) |
| 0003 | `workspace_sessions` | ❌ khác hoàn toàn (`ssh_hosts/saved_port_forwards`) |
| 0004 | `orca_projects`, `orca_repos`, `orca_ssh_targets`, `orca_global_settings` | ❌ khác hoàn toàn (`automations/automation_runs/notifications/rate_limits`) |
| 0005 | `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies` | ✅ khớp bản cũ |
| 0006 | `orca_companies` (số nhiều), `orca_departments`, `orca_user_profiles` | mới, chưa có trong tài liệu trước đây |
| 0007 | `orca_v5_projects`, `orca_v5_project_members` | đổi tên (`v5_`) để tránh đụng bảng `orca_projects` đã bị 0004 chiếm |
| 0008 | `orca_ai_provider_accounts`, `orca_provider_usage` | mới |
| 0009 | `orca_workflow_templates`, `orca_workflow_executions`, `orca_workflow_step_executions` | mới |
| 0010 | `orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments`, `orca_team_members` | mới |
| 0011–0013 | `terminal_sessions`, `port_forwards`, `push_subscriptions`, workflow trace correlation (`root_trace_id`) | mới, hoàn toàn chưa có trong tài liệu trước đây |

Lưu ý: `docs/adrs/v2/ADR-016` đề xuất tên bảng khác (`orca_company`, `orca_projects` số ít) cho migration 0006/0007 — ADR đó tự nhận `❌ Chưa implement`, và code thật cũng không theo đúng đề xuất đó (dùng `orca_companies`/`orca_v5_projects`), nên ADR-016 vừa "chưa implement" vừa đã "bị code thật vượt qua theo hướng khác" — không dùng ADR-016 làm nguồn tham chiếu tên bảng.

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
      "direct-websocket mode: Agent → Orca\n⚠️ Listen on ws://orca:6769/agent (KHÔNG phải 6768 — gắn vào\nhttpServer/httpPort, không phải rpcPort; comment trong code tự\nmâu thuẫn với 1 message lỗi runtime vẫn ghi sai :6768)\nHandshake: agent gửi trước agent.handshake{agentToken} (không có\nhandshake-request/handshake-ok riêng, chỉ là JSON-RPC response thường)\nWire WsTransport ↔ SshChannelMultiplexer")

    Component(agent_token_mgr, "AgentTokenManager", "TypeScript",
      "❌ Mô hình khác hẳn thiết kế — không có admin UI/DB table nào.\nAgent tự yêu cầu token qua POST /api/agent-token, auth bằng\nORCA_AGENT_API_SECRET (shared secret, không phải admin session).\nToken dạng đoán được agt-<devServerId>-<timestamp>\n(không phải crypto.randomBytes(32)). Có hash SHA-256 trước khi\nlưu vào Map in-memory (không phải DB). Không có endpoint revoke.\nAgentTokenManager phía agent renew chủ động ở 80% TTL (24h) —\ncơ chế này không tài liệu hoá ở đâu trước audit")

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
      "⚠️ `CredentialService` thật chỉ cover 'bitbucket'|'azure-devops'|'gitea'|\n'linear'|'jira' — GitHub/GitLab KHÔNG đi qua store này, dựa vào\nOS keychain của CLI `gh`/`glab` thay vào đó (xem preflight_proxy bên dưới)\nRoute to correct store (WebCredentialStore / EnvToken)")

    Component(web_cred_store, "WebCredentialStore", "TypeScript / AES-256-GCM",
      "Per-user encrypted file store, đúng AES-256-GCM\n⚠️ Env var thật là `ORCA_SERVER_SECRET` (không phải `ORCA_CREDENTIAL_KEY`\n— tên đó không xuất hiện ở đâu trong code)\n⚠️ salt ngẫu nhiên 32-byte mỗi lần ghi (không derive từ userId), iv=16 byte\n(không phải 12); có cơ chế migrate V1→V2 chưa tài liệu hoá")

    Component(cred_rpc, "Credentials RPC Methods", "TypeScript",
      "credentials.set(service, token) → encrypt + store\ncredentials.revoke(service) → delete\ncredentials.status(service) → { configured, lastValidated }\ncredentials.list() → [service names only, no tokens]")

    Component(preflight_proxy, "Preflight Check Proxy", "TypeScript",
      "Category A: CLI-based integrations\n✅ CHỈ `preflight.check` thật sự relay qua SSH tới Dev Server\n❌ Mọi nghiệp vụ GitHub/GitLab hàng ngày khác (list PR/issues, create,\nrate-limit, project-view...) chạy `gh`/`glab` NGAY TRONG PROCESS BACKEND\n(`ghExecFileAsync`→`child_process.execFile`), KHÔNG relay tới Dev Server —\nvi phạm nguyên tắc \"Auth never through Gateway\" của\n`docs/hld/dev-server-architecture.md §12`. Implementation đúng thiết kế\nđã có sẵn ở agent/ (agent-git-handler.ts, external-api-connector.ts)\nnhưng Backend không hề gọi tới — dead code từ góc nhìn Backend\nmergePreflightStatuses(local, relay)")

    Component(gh_config_isolation, "GH Config Session Isolation", "TypeScript",
      "GH_CONFIG_DIR=~/.config/gh/<userId>/\nGLAB_CONFIG_DIR=~/.config/glab-cli/<userId>/\n❌ Backend không hề set 2 env này (grep xác nhận 0 kết quả trong\nbackend/src/main) — kể cả 2 luồng relay hẹp (`pty.spawn` gọi `gh`)\ncũng truyền `env:{}` rỗng. Trong Web Server multi-user mode, mọi user\nchia sẻ CHUNG một auth context `gh`/`glab` trên host Backend")

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

⚠️ **Sai khác với code thật:** không có env `ORCA_STORAGE_BACKEND` — lựa chọn json/sql dựa vào `loadDatabaseConfig()` (`server-bootstrap.ts`) trả `null` hay không. File JSON thật tên **`store.json`**, không phải `orca-data.json`. Tên class thật `JsonFileStateRepository`/`SqlStateRepository`.

```
loadDatabaseConfig() (server-bootstrap.ts)
      │
      ├── null            → JsonFileStateRepository (store.json)
      └── DatabaseConfig  → SqlStateRepository
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

⚠️ Port thật là **6769** (không phải 6768), và handshake do **agent chủ động gửi trước** — không có bước `handshake-request` riêng từ Orca. Xem `audit/agent/connection-wire-protocol-vs-design-review.md` §2.1, §2.3.

```
relay-websocket mode (Orca → Agent):
  Orca → HTTP Upgrade: ws://agent:PORT/orca-relay
       Header: Authorization: Bearer <agentToken>
  ► WsTransport ⇔ SshChannelMultiplexer ⇔ JSON-RPC

direct-websocket mode (Agent → Orca):
  Agent → ws://orca:6769/agent   (⚠️ code thật, không phải 6768)
  Agent → JSON-RPC request id:1, method: 'agent.handshake' { agentToken, capabilities }
          (agent gửi trước ngay khi WS mở — không có handshake-request/handshake-ok
           riêng biệt; đây là response JSON-RPC {result:{ok:true}} thông thường)
  ► WsTransport ⇔ AgentWebSocketServer ⇔ JSON-RPC
```

### 8. Integration Preflight Proxy Pattern (github CRs)

⚠️ Sơ đồ dưới đây mô tả đúng cho **riêng** `preflight.check`. Các RPC nghiệp vụ GitHub/GitLab khác (`github.*`/`gitlab.*` — list issue, PR, merge...) **không** đi qua relay này — Backend tự chạy `gh`/`glab` cục bộ, không per-user isolation. Xem `preflight_proxy` ở C3.9 và `audit/backend/backend-vs-design-review.md` §2.12b.

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
      "⚠️ profile.getResolved(userId) (không phải getEffective — tên đó\nkhông tồn tại). profile.updateUser/updateDepartment/updateCompany đúng tên.\n❌ BUG BẢO MẬT NGHIÊM TRỌNG: \"— admin only\" trên updateCompany/\nupdateDepartment KHÔNG được enforce thật. `requireAdmin(ctx)` trong\nprofile-rpc-handler.ts chỉ check ctx.userId tồn tại (đã login), KHÔNG\ncheck role==='admin' — bất kỳ user đã login nào set được chính sách\nbảo mật toàn công ty. Xem `audit/backend/backend-vs-design-review.md` §5.11")
  }

  ContainerDb(db, "Server DB", "PostgreSQL/MySQL/SQLite",
    "orca_companies.profile_json (số nhiều — không phải orca_company)\norca_departments.profile_json\norca_users.profile_json + department_id")

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
      "⚠️ Component thật nằm ở backend/src/main/project/ProfileAwareAgentSpawner.ts\n(Gateway tier) — spawn() LUÔN đi qua relay.call('agent.exec', ...),\nkhông có nhánh node-pty.spawn local nào\n❌ \"Profile hierarchy điều khiển agent spawn\" phần lớn KHÔNG xảy ra\ntrong agent/ (Dev Server tier): OrcaProfile.ts ở agent/ là type-only\ndead code — không hàm runtime nào đọc shell.envVars/pathAdditions/\npreferredModel/trustPreset. Chỉ GH_CONFIG_DIR/GLAB_CONFIG_DIR per-userId\nlà thật (nhưng hardcode theo process.env.HOME, không đọc từ profile)")

    Component(project_context, "ProjectContextInjector", "TypeScript",
      "Build system preamble cho agent:\n  project name, repo URL, branch\n  dev server hostname\n  developer name, team\nInject vào agent pty initFile")

    Component(project_rpc, "project.* RPC methods", "TypeScript",
      "⚠️ Namespace thật là `project.*` (số ít), không phải `projects.*`\nproject.list(userId) / get(projectId) / create(input) /\naddMember(projectId, userId, role) / removeMember(projectId, userId)\n❌ `updateBinding` không tồn tại — KHÔNG rebind/đổi devServerId được\nsau khi project đã tạo (mâu thuẫn với F34 mô tả Lead/Admin đổi qua\nProject Settings). `project.create` cũng không giới hạn Lead/Admin\n— bất kỳ user login nào tạo được project + bind dev server")
  }

  Component(relay_bridge, "DevServerRelayBridge", "TypeScript",
    "SSH relay / WS relay connection\nForwards RPC to dev server")

  ContainerDb(db, "Server DB", "",
    "orca_projects (id, dev_server_id, repo_path)\norca_project_members (project_id, user_id, role)")

  Person(lead, "Lead / Admin", "Tạo project, assign server, manage members")
  Person(dev, "Developer", "Chọn project → tạo worktree → start agent")

  Rel(lead, project_rpc, "Tạo project (⚠️ không rebind được sau khi tạo)", "RPC: project.create")
  Rel(dev, project_rpc, "Xem danh sách projects", "RPC: project.list")
  Rel(dev, project_router, "Tạo worktree cho project", "Click: New Worktree")
  Rel(project_router, project_service, "Get devServerId", "project.devServerId")
  Rel(project_router, relay_bridge, "Route relay call", "relay.call('git.worktree.add', ...)")
  Rel(project_router, profile_spawner, "Spawn agent trên dev server", "spawnAgent(project, userId)")
  Rel(profile_spawner, project_context, "⚠️ initFile/system preamble không tìm thấy trong agent/", "buildProjectContext() (không xác nhận được)")
  Rel(profile_spawner, relay_bridge, "Spawn agent trên dev server", "relay.call('agent.exec', {env}) — ⚠️ không phải 'pty.spawn': pty.spawn thật chỉ spawn shell, agent binary spawn qua agent.spawn/agent.exec")
  Rel(project_service, db, "Persist", "SQL: orca_projects + orca_project_members")
```

### C3.10c — Inheritance Resolution Flow

⚠️ **Phần merge 3-tầng (company←dept←user) và `ProfileCache` là đúng thiết kế và có code thật** (`ProfileResolver`, cache TTL 60s). Nhưng khối **"Agent spawn uses:"** ở cuối sơ đồ là **KHÔNG implement trong `agent/`** — `OrcaProfile.ts` phía agent là type-only dead code, không hàm runtime nào đọc `ResolvedProfile`. `ANTHROPIC_MODEL` và `PATH` mở rộng từ `pathAdditions` **không được set ở đâu cả**; chỉ `GH_CONFIG_DIR` tồn tại nhưng hardcode theo `userId`/`HOME`, không đọc từ profile đã resolve. `trustPreset` có khai báo trong interface nhưng không được đọc trong `buildAgentEnv()`. Xem `audit/agent/pty-ai-cli-vs-design-review.md` §2.3.

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
      "Background check mỗi 15 phút\nTest connection qua relay → dev server\nQuota tracking: tokens_used/day\n❌ Alert at 80% quota không tồn tại — chỉ phát hiện SAU khi đã\nvượt quota (status 'quota_exceeded'), không cảnh báo sớm")

    Component(credential_relay, "ProviderCredentialRelay", "TypeScript",
      "⚠️ Class này không tồn tại — logic nằm trong\nAIProviderService.writeCredentialToDevServer()\nEncrypt credentials in browser (SubtleCrypto)\nRelay encrypted blob qua SSH\n⚠️ Dev server KHÔNG decrypt Layer-1 — chỉ bọc thêm 1 lớp AES-256-GCM\nngoài blob còn nguyên mã hoá (double-encrypt, chặt hơn thiết kế nhưng\nkhác cơ chế mô tả)\n❌ Path lưu thật: ~/.orca/credentials/<accountId>.enc — file khớp path\ntài liệu này (~/.orca/ai-providers/) là code CHẾT, 0 caller\n(agent/src/relay/ai-provider-handler.ts). Xem C3.13 ai_cred_store")

    Component(provider_rpc, "aiProvider.* RPC", "TypeScript",
      "⚠️ Namespace thật là `aiProvider.*` (camelCase), không phải\n`ai-providers.*`\naiProvider.list(devServerId) / create(input) — admin/lead /\ntestConnection() / getUsage(accountId, date)\n❌ rotateKey(accountId) không tồn tại — key rotation (grace period,\nstatus 'rotating', audit log) hoàn toàn chưa implement; update key\nhiện tại chỉ ghi đè trực tiếp qua writeCredential")
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
  Rel(provider_rpc, credential_relay, "Save credentials (rotate không tồn tại)", "")
```

⚠️ **"Orca Server không thấy plaintext key" chỉ đúng cho luồng ghi.** Ở luồng spawn agent (dùng key), comment trong code (`agent-spawner.ts`) tự thừa nhận Orca Server phải cung cấp `resolvedApiKey` dạng **plaintext** qua params khi có Layer-1 session key — mâu thuẫn với khẳng định bảo mật "Gateway không thấy plaintext". Nhánh fallback (không có `resolvedApiKey`) còn tệ hơn: inject thẳng ciphertext Layer-1 chưa giải mã vào biến env API key — tự nhận trong code là có thể fail auth. Xem `audit/agent/credential-fswatch-telemetry-vs-design-review.md` §2.1.

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

    Component(task_rpc, "task.* RPC", "TypeScript",
      "⚠️ Namespace thật là `task.*` (số ít), không phải `tasks.*`\ntask.list / get / create / update / delete\n⚠️ addDependency→addEdge; aiPlan tách thành aiDecompose+aiApply;\nrunAgent→execute (tên khác thiết kế)\ntask.getGraph(rootId) / grant / revoke / listGrants / shareLink(taskId)")
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
      "project:<id> → project.devServerId (✅ hoạt động)\n❌ server:<id> → ném lỗi tường minh \"not yet implemented\"\n❌ fleet:tag:<tag> → hoàn toàn không xử lý (hạ tầng fleet tồn tại\nriêng nhưng không wire vào workflow)")

    Component(step_executors, "Step Executors", "TypeScript",
      "⚠️ Thực tế chỉ có 5 step type: agent/shell/webhook/notification/condition\n(thiếu 'action' generic dispatcher; 'parallel' chỉ đạt được ngầm định\nqua wave execution, không phải executor riêng)\n❌ Provider selection theo từng step: 0% code — StepExecutors không hề\nimport AIProviderService/ProviderResolver dù đây là tính năng chính\ncủa F36 (mix Claude/GPT-4o giữa các bước)\n❌ Không có retry/backoff logic cho step")

    Component(wf_rpc, "workflow.* RPC", "TypeScript",
      "⚠️ Namespace thật là `workflow.*` (số ít), không phải `workflows.*`\nworkflow.listTemplates / create/update/delete / getExecution(execId)\n❌ Không có `run` — tên thật là `execute`/`cancel`\n❌ pause/resume hoàn toàn không tồn tại — WorkflowStatus không có\n'paused'; resumeRunningExecutions() chỉ là crash-recovery khi server\nrestart, không phải user-triggered pause/resume qua UI\n❌ streamStepOutput không tồn tại — UI chỉ poll, không streaming/WS push")
  }

  ContainerDb(db, "Server DB", "",
    "orca_workflow_templates\norca_workflow_executions\norca_step_executions")

  Rel(orchestrator, server_resolver, "Resolve devServerId per step", "")
  Rel(orchestrator, step_executors, "Execute each step", "")
  Rel(orchestrator, db, "Persist execution state", "SQL")
  Rel(step_executors, provider_resolver, "❌ Không tồn tại trong code — 0% implement, xem step_executors ở trên", "BL-AIP-02 (aspirational)")
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
      "git.status / diff / add / commit\ngit.push / pull (stream)\n⚠️ Không có sub-namespace lồng git.branch.*/git.worktree.* — thực tế\nphẳng (vd. git.branchCompare, git.worktree.list) — ~35 method, nhiều\nhơn tài liệu liệt kê")

    Component(fs_rpc, "fs.* RPC methods", "TypeScript",
      "❌ `fs.*` chỉ là giao thức nội bộ backend→dev-agent — API client\nmà frontend thật sự gọi là `files.*` (28 method, không phải `fs.*`)\nfs.search không tồn tại (tên thật fs.grep, ở tầng nội bộ)")
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

## C3.13 — Dev Server Agent Components — 🚧 Proposed, chưa implement

> **Toàn bộ C3.13 trở xuống (bao gồm "Feature → Component Map (v6.0 updated)" và "Component Dependency Graph (v6.0)") mô tả tầm nhìn kiến trúc "Dev Server Agent v6.0"** — layer model A0–A4, `ContextVerifier`, `SignedExecutionContext` (HMAC-SHA256), Control Plane/Data Plane tách biệt nghiêm ngặt — được đặc tả ở `docs/adrs/v2/ADR-013..020` (đặc biệt ADR-017 "layer model", ADR-018 "control/data plane separation"). **Không một phần nào của layer model này tồn tại trong code hiện tại.** Cả 8 ADR liên quan tự ghi rõ trạng thái `❌ Chưa implement (v6.0 proposed)` / `🚧 Proposed`. Grep xác nhận: không có `ContextVerifier`, `SignedExecutionContext`, field `_ctx`, hay cấu trúc thư mục `src/agent/{rpc,pty,worktree,git,fs,execution,storage,reporting}/` ở bất kỳ đâu trong `agent/src`.
>
> **Code thật (`agent/src/relay/*`) triển khai kiến trúc khác hẳn** — không phân lớp A0–A4, không signed context: framing nhị phân 13-byte (`agent-wire.ts`/`relay-protocol.ts`, đúng thiết kế Phase-2 CR-AG-001), dispatcher phẳng `agent-rpc-dispatch.ts` (`route()`, ~40 case), và agent **tin tưởng hoàn toàn** vào renderer/Orca Server phía trên thay vì tự verify mọi request — `agent/src/relay/context.ts`'s `registerRoot()` là stub rỗng **có chủ đích** (comment trỏ `docs/relay-fs-allowlist-removal.md`: FS allowlist đã bị gỡ chủ động vì "the relay runs as the SSH user and trusts the renderer process"). Đây là quyết định kiến trúc thật, không phải thiếu sót.
>
> Bảng dưới đây giữ nguyên sơ đồ đề xuất (giá trị tham khảo thiết kế) kèm chú thích 🚧/❌ cho từng component ánh xạ sang code thật (nếu có). Xem `audit/agent/rpc-dispatch-lifecycle-vs-design-review.md` §2.1-2.7 và `audit/agent/connection-wire-protocol-vs-design-review.md` §2.7.

```mermaid
C4Component
  title Components — Dev Server Agent (headless, no UI)

  Container_Boundary(agent, "Dev Server Agent (Node.js binary)") {

    Component(rpc_server, "🚧 Agent RPC Server", "TypeScript / WebSocket",
      "JSON-RPC 2.0 over WebSocket\\nHandshake + capability advertisement\\n🚧 Context verification (HMAC-SHA256) — KHÔNG tồn tại\\nMethod router + event emitter\\n≈ Thật: createRpcDispatcher()/route() (agent-rpc-dispatch.ts) —\\nvẫn dùng binary 13-byte framing, ADR-014 đề xuất bỏ framing này\\nnhưng chưa implement")

    Component(context_verifier, "🚧 Context Verifier — KHÔNG TỒN TẠI", "TypeScript",
      "Verify signed RpcExecutionContext\\nHMAC-SHA256 signature check\\nExpiry validation (30s TTL)\\n🚧 Path traversal prevention — chủ động BỊ GỠ (không phải thiếu sót):\\ncontext.ts's registerRoot() là stub rỗng, xem banner phía trên")

    Component(pty_manager, "🚧 PTY Manager", "TypeScript / node-pty",
      "Create/resize/write/kill PTY sessions\\n≈ Thật: PTY_REGISTRY (agent-spawner.ts, cho agent.spawn PTYs)\\nTÁCH BIỆT khỏi pty-daemon-client/server.ts (cho pty.create terminals)\\n— 2 quần thể PTY khác nhau, không hợp nhất\\nSession state persistence (local SQLite) — không tồn tại")

    Component(agent_spawner, "🚧 Profile-Aware Agent Spawner (tên gọi)", "TypeScript",
      "≈ Thật: SubAgentSpawner (agent/src/relay/agent-spawner.ts, Dev Server\\ntier) — chỉ map 5 model family (claude/codex/gemini/opencode/ollama),\\nkhông phải 30+ agent như F04 công bố\\n🚧 Validate approvedModels whitelist — KHÔNG có check nào trước spawn\\n⚠️ OSC state detection — có 3 state machine rời rạc không hợp nhất,\\nkhông cái nào khớp idle→running→complete (xem Agent Isolation Model)")

    Component(worktree_engine, "🚧 Worktree Engine (tên gọi)", "TypeScript",
      "≈ Thật: git.worktree.* case trong GitHandler (agent/src/relay/git-handler.ts)\\ngit worktree add/remove/list — có thật, đúng hành vi\\n❌ Fan-out (worktree.fanout RPC): KHÔNG tồn tại\\nLocal SQLite: worktree metadata — không tồn tại")

    Component(git_engine, "🚧 Git Engine — 3 bộ thực thi song song, không hợp nhất", "TypeScript",
      "git status, diff, add, commit, push, log — có thật (GitHandler)\\n❌ KHÔNG chỉ 1 Git Engine: GitHandler + agent-git-handler.ts +\\nexternal-api-connector.ts trùng lặp — cả 'git.pr.create' và\\n'github.pr.create' đăng ký cùng chức năng tạo PR (xem C3.3)\\n❌ Commit author injection from ctx.userEmail — KHÔNG tồn tại,\\nchỉ có preflight.setGitIdentity ghi git config --global 1 lần,\\ncó thể bị override bởi bất kỳ git.exec nào sau đó")

    Component(fs_engine, "🚧 File System Engine (tên gọi)", "TypeScript",
      "≈ Thật: fs.* case trong agent-rpc-dispatch.ts (fs-handler.ts /\\nfs-agent-extensions.ts) — readDir/readFile/writeFile có thật\\n🚧 Path traversal enforcement via SecureFs — KHÔNG tồn tại (xem\\nContext Verifier ở trên, chủ động bị gỡ)")

    Component(ai_cred_store, "🚧 AI Provider Credential Store — path sai", "TypeScript",
      "AES-256-GCM encrypted files (.enc) — đúng thuật toán\\n❌ Path thật: ~/.orca/credentials/<accountId>.enc (agent-credential-store.ts,\\ncó caller thật), KHÔNG phải ~/.orca/ai-providers/<accountId>.enc\\n(path đó chỉ tồn tại trong code CHẾT: ai-provider-handler.ts, 0 caller)\\n⚠️ Salt ngẫu nhiên mỗi lần ghi, KHÔNG phải scrypt(KEY+accountId,\\nsalt=accountId) như mô tả")

    Component(step_executor, "🚧 Workflow Step Executor — KHÔNG TỒN TẠI trong agent/", "TypeScript",
      "❌ Nhóm RPC step.*/health.* hoàn toàn vắng mặt trong\\nagent-rpc-dispatch.ts — grep 0 kết quả cho step.execute/step.cancel")

    Component(health_reporter, "🚧 Health Reporter — KHÔNG TỒN TẠI trong agent/", "TypeScript",
      "Collect CPU%, RAM, disk, network latency — khái niệm này không tồn\\ntại ở bất kỳ đâu (kể cả FleetHealthMonitor phía Gateway, C3.5 —\\nchỉ đọc SshConnectionStatus, không đo CPU/RAM/disk)")

    Component(local_db, "🚧 Local SQLite — KHÔNG TỒN TẠI trong agent/", "better-sqlite3",
      "Agent-local state (no sharing with Gateway DB)\\nTables: agent_worktrees, agent_sessions,... — không migration/schema\\nnào cho các bảng này tìm thấy trong agent/")

    Component(event_bus, "🚧 Event Bus (tên gọi)", "TypeScript / EventEmitter",
      "≈ Thật: notification JSON-RPC trực tiếp qua ws.send/dispatcher.notify\\n(agent-rpc-dispatch.ts) — không có class EventBus riêng\\nBuffer events when Gateway disconnected — không xác nhận được")

    Component(reconnect_manager, "🚧 Reconnect Manager (tên gọi)", "TypeScript",
      "≈ Thật: reconnect logic trong agent-connection-direct.ts (có thật,\\nkhông phải class riêng)\\n⚠️ RECONNECT_DELAYS_MS = [1s,2s,5s,15s,30s] — khác công thức\\n5s→60s mô tả ở đây và khác cả ADR-019's 1s→2s→4s→8s→16s→30s")
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

### Agent Startup Sequence — 🚧 Proposed, chưa implement

`orca-agent` CLI với config `/etc/orca-agent/config.yaml`, systemd/Docker/launchd lifecycle không tồn tại trong `agent/` hiện tại (đặc tả ở CR-DS-001/CR-DS-004, tự nhận `Proposed`). Entry point thật là `agent-entry.ts` build ra `out/agent.js` (chạy trực tiếp bằng Node, không có bước cài đặt/update/version-negotiation nào).

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

### Agent Isolation Model (without fork()) — 🚧 phần lớn Proposed, chưa implement

Dev Server Agent không fork() per user như Gateway (F24). Bảng dưới đây là đề xuất isolation — cột "Thực tế" cho biết trạng thái thật trong `agent/`:

| Mechanism | How (đề xuất) | Thực tế |
|-----------|-----|---------|
| PTY ownership | `ptyId` bound to `userId`, router checks ownership | ⚠️ `ptyId` có chứa `userId` trong tên (`pty-${userId}-${taskId}-${Date.now()}`) nhưng không có dedupe/ownership-check thật ("1 agent chính per worktree per userId" không được enforce) |
| File path | `SecureFs.validatePath()` checks `projectRoot` + `allowedRoots` | ❌ Không tồn tại — bị gỡ chủ động, xem banner đầu C3.13 |
| Git author | Injected from `ctx.userEmail`, cannot be overridden | ❌ Không tồn tại — chỉ có `preflight.setGitIdentity` ghi `git config --global user.name/email` một lần, mutable, không gắn `RequestContext`, có thể bị override bởi bất kỳ `git.exec` nào sau đó |
| AI env | `GH_CONFIG_DIR` per-userId (isolated GitHub CLI config) | ✅ Có thật trong `agent/` (`agent-spawner.ts`), nhưng path hardcode theo `process.env.HOME`, không đọc từ profile — và Backend (Gateway) không bao giờ set 2 biến này khi relay, xem C3.9 |
| Shell commands | `execFile()` (no shell), `disallowedCommands` whitelist | ⚠️ `execFile()`/`spawn({shell:false})` đúng ở các implementation git/gh; không xác nhận được whitelist `disallowedCommands` cho `agent.exec` |
| Audit | All RPC calls logged with `userId + outcome` | ⚠️ Không xác nhận được trong phạm vi audit đã đọc |

---

## Feature → Component Map (v6.0 updated) — 🚧 Proposed, cột "Agent Component" chưa implement

> Cột "Gateway Component" phần lớn tồn tại (dù nhiều tên/namespace cần sửa — xem C3.1-C3.12 ở trên). Cột "Agent Component" trỏ tới các component C3.13 vừa đánh dấu 🚧 ở trên — KHÔNG coi các dòng dưới đây là bản đồ hiện trạng, chỉ là tài liệu đề xuất.

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

## Component Dependency Graph (v6.0) — 🚧 Proposed, chưa implement

> Sơ đồ dưới đây (bao gồm `AgentConnectionManager`, `Signed Context Issuer`, `ContextVerifier`) mô tả kiến trúc Control Plane/Data Plane tách biệt nghiêm ngặt của ADR-018 — chưa có trong code. Kết nối thật là agent tự mở WS outbound tới Gateway qua `agent-connection-direct.ts`/`agent-connection-relay.ts` (không có `AgentConnectionManager`/`Signed Context Issuer` phía Gateway tương ứng trong phạm vi đã audit).

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
