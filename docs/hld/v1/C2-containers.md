# C2 — Container Diagram

**Level:** 2 — Containers  
**Mô tả:** Các containers (executables, services) tạo nên hệ thống Orca  
**Cập nhật:** 2026-07-28 (v5.0 — thêm Profile/Project, AI Provider, Workflow, Task Graph layers)

---

## Sơ đồ Container

```mermaid
C4Container
  title Container Diagram — Orca AI Orchestrator IDE

  Person(user, "User", "Developer, Tech Lead, Remote Dev, Mobile User, QA, DevOps")
  Person(admin, "Admin", "Quản lý users, sessions, audit log")
  Person(agentdev, "Agent Developer", "Viết AI agent WebSocket")

  Container_Boundary(orca_desktop, "Orca Desktop (Electron App)") {
    Container(renderer, "Renderer Process", "React 19 / TypeScript / Vite",
      "UI layer: sidebar, terminal tabs, diff viewer,\nfile explorer, browser panel, notifications")
    Container(main_process, "Main Process", "Node.js 22+ / TypeScript",
      "Core logic: worktree mgmt, agent orchestration,\ngit ops, SSH, persistence, IPC handlers")
    Container(preload, "Preload Script", "TypeScript / contextBridge",
      "Electron context bridge: expose safe API\nfrom main process to renderer (no nodeIntegration)")
    Container(daemon, "Orca Daemon", "Node.js / Unix Socket",
      "Background PTY server: manages terminal sessions,\nservices multiple clients (GUI + CLI)")
  }

  Container_Boundary(orca_web_boundary, "Orca Web Server (Node.js)") {
    Container(web_http, "HTTP Server", "Express / Node.js :6769",
      "SPA serving + health + auth routes + admin API\nPOST /auth/local, GET /admin/api/*, /health/ready")
    Container(web_ws, "WebSocket Server", "ws / Node.js :6768",
      "RPC over WebSocket (web mode)\nAgent WebSocket endpoint: /agent\nWsSessionRouter: proxy per-user")
    Container(auth_layer, "Auth Layer", "AuthManager / bcrypt",
      "auth-router.ts, auth-manager.ts, auth-session-store.ts\nbcrypt 12 rounds + HTTP-only cookie + session TTL")
    Container(session_mgr, "Session Manager", "SessionManager / fork()",
      "Per-user Node.js process isolation\nIdle 4h timeout, max 3 respawns\nUnix socket: ~/.orca/users/<userId>/orca.sock")
    Container(admin_spa, "Admin SPA", "React / Vite",
      "Dashboard /admin: Users CRUD, Sessions, Audit Log\nrequireAdmin guard")
    Container(agent_ws, "AgentWebSocketServer", "ws / Node.js",
      "direct-websocket mode: agent connects as WS client\nValidate agentToken, wire Multiplexer")
    Container(fleet_monitor, "FleetHealthMonitor", "Node.js",
      "Poll dev servers every 60s\nMetrics: CPU%, RAM%, disk%, latency\nWebhook alerts")
    Container(web_db, "Multi-DB Layer", "IConnectionPool / MigrationRunner",
      "Dialects: SQLite, MySQL, PostgreSQL, TiDB\nMigrations 0001-0010\nDSN: ORCA_DB_URL env")

    Container(profile_svc, "Profile & Project Service", "TypeScript",
      "Company/Dept/User profile 3-layer inheritance\nProfileResolver + cache (TTL 60s)\nProject registry + Dev Server binding\nProjectAwareAgentSpawner")

    Container(ai_provider_svc, "AI Provider Service", "TypeScript",
      "Multi-provider account registry\nAnthropaic/OpenAI/Google/Azure/AWS/Ollama/vLLM\nCredential relay → Dev Server (AES-256-GCM)\nHealth check + quota tracking")

    Container(workflow_engine, "Workflow Orchestrator", "TypeScript",
      "Multi-server workflow execution\nDAG builder + topological wave dispatch\nTemplate registry: company/team/personal\nInheritance + sharing + library")

    Container(task_svc, "Task Graph Service", "TypeScript",
      "Task management as DAG (parent-child + depends-on)\nAI-assisted planning & decomposition\nGrant system: company/team/user × 5 levels\nTask-linked agent execution")
  }

  Container(mobile_app, "Orca Mobile", "React Native / TypeScript",
    "Companion app iOS/Android:\nPush notifications, agent status,\nremote dispatch via WebSocket")

  Container(orca_cli, "Orca CLI", "Node.js / TypeScript",
    "Command-line interface:\norca worktree, orca agent, orca serve\nCI/CD integration, headless mode")

  Container(relay, "Orca Relay", "Node.js / TypeScript (compiled binary)",
    "Deployed on remote SSH host:\nPTY bridging, file ops, git ops,\nport scanning, agent hooks")

  ContainerDb(sqlite, "SQLite Database", "better-sqlite3",
    "Persistence: worktrees, sessions,\nscrollback snapshots, settings,\nautomations, notifications history")

  ContainerDb(server_db, "Server Database", "SQLite/MySQL/PostgreSQL",
    "orca_users, orca_sessions, orca_audit_log\norca_access_policies, orca_company, orca_departments\norca_projects, orca_ai_provider_accounts\norca_workflow_templates, orca_workflow_executions\norca_tasks, orca_task_edges, orca_task_grants")

  System_Ext(ai_agents, "AI Agents", "Claude Code, Codex, OpenCode,\nGemini, Cursor, etc.")
  System_Ext(custom_agent, "Custom AI Agent", "Python/Go/TS agent via WebSocket")
  System_Ext(git, "Git", "Local git VCS\n(git worktree, git diff, git status)")
  System_Ext(remote_host, "Remote SSH Host", "Cloud server / VPS")
  System_Ext(github_linear, "GitHub / Linear / GitLab", "Source control + PM APIs")

  Rel(user, renderer, "Interacts with", "Mouse / Keyboard")
  Rel(user, web_http, "Web browser access", "HTTPS")
  Rel(user, mobile_app, "Uses on mobile", "Touch UI")
  Rel(user, orca_cli, "CLI commands", "Terminal")
  Rel(admin, admin_spa, "Manages users/sessions", "HTTPS /admin")
  Rel(agentdev, agent_ws, "Connects agent", "ws://orca:6768/agent")
  Rel(custom_agent, agent_ws, "WS connection", "Binary frames + JSON-RPC")

  Rel(renderer, preload, "Calls API via", "contextBridge")
  Rel(preload, main_process, "Forwards calls to", "Electron IPC (ipcMain/ipcRenderer)")
  Rel(main_process, daemon, "Connects to", "Unix Socket (NDJSON protocol)")
  Rel(main_process, sqlite, "Reads/Writes", "SQLite via better-sqlite3")
  Rel(main_process, git, "Executes", "Child process / git CLI")
  Rel(main_process, github_linear, "API calls", "HTTPS REST/GraphQL")
  Rel(main_process, ai_agents, "Spawns", "PTY (node-pty)")
  Rel(main_process, remote_host, "Connects via", "SSH (ssh2 library)")

  Rel(web_http, auth_layer, "Authenticates", "Cookie session")
  Rel(web_ws, auth_layer, "Validates session", "WS upgrade")
  Rel(web_ws, session_mgr, "Routes WS", "Unix socket per userId")
  Rel(web_ws, agent_ws, "Agent endpoint", "/agent path")
  Rel(web_http, fleet_monitor, "Fleet API", "REST")
  Rel(web_http, web_db, "Reads/Writes", "IConnectionPool")
  Rel(auth_layer, server_db, "Users/Sessions", "SQL")
  Rel(fleet_monitor, remote_host, "Health poll", "SSH")

  Rel(profile_svc, server_db, "Company/Dept/User profiles", "SQL")
  Rel(ai_provider_svc, server_db, "Provider account metadata", "SQL")
  Rel(ai_provider_svc, relay, "Relay credentials to Dev Server", "SSH")
  Rel(workflow_engine, ai_provider_svc, "Resolve AI provider per step", "")
  Rel(workflow_engine, relay, "Dispatch steps to dev servers", "relay RPC")
  Rel(workflow_engine, server_db, "Persist execution state", "SQL")
  Rel(task_svc, server_db, "Tasks + grants + comments", "SQL")
  Rel(task_svc, ai_provider_svc, "Resolve provider for task agent", "")
  Rel(task_svc, relay, "Spawn agent on project dev server", "relay RPC")
  Rel(task_svc, profile_svc, "Resolve user profile for agent env", "")

  Rel(daemon, sqlite, "Session state", "SQLite")
  Rel(daemon, ai_agents, "Spawns locally", "PTY (node-pty)")

  Rel(mobile_app, main_process, "WebSocket (E2E encrypted)", "TweetNaCl box cipher")

  Rel(orca_cli, daemon, "Commands", "Unix Socket / HTTP API")

  Rel(remote_host, relay, "Contains (deployed)", "SFTP upload")
  Rel(main_process, relay, "Connects to", "WebSocket (relay protocol)")
  Rel(relay, ai_agents, "Spawns remotely", "PTY (node-pty on remote)")
  Rel(relay, git, "Executes on remote", "Child process")
```

---

## Mô tả từng Container

### 1. Renderer Process
**Runtime:** Chromium (Electron renderer)  
**Tech:** React 19, TypeScript, Vite, xterm.js, Monaco Editor  
**Chức năng:**
- Render toàn bộ UI (sidebar, terminal panels, diff viewer, file explorer)
- Nhận events từ Main Process qua IPC
- Không có quyền truy cập Node.js trực tiếp (sandboxed)
- Communicate với Main Process qua `window.orcaAPI` (contextBridge)

**Source:** `src/renderer/src/`  
**Entry:** `src/renderer/src/main.tsx`, `src/renderer/src/App.tsx`

---

### 2. Main Process
**Runtime:** Node.js 22+  
**Tech:** TypeScript, Electron Main API, better-sqlite3, ssh2, node-pty  
**Chức năng:**
- **Worktree Management:** tạo, xóa, list git worktrees
- **Agent Orchestration:** khởi động, dừng, monitor agent processes
- **SSH Management:** kết nối, relay deploy, port forwarding
- **Git Operations:** diff, status, commit, push
- **GitHub/Linear Integration:** REST/GraphQL API calls
- **IPC Handlers:** nhận request từ renderer, trả về kết quả
- **Persistence:** read/write SQLite

**Source:** `src/main/`  
**Entry:** `src/main/index.ts` (~100K lines — monolithic main)  
**Key modules:**
- `ipc/` — IPC handler registration
- `ssh/` — SSH connection, relay, port forward
- `git/` — Git operations
- `github/`, `gitlab/`, `linear/` — Platform integrations
- `automations/` — Automation engine
- `persistence.ts` — SQLite schema & queries

---

### 3. Preload Script
**Runtime:** Electron Preload (Node.js + limited browser)  
**Tech:** TypeScript, Electron contextBridge  
**Chức năng:**
- Expose secure API bridge từ main process tới renderer
- Không expose raw `require()` hoặc Node.js APIs
- Tất cả API phải được whitelist trong contextBridge

**Source:** `src/preload/`  
**Entry:** `src/preload/index.ts`

---

### 4. Orca Daemon
**Runtime:** Node.js (background process)  
**Tech:** TypeScript, node-pty, Unix socket, NDJSON  
**Chức năng:**
- **PTY Server:** quản lý tất cả PTY sessions (local và remote)
- **Session Persistence:** lưu scrollback buffer, khôi phục sau reconnect
- **Multi-client:** serve cả GUI (main process) và CLI đồng thời
- **Headless mode:** chạy không có GUI cho CI/CD

**Source:** `src/main/daemon/`  
**Protocol:** NDJSON over Unix socket  
**Key files:**
- `daemon-init.ts` — initialization, handler registration
- `daemon-server.ts` — Unix socket server
- `daemon-pty-adapter.ts` — PTY lifecycle management
- `daemon-pty-router.ts` — Route PTY I/O between clients

---

### 5. Orca Mobile (React Native)
**Runtime:** iOS / Android native  
**Tech:** React Native, TypeScript, TweetNaCl  
**Chức năng:**
- Nhận push notifications từ Desktop
- Hiển thị agent status real-time
- Gửi remote dispatch (prompt từ mobile về agent)
- QR code pairing với Desktop

**Source:** Separate repo (mobile companion)  
**Communication:** WebSocket với E2E encryption (TweetNaCl box)

---

### 6. Orca CLI
**Runtime:** Node.js  
**Tech:** TypeScript  
**Chức năng:**
- `orca worktree create/list/remove`
- `orca agent status/wait/send`
- `orca serve` — start headless daemon
- `orca snapshot` — capture terminal output

**Source:** `src/cli/`  
**Communication:** Unix socket tới Orca Daemon

---

### 7. Orca Relay
**Runtime:** Node.js (compiled single binary, chạy trên remote host)  
**Tech:** TypeScript, node-pty, WebSocket  
**Chức năng:**
- Bridge PTY sessions từ remote host về Desktop
- File system operations (read, write, list, watch)
- Git operations trên remote
- Port scanning và forwarding
- Agent hook interception

**Source:** `src/relay/`  
**Key files:**
- `relay.ts` — Main relay loop
- `pty-handler.ts` — PTY I/O bridging
- `fs-handler.ts` — File system operations
- `git-handler.ts` — Git operations
- `port-scan-handler.ts` — Port detection
- `agent-hook-server.ts` — Agent hook interception

**Deploy:** Upload via SFTP, execute on remote host

---

### 8. SQLite Database
**Location:** `~/.config/orca/` (macOS: `~/Library/Application Support/orca/`)  
**Library:** better-sqlite3 (synchronous, in-process)  

**Key tables (Desktop SQLite):**
| Table | Nội dung |
|-------|---------|
| `projects` | Repositories đã mở |
| `worktrees` | Worktree records |
| `sessions` | Agent session history |
| `terminal_scrollback_snapshots` | Terminal state snapshots |
| `automations` | Automation definitions |
| `automation_runs` | Automation execution history |
| `notifications` | Notification history |
| `settings` | User preferences |
| `rate_limits` | Agent rate limit tracking |

**Key tables (Server Database — migration 0005):**
| Table | Nội dung |
|-------|---------|
| `orca_users` | Users (id, email, name, role, is_active) |
| `orca_sessions` | Auth sessions (token, userId, expires_at, last_seen_at) |
| `orca_audit_log` | Audit entries (actor, action, target, timestamp, ip) |
| `orca_access_policies` | RBAC policies (role, resource, action) |

---

## Communication Protocols

| From | To | Protocol | Format |
|------|----|---------|--------|
| Renderer | Preload | contextBridge API | TypeScript function calls |
| Preload | Main | Electron IPC | JSON (ipcRenderer.invoke) |
| Main | Daemon | Unix Socket | NDJSON |
| CLI | Daemon | Unix Socket / HTTP | NDJSON / JSON |
| Desktop | Remote Host | SSH (ssh2) | SSH protocol |
| Desktop | Relay | WebSocket | Binary frames (relay protocol) |
| Mobile | Desktop | WebSocket | TweetNaCl encrypted JSON |
| Main | AI Agents | PTY (stdin/stdout) | Text + OSC escape sequences |
| Main | GitHub | HTTPS | JSON (REST/GraphQL) |
| **Browser** | **Orca Web** | **HTTP/WebSocket** | **JSON (auth cookie)** |
| **Agent** | **AgentWS** | **WebSocket** | **Binary frames + JSON-RPC** |
| **Orca** | **Agent WS Server** | **WebSocket** | **Bearer token + Binary frames** |
| **Auth Layer** | **Server DB** | **SQL** | **IConnectionPool queries** |

---

## Container Boundaries và Isolation

```
┌─── Electron App Process ─────────────────────────────────────┐
│                                                               │
│  ┌─── Renderer (Chromium) ────────┐   ┌─── Main (Node.js) ──┤
│  │  React UI + xterm.js           │   │  Business Logic      │
│  │  SANDBOXED                     │   │  SSH, Git, APIs      │
│  │  No direct Node.js access      │   │  SQLite persistence  │
│  └──────────────┬─────────────────┘   └────────┬────────────┤
│                 │  contextBridge (whitelist)     │            │
│                 └───────────────────────────────┘            │
└───────────────────────────────────────────────────────────────┘

┌─── Daemon Process (separate) ────────────────────────────────┐
│  PTY Server + Session Manager                                 │
│  Serves: Main Process + CLI                                   │
└───────────────────────────────────────────────────────────────┘

┌─── Remote Host ──────────────────────────────────────────────┐
│  Relay Binary (Node.js)                                       │
│  PTY + FS + Git + Port Scanner                               │
└───────────────────────────────────────────────────────────────┘
```
---

## Containers mới (v5.0)

### 13. Profile & Project Service
**Runtime:** Node.js / TypeScript (trong Orca Web Server process)  
**Chức năng:**
- `CompanyService` / `DepartmentService`: CRUD profile 3 tầng
- `ProfileResolver`: deep-merge Company ← Dept ← User, cache TTL 60s
- `ProjectService`: CRUD project + dev server binding (`project.devServerId`)
- `ProjectServerRouter`: auto-route worktree/agent/terminal đến đúng dev server
- `ProfileAwareAgentSpawner`: inject resolved profile vào agent env (PATH, envVars, model)

**Source:** `src/main/profile/`, `src/main/project/`  
**DB tables:** `orca_company`, `orca_departments`, `orca_users.profile_json`, `orca_projects`, `orca_project_members`

---

### 14. AI Provider Service
**Runtime:** Node.js / TypeScript (trong Orca Web Server process)  
**Chức năng:**
- `AIProviderService`: CRUD `orca_ai_provider_accounts` (metadata only — no credentials)
- `ProviderCredentialRelay`: encrypt credentials in browser → relay SSH → write `~/.orca/ai-providers/<id>.enc` trên Dev Server
- `AIProviderResolver`: priority cascade (user > project > server-default) + model detection
- `ProviderHealthChecker`: background cron mỗi 15 phút, quota tracking

**Source:** `src/main/ai-providers/`  
**DB tables:** `orca_ai_provider_accounts`, `orca_provider_usage`  
**Providers:** Anthropic, OpenAI, Google Gemini, Azure OpenAI, AWS Bedrock, Ollama, vLLM  
**Credential security:** AES-256-GCM, key = scrypt(ORCA_AI_CREDENTIAL_KEY + accountId)

---

### 15. Workflow Orchestrator
**Runtime:** Node.js / TypeScript (trong Orca Web Server process)  
**Chức năng:**
- `TemplateRegistry`: CRUD workflow templates (company/team/personal scopes)
- `TemplateResolver`: inheritance chain merge + overrides + inject/remove steps
- `WorkflowOrchestrator`: build DAG → topological sort → wave-based parallel execution
- `StepExecutors`: agent, shell, action, webhook, parallel, condition
- `WorkflowServerResolver`: `project:<id>` / `server:<id>` / `fleet:tag:<tag>` → devServerId
- State persistence: resumable sau Orca restart

**Source:** `src/main/workflow/`  
**DB tables:** `orca_workflow_templates`, `orca_workflow_executions`, `orca_step_executions`

---

### 16. Task Graph Service
**Runtime:** Node.js / TypeScript (trong Orca Web Server process)  
**Chức năng:**
- `TaskService`: CRUD + progress calculation (recursive subtask aggregation)
- `TaskDAGValidator`: cycle detection, auto-block trên unresolved deps
- `TaskGraphBuilder`: BFS subtree traversal với access filter
- `TaskAIPlanner`: AI decomposition (subtask suggestions + dependency graph + estimates)
- `TaskGrantService`: grant resolution (owner > admin > user > team > company), apply_tree inheritance
- `TaskAgentExecutor`: build task preamble → inject → spawn agent → stream to activity feed

**Source:** `src/main/task/`  
**DB tables:** `orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments`

---

## Communication Matrix (cập nhật v5.0)

| From | To | Protocol | Format |
|------|----|----------|--------|
| Browser | Profile Service | WebSocket RPC | JSON |
| Browser | AI Provider Service | WebSocket RPC | JSON (credential: SubtleCrypto encrypted) |
| Browser | Workflow Orchestrator | WebSocket RPC + Stream | JSON events |
| Browser | Task Graph Service | WebSocket RPC | JSON |
| AI Provider Svc | Dev Server (relay) | SSH relay | Encrypted blob |
| Workflow Engine | Dev Server (relay) | relay RPC | JSON-RPC |
| Task Service | Dev Server (relay) | relay RPC | JSON-RPC |
| All v5 Services | Server DB | SQL | IConnectionPool |
