# Orca Backend Server — Vai trò, Chức năng & Kết nối

**Nguồn:** Trích xuất từ HLD v1 (C1, C2, C3, C4)
**Cập nhật:** 2026-08-14 (đợt hiệu đính lớn — đối chiếu lại toàn bộ nội dung với code thật
`backend/src/**` tính đến commit `72ace6187`. Nguồn: `audit/backend/backend-vs-design-review.md`
(12 agent, file:line-cited) + `docs/guides/audit-backend-agent-2026-08-13.md` +
`docs/guides/roadmap-orca-project-task-rbac.md` cho các mục đã được sửa SAU ngày audit gốc
2026-08-08. Trước bản này tài liệu mô tả nhiều hành vi aspirational/tương lai như thể đã có —
bản này chỉ mô tả code đang chạy thật, đánh dấu rõ phần nào còn là đề xuất.)

**Quy ước trạng thái dùng trong tài liệu này:** ✅ đã implement & khớp mô tả · ⚠️ có implement
nhưng khác chi tiết (tên/port/số lượng...) so với những gì tài liệu từng ghi · 🚧 Proposed /
chưa implement · ❌ mô tả cũ sai hẳn so với code.

---

## 1. Orca Backend là gì?

Orca có **hai runtime modes** — cùng một codebase, hai adapter khác nhau:

| Mode | Runtime | Entry point | Dùng khi |
|------|---------|-------------|----------|
| **Electron Desktop** | Electron main process | `src/main/index.ts` | Single-user, developer workstation |
| **Orca Web Server** | Node.js server | `src/server/index.ts` | Multi-user, team deployment, CI/CD |

⚠️ Cả hai **không** dùng chung một Platform Abstraction Layer đối xứng như lời hứa ban đầu — xem
§3: chỉ nhánh Web Server thật sự đi qua `IPlatformServices`; Electron Desktop dùng thẳng SDK
`electron`.

---

## 2. Vai trò của Orca Backend Server

| Vai trò | Mô tả |
|---------|-------|
| **Control Plane** | Điều phối phần lớn hệ thống qua relay tới Dev Server Agent — **ngoại lệ quan trọng**: GitHub/GitLab hiện tự thực thi CLI (`gh`/`glab`) ngay trong process Backend cho hầu hết nghiệp vụ, không relay (xem §6.4) |
| **Auth & Session Gateway** | Xác thực users, quản lý session cookie, RBAC (RBAC hiện phân mảnh — xem §5, hàng RBAC) |
| **RPC Router** | Route tất cả JSON-RPC calls từ browser/CLI đến đúng service/relay |
| **Fleet Manager** | Quản lý fleet Dev Servers, health monitor (CPU/RAM/disk/latency — nay đã thu thập thật, xem §5), provisioning (CLI `orca fleet provision` 🚧 chưa tồn tại) |
| **Profile Resolver** | Deep-merge Company ← Dept ← User profile, cache TTL 60s |
| **Project Registry** | Binding project → dev server (rebind được qua `project.update`, xem §9), manage membership + RBAC |
| **AI Provider Manager** | Manage provider accounts metadata, relay credentials (không lưu plaintext), key rotation (nay đã implement — xem §5) |
| **Workflow Orchestrator** | Build DAG, topological dispatch steps đến đúng dev server; pause/resume nay đã implement (xem §5) |
| **Task Graph** | Manage task DAG, AI decomposition, spawn agents per task |
| **Admin Panel Host** | Serve REST `/admin/api/*` (Users CRUD, Sessions, Audit Log, Access Policies) — SPA shell `/admin` từng 404 do thiếu route static-serving, đã vá |
| **Agent WebSocket Hub** | Accept agent connections (direct-websocket mode) — chạy trên **HTTP port** (`:6769/agent`), không phải RPC port, xem §4/§8 |

---

## 3. Kiến trúc nội bộ — Platform Abstraction Layer

```
          IPlatformServices (interface)
                       │
                       │            ❌ ElectronAdapter KHÔNG tồn tại trong code — box
                       │            này là aspirational. Electron Desktop
                       │            (src/main/index.ts) import thẳng package `electron`
                       │            thật, không qua interface trừu tượng nào.
                       │
                  NodeAdapter                        (🚧 ElectronAdapter — Proposed,
                    NodeApp (~/.orca)                  chưa implement)
                    NodeWindow (noop)
              WS JSON-RPC dispatch
              NodeSecureStorage (AES-256-GCM file)
                       │
              Orca Web Server
              (src/server/index.ts)
```

- **`IPlatformServices`** — ✅ tồn tại đúng shape (`backend/src/platform/types.ts`).
- **`NodeAdapter`** — ✅ implement đầy đủ: `createNodeAdapter()`, compose `NodeApp` (dùng
  `~/.orca`), `NodeWindowManager`/`NodeWindow` (no-op UI), `NodeSecureStorage` (AES-256-GCM).
- **`ElectronAdapter`** — ❌ **không tồn tại**. Comment trong `platform/types.ts` mô tả
  "Implementations: ElectronAdapter (desktop) and NodeAdapter (server)" chỉ là aspirational.
  `desktop/src/main/index.ts` dùng trực tiếp SDK `electron` gốc. Các file
  `platform/stubs/electron-node-wrapper.ts`/`electron-web-stub.ts` làm chiều **ngược lại** (giả
  lập Electron API cho NodeAdapter/web), không phải một `ElectronAdapter` thật.
- **Kết luận:** Platform Abstraction Layer bất đối xứng — chỉ nhánh Web Server (Node) thật sự đi
  qua interface; nhánh Electron Desktop dùng thẳng SDK gốc. Quyết định có xây `ElectronAdapter`
  thật hay chính thức bỏ lời hứa này vẫn đang mở (`specs/backend/bugs/hld-v1/BUG-BE-HLD-017`).

---

## 4. Web Server Bootstrap Flow

```
src/server/index.ts
    │
    ├── new NodeAdapter({ userDataPath: ~/.orca })
    ├── initializeOrcaServices(nodeAdapter)      ← tên hàm thật (import từ ../main/server-bootstrap;
    │        │                                     KHÔNG phải "bootstrapWebApp" — tên đó thuộc về
    │        │                                     frontend/src/renderer/src/web/main-web-bootstrap.tsx,
    │        │                                     tầng trình duyệt, khác hoàn toàn)
    │        ├── new AuthManager(db, auditLogger)         ← constructor, không có static .init()
    │        ├── new SessionManager(...) — chỉ khi ORCA_MULTI_USER=1
    │        │        (idle timeout mặc định 4h, cấu hình được qua env SESSION_IDLE_TIMEOUT_MS;
    │        │         auto-respawn khi child crash, tối đa maxRespawnAttempts=3 — nay có logic
    │        │         thực thi thật, xem §7)
    │        ├── new AgentWebSocketServer() rồi .attach(httpServer)  ← gắn vào httpPort, KHÔNG
    │        │        phải rpcPort — xem cảnh báo port dưới đây
    │        └── FleetHealthMonitor.start()     ← class thật (backend/src/main/ssh/fleet-health-monitor.ts),
    │                 poll 30s, exec SSH thật để đo CPU/RAM/disk + ping latency (xem §5)
    │
    ├── Express HTTP :6769  (ORCA_HTTP_PORT, mặc định = ORCA_PORT + 1)
    │        ├── POST /auth/local           ← đăng nhập
    │        ├── GET  /admin/api/*          ← Admin REST API (requireAdmin guard)
    │        ├── GET  /admin, /admin-index.html ← SPA shell (từng 404 — route static-serving
    │        │        thiếu, đã vá; xác nhận sống qua `GET /admin-index.html` → 200)
    │        ├── GET  /health/ready         ← liveness probe
    │        └── GET  /health/metrics       ← Prometheus endpoint
    │
    └── WebSocket Server :6768  (ORCA_PORT)
             ├── /           ← Browser RPC, single-user mode (OrcaRuntimeRpcServer)
             │                  Ở multi-user mode (ORCA_MULTI_USER=1), browser WS thực chạy
             │                  qua WsSessionRouter chặn ngay trên 'upgrade' của PORT 6769,
             │                  KHÔNG phải 6768 — xem §8.
             └── (Agent connections KHÔNG nằm ở port này)

WebSocket Agent (direct-websocket mode) :6769/agent  ← trên HTTP port, KHÔNG phải RPC port
             AgentWebSocketServer intercept path '/agent' trên cùng http.Server với Express
```

> ⚠️ **Port Agent WS là phát hiện quan trọng nhất của mục này**: tài liệu cũ ghi Agent kết nối
> `wss://backend:6768/agent`. Code thật gắn `AgentWebSocketServer` vào **httpPort (6769)** —
> comment ngay trong `agent-ws-server.ts`: *"Browser → ws://:6768/ (existing OrcaRuntimeRpcServer);
> Agent → ws://:6769/agent"*.

---

## 5. Domain Services bên trong Backend

| Domain | Module | Chức năng chính |
|--------|--------|----------------|
| **Auth** | `AuthManager` / `auth-router.ts` | bcrypt 12 rounds + HTTP-only cookie (`sameSite:'strict'`, `secure` chỉ bật khi `NODE_ENV==='production'`) + session TTL, cleanup timer 30 phút |
| **Session** | `SessionManager` + `fork()` | Method thật là `getOrSpawnUserProcess()` (không phải `getOrCreate`). Mỗi user = Node.js process riêng, Unix socket `~/.orca/users/<userId>/orca.sock`, giới hạn heap `--max-old-space-size=512`. **Auto-respawn (max 3) và idle-timeout qua `SESSION_IDLE_TIMEOUT_MS` nay đã có logic thực thi thật** (trước đây chỉ có hằng số, không dùng) |
| **Fleet** | `FleetHealthMonitor` (`backend/src/main/ssh/fleet-health-monitor.ts`) | Poll dev servers mỗi **30s** (không phải 60s). **Nay thu thập CPU/RAM/disk thật** qua `collectRemoteResourceMetrics()` (SSH exec) + ping latency thật qua `execCommand(conn, 'true')` — trước đây chỉ đọc lại `SshConnectionStatus` có sẵn, không đo gì |
| **Profile** | `ProfileResolver` | 3-layer merge (Company ← Dept ← User), cache 60s |
| **Project** | `ProjectService` + `ProjectServerRouter` | Project → devServerId binding, auto-route requests. `project.update` nay chấp nhận `devServerId` (rebind được) — nhưng thiếu guard chặn rebind khi project đang có workflow/task execution chạy dở (`PROJECT_HAS_ACTIVE_WORKFLOWS`, TODO còn mở) |
| **AI Provider** | `AIProviderService` + `ProviderResolver` + `ProviderHealthChecker` | Metadata CRUD, credential relay qua SSH. ⚠️ Class `ProviderCredentialRelay` **không tồn tại** — logic nằm trong `AIProviderService.writeCredentialToDevServer()`. **Key rotation nay đã implement** (`aiProvider.rotateKey` RPC, cột `rotation_grace_until` — migration 0015) |
| **Workflow** | `WorkflowOrchestrator` + `DAGBuilder` | Template inheritance (chain resolve, depth 5), wave-based execution. **Pause/Resume nay đã implement** (`workflow.pause`/`workflow.resume` RPC, cột `paused_at` — migration 0014); trước đây chỉ có crash-recovery resume, không có user-triggered pause |
| **Task Graph** | `TaskService` + `TaskAgentExecutor` | Cụm đầy đủ `TaskDAGValidator`/`TaskService`/`TaskGrantService`/`TaskAIPlanner`/`TaskAgentExecutor`; 18 RPC method thật; grant resolution BFS đúng thuật toán |
| **Credentials** | `WebCredentialStore` | AES-256-GCM, path per-user. **Không bao phủ GitHub/GitLab** — 2 provider này dựa vào OS keychain của CLI `gh`/`glab`, không qua store này (xem §6.4) |
| **DB** | `IConnectionPool` + adapters | SQLite / MySQL / PostgreSQL / TiDB (mysql2 + dialect flag) |
| **RBAC** | *(không có `hasPermission()` thống nhất)* | ❌ Không có 1 policy table "Role → resource → action" duy nhất như tài liệu cũ mô tả. Thay vào đó có **nhiều cơ chế permission tách biệt**: `resolveUserPermissions()` (fleet/server-level, union allowlist + agentTrust) và `TaskGrantService.resolvePermission()` (task graph riêng, BFS ancestor). ✅ **Tin tốt**: `requireAdmin(ctx)` (`profile-rpc-handler.ts`) và `requireOwnerOrAdmin` (`project-rpc-handler.ts`) — từng là bug bảo mật nghiêm trọng (chỉ check đã-login, không check role thật) — **đã được vá**: cả hai nay tra `role` thật qua `getUserRole` closure (trỏ tới `AuthUserStore.getUser()`), chặn đúng theo `role==='admin'`/owner. Rủi ro còn lại: vẫn là nhiều cơ chế rời rạc, chưa hợp nhất thành 1 policy table |

---

## 6. Kết nối với các thành phần khác

### 6.1 Browser / Frontend → Orca Backend

```
Browser
  │
  ├── HTTPS :6769  (thực chất là node:http thuần — "HTTPS" chỉ đúng nếu có
  │     │           reverse-proxy/TLS terminator phía trước)
  │     ├── POST /auth/local  → AuthManager (bcrypt verify) → set orca_session cookie
  │     ├── GET  /admin/api/* → requireAdmin → Admin CRUD handlers
  │     └── GET  /health/*    → health endpoints
  │
  └── WebSocket :6768/         (single-user mode)
        │ (WS upgrade + cookie auth)
        └── WsSessionRouter → per-user Unix socket
                  └── User process (fork) → JSON-RPC dispatch

      ⚠️ Ở multi-user mode (ORCA_MULTI_USER=1), WsSessionRouter thực chặn
      upgrade trên PORT 6769 (không phải 6768) — browser WS thật sự dùng
      chung port với HTTP trong mode này.

      Agent connections (direct-websocket mode) đi qua path /agent, NHƯNG
      trên port 6769 (AgentWebSocketServer), không phải :6768 — xem §4/§8.

Protocol: JSON-RPC 2.0 qua WebSocket text frame (`\n`-delimited) + một số
          frame binary nhận diện bằng 1 byte đầu. ❌ KHÔNG phải "13-byte
          binary header" — header 13-byte (TYPE/SEQ/ACK/LEN) là có thật
          nhưng thuộc kênh khác: Agent Wire Protocol (Backend ↔ Dev Server/
          Agent, xem §6.2), không phải kênh Browser ↔ Backend này.
Auth: HTTP-only cookie `orca_session`, sameSite='strict', secure chỉ bật
      khi NODE_ENV==='production'
```

### 6.2 Orca Backend → Dev Server (3 modes)

```
Mode 1: relay-ssh
  Backend → ssh2 library → SSH exec channel → SshChannelMultiplexer → JSON-RPC

Mode 2: relay-websocket (Orca outbound)
  Backend → HTTP Upgrade ws://agent:PORT/<path theo config.wsUrl của Dev Server>
           Header: Authorization: Bearer <agentToken>
           → SshChannelMultiplexer → JSON-RPC
           (path "/orca-relay" chỉ là quy ước gợi ý trong thông báo lỗi,
            KHÔNG phải route cố định — URL thật do config.wsUrl quyết định)

Mode 3: direct-websocket (Agent inbound)
  Dev Server Agent → wss://backend:6769/agent      ← PORT 6769, không phải 6768
           Handshake: { agentToken } → { sessionId }; token được hash SHA-256
           trước khi lưu (không lưu plaintext)
           → AgentWebSocketServer → JSON-RPC

Cả 3 mode dùng chung 13-byte frame header (TYPE/SEQ/ACK/LEN) của Agent Wire
Protocol — đây mới là kênh có header nhị phân 13-byte thật (khác §6.1).

Keepalive: gửi mỗi 5s (KEEPALIVE_SEND_MS=5_000), timeout sau 20s không nhận
data (TIMEOUT_MS=20_000) — không phải "30s/3 lần miss (90s)" như tài liệu cũ.
Close code: dùng mã WS chuẩn 1008 (token/version sai) hoặc 1005 mặc định
(handshake timeout) — không có mã tuỳ biến 4001-4003.
Version-mismatch check (AGENT_MIN_VERSION) nay đã được enforce thật trong
ws-handshake.ts (trước đây là hằng số khai báo nhưng không ai kiểm tra).
relay-websocket mode có reconnect exponential backoff (2s→60s, jitter);
direct-websocket mode không có Orca-side reconnect — dựa vào agent tự
reconnect (thường qua systemd).
```

### 6.3 Orca Backend → AI Agent CLIs (PTY)

```
ProfileAwareAgentSpawner.spawn()
    │
    └── LUÔN đi qua relay: relay.call('agent.exec', ...) tới Dev Server Agent
              │
              Dev Server Agent (agent/src/relay/agent-spawner.ts)
              └── node-pty.spawn(agentBinary, args, { cwd, env })
                        ├── claude
                        ├── codex
                        ├── gemini
                        └── custom agent binary (map qua resolveAgentSpec())
                        │
                        PTY stdin/stdout ← OSC escape sequence parsing
                        (backend/src/shared/terminal-title-status.ts,
                         detectAgentStatusFromTitle() — KHÔNG phải AgentAwakeService)
```

- ❌ **`AgentOrchestrator` không tồn tại.** Class thật là `ProfileAwareAgentSpawner`, và `spawn()`
  **luôn** relay `agent.exec` tới Dev Server — không có nhánh gọi `node-pty.spawn` trực tiếp
  ngay trong Backend cho AI-agent-exec. `node-pty.spawn` thật sự chạy ở phía **Dev Server Agent**.
  Backend chỉ tự spawn `node-pty` local cho terminal thường (không phải AI agent CLI).
- ❌ **`AgentAwakeService`** không phải service parse OSC/điều khiển state machine — đó là service
  quản lý *power-save-blocker* (giữ máy không ngủ khi agent chạy), chỉ tồn tại ở
  `desktop/src/main/agent-awake-service.ts` (Desktop, không có bản Backend riêng), nhận trạng thái
  `working` từ bên ngoài chứ không tự parse gì.
- ❌ **State machine `idle → running → waiting → completed`** không tồn tại đúng như mô tả. Có
  **2 state machine riêng biệt, không machine nào khớp**: `AgentStatus = 'working'|'permission'|
  'idle'` (3-state, terminal title detection) và `AgentLifecycleState = 'idle'|'spawning'|
  'running'|'stopping'|'stopped'|'error'` (6-state, phía Dev Server Agent).

### 6.4 Orca Backend → Git Platforms

```
GitHub / GitLab  → shell ra CLI (gh / glab) — KHÔNG phải HTTP client thuần
Linear           → HTTPS REST (SDK LinearClient chính thức)
Jira             → HTTPS Basic Auth (base64(email:apiToken))
Bitbucket        → HTTPS App Password (Basic Auth)
Azure DevOps     → HTTPS PAT token (Basic Auth base64(username:pat))

WebCredentialStore (AES-256-GCM per-user) bao phủ: bitbucket/azure-devops/
gitea/linear/jira — KHÔNG bao phủ github/gitlab (2 cái này dựa vào OS
keychain riêng của CLI gh/glab).

Env var mã hoá master key: ORCA_SERVER_SECRET (không phải ORCA_CREDENTIAL_KEY).
```

> ❌ **Sai lệch kiến trúc quan trọng nhất mục này — vẫn còn nguyên vẹn tính đến 2026-08-14**:
> Theo đúng thiết kế Gateway/Dev-Server-Fleet (`docs/hld/dev-server-architecture.md §12`),
> Backend/Gateway **không nên** tự thực thi `gh`/`glab` — vai trò "External API Caller" thuộc về
> Dev Server, với per-user `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` isolation, để token không bao giờ
> nằm trên Gateway. Phía Dev Server Agent **đã implement đúng** thiết kế này
> (`agent/src/relay/agent-git-handler.ts` + `external-api-connector.ts`, per-user config dir đầy
> đủ) — nhưng **Backend không gọi các method này** cho nghiệp vụ hàng ngày.
>
> Thực tế: **hầu hết thao tác GitHub/GitLab** (list issue/PR, rate-limit, project-view, comments,
> work-item-details...) đi qua `ghExecFileAsync`/`gitExecFileAsync` → `child_process.execFile`
> **chạy ngay trong process của Backend** (`backend/src/main/github/issues.ts` và tương tự cho
> `gitlab/`), không có `cwd`/relay nào, dùng chung 1 auth context (`gh`/`glab` đăng nhập trên host
> Backend) cho **mọi user** trong Web Server multi-user mode — vi phạm "Auth never through
> Gateway" và không có per-user isolation cho phần này.
>
> ⚠️ **Cập nhật một phần (đã vá, chưa vá hết)**: per-user `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` nay
> **đã** được truyền đúng cho 2 luồng hẹp: `github.startAuthLogin`/`gitlab.startAuthLogin` (đăng
> nhập CLI) và `preflight.check` khi có `devServerId`. Nhưng toàn bộ phần còn lại — đọc/ghi
> issue, PR, MR, rate-limit... — vẫn chạy không cô lập theo user như mô tả ở trên. Implementation
> đầy đủ phía Agent (`git.pr.create` và các RPC `git.*` nghiệp vụ GitHub/GitLab khác) vẫn là
> **dead code** từ góc nhìn Backend — không có caller nào gọi tới.

### 6.5 Orca Backend → Database

```
loadDatabaseConfig() trả về null hay không → chọn json/sql
  │                     (❌ KHÔNG có env ORCA_STORAGE_BACKEND như tài liệu cũ ghi)
  ├── null   → JsonFileStateRepository (dataFile: store.json — không phải "orca-data.json")
  └── config → SqlStateRepository
                    │
               ORCA_DB_URL (DSN)
                    │
       ┌────────────┼────────────┐
  SQLiteAdapter  MySQLAdapter  PostgreSQLAdapter
  (file://...)   (mysql://...)  (postgresql://...)
  [TiDB/MariaDB = mysql2 + dialect flag]

Migration runner: 0001 → 0017 (sequential, idempotent) — KHÔNG dừng ở 0010
Health monitor: ping DB mỗi 30s. "auto-reconnect" — 🚧 chưa thấy code hiện
thực (chỉ emit trạng thái unhealthy/degraded, không tự kết nối lại)
```

### 6.6 Orca Backend → Mobile App

```
Orca Backend
    │
    ├── QR pairing (PairingOfferSchema thật):
    │     { v, endpoint, deviceToken, publicKeyB64, scope? }
    │     ⚠️ KHÁC tên field tài liệu cũ ({pubKey, host, port, token}) —
    │        host/port gộp vào `endpoint`, token→deviceToken, pubKey→publicKeyB64
    │         → TweetNaCl key exchange (Curve25519/NaCl box) → shared secret
    │
    └── WebSocket (E2E encrypted TweetNaCl box cipher) — handshake `e2ee_auth`
              rồi mã hoá/giải mã mọi frame
              🚧 Message type `'agent:completed'`/`'dispatch'` — chưa xác nhận
              được literal string này trong code đã khảo sát; có cơ chế
              NotificationEvent + inject input vào PTY (Session.write()) nói
              chung, nhưng chưa chắc dùng đúng tên message như tài liệu.
```

### 6.7 Orca Backend → Orca CLI

```
Orca CLI (orca worktree/agent/serve)
    │
    └── Unix Socket → Orca Daemon (NDJSON protocol, encodeNdjson = JSON.stringify(msg)+'\n',
              NDJSON_MAX_LINE_BYTES=16MB)
              ├── Headless mode: daemon chạy như tiến trình Node độc lập, không phụ thuộc Electron
              └── Reattach: khôi phục PTY sessions qua Session.getSnapshot()/takePendingOutput()
```

✅ Khớp hoàn toàn với code — không có sai lệch nào ở mục này.

---

## 7. Per-User Session Isolation (Web Server Mode)

```
User đăng nhập → POST /auth/local → set cookie
    │
    ▼ WebSocket connect → WsSessionRouter
    │   → validate cookie → userId
    │   → SessionManager.getOrSpawnUserProcess(userId)   ← tên hàm thật (không phải getOrCreate)
    │
    ▼ Session process (fork())
    │   Process: ~/.orca/users/<userId>/orca.sock  (quyền thư mục 0o700)
    │   Timeout idle: mặc định 4h, sweep mỗi 5 phút, cấu hình được qua
    │                 env SESSION_IDLE_TIMEOUT_MS (nay đã implement, trước đây bị bỏ qua)
    │   Max respawns: 3 (maxRespawnAttempts) — child.on('exit') nay CÓ logic
    │                 thực thi respawn thật (trước đây chỉ dọn dẹp, không spawn lại)
    │   Giới hạn heap: --max-old-space-size=512 mỗi user process
    │
    └── Mọi RPC call trong session đều isolated trong process riêng
```

### 7.1 Dev Server Provider Registry qua IPC (2026-08)

✅ **Khớp hoàn toàn với thiết kế**, kể cả cơ chế `devServer:proxyNotification` (2026-08):

Provider registry (`IFilesystemProvider`/`IGitProvider`/`IPtyProvider`, dùng chung với SSH Targets
— xem [dev-server-architecture.md](./dev-server-architecture.md) §15) là **transport-agnostic đối
với process nào giữ connection thật**: connection WebSocket outbound của một Dev Server luôn sống
trong **process cha (Gateway)**, không phải trong per-user child process — nhưng mọi user (ở mọi
child process) đều phải gọi được provider của Dev Server đó, và còn phải **nhận được**
notification agent chủ động push (`pty.data`, `pty.exit`, `fs.changed`).

`GatewayDevServerManagerProxy` (chạy trong mỗi child process) forward mọi RPC call của provider
qua IPC (`process.send`) về `SessionManager` ở process cha, và ngược lại nhận 2 loại broadcast:

| IPC message type | Nguồn phát | Mục đích |
|-------------------|-----------|----------|
| `devServer:event` | `devServerManager.on('devServer:added'\|'removed'\|'statusChanged')` | Đồng bộ trạng thái Dev Server (connect/disconnect) tới mọi child process |
| `devServer:proxyNotification` | `devServerManager.on('devServer:notification')` | Relay notification agent chủ động push (`pty.data`/`pty.exit`/`fs.changed`) tới mọi child process; mỗi child tự lọc theo `devServerId` |

---

## 8. Communication Matrix đầy đủ

| From | To | Protocol | Port/Path | Format |
|------|----|----------|-----------|--------|
| Browser | Orca HTTP | HTTP(S)* | `:6769` | REST JSON — *"HTTPS" chỉ đúng nếu có reverse-proxy/TLS terminator phía trước; server tự nó là `node:http` thuần |
| Browser | Orca WS | WebSocket | `:6768/` (single-user) hoặc `:6769` (multi-user, `ORCA_MULTI_USER=1`, qua `WsSessionRouter`) | JSON-RPC (text frames + một số binary nhận diện bằng byte đầu) |
| Dev Agent | Orca WS | WebSocket | `:6769/agent` | 13-byte header (TYPE/SEQ/ACK/LEN) + JSON-RPC |
| Orca | Dev Server | SSH exec | `:22` | relay protocol |
| Orca | Dev Server | WebSocket | `agent:PORT/<config.wsUrl>` (path không cố định) | 13-byte header + JSON-RPC |
| Orca | AI Agent CLIs | — | — | Luôn qua relay `agent.exec` tới Dev Server Agent; không spawn local cho AI agent |
| Orca | GitHub/GitLab | CLI `gh`/`glab` (execFile, **chạy trong process Backend**) | `:443` (do CLI tự gọi) | REST/GraphQL JSON qua CLI stdout |
| Orca | Linear/Jira/Bitbucket/Azure DevOps | HTTPS | `:443` | REST JSON |
| Orca | Database | SQL | DSN | `IConnectionPool` queries |
| Orca | Mobile | WebSocket | dynamic | TweetNaCl encrypted JSON |
| CLI | Daemon | Unix Socket | `~/.orca/daemon.sock` | NDJSON |
| Daemon | AI Agents | PTY | — | Text |
| Gateway (parent) | User child process | IPC (`fork()` channel) | — | `devServer:event` / `devServer:proxyNotification` (xem §7.1) |

---

## 9. RPC Method Namespaces (JSON-RPC 2.0)

⚠️ Namespace thật dùng **số ít/camelCase** (`project.*`, `aiProvider.*`, `workflow.*`, `task.*`),
KHÔNG phải số nhiều/gạch nối như tài liệu cũ (`projects.*`, `ai-providers.*`, `workflows.*`,
`tasks.*`). `fs.*`/`pty.*` chỉ là giao thức nội bộ Backend→Dev Server Agent — API client thật
dùng namespace `files.*`/`terminal.*`.

| Namespace | Số methods | Ví dụ |
|-----------|-----------|-------|
| `profile.*` | 11 | `getResolved`, `getUserProfile`, `updateUser`, `updateCompany`, `updateDept`, `createCompany`, `createDept`, `listDepts`, `setUserDept`, `invalidate`, `getCompany` |
| `project.*` | 10 | `list`, `get`, `create`, `update` (nhận `devServerId` để rebind), `delete`, `addMember`, `removeMember`, `updateMemberRole`, `getMembers`, `agentSpawn` |
| `aiProvider.*` | 10 | `list`, `create`, `get`, `update`, `delete`, `writeCredential`, `rotateKey`, `testConnection`, `getUsageToday`, `resolve` |
| `workflow.*` | 9 | `execute`, `getExecution`, `listExecutions`, `cancel`, `pause`, `resume`, `template.create`, `template.list`, `template.resolve` |
| `task.*` | 18 | `create/get/update/delete/list/getChildren/getAncestors/getSubtree/addEdge/removeEdge/getDependencies/recalculateProgress/addComment/grant/resolvePermission/aiDecompose/aiApply/execute` |
| `credentials.*` | 4 | `set`, `revoke`, `status`, `list` (✅ namespace duy nhất khớp 1:1 với tài liệu cũ) |
| `preflight.*` | 5 | `check` + 4 method khác chưa từng được tài liệu hoá |
| `git.*` | ~35 | Namespace **phẳng** (`git.branchCompare`, `git.commit`, ...) — KHÔNG có sub-namespace lồng kiểu `git.branch.*` |
| `files.*` (client API) — `fs.*` (internal Backend↔Agent) | 28 | `readDir`, `readFile`, `glob`, `grep` — `fs.*` không phải namespace client-facing, chỉ là giao thức nội bộ tới Dev Server Agent |
| `terminal.*` (client API) — `pty.*` (internal Backend↔Agent) | 28 | `spawn`, `write`, `resize`, `kill` — tương tự `fs.*`, `pty.*` chỉ nội bộ |

---

## 10. DB Schema Overview (Migrations 0001–0017)

❌ Tài liệu cũ mô tả 10 migration, hầu hết tên bảng của 0001-0004 và 0007 **sai hoàn toàn** so
với thực tế. Bảng dưới đối chiếu lại toàn bộ 17 migration thật:

| Migration | Tables |
|-----------|--------|
| 0001 `initial_schema` | `settings`, `projects`, `repos`, `ssh_targets` |
| 0002 `add_automations` | `automations` |
| 0003 `add_workspace_sessions` | `workspace_sessions` |
| 0004 `orca_app_tables` | `orca_projects` (legacy, tab/state cho desktop/single-user mode — KHÔNG liên quan Project↔DevServer binding), `orca_repos`, `orca_ssh_targets`, `orca_global_settings` |
| 0005 `add_auth_schema` | `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies` |
| 0006 `company_dept` | `orca_companies`, `orca_departments`, `orca_user_profiles` + ALTER `orca_users` (thêm `department_id`) |
| 0007 `projects` | `orca_v5_projects`, `orca_v5_project_members` — dùng tiền tố `v5` để tránh đụng độ với bảng `orca_projects` legacy của migration 0004. Đây là bảng project thật cho Project↔DevServer binding |
| 0008 `ai_providers` | `orca_ai_provider_accounts`, `orca_provider_usage` |
| 0009 `workflows` | `orca_workflow_templates`, `orca_workflow_executions`, `orca_workflow_step_executions` |
| 0010 `tasks` | `orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments`, `orca_team_members` |
| 0011 `terminal_sessions` | `orca_terminal_sessions` |
| 0012 `port_forwards_push` | `orca_port_forwards`, `orca_push_subscriptions` |
| 0013 `workflow_trace_correlation` | ALTER `orca_workflow_executions` thêm `root_trace_id` — cơ chế trace-correlation-qua-restart cho Workflow |
| 0014 `workflow_pause_state` | ALTER `orca_workflow_executions` thêm `paused_at` — hỗ trợ user-triggered pause/resume |
| 0015 `ai_provider_rotation` | ALTER `orca_ai_provider_accounts` thêm `rotation_grace_until` + index — hỗ trợ AI provider key rotation |
| 0016 `team_project_sharing_task_exec` | `orca_teams` (mới), ALTER `orca_team_members` thêm `priority`, `orca_project_source_projects` (join OrcaProject↔Project per-user), ALTER `orca_tasks` thêm `active_execution_task_id`/`agent_session_id` |
| 0017 `team_profile_json` | ALTER `orca_teams` thêm `profile_json` |

> ⚠️ **Lưu ý naming:** `orca_projects` (0004) và `orca_v5_projects` (0007) là **hai bảng khác
> nhau, cùng tồn tại song song trong DB**, không phải hai phiên bản của cùng một bảng.
> `orca_projects` lưu state/tab đơn giản (desktop/single-user mode, không có `dev_server_id`);
> `orca_v5_projects` là entity project đầy đủ gắn với dev server (server mode). Khi viết SQL hoặc
> query trực tiếp, LUÔN xác nhận đang thao tác đúng bảng theo mục đích.

---

## 11. Sơ đồ tổng quan kết nối

```
                    ┌──────────────────────────────────────┐
                    │        ORCA BACKEND SERVER            │
                    │     (Control Plane / Gateway)         │
                    │                                       │
  Browser ─HTTP───→│ :6769  AuthManager + Admin REST + SPA │
  Browser ─WS──────│ :6768  WsSessionRouter (single-user)  │
                    │        hoặc :6769 khi multi-user mode │
  Dev Agent ─WS────│ :6769/agent  AgentWebSocketServer      │
                    │                                       │
                    │  ProfileResolver   (cache 60s)        │
                    │  ProjectService    (binding, rebind)   │
                    │  AIProviderSvc     (metadata + rotate) │
                    │  WorkflowOrch      (DAG + pause/resume)│
                    │  TaskService       (grant/plan/exec)   │
                    │  FleetHealthMonitor (30s poll, CPU/RAM/│
                    │                      disk/latency thật)│
                    │  WebCredentialStore (AES-256-GCM,      │
                    │    KHÔNG cover github/gitlab)          │
                    │  github/gitlab: gh/glab CLI CHẠY TẠI   │
                    │    CHỖ trong process Backend (§6.4)    │
                    │  IConnectionPool   (DB abstraction)    │
                    └───────────────┬──────────────────────┘
                                    │
             ┌──────────────────────┼──────────────────┐
             ↓                      ↓                   ↓
     Dev Server(s)           AI Agent CLIs       External APIs
   (SSH / WS relay)      (spawn qua relay,       (GitHub/GitLab qua
   git, fs, pty, shell    KHÔNG local trong        CLI local; Linear/
   AiCredStore             Backend)                Jira/Bitbucket/Azure
   StepExecutor                                     qua HTTPS thật)
```
