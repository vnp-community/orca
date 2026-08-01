# Product Requirements Document (PRD)

**Sản phẩm:** Orca — AI Orchestrator IDE  
**Phiên bản tài liệu:** 3.0  
**Ngày:** 2026-07-21 | **Cập nhật:** 2026-08-01  
**Tác giả:** Nhóm StablyAI  
**Phiên bản sản phẩm:** 5.0 (Web Server + Profile Hierarchy + AI Provider Management + Multi-Server Workflows + Task Graph + Project Workspace)  

---

## 1. Tổng quan sản phẩm

### 1.1 Tầm nhìn

Orca là một **nền tảng phát triển phần mềm AI-native thế hệ tiếp theo** dành cho cá nhân, team và doanh nghiệp. Orca cho phép developers, teams và tổ chức điều phối nhiều AI agent, tự động hóa workflow phức tạp trên nhiều dev servers, quản lý tác vụ theo đồ thị, và làm việc trực tiếp trên code thông qua Project Workspace tích hợp — tất cả trong một nền tảng hợp nhất.

> **Slogan:** *The AI-Native Platform for Modern Engineering Teams.*

### 1.2 Mục tiêu sản phẩm

| Mục tiêu | Mô tả |
|----------|-------|
| **Tăng tốc độ phát triển** | Chạy nhiều AI agent song song, workflows tự động trên nhiều dev servers |
| **Project-Centric Workspace** | File Explorer + Git UI + Agent + Tasks trong một giao diện thống nhất per project |
| **Hỗ trợ làm việc từ xa** | Kết nối SSH/relay, file browse, git operations trực tiếp trên dev server từ browser |
| **Team & Enterprise Ready** | Profile hierarchy (Company→Dept→User), RBAC, task sharing, workflow templates |
| **AI Provider Flexibility** | Hỗ trợ 7+ AI providers per dev server, credentials bảo mật trực tiếp trên server |
| **Task & Workflow Governance** | Task graph với AI planning, workflow template library kế thừa company/team/personal |

### 1.3 Phạm vi sản phẩm

Orca là một **nền tảng phát triển AI-native** gồm: ứng dụng desktop Electron đa nền tảng, **Orca Web Server** (Node.js, multi-user, deploy container), companion mobile app (iOS/Android), và CLI. Phiên bản v5.0 bổ sung Profile Hierarchy, AI Provider Management, Multi-Server Workflow Orchestration, Task Graph, và Project Workspace (IDE-in-browser với File Explorer + Remote Git UI). Sản phẩm là open-source theo giấy phép MIT.

---

## 2. Đối tượng người dùng

### 2.1 Người dùng chính

| Nhóm | Mô tả | Nhu cầu chính |
|------|-------|---------------|
| **Senior Developer** | Lập trình viên giàu kinh nghiệm, muốn tăng năng suất với AI | Project Workspace, agent control, task graph, remote git |
| **AI-Native Developer** | Developer quen làm việc với Claude Code, Codex, Gemini | Multi-provider accounts, workflow orchestration, task AI planning |
| **Tech Lead / Architect** | Người dẫn dắt team, review code và phê duyệt PR | Task tree sharing, team workflow templates, profile management |
| **Remote Developer** | Developer làm việc trên máy chủ từ xa | File explorer, git UI, agent — tất cả qua browser không cần SSH |
| **Company Admin** | Quản trị nền tảng Orca cho toàn tổ chức | Company profile, AI provider setup, fleet management, audit log |
| **Team Lead** | Quản lý team và chuẩn hóa quy trình | Team profile, workflow templates, task assignment, grant management |

### 2.2 Người dùng phụ

- **QA Engineer**: Sử dụng Design Mode và Computer Use để kiểm thử UI
- **DevOps Engineer**: Sử dụng headless mode và CLI để tích hợp vào CI/CD pipeline

---

## 3. Tính năng cốt lõi

### 3.1 Parallel Worktrees (Ưu tiên: P0)

**Mô tả:** Tạo nhiều git worktree độc lập, mỗi worktree chạy một AI agent riêng với cùng prompt ban đầu.

**Tính năng chi tiết:**
- Fan-out một prompt tới N agent, mỗi agent làm việc trong worktree riêng
- Xem và so sánh kết quả giữa các worktree
- Merge worktree thắng vào nhánh chính
- Worktree isolation đảm bảo không có conflict giữa các agent
- Hỗ trợ worktree trên SSH host (remote worktrees)

**Tiêu chí thành công:**
- Người dùng có thể tạo 5+ worktree cùng lúc mà không giảm hiệu năng đáng kể
- Không có data corruption giữa các worktree độc lập

---

### 3.2 Terminal Splits (Ưu tiên: P0)

**Mô tả:** Terminal tích hợp với hiệu năng cao tương đương Ghostty, hỗ trợ chia màn hình vô hạn.

**Tính năng chi tiết:**
- WebGL rendering cho hiệu năng cao
- Infinite splits (chia terminal theo chiều ngang/dọc)
- Scrollback buffer tồn tại qua khởi động lại (terminal session persistence)
- Hỗ trợ đầy đủ màu sắc, ligatures, unicode
- OSC 133 command tracking (shell integration)
- Kitty keyboard protocol

**Tiêu chí thành công:**
- Độ trễ nhập liệu (typing latency) < 16ms
- Terminal rendering không bị freeze khi output lớn
- Scrollback restore chính xác sau khi restart

---

### 3.3 Mobile Companion App (Ưu tiên: P0)

**Mô tả:** Ứng dụng di động (iOS/Android) để giám sát và điều khiển agent từ xa.

**Tính năng chi tiết:**
- Nhận thông báo push khi agent hoàn thành task
- Gửi follow-up instructions từ điện thoại
- Xem trạng thái tất cả agent đang chạy
- Mã QR để pairing với desktop app
- E2E encryption cho kênh truyền thông tin

**Tiêu chí thành công:**
- Pairing hoàn thành trong < 30 giây
- Notification delivery trong < 5 giây khi agent kết thúc
- Hỗ trợ iOS 15+ và Android 8+

---

### 3.4 Design Mode (Ưu tiên: P1)

**Mô tả:** Tích hợp browser Chromium thực để click vào UI element và gửi context vào agent prompt.

**Tính năng chi tiết:**
- Embedded Chromium browser window trong IDE
- Click vào bất kỳ element nào để trích xuất HTML, CSS, screenshot
- Inject context trực tiếp vào agent prompt
- Hỗ trợ browser viewport presets
- Cookie import từ browser ngoài

**Tiêu chí thành công:**
- Người dùng có thể chụp và inject UI element context trong < 3 clicks

---

### 3.5 GitHub & Linear Integration (Ưu tiên: P1)

**Mô tả:** Duyệt PR, issues, và project board ngay trong ứng dụng.

**Tính năng chi tiết:**
- Xem danh sách PR, diff, comments
- Tạo worktree từ GitHub issue hoặc Linear task
- Annotate diff: thêm comment vào từng dòng và gửi về agent
- Auto-generate PR/commit message bằng AI
- Tích hợp GitLab, Gitea, Bitbucket, Azure DevOps
- Linear SDK: quản lý issues, projects, cycles

---

### 3.6 SSH Worktrees (Ưu tiên: P1)

**Mô tả:** Chạy agent trên máy chủ remote qua SSH với đầy đủ file editing, git, và terminal.

**Tính năng chi tiết:**
- Kết nối SSH với xác thực key/password/agent
- Auto-reconnect khi mất kết nối
- Port forwarding tự động (local port → remote service)
- Deploy Orca relay binary tự động lên remote
- Hỗ trợ SSH config file (includes, host patterns)
- Channel multiplexing để tối ưu hiệu năng

---

### 3.7 Annotate AI Diffs (Ưu tiên: P1)

**Mô tả:** Drop comments trên bất kỳ dòng diff nào và gửi về agent để sửa.

---

### 3.8 Orca CLI (Ưu tiên: P1)

**Mô tả:** CLI tool để agent và script tự động hóa mọi workflow của Orca.

**Tính năng chi tiết:**
- `orca worktree create` — tạo worktree mới
- `orca snapshot` — chụp trạng thái terminal
- `orca click` / `orca fill` — tương tác với UI
- `orca serve` — chạy headless server mode

---

### 3.9 AI Agent Support (Ưu tiên: P0)

**Agents được tích hợp sâu (first-class):**
Claude Code, Codex, GitHub Copilot, Grok, Cursor, OpenCode, Gemini/Antigravity, Pi, Amp, Devin, Goose, Cline, Continue, Qwen Code, và 20+ agent khác.

**Tính năng per-agent:**
- Session resume (tiếp tục session đã có)
- Usage tracking và rate limit monitoring
- Account switcher (hot-swap accounts)
- Trust presets (permissions management)

---

### 3.10 Tính năng bổ sung

| Tính năng | Mô tả | Ưu tiên |
|-----------|-------|---------|
| **Quick Open** | Tìm kiếm worktree, file, agent, command | P1 |
| **Automations** | Schedule automation theo cron hoặc trigger | P2 |
| **Computer Use** | Agent điều khiển desktop UI thực tế | P2 |
| **Notifications** | Tracking trạng thái agent, mark as unread | P1 |
| **File Explorer + Editor** | VSCode-style editor với autosave | P1 |
| **Rich Repo Previews** | Preview Markdown, PDF, images | P2 |
| **Memory / AI Vault** | Lưu trữ AI session context | P2 |
| **Ephemeral VM** | Chạy agent trong VM tạm thời | P2 |
| **Text Search** | Tìm kiếm toàn bộ workspace | P1 |
| **Localization** | Đa ngôn ngữ (EN, ZH, JA, KO, ES, PT) | P2 |
| **Speech Input** | Nhập liệu bằng giọng nói (Sherpa-ONNX) | P3 |
| **Full-Flow Tracing** | Structured observability xuyên suốt Browser → Main → Relay → Agent | P1 |

---


---

### 3.11 Web Server Mode (F22) — Ưu tiên: P0 ✅ Implemented

**Mô tả:** Orca chạy như một Node.js web server (không cần Electron) — phục vụ SPA qua HTTP và WebSocket. Nền tảng cho Orca Cloud.

**Tính năng chi tiết:**
- Platform Abstraction Layer (`IPlatformServices`) — tách Electron khỏi business logic
- `NodeAdapter` — implement đầy đủ cho server mode; `ElectronAdapter` giữ desktop mode
- HTTP `:6769` (Express: SPA + health + auth + admin) + WebSocket `:6768`
- `bootstrapWebApp()` + `ConnectionStatusProvider` + `ConnectionStatusBanner`
- `IRpcClient` abstraction — `WebSocketRpcClient` (web) vs `ElectronRpcClient` (desktop)
- Docker deployment: `deploy/prod/Dockerfile`, healthcheck `/health/ready`

---

### 3.12 Multi-User Login & Auth (F23) — Ưu tiên: P0 ✅ Implemented

**Mô tả:** Hệ thống xác thực multi-user khi chạy server mode. PairCode vẫn hoạt động song song.

**Tính năng chi tiết:**
- `POST /auth/local` — email + bcrypt password, trả `Set-Cookie: orca_session` (8h TTL)
- Session store SQLite: `orca_users`, `orca_sessions` với `last_seen_at` per-request
- `GET /auth/me`, `POST /auth/logout`, SSO stub (`GET /auth/sso/:provider`)
- `AuthMiddleware` — `requireAuth()` guard cho `/admin/api/*` và WebSocket
- Migration 0005: `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies`
- Login SPA: `LoginPage`, `LoginForm`, `SsoButton`, `PairCodeFallback`
- SSO Phase 2 (GitHub, Google, Keycloak): **DEFERRED**
- Kích hoạt bởi `ORCA_MULTI_USER=1`

---

### 3.13 Per-User Process Sandbox (F24) — Ưu tiên: P0 ✅ Implemented

**Mô tả:** Mỗi user cô lập trong Node.js process riêng (`fork()`). Data isolation hoàn toàn.

**Tính năng chi tiết:**
- `SessionManager` — `fork()` per userId, Unix socket isolation
- Idle timeout: 4h | Spawn timeout: 30s | Max respawn: 3
- `WsSessionRouter` — proxy WS ↔ Unix socket by userId
- Per-user `userDataPath`: `~/.orca/users/<userId>/orca.sock`, `orca.db`, `worktrees/`
- SSH connection store isolated per user

---

### 3.14 Admin Panel (F25) — Ưu tiên: P1 ✅ Implemented

**Mô tả:** SPA riêng biệt (`/admin`) để quản trị viên quản lý users, sessions, audit log. Chỉ accessible bởi role `admin`.

**Tính năng chi tiết:**
- REST API `/admin/api/*` — users CRUD, session management, stats, audit
- Audit log sync: `login.success/fail`, `user.create/deactivate`, `session.kill`, `ssh.connect`
- First-run: auto-create admin user, in credentials ra stdout
- React SPA: `AdminDashboard`, `UsersPage`, `SessionsPage`, `AuditPage`, `PoliciesPage`
- Search/filter users theo role + status
- `requireAdmin` guard (403 nếu không phải admin)

---

### 3.15 Multi-Database Support (F26) — Ưu tiên: P1 ✅ Implemented

**Mô tả:** Hỗ trợ SQLite, MySQL, PostgreSQL, TiDB qua lớp abstraction thống nhất.

**Tính năng chi tiết:**
- `IConnectionPool` — `SqliteSingleConnectionPool` + `GenericConnectionPool` (async)
- DSN config: `ORCA_DB_URL=postgresql://user:pass@host/db` (env hoặc YAML)
- `MigrationRunner` — apply/rollback/status, idempotent, cross-dialect
- `IStateRepository` — `JsonFileStateRepository` (desktop) + `SqlStateRepository` (server)
- Health endpoints: `GET /health`, `/health/ready`, `/health/metrics` (Prometheus)
- 5 migrations: initial → automations → sessions → app tables → auth schema

---

### 3.16 Fleet Health Monitoring (F27) — Ưu tiên: P1 ✅ Implemented

**Mô tả:** Theo dõi sức khỏe fleet dev servers theo thời gian thực, Prometheus metrics, webhook alert.

**Tính năng chi tiết:**
- `FleetHealthMonitor` — poll mỗi server theo interval (mặc định 60s)
- Status: `healthy` | `degraded` | `unhealthy` | `unreachable`
- Prometheus metrics tại `/metrics` (CPU%, RAM%, disk%, latency)
- Webhook alert khi status thay đổi (`fleetAlertWebhookUrl` trong GlobalSettings)
- Fleet Dashboard UI — realtime status cards

---

### 3.17 Dev Server Onboarding (F28) — Ưu tiên: P1 ✅ Implemented

**Mô tả:** Guided wizard để đăng ký và cấu hình dev server từ xa, tự động deploy relay.

**Tính năng chi tiết:**
- Platform-aware wizard (Linux/Mac/Windows branching)
- Remote agent detection qua SSH
- Preflight check: OS, ports, disk space
- Auto relay binary deployment + SSH key authorization
- `SshProvisioningProgress` progress bar UI
- Web push notification khi onboarding complete
- Multi-dev-server checklist UI

---

### 3.18 Agent WebSocket Protocol (F29) — Ưu tiên: P1 ✅ Implemented

**Mô tả:** Cho phép AI agents (TypeScript, Python, Go, Java...) kết nối với Orca qua WebSocket. Hai chế độ kết nối:

**Tính năng chi tiết:**
- **Mode 1 — relay-websocket**: Orca kết nối tới Agent WS Server (`ws://agent:6799/orca-relay`) — Bearer token auth
- **Mode 2 — direct-websocket**: Agent kết nối vào Orca WS Server (`ws://orca:6768/agent`) — `agentToken` handshake
- Wire Protocol: 13-byte binary header `[TYPE(1)][SEQ u32 BE][ACK u32 BE][LEN u32 BE]` + JSON-RPC payload
- `AgentWebSocketServer` — WS server nhận incoming agent connections
- `WsTransport` adapter — wrap WebSocket vào `MultiplexerTransport` interface
- Language-agnostic: SDK guide cho TypeScript, Python, Go, Java
- Agent token management UI trong `DevServerPane`

---

### 3.19 Remote Source Control Integrations (F30) — Ưu tiên: P1 ✅ Implemented

**Mô tả:** GitHub, GitLab, Bitbucket, Azure DevOps, Gitea, Linear, Jira hoạt động đúng trong server mode với per-user credential isolation.

**Tính năng chi tiết:**
- **Category A — CLI-based** (GitHub `gh`, GitLab `glab`): proxy `preflight.check` qua SSH relay tới Dev Server; `devServerId` context routing
- **Category B — HTTP API Token** (Bitbucket, Azure DevOps, Gitea): `WebCredentialStore` (AES-256-GCM) per-user
- **Category C — File Token** (Linear, Jira): `WebCredentialStore` thay thế global env vars
- `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` session isolation qua env injection
- `mergePreflightStatuses()` — ưu tiên relay CLI checks; fallback local API checks
- Frontend: `WebModeCliAuthSection` (PTY auth login), `CredentialInputForm` (token input)
- `RpcMethodContext` injection: `devServerManager` + `userId` available trong mọi RPC method

---

### 3.20 User Profile Hierarchy (F33) — Ưu tiên: P0 🚧 Phát triển

**Mô tả:** Hệ thống profile 3 tầng (Company → Department → User) cho phép cấu hình thừa kế và override AI agent, editor, shell settings.

**Tính năng chi tiết:**
- `OrcaProfile` schema: 6 sections (agent, editor, shell, mcp, security, envVars)
- Deep-merge algorithm: Company ← Dept ← User (user có highest priority)
- `security` và `approvedModels` bị lock ở Company level
- `envVars`: override merge (User > Dept > Company)
- `pathAdditions`: concatenation (tất cả tầng đều append)
- Profile editor UI 3-tầng, effective profile panel với source attribution
- Cache profile resolve TTL 60s — auto-invalidate khi parent thay đổi

---

### 3.21 Project-Dev Server Binding (F34) — Ưu tiên: P0 🚧 Phát triển

**Mô tả:** Mỗi project gắn với một dev server cụ thể. Mọi hoạt động (agent, worktree, terminal, git) đều chạy trên đúng server đó.

**Tính năng chi tiết:**
- `ProjectService`: CRUD projects với `devServerId` binding bắt buộc
- `ProjectServerRouter`: auto-route agent/worktree/terminal đến project server
- `ProfileAwareAgentSpawner`: inject resolved profile + ORCA_PROJECT_ID vào agent env
- Project membership: user phải là member để access project
- Project-scope AI provider selection

---

### 3.22 AI Provider Account Management (F35) — Ưu tiên: P0 🚧 Phát triển

**Mô tả:** Admin setup nhiều AI Provider Accounts (Anthropic, OpenAI, Google, Azure, AWS, Ollama, vLLM) trên từng dev server. Credentials được mã hóa AES-256-GCM và lưu **trực tiếp trên Dev Server**.

**Tính năng chi tiết:**
- 7+ providers: Anthropic, OpenAI, Google Gemini, Azure OpenAI, AWS Bedrock, Ollama, vLLM
- 3 scopes: server / project / user
- Credential storage trên Dev Server (không qua Orca Server): `~/.orca/ai-providers/<id>.enc`
- Priority resolution: user > project > server-default
- Background health check mỗi 15 phút, quota tracking, 80% alert
- Key rotation với 30s grace period
- Admin Panel UI: provider list + status badges + test connection

---

### 3.23 Multi-Server Workflow Orchestration (F36) — Ưu tiên: P1 🚧 Phát triển

**Mô tả:** Workflows YAML-based span nhiều dev servers, dùng nhiều AI providers, kế thừa template 3 tầng (company/team/personal), và chia sẻ qua library.

**Tính năng chi tiết:**
- Step types: agent, shell, action, webhook, parallel, condition
- Server resolution per step: `project:<id>` / `server:<id>` / `fleet:tag:<tag>`
- DAG builder + topological sort → wave-based parallel execution
- Template inheritance: overrides + inject_steps + remove_steps
- Template library: company/team/personal scopes, visibility: private/team/company/public
- Sharing: admin approval cho company scope, share link cho public
- State persistence + resumability sau Orca restart
- Real-time step status stream → UI

---

### 3.24 Task Graph Management System (F37) — Ưu tiên: P0 🚧 Phát triển

**Mô tả:** Quản lý tác vụ theo mô hình đồ thị (DAG): parent-child (phân rã) + depends-on (dependency). AI hỗ trợ planning. Mỗi task có prompt field cho agent. Sharing và grant theo company/team/user.

**Tính năng chi tiết:**
- Task types: epic/story/task/subtask/bug/spike
- Graph model: parent-child (decomposition) + dependency edges (depends-on)
- DAG cycle detection trước khi add dependency
- AI decompose: gửi task + project context → subtask suggestions + estimates
- AI prompt generation từ task metadata
- Grant system: 5 levels (view/comment/edit/execute/manage) × 3 scopes (company/team/user)
- apply_tree: grant propagates xuống toàn bộ subtask tree
- Run Agent từ task: inject task preamble vào agent env + stream output vào task activity feed
- Progress tracking: done_subtasks / total_subtasks (recursive)
- Critical path calculation từ estimates

---

### 3.25 Project Workspace — Unified Project IDE (F38) — Ưu tiên: P0 🚧 Phát triển

**Mô tả:** Khi user chọn project, toàn bộ giao diện chuyển sang Project Workspace — một IDE-like environment gồm Explorer, Git, Agent, Workflows, Tasks, và Terminal — tất cả hoạt động trên dev server của project.

**Tính năng chi tiết:**
- WorkspaceContext: central state (project, relay, profile, gitStatus, currentWorktree)
- RelayConnectionPool: reuse SSH connections, cleanup idle > 5min
- Offline mode: banner + cached file tree + disable write ops
- Git status poll 5s khi tab active
- Cross-panel event bus: agent.complete → refresh Git + Explorer + Tasks

---

### 3.26 Remote File Explorer (F38 sub-feature) — Ưu tiên: P0 🚧 Phát triển

**Mô tả:** Duyệt cây thư mục và files trên dev server qua relay. Lazy-load, git decorations inline, file viewer, file search.

**Tính năng chi tiết:**
- Lazy-load directories khi expand (depth=1 per relay call)
- Git status decorations inline (M/A/D/? color badges)
- File viewer: read-only, syntax highlight, max 5MB
- File search: glob (by name) + grep (by content) qua relay
- Context menu: copy path, open in terminal, git actions
- Toggle hidden files (.gitignore, .env...)

---

### 3.27 Remote Git UI (F39) — Ưu tiên: P0 🚧 Phát triển

**Mô tả:** Toàn bộ git workflow thực hiện trực tiếp trên dev server từ browser UI, không cần SSH terminal thủ công.

**Tính năng chi tiết:**
- Git status: modified/staged/untracked list, real-time poll
- Visual diff viewer (unified, syntax highlighted)
- Stage/Unstage individual files hoặc tất cả
- Commit với manual message + AI commit message generation từ staged diff
- Push/Pull với progress stream (line-by-line output)
- Conflict detection sau pull → list conflict files → AI conflict resolution
- Branch management: list, create, checkout, delete, merge
- Stash push/pop
- Git log (50 commits, branch graph)
- Create Pull Request (GitHub CLI hoặc API token) + AI PR description
- Worktree switcher: list + create + switch

---

## 4. Kiến trúc hệ thống (Tóm tắt)

### 4.1 Platform

| Thành phần | Công nghệ |
|-----------|-----------|
| **Desktop App** | Electron v43 (multi-process) |
| **Web Server** | Node.js 22+ Express HTTP :6769 + WebSocket :6768 |
| **Frontend** | React 19, TypeScript 7, Tailwind CSS v4 |
| **Admin SPA** | React SPA riêng tại `/admin` |
| **Terminal** | xterm.js, WebGL addon, node-pty |
| **State Management** | Zustand + React Context (WorkspaceContext) |
| **IPC** | Electron IPC (desktop) + WebSocket RPC (server mode) |
| **Persistence** | SQLite (default) + MySQL/PostgreSQL/TiDB (server), migrations 0001–0010 |
| **Auth** | bcrypt 12 rounds + HTTP-only cookie session (8h TTL) |
| **Credential Store** | AES-256-GCM per-user (`WebCredentialStore` + Dev Server credential files) |
| **Agent Protocol** | Binary WebSocket frames (13-byte header + JSON-RPC) |
| **CLI** | TypeScript compiled binary |
| **Relay** | Binary relay cho SSH remote execution (fs, git, pty, ai) |
| **Mobile** | React Native (iOS/Android) |
| **Build** | electron-vite, Vite, PNPM, Docker |
| **Platform Abstraction** | `IPlatformServices` — NodeAdapter + ElectronAdapter |
| **Profile System** | 3-layer deep-merge (Company←Dept←User), cache TTL 60s |
| **AI Provider** | 7 providers, AES-256-GCM on Dev Server, health check cron |
| **Workflow Engine** | DAG orchestrator, template inheritance 3-tầng, multi-server dispatch |
| **Task Graph** | DAG + BFS traversal, AI decompose (LLM), 5-level grants |
| **Project Workspace** | RelayConnectionPool, WorkspaceContext, Remote Explorer + Git UI |
| **Trace System** | Isomorphic span tracing (Node.js + browser), sink registry, SSE push stream |

### 4.2 Observability

**Mô tả:** Orca tích hợp **Full-Flow Tracing** (F40) — hệ thống observability isomorphic cho phép trace mọi thao tác từ Browser → RPC → IPC → Relay → Agent trong cùng một span với ID duy nhất.

**Tính năng chi tiết:**
- `createTracer(flow)` — tạo named tracer, isomorphic (Node.js + browser)
- Span lifecycle: `start → step* → ok | fail` với `elapsedMs` đo chính xác
- `fail` events **luôn log** bất kể flag `ORCA_TRACE` (diagnostic always-on)
- Sink registry: pluggable, hỗ trợ Zustand store, SSE broadcast, remote shipper
- Backend SSE endpoint `/api/trace-stream` push events về browser real-time
- Browser adapter: `initBrowserTrace()` với localStorage toggle
- Pre-built tracers: `browseDirFlow`, `mkdirFlow`, `rmdirFlow`, `agentWsFlow`, `ipcProxyFlow`
- Enable: `ORCA_TRACE=1` (Node.js) / `localStorage.ORCA_TRACE = '1'` (browser)

**Tiêu chí thành công:**
- Span ID nhất quán xuyên suốt toàn bộ stack
- TracePanel UI hiển thị toàn bộ timeline với elapsed time
- Fail events hiển thị ngay cả khi ORCA_TRACE tắt

---

### 4.3 Nền tảng hỗ trợ

| Platform | Kiến trúc | Phân phối |
|----------|-----------|-----------|
| macOS | Apple Silicon, Intel | DMG, Homebrew |
| Windows | x64 | NSIS (.exe) |
| Linux | x64, arm64 | AppImage, .deb, AUR |
| iOS | arm64 | App Store, TestFlight |
| Android | arm64 | APK |

---

---

## 5. Yêu cầu phi chức năng

### 5.1 Hiệu năng

| Chỉ số | Mục tiêu |
|--------|----------|
| Startup time | < 3 giây (cold start) |
| Terminal typing latency | < 16ms |
| Idle CPU usage | < 1% CPU |
| Main thread jank | < 5ms frame drop |

### 5.2 Độ tin cậy

- Auto-update với fallback khi update thất bại
- Worktree removal safety (không xóa khi có uncommitted changes)
- SSH auto-reconnect với exponential backoff
- Crash reporting (opt-in)

### 5.3 Bảo mật

- E2E encryption cho mobile pairing (TweetNaCl)
- Secure file storage cho credentials
- SSH identity filter
- Agent trust presets (granular permissions)
- **bcrypt** password hashing (12 rounds) cho local auth
- **HTTP-only cookie** session, SameSite=Lax, Secure flag
- **Per-user process isolation** — data không accessible giữa users
- **Admin audit log** — ghi mọi action vào `orca_audit_log`
- **`requireAdmin`** guard trên tất cả admin API endpoints

### 5.4 Khả năng mở rộng

- Hỗ trợ 30+ AI agent không cần sửa core
- CLI-based extension
- Orca YAML config per-project
- MCP (Model Context Protocol) support
- Skills/hooks system
- **Multi-database** — plugin-style dialect adapters (`registerDatabaseProvider()`)
- **Platform abstraction** — `NodeAdapter` / `ElectronAdapter` switch
- **ORCA_MULTI_USER** env flag — enable/disable multi-user mode
- **Agent WebSocket** — language-agnostic wire protocol, relay + direct mode
- **WebCredentialStore** — per-user AES-256-GCM credential isolation

---

## 6. Mô hình phân phối

- **Free & Open Source** (MIT License)
- Download tại [onorca.dev/download](https://onorca.dev/download)
- Package managers: Homebrew, AUR
- Auto-update qua electron-updater
- Pre-release channel (opt-in)
- Telemetry: usage ẩn danh, opt-out được

---

## 7. Metrics thành công

| KPI | Mục tiêu |
|----|---------|
| Agent Sessions / User | > 5 sessions/ngày |
| Mobile Pairing Rate | > 30% users |
| SSH Usage | > 20% users |
| Crash Rate | < 0.1% sessions |
| Auto-update Success | > 99% |

---

## 8. Rủi ro

| Rủi ro | Mức độ | Giải pháp |
|--------|--------|-----------|
| API breaking changes từ AI providers | Cao | Adapter pattern, version detection |
| Performance degradation với nhiều worktrees | Trung bình | Benchmarking CI, performance budgets |
| SSH security vulnerabilities | Thấp/Cao | Security audit, minimal attack surface |
| Open-source forks/competitors | Cao | Ship fast, community building |
| Multi-user session management complexity | Trung bình | Per-process isolation, bcrypt auth |
| Database migration compatibility | Thấp | Cross-dialect tests, rollback support |

---

*Tài liệu được cập nhật từ codebase Orca v5.0 — Web Server Mode, Profile Hierarchy, AI Provider Management, Multi-Server Workflow Orchestration, Task Graph Management, Project Workspace, Remote Git UI, Full-Flow Tracing (2026-08-01)*
