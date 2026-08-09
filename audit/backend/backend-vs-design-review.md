# Đánh giá Backend Code vs Thiết kế (HLD)

**Ngày:** 2026-08-08 (Vòng 1 + Vòng 2 mở rộng)
**Phạm vi:** `backend/src/**` đối chiếu với toàn bộ nguồn thiết kế liên quan đến Backend: `docs/hld/backend-server-architecture.md` + `dev-server-architecture.md` (Vòng 1), và `docs/features/F22–F40` — các feature doc chi tiết hơn cho Web Server / Multi-user / Fleet / AI Provider / Workflow / Task Graph (Vòng 2).
**Phương pháp:** 12 agent song song qua 2 vòng, mỗi agent đối chiếu một mảng thiết kế với code thật bằng CodeGraph/GitNexus (đọc symbol trực tiếp, không suy đoán), trích dẫn bằng chứng `file:line`.
**Trạng thái:** Hoàn tất Vòng 1 (5/5 mảng theo `backend-server-architecture.md`) + bổ sung 2.12b (GitHub/GitLab relay). Vòng 2 (F22–F40, 7 mảng) — xem §5.
**Ngoài phạm vi (đã xác nhận không chạm backend/):** F19-localization, F20-speech-input, F21-auto-update (dùng `desktop/src/main/updater-prerelease-feed.ts`, không phải `backend/`), F41-desktop-pet-companion, F42-contextual-onboarding-tours — 5 feature này là frontend/desktop-only, 0 tham chiếu backend, không đưa vào audit.

## 0. Bối cảnh quan trọng: `docs/adrs/v2/ADR-016..020` là proposal CHƯA triển khai

Khi rà soát "phiên bản HLD mới hơn", phát hiện thư mục `docs/adrs/v2/` chứa 5 ADR (ADR-016 → ADR-020, ngày 2026-07-30) mô tả một kiến trúc "HLD v6.0": tách Control Plane/Data Plane nghiêm ngặt (ADR-018), mô hình layer A0–A4 cho Dev Server Agent (ADR-017), schema migration 0006–0010 "chuẩn" (ADR-016), signed execution context HMAC-SHA256 + reconnect strategy (ADR-019), feature-flag theo migration version + dual-mode AgentDispatcher (ADR-020).

**Cả 5 ADR này đều ở trạng thái `🚧 Proposed` và tự ghi rõ trong mục "Trạng thái Implementation" là `❌ Chưa implement` / `❌ chưa tồn tại`.** Chúng liên tục tham chiếu tới `docs/hld/README.md` và `docs/hld/C4-code.md` ("HLD v6.0") — **hai file này không tồn tại trong repo** (chỉ có `docs/hld/v1/README.md` và `docs/hld/v1/C4-code.md`, vốn là baseline v5.0 theo chính `docs/adrs/v2/README.md` dòng 26). Nói cách khác: **"HLD v6.0" mà ADR v2 dẫn chiếu là một tài liệu ma — chưa từng được tạo ra**, và ADR v2 là proposal độc lập, chưa được chấp nhận (không phải "Accepted"), chưa nối vào một HLD gốc nào.

**Hệ quả cho audit này:** khi code không khớp với `docs/adrs/v2/*`, đây KHÔNG được tính là lỗi/sai lệch — đó là khoảng cách giữa một đề xuất kiến trúc còn treo và trạng thái hiện tại, đúng như ADR tự thừa nhận. Các agent audit vòng 2 đã được yêu cầu phân biệt rõ 2 loại: (a) sai lệch so với `docs/features/F*.md` (mô tả feature hiện tại/yêu cầu → finding thật), và (b) chưa khớp `docs/adrs/v2/*` (proposal treo → không tính là lỗi, chỉ ghi nhận thông tin). Bản thân việc ADR v2 dẫn chiếu tới tài liệu không tồn tại (`docs/hld/README.md`, `docs/hld/C4-code.md`) **là một lỗ hổng tài liệu thật** đáng ghi nhận riêng.

Bằng chứng cụ thể đã đọc trực tiếp: `docs/adrs/v2/ADR-016-db-migrations-0006-0010-schema.md:271` (`❌ Migrations 0006–0010 chưa được tạo`), `ADR-017-dev-server-agent-layer-model.md:221` (`❌ src/agent/ package chưa tồn tại`), `ADR-018-control-plane-data-plane-separation.md:175` (`🚧 Pattern định nghĩa; cần enforce qua code review`), `ADR-020-enterprise-rollout-phases-backward-compat.md:230` (`❌ Chưa implement`), `docs/adrs/v2/README.md:16-20` (bảng trạng thái tất cả `🚧 Proposed`).

**Lưu ý quan trọng khác:** thực tế migrations 0006–0010 ĐÃ tồn tại trong code (xem §2.7) nhưng với tên bảng KHÁC với đề xuất trong ADR-016 (vd ADR-016 đề xuất `orca_company`/`orca_projects`, code thực tế dùng `orca_companies`/`orca_v5_projects` — comment trong code giải thích đổi tên để tránh đụng độ với bảng `orca_projects` đã bị chiếm bởi migration 0004). Điều này cho thấy đội ngũ đã tự triển khai một schema *khác* với đề xuất ADR-016, rồi không quay lại cập nhật ADR — nghĩa là ADR-016 vừa "chưa implement" (theo tự nhận) vừa "đã bị code thực tế vượt qua theo hướng khác" cùng lúc.

---

## 1. Tổng kết mức độ khớp (theo mục tài liệu)

| Mục thiết kế | Trạng thái | Vấn đề chính |
|---|---|---|
| §3 Platform Abstraction Layer | ❌ Sai lệch nghiêm trọng | `ElectronAdapter` **không tồn tại** — Electron desktop dùng thẳng module `electron`, không qua `IPlatformServices` |
| §4 Web Server Bootstrap Flow | ⚠️ Một phần | Tên hàm `bootstrapWebApp` sai (hàm thật: `initializeOrcaServices()`); `AuthManager.init()`/`SessionManager.init()`/`FleetHealthMonitor.start()` không tồn tại dạng đó |
| §6.1 Browser ↔ Backend | ⚠️ Một phần | Cơ chế cookie/bcrypt/route đúng; nhưng "13-byte binary header" là của kênh khác (Agent Wire Protocol), không phải kênh browser |
| §6.2 Backend ↔ Dev Server (3 modes) | ✅ Đúng cơ chế, ⚠️ sai chi tiết | Logic 3 mode đúng; nhưng **port Agent WS sai** (thiết kế ghi 6768, thực tế 6769); path `/orca-relay` không cố định |
| §5 Domain Services | ⚠️ Một phần | Đa số module tồn tại đúng hành vi (bcrypt 12 rounds, cache 60s, AES-256-GCM...); nhưng `ProviderCredentialRelay` và `hasPermission()` **không tồn tại** |
| §6.5 Database | ⚠️ Một phần | `ORCA_STORAGE_BACKEND` không tồn tại; file JSON thật là `store.json` không phải `orca-data.json`; "auto-reconnect" chưa thấy code |
| §10 DB Schema (Migrations 0001–0010) | ❌ Sai lệch nghiêm trọng | Thực tế có **13 migration**; nội dung/tên bảng 0001–0004, 0007 **sai gần như hoàn toàn** so với mô tả |
| §7 Session Isolation | ✅ Khớp phần lớn | Chỉ khác tên hàm (`getOrSpawnUserProcess` thay vì `getOrCreate`); auto-respawn (max 3) thiếu logic thực thi |
| §7.1 Dev Server Provider Registry qua IPC | ✅ Khớp hoàn toàn | Kể cả cơ chế mới `devServer:proxyNotification` (2026-08) |
| RBAC `hasPermission()` | ❌ Không tồn tại | Có 2 cơ chế permission riêng biệt, không thống nhất, khác hẳn "Role→resource→action" |
| §9 RPC Method Namespaces | ❌ Sai lệch nghiêm trọng | Hầu hết namespace sai tên (số ít/không gạch nối) + undercount số lượng method đáng kể; `fs.*`/`pty.*` nhầm lớp giao thức |
| §6.3 AI Agent CLIs (PTY) | ⚠️ Một phần | `AgentOrchestrator` không tồn tại; state machine thực tế không phải 4-state `idle→running→waiting→completed` |
| §6.4 Git Platforms | ❌ Sai lệch kiến trúc nghiêm trọng *(cập nhật 2026-08-08, xem §2.12b)* | GitHub/GitLab đi qua CLI (`gh`/`glab`) — đúng; nhưng **CLI này chạy trực tiếp trên host Backend/Gateway**, KHÔNG relay đến Dev Server Agent như `docs/hld/dev-server-architecture.md §12` mô tả. Implementation phía Agent (`git.pr.create`, per-user `GH_CONFIG_DIR`) tồn tại nhưng **không có caller nào** — dead code. Không có per-user CLI-auth isolation ở Backend. |
| §6.6 Mobile App | ⚠️ Một phần | Cơ chế QR pairing + TweetNaCl đúng nhưng tên field khác hẳn; message type `agent:completed`/`dispatch` chưa xác nhận được trong code |
| §6.7 Orca CLI/Daemon | ✅ Khớp hoàn toàn | Unix socket + NDJSON + headless + reattach PTY đều đúng |

**Chỉ 3 điểm khớp hoàn toàn 1:1 với thiết kế:** `credentials.*` RPC namespace, §7.1 Dev Server Provider Registry qua IPC, và phần lớn cơ chế Session Isolation (§7).

---

## 2. Chi tiết theo mục

### 2.1 §3 — Platform Abstraction Layer

- **IPlatformServices** — ✅ tồn tại đúng shape tại `backend/src/platform/types.ts:20-27`.
- **NodeAdapter** — ✅ khớp đầy đủ: `createNodeAdapter()` (`backend/src/platform/adapters/node/index.ts:31-46`), compose `NodeApp` (dùng `~/.orca` — `.../adapters/node/app.ts:28`), `NodeWindowManager`/`NodeWindow` (no-op UI), `NodeSecureStorage` (AES-256-GCM, `.../adapters/node/storage.ts:11,25-85`).
- **ElectronAdapter** — ❌ **không tồn tại trong code**. Comment trong `backend/src/platform/types.ts:5` mô tả "Implementations: ElectronAdapter (desktop) and NodeAdapter (server)" chỉ là aspirational. Thực tế `desktop/src/main/index.ts:8` import trực tiếp từ package `electron` thật, không qua interface trừu tượng nào. Các file `platform/stubs/electron-node-wrapper.ts` / `electron-web-stub.ts` làm chiều ngược lại (giả lập Electron API cho NodeAdapter/web), không phải "ElectronAdapter".
- **Kết luận:** Platform Abstraction Layer bất đối xứng — chỉ nhánh server (Node) thực sự đi qua interface; nhánh Electron Desktop dùng thẳng SDK gốc.

### 2.2 §4 — Web Server Bootstrap Flow

- `new NodeAdapter({userDataPath: ~/.orca})` + `setPlatform()` — ✅ `backend/src/server/index.ts:28-36`.
- `bootstrapWebApp(nodeAdapter)` — ❌ **tên hàm sai**. Không tồn tại trong `backend/src/server/index.ts`. Tên này thực ra thuộc về `frontend/src/renderer/src/web/main-web-bootstrap.tsx:253` (bootstrap phía **trình duyệt**, khác tầng hoàn toàn). Hàm backend thật là `initializeOrcaServices()` (`backend/src/server/index.ts:82-86`, import từ `../main/server-bootstrap`).
- `AuthManager.init(db)` / `SessionManager.init()` / `AgentWebSocketServer.init()` / `FleetHealthMonitor.start()` — ⚠️ tên method không khớp: `AuthManager` dùng constructor `new AuthManager(db, auditLogger)`, không có static `.init()`; `AgentWebSocketServer` dùng constructor rồi `.attach(httpServer)`; `FleetHealthMonitor` không tồn tại — chỉ có `FleetHealthStore` (in-memory store, không phải monitor loop).
- Express HTTP :6769 — ✅ khớp (`httpPort = rpcPort + 1`, mặc định 6769). 4 route con (`/auth/local`, `/admin/api/*`, `/health/ready`, `/health/metrics`) đều ✅ khớp.
- **WebSocket Server :6768 với 2 path `/` và `/agent`** — ❌ **sai port, phát hiện quan trọng nhất của mảng này**: `AgentWebSocketServer.attach(httpServer)` gắn vào **httpPort (6769)**, không phải rpcPort (6768). Bằng chứng: `backend/src/server/index.ts:106,108` (log `ws://0.0.0.0:${httpPort}/agent`) và comment ngay trong `backend/src/main/dev-server/agent-ws-server.ts:5-6`: *"Browser → ws://:6768/ (existing OrcaRuntimeRpcServer); Agent → ws://:6769/agent"*. `WsSessionRouter` (browser multi-user) cũng dùng httpPort 6769, chỉ kích hoạt khi `ORCA_MULTI_USER=1`; ở single-user mode mặc định, browser WS mới thật sự dùng port 6768 qua RPC server riêng.

### 2.3 §6.1 — Browser ↔ Backend

- `POST /auth/local` → `AuthManager.login()` → bcrypt.compare (`BCRYPT_ROUNDS=12`) → cookie `orca_session` HttpOnly, `sameSite:'strict'`, `maxAge:8h` — ✅ khớp hoàn toàn (`backend/src/main/auth/auth-router.ts:23-27,51`, `auth-user-store.ts:12,48-59`).
- `GET /admin/api/*` + `requireAdmin`, `GET /health/*` — ✅ khớp.
- WS upgrade + cookie → `WsSessionRouter` → Unix socket per-user → fork → JSON-RPC — ✅ khớp cơ chế (chỉ hoạt động khi `ORCA_MULTI_USER=1`).
- **"JSON-RPC 2.0 over binary WebSocket frames (13-byte header)"** — ⚠️ **overgeneralize/gây hiểu nhầm**. Header 13-byte là có thật (`HEADER_LENGTH=13`, `agent/src/main/ssh/relay-protocol.ts:14`) nhưng thuộc **Agent Wire Protocol** (kênh Backend↔Dev-Server/Agent), không phải kênh Browser↔Backend. Ở `WsSessionRouter`, dữ liệu là JSON-RPC text phân tách bằng `\n` + một số frame binary nhận diện bằng 1 byte đầu, không parse header 13-byte.

### 2.4 §6.2 — Backend ↔ Dev Server (3 modes)

Bằng chứng tại `DevServerRelayBridge.connect()` (`backend/src/main/dev-server/dev-server-relay-bridge.ts:139-207`):

- **Mode 1 `relay-ssh`** — ✅ khớp: `SshConnectionManager` → `client.exec()` (ssh2 thật) → `SshChannelMultiplexer`.
- **Mode 2 `relay-websocket`** — ✅ khớp phần lớn: token qua header `Authorization: Bearer <token>` đúng thiết kế. ⚠️ path `/orca-relay` chỉ là quy ước gợi ý trong thông báo lỗi, không phải route cố định — URL thật do `config.wsUrl` của từng Dev Server quyết định. Tên "WsTransport" trong thiết kế dễ nhầm vì có 2 khái niệm khác nhau cùng tên trong repo (`ws-transport.ts` cho relay agent vs `runtime/rpc/ws-transport.ts` cho RPC nội bộ).
- **Mode 3 `direct-websocket`** — ⚠️ khớp cơ chế (handshake `{agentToken}→{sessionId}`, token hash SHA-256 trước khi lưu — bảo mật tốt hơn thiết kế mô tả), nhưng **sai port**: thiết kế ghi `wss://backend:6768/agent`, thực tế là httpPort **6769** (xem 2.2).

### 2.5 §8 — Communication Matrix

| Dòng thiết kế | Kết luận |
|---|---|
| Browser→Orca HTTP: HTTPS :6769 | ⚠️ Port đúng nhưng server dùng `node:http` thuần — "HTTPS" chỉ đúng nếu có reverse-proxy/TLS terminator phía trước |
| Browser→Orca WS: :6768/ | ⚠️ Chỉ đúng ở single-user mode; multi-user (`ORCA_MULTI_USER=1`) thực tế dùng 6769 |
| Dev Agent→Orca WS: :6768/agent | ❌ Sai port — thực tế là **6769**/agent |
| Orca→Dev Server SSH :22 | ✅ khớp |
| Orca→Dev Server WS agent:PORT/orca-relay | ✅ khớp khái niệm (path chỉ là quy ước) |
| Orca→Database SQL DSN qua IConnectionPool | ✅ hợp lý, nhất quán với code (`server/index.ts:59-73`) |

### 2.6 §5 — Domain Services

| Domain/Module | Kết luận | Ghi chú |
|---|---|---|
| Auth — AuthManager/auth-router | ⚠️ | bcrypt **đúng 12 rounds** (`auth-user-store.ts:18`); session TTL qua bảng `orca_sessions.expires_at` + cleanup timer 30 phút |
| Session — SessionManager + fork() | ✅ | Env `ORCA_USER_ID/ORCA_USER_DATA_PATH/ORCA_SOCKET_PATH`; chỉ khởi tạo khi `ORCA_MULTI_USER=1` |
| Fleet — FleetHealthMonitor | ⚠️ | Poll interval **đúng 60s** (`fleet-health-monitor.ts:8,34`), nhưng chỉ ghi nhận SSH connection status — KHÔNG thấy thu thập CPU/RAM/disk/latency như thiết kế |
| Profile — ProfileResolver | ✅ | 3-layer merge đúng; cache TTL **đúng 60s** |
| Project — ProjectService + ProjectServerRouter | ✅ | Binding devServerId + auto-route đúng |
| AI Provider — AIProviderService + ProviderCredentialRelay | ⚠️ | Metadata CRUD ✅; nhưng class **`ProviderCredentialRelay` không tồn tại** — logic nằm trong `AIProviderService.writeCredentialToDevServer()`. Server-bootstrap thực tế dùng `AIProviderService + ProviderResolver + ProviderHealthChecker` |
| Workflow — WorkflowOrchestrator + DAGBuilder | ✅ | Wave-based execution + template inheritance (`TemplateResolver`, `MAX_INHERIT_DEPTH=5`) đúng |
| Task Graph — TaskService + TaskAgentExecutor | ✅ | Đầy đủ cụm `TaskDAGValidator/TaskService/TaskGrantService/TaskAIPlanner/TaskAgentExecutor`; grant resolution BFS đúng |
| Credentials — WebCredentialStore | ✅ | AES-256-GCM đúng thuật toán, path per-user đúng |
| DB — IConnectionPool + adapters | ✅ | SQLite/MySQL/PostgreSQL đủ; TiDB = mysql2 + dialect flag đúng |
| RBAC — hasPermission() | ❌ | **Không tồn tại**. Có 2 cơ chế permission tách biệt: `resolveUserPermissions()` (`shared/rbac-types.ts:73-119`, merge allowlist server/project + agentTrust — khác hẳn "Role→resource→action") và `TaskGrantService.resolvePermission()` (RBAC riêng cho task graph, BFS ancestor) |

### 2.7 §6.5 — Database & §10 — DB Schema

- `ORCA_STORAGE_BACKEND` env — ❌ **không tồn tại trong code**. Lựa chọn json/sql thực tế dựa trên `loadDatabaseConfig()` trả về null hay không (`server-bootstrap.ts:219-266`).
- File JSON thật tên **`store.json`**, không phải `orca-data.json` như thiết kế.
- Tên class thật: `JsonFileStateRepository`/`SqlStateRepository` (không phải `JsonFileRepository`/`SqlRepository`).
- Health monitor ping DB **đúng 30s** nhưng "auto-reconnect" — ❌ chưa thấy code hiện thực (chỉ emit trạng thái unhealthy/degraded).
- **Migration runner: thực tế có 13 file (0001→0013), không dừng ở 0010.**

| # | Thiết kế | Thực tế | Kết luận |
|---|---|---|---|
| 0001 | projects, worktrees, agent_sessions, settings | `settings, projects, repos, ssh_targets` | ❌ sai |
| 0002 | terminal_scrollback_snapshots | `automations` | ❌ hoàn toàn khác |
| 0003 | ssh_hosts, saved_port_forwards | `workspace_sessions` | ❌ hoàn toàn khác |
| 0004 | automations, automation_runs, notifications, rate_limits | `orca_projects, orca_repos, orca_ssh_targets, orca_global_settings` | ❌ hoàn toàn khác |
| 0005 | orca_users, orca_sessions, orca_audit_log, orca_access_policies | khớp chính xác | ✅ |
| 0006 | orca_company, orca_departments | `orca_companies` (số nhiều) + `orca_departments` + `orca_user_profiles` (thêm) | ⚠️ |
| 0007 | orca_projects, orca_project_members | `orca_v5_projects, orca_v5_project_members` (đổi tên tránh đụng độ với 0004) | ❌ |
| 0008 | orca_ai_provider_accounts, orca_provider_usage | khớp chính xác | ✅ |
| 0009 | orca_workflow_templates/executions/step_executions | khớp gần đúng (tên bảng thứ 3 có thêm "workflow_") | ⚠️ |
| 0010 | orca_tasks/task_edges/task_grants/task_comments | khớp + thêm `orca_team_members` | ✅ |
| 0011–0013 | *(không có trong thiết kế)* | terminal_sessions, port_forwards, push_subscriptions, workflow trace correlation | ❌ tài liệu thiếu hoàn toàn |

**Đây là sai lệch nghiêm trọng nhất trong toàn bộ review** — phần "DB Schema Overview" của tài liệu cần viết lại toàn bộ để khớp với migration hiện tại.

### 2.8 §7 — Per-User Session Isolation

| Chi tiết | Kết luận |
|---|---|
| `POST /auth/local` set cookie | ✅ |
| WS → `WsSessionRouter` → validate cookie → userId | ✅ |
| `SessionManager.getOrCreate(userId)` | ⚠️ tên hàm thật là `getOrSpawnUserProcess()` — logic đúng |
| fork() | ✅ |
| Path `~/.orca/users/<userId>/orca.sock` | ✅ |
| Timeout idle 4h | ✅ `DEFAULT_IDLE_TIMEOUT_MS = 4*60*60*1000`, sweep mỗi 5 phút |
| Max respawns: 3 | ⚠️ hằng số `DEFAULT_MAX_RESPAWN=3` tồn tại nhưng **không thấy logic thực thi respawn** khi child crash — `child.on('exit')` chỉ dọn dẹp, không spawn lại |
| RPC isolated theo process | ✅ |

### 2.9 §7.1 — Dev Server Provider Registry qua IPC

✅ **Khớp hoàn toàn với thiết kế**, kể cả cơ chế mới `devServer:proxyNotification` (2026-08):
- `GatewayDevServerManagerProxy` forward RPC qua `process.send` — ✅ (`backend/src/main/dev-server/gateway-proxy.ts:29-182`)
- `devServer:event` (added/removed/statusChanged) — ✅ broadcast qua `this.processes`
- `devServer:proxyNotification` — ✅ nguồn phát đúng, broadcast cùng cơ chế, child lọc theo `devServerId` đúng

### 2.10 §9 — RPC Method Namespaces

**Sai lệch nghiêm trọng và có hệ thống** — tài liệu dùng namespace số nhiều/gạch nối, code thực tế dùng số ít/camelCase:

| Namespace (doc) | Namespace thực tế | Số method (doc → thực tế) | Vấn đề |
|---|---|---|---|
| `profile.*` | `profile.*` | 8 → 10 | `getEffective` không tồn tại, thực tế là `getResolved` |
| `projects.*` | `project.*` | 9 → 10 | `updateBinding` không tồn tại |
| `ai-providers.*` | `aiProvider.*` | 5 → 9 | `add`→thực tế `create`; `rotateKey` không tồn tại |
| `workflows.*` | `workflow.*` | 7 → 7 | Số khớp nhưng tên sai gần hết: không có `run/pause/resume/streamStepOutput`, thực tế `execute/cancel`, không có pause/resume/streaming riêng |
| `tasks.*` | `task.*` | 11 → 18 | `addDependency`→`addEdge`; `aiPlan`→tách `aiDecompose`+`aiApply`; `runAgent`→`execute` |
| `credentials.*` | `credentials.*` | 4 → 4 | ✅ khớp hoàn toàn |
| `preflight.*` | `preflight.*` | 1 → 5 | Thêm 4 method không tài liệu hoá |
| `git.*` | `git.*` | ~10 → 35 | Không có sub-namespace lồng `git.branch.*`, thực tế phẳng (`git.branchCompare`...) |
| `fs.*` | *(không có)* → `files.*` (28 methods) | — | Tài liệu nhầm lớp: `fs.*` chỉ là giao thức nội bộ backend→dev-agent, API client thật là `files.*` |
| `pty.*` | *(không có)* → `terminal.*` (30 methods) | — | Tương tự — `pty.*` là giao thức nội bộ, client API thật là `terminal.*` |

Chỉ `credentials.*` khớp 1:1 hoàn toàn.

### 2.11 §6.3 — Backend → AI Agent CLIs (PTY)

- **AgentOrchestrator / ProfileAwareAgentSpawner** — ⚠️ một phần. Không có symbol tên `AgentOrchestrator`. Có `ProfileAwareAgentSpawner` (`backend/src/main/project/ProfileAwareAgentSpawner.ts:53-156`), nhưng `spawn()` **luôn** đi qua relay (`relay.call('agent.exec', ...)`, dòng 130) — không có nhánh gọi `node-pty.spawn` trực tiếp cho "local machine" như thiết kế mô tả.
- **node-pty.spawn(agentBinary, args, {cwd, env})** — ✅ có thật, nhưng nằm ở phía **Dev Server Agent** (`agent/src/relay/agent-spawner.ts:92-111`, `PTY_REGISTRY` dòng 84, `resolveAgentSpec()` dòng 153 map model→binary claude/codex/gemini/…) — khớp đúng nhánh "spawn trên Dev Server → relay pty.spawn". Nhánh local-machine trực tiếp gọi node-pty cho AI-agent-exec **không thấy bằng chứng** trong `backend/src/main` (chỉ có local PTY cho terminal thường: `providers/local-pty-provider.ts`, `daemon/pty-subprocess.ts`).
- **AgentAwakeService** — ⚠️ khác bản chất thiết kế mô tả. Tồn tại ở `desktop/src/main/agent-awake-service.ts:42` (không có bản backend riêng), nhưng đây là service quản lý *power-save-blocker* (giữ máy không ngủ khi agent chạy), nhận `AgentAwakeStatus.state==='working'` từ bên ngoài — **không tự parse OSC**, không phải service điều khiển state machine như thiết kế ngụ ý.
- **OSC escape sequence parsing** — ✅ có, nhưng ở module khác: `backend/src/shared/terminal-title-status.ts` (`detectAgentStatusFromTitle()`, dòng 138-229) và `agent-title-core.ts` parse title/OSC glyphs (Claude ✳, Gemini ✦/◇/✋, braille spinner...).
- **State: idle → running → waiting → completed** — ❌ sai lệch. Có 2 state machine thực tế, không machine nào khớp đúng thiết kế:
  - `AgentStatus = 'working' | 'permission' | 'idle'` (`agent-title-core.ts:12`) — 3 state.
  - `AgentLifecycleState = 'idle' | 'spawning' | 'running' | 'stopping' | 'stopped' | 'error'` (`agent/src/relay/agent-spawner.ts:46`) — 6 state.

### 2.12 §6.4 — Backend → Git Platforms

- **Đủ 6 provider** — ✅ `github/`, `gitlab/`, `linear/`, `jira/`, `bitbucket/`, `azure-devops/` đều tồn tại trong `backend/src/main/`.
- **GitHub/GitLab → HTTPS REST/GraphQL** — ⚠️ thực chất **shell ra CLI** (`gh`/`glab`), không phải HTTP client thuần: `runGraphql()` (`backend/src/main/github/project-view/internals.ts:265-333`) build argv cho `gh api graphql`, có `rateLimitGuard('graphql')` (đúng phần "rate limit handling"); GitLab tương tự qua `glab api` (`backend/src/main/gitlab/glab-api-response.ts`).
- **Linear → HTTPS REST** — ✅ khớp: `new LinearClient({apiKey: token})` SDK chính thức (`backend/src/main/linear/client.ts:66-70`).
- **Jira → HTTPS Basic Auth** — ✅ khớp chính xác: `Basic base64(email:apiToken)` (`backend/src/main/jira/client.ts:323-325`).
- **Bitbucket → HTTPS App Password** — ✅ khớp (Basic Auth email:apiToken khi không có access token), `backend/src/main/bitbucket/client.ts:56-65`.
- **Azure DevOps → HTTPS PAT token** — ✅ khớp: `Basic base64(username:pat)` (`backend/src/main/azure-devops/azure-devops-api-request.ts:43-52`).
- **WebCredentialStore AES-256-GCM cho per-user token** — ⚠️ đúng thuật toán (`web-credential-store.ts:78`) nhưng **KHÔNG bao phủ github/gitlab** — `CredentialService` chỉ gồm `'bitbucket'|'azure-devops'|'gitea'|'linear'|'jira'` (dòng 13-18); GitHub/GitLab dựa vào OS keychain của `gh`/`glab` CLI.
- **Preflight proxy Category A (gh/glab) → relay Dev Server** — ✅ khớp: `PreflightStatus` phân biệt `gh`/`glab` (`backend/src/main/ipc/preflight.ts:30-53`), `detectRemoteAgents()` forward qua `mux.request('preflight.detectAgents', ...)`.

### 2.12b — Bổ sung (2026-08-08): Backend có tự thực thi GitHub/GitLab, hay chỉ relay đến Dev Server Agent?

**Câu hỏi đặt ra:** theo đúng kiến trúc Gateway/Dev-Server-Fleet, Backend (Gateway) không nên tự thực thi lệnh `gh`/`glab` — chỉ nên chuyển tiếp (relay) yêu cầu từ frontend đến Dev Server Agent để agent thực thi, vì token/CLI-auth phải nằm trên Dev Server, không phải trên Gateway. Đã rà soát lại toàn bộ `backend/`, `agent/`, `desktop/` và `docs/hld/*` để xác minh.

**1) Tài liệu thiết kế xác nhận rõ ràng — đây LÀ ý định thiết kế đúng**, nằm ở `docs/hld/dev-server-architecture.md` (không phải `backend-server-architecture.md`, tài liệu đó chỉ nói mơ hồ "GitHub Client → HTTPS REST/GraphQL" không nêu rõ chạy ở đâu):
- Dòng 28: `External API Caller | Gọi GitHub/GitLab API từ Dev Server qua gh/glab CLI với per-user auth isolation` — vai trò này được gán cho **Dev Server**, không phải Backend.
- Dòng 442: *"Dev Server gọi External APIs thông qua CLI tools (gh, glab)... Đây là External API Connector layer — độc lập khỏi Gateway, không expose credentials qua network."*
- Dòng 463-471, 498-505: bảng RPC method `git.pr.create`, `git.pr.view`, `git.pr.merge`, `git.issue.list`, `git.issue.create`, `git.repo.clone`, `git.mr.create`, `git.mr.view`, `git.mr.list`, `git.pipeline.status`, `preflight.check` — tất cả chạy **trên Dev Server**, gọi qua `gh`/`glab` CLI với `GH_CONFIG_DIR=~/.config/gh/<userId>/` và `GLAB_CONFIG_DIR=~/.config/glab-cli/<userId>/` riêng từng user (dòng 338-339, 451-453, 489-490).
- Dòng 540: `Gateway → relay.call('preflight.check', { userId })` — minh hoạ đúng pattern "Gateway chỉ relay".
- Dòng 568, 575: nguyên tắc rõ ràng — *"CLI-based, not SDK"*, và quan trọng nhất: **"Auth never through Gateway | GitHub/GitLab tokens chỉ nằm trên Dev Server filesystem."**

**2) Phía Dev Server Agent (`agent/`) ĐÃ implement đúng thiết kế này:**
- `agent/src/relay/agent-git-handler.ts:270-322` — implement `git.pr.create` (comment ngay đầu: *"git.pr.create (via gh CLI)"*), dispatch qua `agent/src/relay/agent-rpc-dispatch.ts:410-417` (`case 'git.pr.create':`).
- `agent/src/relay/external-api-connector.ts:1-13` — comment thiết kế trùng khớp 100% với tài liệu: *"CLI-based: gh/glab — NOT SDK; Per-user isolation: GH_CONFIG_DIR/GLAB_CONFIG_DIR per userId; Auth never through Gateway: tokens stay on dev server filesystem"*; dùng `spawn(binary, args, {shell: false})` (không phải `ghExecFileAsync` của backend).
- (`desktop/src/relay/agent-git-handler.ts` và `desktop/src/relay/external-api-connector.ts` có bản sao y hệt — dùng khi Desktop tự đóng vai "agent" cho SSH Target riêng của nó.)

**3) Nhưng phía Backend (`backend/src/main`) — nơi thực sự phục vụ RPC `github.*`/`gitlab.*` cho frontend — KHÔNG hề gọi các method `git.pr.create`/`git.issue.list`/`git.mr.*` này:**
- Đã grep toàn bộ repo (loại trừ `node_modules`) cho các literal `git.pr.`, `git.issue.`, `git.mr.`, `git.repo.clone`, `git.pipeline.` — **chỉ xuất hiện ở nơi định nghĩa handler** (`agent/src/relay/agent-git-handler.ts`, `agent/src/relay/agent-rpc-dispatch.ts` và bản sao ở `desktop/`). **Không có bất kỳ nơi nào trong `backend/`, `frontend/`, `mobile/` gửi các RPC method này.** → Toàn bộ implementation phía Agent cho GitHub/GitLab CLI (đúng thiết kế, có per-user isolation) hiện là **dead code / chưa được wire**, ít nhất là từ phía Backend Web Server.
- Đã grep toàn bộ `backend/src/main` cho `relay.call(` và `mux.request(` (namespace `git.*`/`github.*`/`gitlab.*`) — chỉ tìm thấy `git.exec`/`git.clone`/`git.status`/`git.history`/`git.commit`/`git.listWorktrees`/`agent.exec`/`shell.exec`/`ai.complete`/`preflight.detectAgents`/`session.*`/`ports.detect` — **không có một relay call nào cho nghiệp vụ GitHub/GitLab (issue/PR/MR)**.
- Luồng thực tế mà RPC `github.*`/`gitlab.*` của Backend đang chạy: `backend/src/main/runtime/rpc/methods/github.ts` (handler `handler: async (params, {runtime}) => runtime.listRepoIssues(...)`, dòng 316 và tương tự cho các method khác) → `OrcaRuntimeService` tại `backend/src/main/runtime/orca-runtime.ts:12515` (`listRepoIssues`) → `listIssues` re-export từ `backend/src/main/github/issues.ts:100` → `ghExecFileAsync(...)` tại `backend/src/main/github/gh-utils.ts:3` (re-export từ `../git/runner`) → **`node:child_process.execFile('gh', ...)` chạy NGAY TRONG PROCESS CỦA BACKEND** (`backend/src/main/git/runner.ts`).
- Bằng chứng then chốt cho việc "chạy cục bộ trên Backend, không đi qua Dev Server": hàm `ghRepoExecOptions()` tại `backend/src/main/github/github-repository-identity.ts:39-50` — khi repo có `connectionId` (SSH Target), trả về `{}` (không set `cwd`), nghĩa là lệnh `gh` chạy **không cwd, dựa vào `--repo owner/repo`**, thực thi ngay trên host Backend chứ không được đẩy qua kênh SSH/relay nào. Chỉ có việc lấy `git remote get-url` (qua `getRemoteUrlForRepo`, dòng 91-108) là được relay qua `getRemoteGitProvider(connectionId).exec(...)` — tức **chỉ phần git plumbing thuần được relay, còn lệnh `gh`/`glab` gọi GitHub/GitLab API thì luôn chạy tại chỗ trên Backend.**
- Tương tự hoàn toàn với GitLab: `backend/src/main/gitlab/*.ts` dùng `glab` CLI qua cùng cơ chế `ghExecFileAsync`/`git/runner.ts`, cùng một điểm yếu.
- **Không có `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` nào được set trong toàn bộ `backend/src/main`** (đã grep xác nhận 0 kết quả) — nghĩa là trong Web Server multi-user mode, `gh`/`glab` trên host Backend dùng **chung một auth context cho mọi user**, vi phạm trực tiếp nguyên tắc "per-user auth isolation" và "Auth never through Gateway" của `dev-server-architecture.md`.

**Kết luận mục 2.12b — đây là sai lệch nghiêm trọng nhất được phát hiện thêm, thuộc về CODE (không phải lỗi tài liệu):**
> Nhận định của người dùng là đúng theo *thiết kế* (`dev-server-architecture.md §12`): Backend/Gateway không nên tự thực thi GitHub/GitLab, chỉ nên relay đến Dev Server Agent. Nhưng **code thực tế làm ngược lại** — Backend tự thực thi `gh`/`glab` ngay trong process của mình (không per-user isolation, không relay), trong khi phần implementation đúng thiết kế đã có sẵn ở phía Agent (`agent/src/relay/agent-git-handler.ts` + `external-api-connector.ts`) nhưng **hoàn toàn không được gọi tới** — có nguy cơ là code chết hoặc một luồng tích hợp dở dang, chưa migrate hết từ kiến trúc "Desktop monolith" (dùng chung `OrcaRuntimeService`) sang kiến trúc "Gateway ⇄ Dev Server Fleet" mới hơn.
>
> **Rủi ro thực tế:** (a) trong Web Server multi-user mode, mọi user share chung 1 gh/glab session trên host Backend — rò rỉ dữ liệu/nhầm quyền giữa các user; (b) Backend host bắt buộc phải cài + đăng nhập sẵn `gh`/`glab` — trái với mô hình "Gateway không giữ token" của thiết kế; (c) hai bộ code (Backend's `github/issues.ts` và Agent's `agent-git-handler.ts`) sẽ tiếp tục lệch nhau (feature drift) nếu không hợp nhất.

**File/dòng bằng chứng chính (đường dẫn tuyệt đối `/opt/repos/orca`):**
- `docs/hld/dev-server-architecture.md:28,338-339,442,451-453,463-471,489-505,540,568,575`
- `agent/src/relay/agent-git-handler.ts:270-322`
- `agent/src/relay/agent-rpc-dispatch.ts:410-417`
- `agent/src/relay/external-api-connector.ts:1-13`
- `backend/src/main/runtime/rpc/methods/github.ts:313-316`
- `backend/src/main/runtime/orca-runtime.ts:12515-12526`
- `backend/src/main/github/issues.ts:100,118`
- `backend/src/main/github/gh-utils.ts:3,8`
- `backend/src/main/git/runner.ts` (spawn `gh`/`git` cục bộ, WSL-aware, không có nhánh relay)
- `backend/src/main/github/github-repository-identity.ts:39-50,91-108`
- `backend/src/main/gitlab/issues.ts`, `gl-utils.ts` (cùng mẫu hình với GitHub)

### 2.13 §6.6 — Backend → Mobile App & §6.7 — Backend → Orca CLI

**Mobile (§6.6):**
- QR pairing `{pubKey, host, port, token}` — ⚠️ cơ chế đúng nhưng **tên field khác hẳn**: schema thật `PairingOfferSchema` (`backend/src/shared/pairing.ts:6-18`) là `{v, endpoint, deviceToken, publicKeyB64, scope?}` — `host/port` gộp vào `endpoint`, `token`→`deviceToken`, `pubKey`→`publicKeyB64`.
- TweetNaCl key exchange → shared secret — ✅ đúng (Curve25519/NaCl box, `mobile/src/transport/e2ee.ts:65`).
- WebSocket E2E encrypted TweetNaCl box — ✅ khớp cơ chế (`agent/src/shared/remote-runtime-request-connection.ts:147-190`, handshake `e2ee_auth` → encrypt/decrypt mọi frame).
- Message type `'agent:completed'` / `'dispatch'` → inject PTY stdin — ❌ **không tìm thấy literal string này** trong code đã khảo sát (có `NotificationEvent` và cơ chế inject input vào PTY nói chung qua `Session.write()`, nhưng chưa xác nhận đúng tên message như tài liệu).

**CLI/Daemon (§6.7):** ✅ khớp hoàn toàn.
- Unix Socket → `DaemonServer` (`backend/src/main/daemon/daemon-server.ts`, khởi qua `startDaemon()` tại `daemon-main.ts:15-28`).
- NDJSON protocol — ✅ chính xác: `encodeNdjson(msg) = JSON.stringify(msg)+'\n'` (`daemon/ndjson.ts:1-3`), giới hạn `NDJSON_MAX_LINE_BYTES=16MB`.
- Headless mode — ✅ daemon chạy như tiến trình Node độc lập, không phụ thuộc Electron renderer.
- Reattach khôi phục PTY — ✅ `Session.getSnapshot()`/`takePendingOutput()` (`daemon/session.ts:309,338-363`) + RPC `createOrAttach`.

---

## 3. Nhận định tổng quan

1. **Tài liệu HLD mô tả đúng ý tưởng kiến trúc** (control plane, platform abstraction, 3-mode relay, per-user session isolation, RBAC, RPC) — cấu trúc tổng thể phản ánh đúng hướng thiết kế thực tế của code.
2. **Nhưng có 2 loại sai lệch lặp lại nhiều lần:**
   - **Sai chi tiết định danh** (tên hàm/class/namespace/port/tên bảng/tên biến env) — code đã đổi tên qua các lần refactor nhưng tài liệu chưa cập nhật theo. Ví dụ: `getEffective`→`getResolved`, `bootstrapWebApp`→`initializeOrcaServices`, port 6768/6769 bị đảo, `orca-data.json`→`store.json`.
   - **Tài liệu lạc hậu so với tốc độ phát triển code** — rõ nhất ở DB Schema (10 vs 13 migration, tên bảng 0001-0004/0007 sai gần hết) và RPC namespaces (undercount method đáng kể ở hầu hết namespace).
3. **Một khoảng trống thiết kế thực sự** (không chỉ lệch tên) là RBAC: tài liệu mô tả `hasPermission()` như một policy table thống nhất Role→resource→action, nhưng code hiện có **2 cơ chế permission tách biệt, không liên kết** (`resolveUserPermissions` cho fleet/server access, `TaskGrantService.resolvePermission` cho task graph) — đây là rủi ro về tính nhất quán bảo mật cần lưu ý, không chỉ là vấn đề tài liệu.
4. **Platform Abstraction Layer bất đối xứng** cũng là một khoảng trống kiến trúc thật: nhánh Electron Desktop không đi qua `IPlatformServices` như thiết kế promise, mà dùng thẳng SDK `electron` — nghĩa là lời hứa "swap adapter mà không đổi business logic" (mục 1 tài liệu) chưa được hiện thực đầy đủ ở chiều Electron.
5. **Nghiêm trọng nhất (bổ sung 2026-08-08, xem §2.12b): Backend tự thực thi GitHub/GitLab thay vì relay đến Dev Server Agent.** Theo `docs/hld/dev-server-architecture.md §12`, Backend/Gateway **không được** tự chạy `gh`/`glab` hay giữ token — vai trò "External API Caller" thuộc về Dev Server, với per-user `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` isolation. Phía Agent (`agent/src/relay/agent-git-handler.ts`, `external-api-connector.ts`) đã implement đúng thiết kế này (`git.pr.create`, `git.issue.list`...), **nhưng không hề được Backend gọi tới** — 100% dead code từ góc nhìn Backend. Backend hiện tự chạy `gh`/`glab` ngay trong process của mình (`backend/src/main/github/*`, `gitlab/*` → `ghExecFileAsync` → `child_process.execFile`), không per-user isolation, không relay — vi phạm trực tiếp nguyên tắc bảo mật cốt lõi "Auth never through Gateway" của thiết kế. Đây là lỗi **thuộc về code** (thiết kế đúng, implementation sai/dở dang), không phải lỗi tài liệu.

## 4. Khuyến nghị

- **Cập nhật tài liệu ngay:** §10 DB Schema Overview (viết lại hoàn toàn theo 13 migration thật), §9 RPC namespaces (đối chiếu lại toàn bộ danh sách method), §4/§8 port Agent WS (6769 không phải 6768).
- **Xem xét nợ kiến trúc:** hợp nhất 2 cơ chế RBAC thành một, hoặc làm rõ trong tài liệu rằng đây là 2 hệ thống permission có phạm vi khác nhau theo chủ ý.
- **Làm rõ Electron Adapter:** hoặc triển khai `ElectronAdapter` thật theo đúng lời hứa của Platform Abstraction Layer, hoặc cập nhật tài liệu để phản ánh đúng rằng Electron Desktop dùng trực tiếp SDK gốc.
- **Bổ sung logic còn thiếu:** auto-respawn khi session process crash (hằng số có nhưng chưa dùng), auto-reconnect DB sau health-check fail, thu thập CPU/RAM/disk/latency trong FleetHealthMonitor (hiện chỉ có SSH connection status), state machine agent 4-state thống nhất (`idle→running→waiting→completed`) thay cho 2 state machine rời rạc hiện có (`AgentStatus` 3-state và `AgentLifecycleState` 6-state).
- **Đồng bộ tên field pairing mobile** (`pubKey/host/port/token` trong tài liệu vs `publicKeyB64/endpoint/deviceToken` trong code) và xác nhận/đặt tên rõ message type push/dispatch giữa backend↔mobile.
- **Làm rõ phạm vi WebCredentialStore**: nêu rõ trong tài liệu rằng GitHub/GitLab dùng OS keychain của `gh`/`glab` CLI (không qua AES-256-GCM store), khác với Bitbucket/Azure DevOps/Jira/Linear/Gitea.
- **Ưu tiên cao nhất — wire GitHub/GitLab qua Dev Server Agent:** thay `backend/src/main/runtime/rpc/methods/github.ts`/`gitlab.ts` (qua `OrcaRuntimeService`) bằng relay `relay.call('git.pr.create'|'git.issue.list'|...)` tới implementation đã có sẵn ở `agent/src/relay/agent-git-handler.ts` + `external-api-connector.ts` (per-user `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR`), đúng như `dev-server-architecture.md §12` đã đặc tả — hoặc nếu chủ ý giữ đường chạy cục bộ cho Desktop/SSH-Target, phải tài liệu hoá rõ ràng 2 luồng khác nhau (Desktop monolith vs Gateway-Fleet) và bổ sung per-user isolation cho nhánh Web Server multi-user.

---

## 5. Vòng 2 — Đối chiếu chi tiết theo Feature Docs (F22–F40)

Vòng 1 chỉ đối chiếu với tài liệu tóm tắt `backend-server-architecture.md`. Vòng 2 đọc từng feature doc chi tiết hơn (F22–F40, phần backend-relevant) và đối chiếu trực tiếp với code — phát hiện nhiều **bug thật** (không liên quan gì đến ADR v2 "Proposed") mà bản tóm tắt HLD không đủ chi tiết để lộ ra.

### 5.0 Bảng tổng kết Vòng 2

| Feature | Trạng thái | Vấn đề nghiêm trọng nhất |
|---|---|---|
| F22 Web Server Mode | ✅ Khớp phần lớn | Sơ đồ 2-port chưa mô tả rõ hành vi khi `ORCA_MULTI_USER=1` (browser WS thực chạy qua port HTTP 6769) |
| F23 Multi-User Auth | ⚠️ Một phần | `/auth/me` thiếu `name`/`provider` (giới hạn kiến trúc `OrcaSession`); cookie `SameSite=strict` thay vì `Lax`, `Secure` chỉ bật khi production |
| F24 Per-User Sandbox | ❌ 2 tiêu chí "đã xong" thực ra chưa làm | **Auto-respawn khi crash (max 3) và idle-timeout qua `SESSION_IDLE_TIMEOUT_MS` đều chưa cài đặt** dù đánh dấu hoàn thành |
| F25 Admin Panel | ❌ 2 tiêu chí "đã xong" thực ra chưa làm | **`GET /admin/api/sessions` là stub rỗng**; **toàn bộ backend API cho Access Policies (PoliciesPage) không tồn tại** dù DB schema đã sẵn |
| F26 Multi-Database | ✅ Khớp | Chỉ lệch nhẹ: tài liệu ghi 5 migration, thực tế 13 |
| F27 Fleet Health Monitoring | ❌ Xác nhận lại | Không CPU/RAM/disk/latency thật — khái niệm này không tồn tại ở bất kỳ đâu trong data model, chỉ poll SSH connection status |
| F28 Dev Server Onboarding | ⚠️ Một phần | Chức năng có nhưng tên hàm/class/endpoint sai khác nhiều (`detectAgentOnRemote`, `FleetBootstrapService`, `/api/push`) |
| F29 Agent WebSocket Protocol | ❌ Nhiều chi tiết sai | Keepalive 30s/90s(3-miss) → thực tế 5s/20s; close code 4001-4003 → thực tế WS 1008/1005; `AGENT_MIN_VERSION` là hằng số chết (version-mismatch check chưa cài) |
| F30 Remote Integrations | ❌ Category A tự mâu thuẫn thiết kế | GitHub/GitLab hầu hết chạy trực tiếp trên Backend (không chỉ PR-create); `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` không bao giờ được truyền dù 2 luồng relay hẹp có tồn tại; env var đúng là `ORCA_SERVER_SECRET` (doc ghi sai `ORCA_CREDENTIAL_KEY`) |
| F31 Fleet Provisioning | ❌ CLI không tồn tại | `orca fleet provision` (CR-003) hoàn toàn chưa implement; bootstrap thiếu disk-check + SHA256 verify |
| F32 Team RBAC | ❌ Bug bảo mật thật | `requireAdmin(ctx)` trong `profile-rpc-handler.ts` là stub không check role — bypass hoàn toàn; không có policy table role×resource×action nào, RBAC phân mảnh 4 cơ chế |
| F33 User Profile Hierarchy | ❌ Schema lệch nhiều | `security`/`editor` section field-set sai gần hết; deep-merge "array union" vi phạm (concat không dedup); kế thừa lỗ hổng RBAC ở F32 |
| F34 Project-Dev Server Binding | ❌ Thiếu rebind + RBAC sai | Không rebind được devServerId sau khi tạo; `requireOwnerOrAdmin` không hề check admin (dead code trong tên hàm) |
| F35 AI Provider Account Mgmt | ❌ 2 tính năng "đã có" thực ra thiếu | Key rotation (grace period, status 'rotating', audit log) hoàn toàn không tồn tại; quota-80%-alert không tồn tại |
| F36 Multi-Server Workflow | ❌ Nhiều gap lớn | **Provider selection theo step: 0% code** dù có ở tài liệu; pause/resume không tồn tại (chỉ có crash-recovery resume); dispatch `fleet:tag:` chưa implement; không streaming step output |
| F37 Task Graph Management | ⚠️ Khớp khá tốt | Cycle detection + grant BFS đúng thuật toán; nhưng progress tracking không rollup từ subtask thật, AI decomposition thiếu dependencies/criticalPath |
| F38 Project Workspace | ⚠️ Backend tốt, FE có bug | Backend/RPC khớp thiết kế tốt; nhưng tab "Agent" ở frontend không render nội dung (dead tab) |
| F39 Remote Git UI | ❌ Dev-Server repo bị giới hạn | `DevServerGitProvider` ném "not supported" cho git log, AI commit message, branch/commit diff — nhiều tiêu chí F39 không hoạt động cho repo trên Dev Server |
| F40 Full-Flow Tracing | ✅ ~95% khớp | Điểm khớp tốt nhất toàn bộ audit — migration 0013 chính là cơ chế trace-correlation-qua-restart cho Workflow, đúng tinh thần thiết kế |

### 5.1 F22 — Web Server Mode
- ✅ Port `ORCA_PORT=6768` (WS/RPC), `ORCA_HTTP_PORT=6769` (HTTP) — `backend/src/server/index.ts:46-47`. HTTP server phục vụ SPA+health+auth+admin trên cùng 1 `http.Server` — `backend/src/server/http-server.ts:84-169,204-238`.
- ⚠️ Khi `ORCA_MULTI_USER=1`, browser WS thực tế được `WsSessionRouter` chặn ngay trên `'upgrade'` của **port 6769** (không phải 6768) — comment code: *"browsers connect to port httpPort (6769) using session cookie auth"* (`backend/src/server/index.ts:149-171`). F22 không mô tả rõ tương tác này với F24.
- ⚠️ Bảng "Yêu cầu kỹ thuật" của F22 trộn path frontend/backend (`web-preload-api.ts`, `main-web-bootstrap.tsx` nằm ở `frontend/`, không phải `backend/`) — do doc viết trước khi repo tách package.

### 5.2 F23 — Multi-User Auth
- ✅ `POST /auth/local`, `POST /auth/logout`, cleanup session 30 phút, `session_id` 64-hex random, bcrypt 12 rounds — đều khớp (`backend/src/main/auth/auth-router.ts`, `auth-manager.ts:20`, `auth-session-store.ts:20`, `auth-user-store.ts:17,30`).
- ⚠️ Cookie: doc ghi `SameSite=Lax` + luôn `Secure`; code dùng `sameSite:'strict'` và `secure` chỉ bật khi `NODE_ENV==='production'` (`auth-router.ts:21-27`).
- ❌ `GET /auth/me` thiếu `name`/`provider` mà doc yêu cầu — nguyên nhân gốc: `OrcaSession` không lưu 2 field này (`auth-types.ts:16-27`), giới hạn kiến trúc chứ không phải chỉ sửa response là xong.
- ⚠️ Role thực tế có thêm `'lead'` ngoài `admin`/`developer` mà F23 liệt kê (`admin-user-handlers.ts:16`); cột `role` không có CHECK constraint.

### 5.3 F24 — Per-User Sandbox
- ✅ `fork()` theo user, env `ORCA_USER_ID`/`ORCA_SOCKET_PATH`, idempotent `getOrSpawnUserProcess`, spawn timeout 30s, idle-shutdown mặc định 4h quét mỗi 5 phút, thư mục `users/<userId>/` quyền `0o700`, `SessionManager.shutdown()` SIGTERM toàn bộ — tất cả khớp (`session-manager.ts`).
- ❌ **`SESSION_IDLE_TIMEOUT_MS` (cấu hình idle timeout qua env) không được implement** — grep 0 kết quả; `SessionManager` khởi tạo ở cả 2 nơi (`server-bootstrap.ts:311-316`, `server/index.ts:137-142`) không truyền `idleTimeoutMs` nên luôn cứng 4h.
- ❌ **Auto-respawn khi crash (max 3 lần) không được implement** — field `respawnCount`/`maxRespawnAttempts` tồn tại nhưng không nơi nào đọc/tăng; `child.on('exit',...)` chỉ dọn dẹp, không spawn lại (`session-manager.ts:161-166`) — xác nhận sâu hơn phát hiện Vòng 1.
- ✅ Điểm cộng ngoài doc: giới hạn heap `--max-old-space-size=512` mỗi user process (`session-manager.ts:106`).

### 5.4 F25 — Admin Panel
- ✅ `requireAdmin` (401/403 đúng), mount `/admin/api`, `stats`, Users CRUD (+ deactivate kill session), `DELETE sessions/:id`, Audit log filter đầy đủ, first-run admin setup — khớp.
- ❌ **`GET /admin/api/sessions` (list toàn bộ session) là stub rỗng**: luôn trả `{sessions:[], total:0, note:'Full listing not yet implemented'}` (`admin-session-handlers.ts:18-22`) — chỉ chức năng "kill" hoạt động thật, "xem" thì không, dù F25 liệt kê SessionsPage là đã Phát hành.
- ❌ **Toàn bộ backend API cho Access Policies (PoliciesPage) không tồn tại** — bảng `orca_access_policies` + type `PolicyInput` đã sẵn sàng nhưng không route `/admin/api/policies*` nào, không file `admin-policy-handlers.ts` nào.
- ⚠️ `pairedDevices` trong admin stats hard-code = 0 (stub, comment "DeviceRegistry not available here").

### 5.5 F26 — Multi-Database
- ✅ Khớp đầy đủ: dialect `sqlite/mysql/postgresql/tidb(+mariadb)`, provider registry, repository pattern (`JsonFileStateRepository`/`SqlStateRepository`), health check 30s. Chỉ ⚠️ doc ghi "5 migrations", thực tế 13 (lỗi thời, không phải bug).

### 5.6 F27 — Fleet Health Monitoring
- ❌ Xác nhận chắc chắn (đọc toàn bộ `fleet-health-monitor.ts`, `fleet-health-store.ts`): `runHealthCheck()` chỉ đọc `SshConnectionStatus` đã có sẵn, không hề exec lệnh đo CPU/RAM/disk, không ping đo latency. Field `pingLatencyMs` tồn tại trong type `HealthRecord` nhưng **không bao giờ được ghi giá trị** (dead field). Khái niệm CPU/RAM/disk **hoàn toàn không tồn tại** trong bất kỳ type nào (`HealthRecord`, `FleetServerStatus`) hay Prometheus `/metrics` (chỉ có `orca_server_connected/uptime/reconnect_attempts/health_score`, không có `cpu_percent`).
- ⚠️ Webhook alert format khác doc (Slack-style `text`/`attachments` thay vì `{serverId,serverName,status,metrics,timestamp}`); status classification 4-mức `healthy/degraded/unhealthy/unreachable` chỉ tồn tại ở DB health, không tồn tại cho fleet.

### 5.7 F28 — Dev Server Onboarding
- ⚠️ Chức năng cốt lõi có nhưng tên sai khác nhiều: `FleetBootstrapService` (class) không tồn tại — chỉ có hàm đơn `bootstrapServer()`, không tách preflight/bootstrap; `detectAgentOnRemote()` không tồn tại, cơ chế thật là `resolveRemoteInstallState()`/`resolveRelayBootstrapState()`; `/api/push` thực tế là 2 route `/api/push-subscribe` + `/api/push-unsubscribe`.
- ✅ `WebPushManager` đầy đủ VAPID lifecycle; auto relay deployment có thật (`deployAndLaunchRelay`) dù launch `node relay.js` chứ không phải binary `orca-relay`.

### 5.8 F29 — Agent WebSocket Protocol
- ✅ Frame 13-byte header (TYPE/SEQ/ACK/LEN), TYPE 0x01/0x09, `agent.handshake`, path `/agent` tách biệt browser, token SHA-256 không lưu plaintext, các RPC method `preflight.check`/`pty.spawn`/`fs.readDir`/`git.exec` — đều khớp.
- ⚠️ "`handshake-ok`" không phải message type riêng — thực chất là JSON-RPC response thường `{result:{ok:true}}`.
- ❌ **Keepalive sai hoàn toàn về số liệu**: doc ghi gửi mỗi 30s, timeout sau 3 lần miss (90s); thực tế `KEEPALIVE_SEND_MS=5_000`, `TIMEOUT_MS=20_000` (`relay-protocol.ts:24-25`, `agent-wire-protocol.ts:21-22`), không có logic "3 lần miss" — ngắt ngay khi quá 20s không nhận data.
- ❌ **Close code 4001/4002/4003 không tồn tại** — code dùng mã WS chuẩn 1008 (token sai) hoặc 1005 mặc định (handshake timeout); grep 0 kết quả cho 4001-4003.
- ❌ **`AGENT_MIN_VERSION` là hằng số chết** — khai báo nhưng không đâu kiểm tra, nghĩa là version-mismatch detection (lý do có mã 4003) chưa từng được cài đặt.
- ⚠️ Phát hiện thêm ngoài doc: `relay-websocket` mode có reconnect exponential backoff đầy đủ (2s→60s cap, jitter); `direct-websocket` mode không có Orca-side reconnect (dựa vào agent tự reconnect qua systemd) — hành vi vận hành quan trọng F29 không hề nhắc tới.

### 5.9 F30 — Remote Integration Management
- ✅ Phân loại Category A (GitHub/GitLab CLI) vs B+C (WebCredentialStore) đúng danh sách provider. **Việc Jira/Bitbucket/Azure/Gitea/Linear gọi trực tiếp từ Backend LÀ đúng thiết kế của chính F30** (không phải sai lệch) — trả lời rõ câu hỏi đặt ra ở §2.12b: không phải MỌI integration đều nên qua Dev Server relay, chỉ Category A.
- ❌ **Nhưng Category A tự mâu thuẫn với chính thiết kế của nó**: mở rộng phát hiện §2.12b — không chỉ `git.pr.create`, mà **hầu hết mọi thao tác GitHub/GitLab nghiệp vụ hàng ngày** (list PR/issues, rate-limit, project-view, comments, work-item-details...) đều gọi thẳng `ghExecFileAsync`/`gitExecFileAsync` → `execFile` **ngay trong Backend process** — chỉ 2 luồng hẹp thật sự qua relay (`*.startAuthLogin` và `preflight.check` khi có `devServerId`).
- ❌ **`GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per-user isolation không hoạt động dù ở 2 luồng relay hẹp đó**: `relay.call('pty.spawn', {command:'gh', args, env:{}, ...})` — `env` rỗng, không truyền `userId` hay `GH_CONFIG_DIR`. Cơ chế isolation chỉ tồn tại phía Agent nhưng Backend không bao giờ kích hoạt nó.
- ❌ **Sai tên biến môi trường**: doc ghi `ORCA_CREDENTIAL_KEY`, biến thật là **`ORCA_SERVER_SECRET`** (`backend/src/main/credentials/index.ts:11,16`) — `ORCA_CREDENTIAL_KEY` không xuất hiện ở bất kỳ đâu trong code.
- ❌ **`mergePreflightStatuses` không tồn tại trong code** dù acceptance criteria đánh dấu hoàn thành — chỉ được nhắc ở tài liệu spec khác.
- ⚠️ `WebCredentialStore` mã hoá V2 khác doc: `salt` ngẫu nhiên 32-byte mỗi lần ghi (không derive từ `userId` như doc mô tả), `iv=16 byte` (không phải 12), có cơ chế migrate V1→V2 doc không nhắc.

### 5.10 F31 — Fleet Provisioning
- ✅ Fleet config parser (YAML↔SshTarget), `groupSshTargetsByProject()` — khớp đầy đủ, đúng logic.
- ❌ **CLI `orca fleet provision --project ... --concurrency N --dry-run` hoàn toàn không tồn tại** — không có `backend/src/cli/`, grep "fleet provision" 0 kết quả. Toàn bộ tính năng bulk provisioning với concurrency/dry-run chưa implement.
- ❌ `FleetBootstrapService.bootstrap()` (class) không tồn tại; hàm thật `bootstrapServer()` thiếu 2/7 bước: **disk-space check** (≥5GB) và **verify SHA256** của relay binary — cả hai hoàn toàn không có code. Cài relay/khởi động relay không nằm trong hàm này mà ở cơ chế riêng biệt không tích hợp.

### 5.11 F32 — Team RBAC
- ❌ **Không có policy table role×resource×action nào** — xác nhận chắc chắn, mở rộng phát hiện Vòng 1: RBAC bị phân mảnh thành **4 cơ chế không tương thích**: `resolveUserPermissions()` (fleet-level, union policy), `requireAdmin` HTTP-route middleware (nhị phân), `requireAdmin` RPC-handler (**stub lỗi**, xem dưới), `requireOwnerOrAdmin` project-level (**dead code**, xem F34).
- ❌ **Bug bảo mật thật, nghiêm trọng nhất toàn bộ audit**: `requireAdmin(ctx)` trong `backend/src/main/profile/profile-rpc-handler.ts:282-293` **chỉ kiểm tra `ctx.userId` tồn tại (đã login)**, KHÔNG kiểm tra `role==='admin'`. Comment code tự thừa nhận đây là TODO chưa làm ("Admin enforcement deferred to AuthManager middleware..."). Hệ quả: **bất kỳ user đã login nào** gọi được `profile.updateCompany`, `profile.updateDept`, `profile.createCompany`, `profile.createDept`, `profile.setUserDept` — set chính sách bảo mật toàn công ty mà không cần quyền admin. Vi phạm trực tiếp KPI "0 permission bypass (P0)" mà chính F32 đặt ra.
- ✅ `orca_audit_log` + `AuditLogger`/`AdminAuditHandlers` khớp thiết kế.
- ⚠️ Không tìm thấy bảng `orca_access_policies` đúng cấu trúc F32 mô tả (project_id/server_id/granted_by/expires_at) — schema thực tế qua `AdminPolicy` type khác hẳn (teams/roles/allowedServers), và (xem F25) không có API CRUD cho nó dù có bảng DB.

### 5.12 F33 — User Profile Hierarchy
- ✅ Cơ chế merge 3-tầng Company←Dept←User + cache 60s + "security company-only" (đúng ở tầng resolver) + source attribution `_sources` — khớp.
- ❌ **`OrcaProfile` schema lệch đáng kể**: doc yêu cầu 6 section `agent/editor/shell/integrations/fleet/security`; code có `agent/editor/shell/mcp/security/envVars` — **`integrations` và `fleet` hoàn toàn không tồn tại**, thay bằng `mcp`/`envVars` không có trong doc.
- ❌ **`security` section field-set sai hoàn toàn**: doc `require2FA/sessionTimeoutHours/allowedIpRanges/auditAllActions` (session/login policy); code `approvedModels/disallowedCommands/requireReviewBeforeCommit/maxSessionHours` (agent policy) — 2 khái niệm khác hẳn nhau dùng chung tên section.
- ❌ **`editor` section thiếu 4/6 field** (`fontSize/fontFamily/keybindings/wordWrap` không tồn tại).
- ❌ **Deep-merge "array union (no duplicates)" bị vi phạm**: `mergeShell()` nối mảng `pathAdditions` bằng concat thẳng, không dedup — sai đúng quy tắc F33 tự đặt ra.
- ❌ Kế thừa trực tiếp lỗ hổng bảo mật F32: acceptance criterion "chỉ company set được `security`" bị phá vỡ ở tầng RPC vì `requireAdmin(ctx)` là stub.

### 5.13 F34 — Project-Dev Server Binding
- ✅ "1 project ↔ 1 dev server" khớp, validate tồn tại trước khi bind, routing `ProjectServerRouter` đúng luồng, user chỉ thấy project mình là member.
- ❌ **Không thể rebind/đổi dev server** sau khi project đã tạo — `devServerId` không nằm trong schema `update()`, không có method `bindDevServer`/`rebindDevServer` nào — mâu thuẫn với F34 mô tả Lead/Admin đổi được qua Project Settings.
- ❌ **`requireOwnerOrAdmin` không hề check admin** — chỉ check `role !== 'owner'`, phần "OrAdmin" trong tên hàm là dead code; global admin (role F32) không có quyền override trên project không sở hữu.
- ❌ `project.create` không giới hạn Lead/Admin — bất kỳ user login nào tạo được project + bind dev server.
- ⚠️ 2 hệ "role" trùng tên khác domain: `ProjectRole = 'owner'|'member'|'viewer'` (project-level) vs `'developer'|'lead'|'admin'` (F32/F33 org-level) — dễ nhầm lẫn.
- ⚠️ Tên bảng khác doc: `orca_v5_projects`/`orca_v5_project_members` (không phải `orca_projects`/`orca_project_members`) — do đụng độ với bảng đã bị chiếm ở migration 0004 (xem §0).

### 5.14 F35 — AI Provider Account Management
- ✅ Account scope (server/project/user), 7 provider type, credential relay nguyên tắc (không lưu key trên Orca Server), quota tracking `orca_provider_usage`, background health check 15 phút — khớp.
- ⚠️ Provider selection priority đảo ngược: doc `project-scope → user-scope`; code `user-scope → project-scope`.
- ❌ **Key rotation (grace period 30s, status 'rotating', audit log) hoàn toàn không tồn tại** — không method `rotateKey`, không status `'rotating'` trong enum, không RPC method — update key hiện tại chỉ ghi đè trực tiếp qua `writeCredential`.
- ❌ **Quota 80% alert không tồn tại** — chỉ phát hiện SAU khi đã vượt quota (`quota_exceeded` status), không cảnh báo sớm.
- ⚠️ Test-connection không tự động khi `create` account — phụ thuộc FE tự gọi 2 RPC riêng.
- ❌ Không có audit log cho CRUD/credential-write của AI provider.

### 5.15 F36 — Multi-Server Workflow Orchestration
- ✅ DAG builder / wave computation (Kahn's algorithm đúng chuẩn, cycle detection), template inheritance (chain resolve, depth 5) — khớp tốt.
- ⚠️ Step types: doc có 6 (`agent/shell/action/webhook/parallel/condition`); code có 5 (`agent/shell/webhook/notification/condition`) — thiếu `action` (generic action dispatcher), `parallel` chỉ đạt được ngầm định qua wave execution.
- ⚠️/❌ Dispatch đa server: chỉ hỗ trợ `project:<id>`; `server:<devServerId>` ném lỗi tường minh "not yet implemented"; `fleet:tag:<tag>` hoàn toàn không xử lý (hạ tầng fleet tồn tại riêng nhưng không wire vào workflow).
- ❌ **Provider selection theo từng step: 0% code**, dù đây là tính năng chính minh hoạ trong doc (mix Claude/GPT-4o giữa các bước) — `WorkflowStepConfig` chỉ là bag opaque, `WorkflowOrchestrator`/`StepExecutors` không hề import `AIProviderService`/`ProviderResolver` (xác nhận bằng grep 0 kết quả).
- ❌ Không có retry/backoff logic cho step — chỉ có `continueOnError` + timeout.
- ❌ **Pause/Resume hoàn toàn không tồn tại** — `WorkflowStatus` không có `'paused'`; `resumeRunningExecutions()` chỉ là crash-recovery khi server restart, không phải user-triggered pause/resume qua UI như doc mô tả nút "[Pause]".
- ❌ Không streaming step output cho UI — không `logStreamId`, không WS push, UI chỉ poll.
- ⚠️ Sharing modes (private/team/company/public) rất sơ khai — `scope` là string tự do, không validate enum, không "share link".

### 5.16 F37 — Task Graph Management
- ✅ Mô hình dual-edge DAG (parent_id + `orca_task_edges`) đúng thiết kế.
- ✅ Cycle detection **đã implement đầy đủ** (`TaskDAGValidator`: DFS/BFS có index, không phải TODO như README v2 Gap G14 ngụ ý) — chỉ có điểm tối ưu khả dĩ là round-trip theo từng node thay vì 1 CTE đệ quy.
- ✅ Grant system 5 permission-level (`view<comment<edit<execute<manage`) + BFS ancestor + `applyTree` — đúng thuật toán thiết kế.
- ⚠️ Scope enum grant khác doc: doc `'company'|'team'|'user'`; code `'user'|'team'|'role'|'everyone'` — không có `'company'` (thay bằng `'everyone'`), thêm `'role'` không có trong doc.
- ⚠️ AI decomposition (`aiDecompose`/`aiApply`) đúng luồng cơ bản nhưng **thiếu dependencies giữa subtask, `totalEstimate`, `criticalPath`** mà doc mô tả — chỉ trả mảng phẳng subtask.
- ❌ **Progress tracking không rollup từ subtask thật** — chỉ là bảng tĩnh map status→% của chính task đó (`backlog:0,in_progress:40,review:80,done:100`), không có hàm tính tỉ lệ subtask hoàn thành như doc yêu cầu.
- ❌ Thiếu field `worktreeId`/`agentSessionId`/`workflowExecutionId`/`actualHours` trên `OrcaTask` mà doc mô tả — "estimated vs actual hours" không có nguồn dữ liệu.

### 5.17 F38 — Project Workspace
- ✅ Backend khớp tốt: RPC namespace riêng `workspace.*` (`init/teardown/refreshFileTree/refreshGitStatus`), không tạo entity DB mới (dùng lại Project/Repo/Worktree đúng tinh thần thiết kế), load song song git-status/worktree-list/fs-readDir/pending-tasks.
- ❌ **Frontend bug thật**: tab "Agent" trong `WorkspaceTabBar` được khai báo nhưng `WorkspaceLayout` không có nhánh render cho `activeTab==='agent'` — click vào hiển thị panel trống (dead tab), dù `AgentPanel.tsx` tồn tại và hoạt động độc lập ở nơi khác.
- ❌ Terminal panel chỉ là placeholder `"Terminal — coming soon"`, chưa có PTY session thật.
- ⚠️ Server-status indicator chỉ nhị phân (`isOffline`), không có "degraded" như doc mô tả 3 trạng thái.

### 5.18 F39 — Remote Git UI
- ✅ Kiến trúc relay có thật cho git-on-dev-server (qua `IGitProvider`→`SshChannelMultiplexer`→agent-side `git-handler.ts` chạy git binary thật) — không phải giả lập. Stage/unstage/commit/checkout/push/pull/fetch/rebase, worktree ops đều đầy đủ.
- ❌ **`DevServerGitProvider` ném lỗi "not supported for Dev Server hosts yet" cho nhiều tính năng nằm trong tiêu chí chấp nhận F39**: `getHistory()` (git log — component `GitLog.tsx` cũng không tồn tại), `getStagedCommitContext()` (AI commit message), `getBranchCompare()`, `getCommitCompare()`, `getBranchDiff()`, `getCommitDiff()`, `getSubmoduleStatus()`, `checkIgnoredPaths()`, `syncForkDefaultBranch()`.
- ⚠️ Path file trong bảng "Yêu cầu kỹ thuật" sai (`src/main/ssh/relay-git-bridge.ts` không tồn tại — logic thật nằm rải ở `agent/src/relay/git-handler.ts` + `backend/src/main/providers/dev-server-git-provider.ts`/`ssh-git-provider.ts`).

### 5.19 F40 — Full-Flow Tracing
- ✅ **Khớp tốt nhất toàn bộ audit (~95%)**: `createTracer`/`registerTraceSink`/`TraceEvent`/`resume:{id}` — đúng chữ ký ở cả 4 package; `fail` log vô điều kiện bất kể `ORCA_TRACE`; SSE endpoint `/api/trace-stream` đúng auth chain + heartbeat 15s + fan-out sink.
- ✅ **Migration 0013 (`root_trace_id`) chính là cơ chế trace-correlation-qua-restart cho Workflow** — dùng đúng API `resume:{id}` để tái tạo parent span sau khi Orca Server restart (`WorkflowOrchestrator.ts:110,120,249-252,359-360`). Đây là ví dụ hiếm hoi code triển khai *vượt* mô tả doc theo đúng tinh thần thiết kế.
- ⚠️ Bảng "Các flows được trace sẵn" trong doc lỗi thời — liệt kê 5 tracer, thực tế có ~50 (bao gồm cả cụm `workflow:execute`/`workflow:stepExecute` liên quan trực tiếp đến migration 0013).

---

## 6. Top 10 phát hiện nghiêm trọng nhất (toàn bộ 2 vòng, xếp theo mức độ rủi ro)

1. **[Bảo mật — CRITICAL] `requireAdmin(ctx)` trong `profile-rpc-handler.ts` là stub không check role** — bất kỳ user login nào set được chính sách bảo mật toàn công ty (§5.11/F32).
2. **[Kiến trúc — HIGH] Backend tự thực thi GitHub/GitLab thay vì relay Dev Server Agent** — vi phạm "Auth never through Gateway", không per-user CLI isolation trong Web Server mode (§2.12b, mở rộng ở §5.9/F30).
3. **[Bảo mật — HIGH] `requireOwnerOrAdmin` không check admin (dead code), `project.create` không giới hạn quyền** — global admin không override được project người khác sở hữu; ai login cũng tạo được project (§5.13/F34).
4. **[Tài liệu — HIGH] §10 DB Schema Overview sai gần hoàn toàn** — nội dung/tên bảng 0001-0004, 0007 không khớp thực tế; 13 migration thật vs 10 trong tài liệu (§2.7).
5. **[Feature gap — HIGH] Admin Panel: session-list là stub rỗng, Access-Policy CRUD API không tồn tại** dù đánh dấu "đã Phát hành" (§5.4/F25).
6. **[Feature gap — HIGH] Workflow: provider-selection theo step 0% code, pause/resume không tồn tại** — 2 tính năng trung tâm của F36 chỉ có ở tài liệu (§5.15/F36).
7. **[Data integrity — MEDIUM-HIGH] FleetHealthMonitor không thu thập CPU/RAM/disk/latency** — khái niệm này không tồn tại trong data model, dù Admin Panel/Prometheus doc mô tả như đã có (§5.6/F27, xác nhận từ Vòng 1).
8. **[Reliability — MEDIUM] Auto-respawn session khi crash (max 3) và idle-timeout qua env var đều chưa cài đặt** dù đánh dấu hoàn thành (§5.3/F24).
9. **[Feature gap — MEDIUM] `orca fleet provision` CLI hoàn toàn không tồn tại**; bootstrap thiếu disk-check + SHA256 verify cho relay binary (§5.10/F31).
10. **[Tài liệu — MEDIUM] `docs/adrs/v2/ADR-016..020` dẫn chiếu "HLD v6.0" không tồn tại trong repo** — 5 ADR "Proposed" treo, không nối vào tài liệu gốc nào, dễ gây hiểu nhầm là spec chính thức (§0).

---

## 7. Khuyến nghị bổ sung (Vòng 2)

- **Vá ngay lỗ hổng bảo mật #1**: sửa `requireAdmin(ctx)` trong `profile-rpc-handler.ts` để check `ctx.userRole === 'admin'` thật, không chỉ check đã login. Áp dụng rà soát tương tự cho `requireOwnerOrAdmin` (project-rpc-handler.ts) — hiện tại tên hàm nói dối về hành vi thật.
- **Thống nhất RBAC thành 1 policy table** thay vì 4 cơ chế rời rạc (`resolveUserPermissions`, `requireAdmin` route, `requireAdmin` RPC-stub, `requireOwnerOrAdmin` project) — đúng như F32 yêu cầu ban đầu.
- **Hoàn thiện Admin Panel**: implement `GET /admin/api/sessions` thật (đang stub rỗng) và toàn bộ API CRUD cho Access Policies (DB schema đã sẵn, chỉ thiếu route + handler).
- **Quyết định rõ về Workflow provider-selection và pause/resume**: hoặc implement đúng F36 (wire `AIProviderService` vào `StepExecutors`, thêm `WorkflowStatus.paused` + method `pause()`/`resume()` thật), hoặc hạ mức ưu tiên/cập nhật F36 để phản ánh đúng phạm vi MVP hiện tại.
- **FleetHealthMonitor**: bổ sung thực sự thu thập CPU/RAM/disk (SSH exec `top`/`df`) và đo latency (ping RTT), không chỉ dựa vào SSH connection status.
- **Rà soát toàn bộ các "acceptance criteria đã tick [x]" trong F24/F25/F29/F30/F31/F35/F36 nêu trên** — nhiều mục được đánh dấu hoàn thành trong feature doc nhưng code không có, nên cần quy trình review lại checklist tài liệu định kỳ đối chiếu code thay vì tin theo trạng thái tự khai.
- **Dọn `docs/adrs/v2/`**: hoặc hoàn thiện `docs/hld/README.md`/`docs/hld/C4-code.md` (HLD v6.0) mà 5 ADR này liên tục dẫn chiếu, hoặc đánh dấu rõ ràng hơn (banner ở đầu mỗi ADR) rằng đây là đề xuất chưa được chấp nhận, tránh người đọc mới nhầm là spec hiện hành.

---

*Báo cáo tổng hợp từ 12 agent đối chiếu song song qua 2 vòng: Vòng 1 (5 agent, `docs/hld/backend-server-architecture.md` mục 1–11) + Vòng 2 (7 agent, `docs/features/F22–F40` phần backend-relevant), cùng rà soát trực tiếp `docs/adrs/v2/ADR-016..020` để xác định trạng thái thật của "HLD v6.0". Tổng cộng đối chiếu `backend/src/**` với 3 tài liệu HLD + 19 feature doc + 5 ADR, mọi kết luận đều kèm bằng chứng `file:line` từ code thật.*
