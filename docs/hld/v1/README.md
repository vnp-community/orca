# High Level Design — Orca AI Orchestrator IDE

**Mô hình:** C4 Architecture (Simon Brown)
**Phiên bản:** 3.1 — 2026-08-14 (correction pass: tách rõ kiến trúc v5.0 đang chạy vs tầm nhìn "v6.0" đề xuất; xem ghi chú bên dưới)
**Tham chiếu:** SRS.md, features/, logic/, crs/v2/dev-server/

> **Trạng thái kiến trúc — đọc trước khi dùng tài liệu này:** File này gộp hai lớp nội dung khác nhau và cần đọc phân biệt:
>
> 1. **Kiến trúc v5.0 hiện hành (ĐÃ triển khai)** — Orca Backend Server giao tiếp với Dev Server Agent qua 3 chế độ (`relay-ssh`, `relay-websocket`, `direct-websocket`). Đây **KHÔNG** phải một Control-Plane thuần túy: Backend hiện vẫn tự thực thi một số thao tác trong process của mình (ví dụ CLI `gh`/`glab` cho GitHub/GitLab) thay vì luôn relay tới Dev Server Agent như thiết kế mong muốn (`audit/backend/backend-vs-design-review.md` §0, §2.12b).
> 2. **Tầm nhìn "v6.0: Dev Server Agent architecture — full backend on dev servers"** (🚧 **Proposed, CHƯA triển khai**) — mô tả tại `docs/adrs/v2/ADR-017` (layer model A0–A4), `ADR-018` (Control Plane/Data Plane separation), `ADR-019` (signed context + reconnect). Cả 3 ADR tự khai báo trạng thái Proposed/chưa implement. `SignedExecutionContext`, `ContextVerifier`, và cấu trúc layer A0–A4 **không tồn tại** trong code — xác nhận bằng grep toàn bộ `agent/src` và `backend/src` (0 kết quả), xem `audit/agent/rpc-dispatch-lifecycle-vs-design-review.md` §2.3, §2.10.
>
> Kiến trúc THỰC TẾ đang chạy hôm nay được tài liệu hoá chi tiết và chính xác hơn tại `docs/hld/backend-server-architecture.md`, `docs/hld/dev-server-architecture.md`, và `docs/hld/web-server-architecture.md` (thư mục **gốc** `docs/hld/`, khác với bộ tài liệu C4 `docs/hld/v1/` — bộ tài liệu bạn đang đọc). Nội dung "v6.0" bên dưới được **giữ lại** vì có giá trị tham khảo/tầm nhìn thiết kế, nhưng không còn được trình bày như thể đó là hệ thống hiện hành.

---

## Tổng quan C4 Model

C4 Model mô tả kiến trúc phần mềm qua 4 mức độ trừu tượng:

| Level | File | Mô tả |
|-------|------|-------|
| L1 | [C1-system-context.md](./C1-system-context.md) | Hệ thống trong môi trườ́ng — users, external systems |
| L2 | [C2-containers.md](./C2-containers.md) | Containers: desktop, web server, mobile, relay/agent, daemon, CLI, Profile/Project, AI Provider, Workflow, Task Graph |
| L3 | [C3-components.md](./C3-components.md) | Components: Main Process, Daemon, Relay, UI, Fleet, Platform, DB, AgentWS, Credentials, Profile+Project, AI Provider, Workflow, Task Graph |
| L4 | [C4-code.md](./C4-code.md) | Module map chi tiết + data flows cho tất cả subsystems (C4.1 – C4.9) |
| — | [deployment.md](./deployment.md) | Deployment trên các platform (macOS, Windows, Linux, Docker) |
| — | [security.md](./security.md) | Security model, auth, E2E encryption, multi-user trust |

Tất cả các đường dẫn trên đều tương đối trong `docs/hld/v1/`. Lưu ý: đây là bộ tài liệu C4 phân tầng L1–L4, mô tả riêng backend **TypeScript** (`backend/`+`desktop/`); kiến trúc **hiện hành, đã kiểm chứng qua audit** cho backend TS nằm ở `docs/hld/backend-server-architecture.md`, `docs/hld/dev-server-architecture.md`, `docs/hld/web-server-architecture.md` (thư mục gốc `docs/hld/`, không phải `docs/hld/v1/`).

> **2026-09-06:** Bộ tài liệu C4 này (và 3 file gốc kể trên) **không mô tả `backend-go/`** — một bản viết
> lại Go microservices (17 service, gRPC, Postgres-per-service, Vault) của cùng Control Plane, chạy
> **song song** với backend TS mà toàn bộ `docs/hld/v1/` mô tả. Xem
> [`docs/hld/backend-go-architecture.md`](../backend-go-architecture.md) cho kiến trúc backend-go, và
> [`docs/adrs/v2/ADR-022`](../../adrs/v2/ADR-022-wscompat-protocol-bridge.md)–[`ADR-024`](../../adrs/v2/ADR-024-dual-workflow-engines-migration.md)
> cho các quyết định kiến trúc tương ứng.

---

## Sơ đồ tóm tắt

```
┌───────────────────────────────────────────────────────────────┐
│                  EXTERNAL USERS & SYSTEMS                       │
│   Developer · Tech Lead · Remote Dev · Mobile User · Admin      │
└───────────────┬─────────────────┬─────────────────┬─────────────┘
                │                 │                 │
        ┌───────┴──────┐  ┌───────┴───────┐  ┌──────┴──────┐
        │ Orca Desktop │  │ Orca Web      │  │ Orca CLI    │
        │ (Electron)   │  │ Server        │  │ (Node.js)   │
        └───────┬──────┘  └───────┬───────┘  └──────┬──────┘
                │                 │                 │
                └────────┬────────┴────────┬────────┘
                         │  IPlatformServices
                ┌────────┴──────────────────────────┐
                │  Orca Main Process / Node Adapter  │
                │  Auth · Session Manager            │
                │  Admin Panel · Audit Log           │
                │  DB Layer (SQLite/MySQL/PgSQL/TiDB) │
                │  Fleet Health Monitor              │
                └────────┬────────────────────────────┘
                         │ relay-ssh / relay-websocket / direct-websocket
                ┌────────┴────────┐
                │  Dev Server(s)  │
                │  Agent (PTY,    │
                │  git, fs, AI)   │
                └─────────────────┘
```

---

## Architecture Layers

Nội dung này bao gồm HAI sơ đồ tách bạch — một mô tả tầm nhìn Proposed, một mô tả gần với hệ thống hiện hành hơn. Trước đây hai sơ đồ này từng bị dán chồng lẫn nhau trong file (code fence + border ký tự box-drawing bị merge làm hỏng cả hai); đã tách lại theo đúng cấu trúc gốc bên dưới.

### (a) 🚧 Proposed — "v6.0" Control Plane / Data Plane split (chưa triển khai)

> Nguồn: `docs/adrs/v2/ADR-017` (layer model A0–A4), `ADR-018` (Control Plane/Data Plane separation), `ADR-019` (signed context + reconnect). Cả 3 đều tự khai báo trạng thái **🚧 Proposed / ❌ Chưa implement**. Không có `SignedExecutionContext`/`ContextVerifier`, và không có cấu trúc thư mục `src/agent/{rpc,pty,git,fs,execution,storage,reporting}/` nào trong code thật — package `agent/` hiện tại vẫn phẳng (`agent/src/relay/*`), không phân lớp A0–A4.

```
═══════════════════ ORCA BACKEND SERVER (Control Plane — PROPOSED) ═══════
┌────────────────────────────────────────────────────────────────────┐
│ L0 UI Layer    React SPA (Renderer + Admin SPA + Task/Workflow UI)  │
├────────────────────────────────────────────────────────────────────┤
│ L1 Platform    IPlatformServices: NodeAdapter (server)              │
│                ElectronAdapter (desktop only)                       │
├────────────────────────────────────────────────────────────────────┤
│ L2 Auth        AuthManager, SessionManager, bcrypt 12r              │
│                HTTP-only session cookie, SSO stub                   │
├────────────────────────────────────────────────────────────────────┤
│ L3 Control     Tenant/Team/User/Profile (F32, F33)                  │
│                Project Registry (F34)                               │
│                AI Provider Registry — metadata only (F35)           │
│                Workflow DAG Builder + Dispatcher (F36)              │
│                Task Graph + Grant System (F37)                      │
│                Fleet Monitor + Provisioning (F27, F31)              │
│                Agent Connection Manager                             │
│                Signed Context Issuer (CR-DS-005)                    │
├────────────────────────────────────────────────────────────────────┤
│ L4 Repository  IStateRepository: SqlRepository                      │
├────────────────────────────────────────────────────────────────────┤
│ L5 Database    IDatabase + IConnectionPool                          │
│                SQLite | MySQL | PostgreSQL | TiDB                   │
│                Migrations 0001–0010                                 │
└────────────────────────────────────────────────────────────────────┘

═══════════════════ DEV SERVER AGENT (Data Plane — PROPOSED) ═════════════
┌────────────────────────────────────────────────────────────────────┐
│ A0 RPC Server  JSON-RPC 2.0 over WebSocket                          │
│                Signed Context Verifier (HMAC-SHA256)                │
├────────────────────────────────────────────────────────────────────┤
│ A1 Operations  PTY Manager (node-pty)                               │
│                Worktree Manager (git worktree)                      │
│                AI Agent Spawner (ProfileAwareAgentSpawner)          │
│                Git Engine (exec, stream)                            │
│                File System Engine (read, write, watch, search)      │
│                SSH Tunnel (outbound, port forwarding)               │
├────────────────────────────────────────────────────────────────────┤
│ A2 Execution   Workflow Step Executor (agent, shell, action)        │
│                Task Agent Executor                                  │
│                Ephemeral VM Runtime (F18)                           │
│                Automation Runner (F14 legacy)                       │
├────────────────────────────────────────────────────────────────────┤
│ A3 Storage     AI Provider Credential Store (AES-256-GCM)           │
│                AI Vault / Session Storage (F17)                     │
│                Local SQLite (worktrees, sessions, task runs)        │
├────────────────────────────────────────────────────────────────────┤
│ A4 Reporting   Event Stream (PTY out, agent status, git, health)    │
│                Local Audit Log (append-only)                        │
│                Health Metrics (CPU, RAM, disk, latency)             │
└────────────────────────────────────────────────────────────────────┘
```

Migrations "0001–0010" trong sơ đồ trên là số proposed gốc của ADR-016 — thực tế code đã vượt qua con số này (13 migration thật, tên bảng cũng khác). Xem [DB Migration → Feature Mapping](#db-migration--feature-mapping) bên dưới cho bảng đã đối chiếu với code.

### (b) Kiến trúc hiện hành (đơn giản hoá)

> Gần với hệ thống thật hơn sơ đồ (a), nhưng vẫn là một sơ đồ đơn giản hoá — các chú thích ¹²³ bên dưới ghi rõ chỗ đã xác nhận sai khác qua audit.

```
┌───────────────────────────────────────────────────────────────────────────┐
│ L0 UI               React Renderer (Desktop) + Web SPA + Admin SPA        │
├─────────────────────────────────────────────────────────────────────────┤
│ L1 Platform         IPlatformServices                                     │
│                     NodeAdapter (server) — ElectronAdapter KHÔNG tồn tại¹  │
├─────────────────────────────────────────────────────────────────────────┤
│ L2 Auth             AuthManager, SessionManager                           │
│                     bcrypt 12r, HTTP-only session cookie                  │
├─────────────────────────────────────────────────────────────────────────┤
│ L3 Business Logic   Worktree, Agent, SSH, Git                             │
│                     Fleet, AgentWS, Integration (GitHub/GitLab)           │
│                     Profile/Project (F33-34)                              │
│                     AI Provider Accounts (F35)                            │
│                     Workflow Orchestration (F36)                          │
│                     Task Graph + AI Planning (F37)                        │
├─────────────────────────────────────────────────────────────────────────┤
│ L4 Repository       IStateRepository                                      │
│                     JsonFileStateRepository | SqlStateRepository²         │
├─────────────────────────────────────────────────────────────────────────┤
│ L5 Database         IDatabase + IConnectionPool                           │
│                     SQLite | MySQL | PostgreSQL | TiDB                    │
│                     Migrations 0001–0013³                                 │
└───────────────────────────────────────────────────────────────────────────┘
```

¹ Electron Desktop dùng thẳng SDK `electron`, không qua `IPlatformServices` — lời hứa "swap adapter mà không đổi business logic" chưa thành hiện thực ở nhánh Desktop (`audit/backend/backend-vs-design-review.md` §2.1).
² Tên class thật — không phải `JsonFileRepository`/`SqlRepository` như bản (a) mô tả (cùng audit, §2.7).
³ 13 migration thật, không dừng ở 0010 — xem [DB Migration → Feature Mapping](#db-migration--feature-mapping).

**Cross-cutting Concerns:**
- **Security**: Audit Log (append-only), RBAC policy table, Agent Token (SHA-256 hash). *Lưu ý: RBAC hiện phân mảnh thành 4 cơ chế không thống nhất, và `requireAdmin()` trong `profile-rpc-handler.ts` có lỗ hổng xác nhận — chỉ kiểm tra đã đăng nhập, không kiểm tra role admin (`audit/backend/backend-vs-design-review.md` §5.11/F32, mục "Top 10 phát hiện" #1).*
- **Observability**: `/health/ready`, `/health/metrics` (Prometheus), Fleet webhooks
- **Credentials**: `WebCredentialStore` AES-256-GCM per-user per-service — *GitHub/GitLab KHÔNG đi qua store này, dùng OS keychain của CLI `gh`/`glab` (chạy trực tiếp trên Backend, không per-user isolation — xem §2.12b cùng audit).*

---

## Công nghệ Stack

> 🚧 = mô tả tầm nhìn "v6.0" Proposed, chưa có trong code hiện tại. Dòng không đánh dấu = đã triển khai (có thể lệch chi tiết tên/hằng số so với mô tả gốc — xem chú thích).

### Orca Backend Server

| Layer | Technology |
|-------|-----------|
| Desktop Shell | Electron 32+ |
| Web Server Mode | Node.js 22+ Express HTTP :6769 + WebSocket :6768 (RPC/browser) |
| Renderer UI | React 19, TypeScript |
| Admin SPA | React SPA tại `/admin` (separate bundle) |
| Platform Abstraction | `IPlatformServices` — `NodeAdapter` thật; `ElectronAdapter` chưa tồn tại (Desktop dùng thẳng `electron`) |
| Auth | bcrypt 12 rounds + HTTP-only cookie session (8h TTL) |
| Multi-DB | `IConnectionPool`, `MigrationRunner`, SQLite/MySQL/PostgreSQL/TiDB |
| IPC | Electron IPC (desktop) + WebSocket RPC (server mode) |
| Mobile | React Native (iOS/Android) |
| Encryption (mobile) | TweetNaCl (NaCl box) |
| Agent Connection | `DevServerRelayBridge` + `GatewayDevServerManagerProxy` — pool kết nối tới Dev Server Agent qua `relay-ssh` / `relay-websocket` / `direct-websocket`. Endpoint Agent WS thật là **`:6769/agent`** (httpPort), không phải `:6768` |
| 🚧 Signed Context | HMAC-SHA256 signed `RpcExecutionContext` (30s TTL) — Proposed (ADR-019), **không tồn tại trong code** |
| Profile System | 3-layer merge (Company←Dept←User), in-memory cache TTL 60s |
| AI Provider Registry | Metadata only, no credentials, 7 providers |
| Workflow Engine | DAG builder + dispatcher → delegates steps to Agent (provider-selection theo từng step: 0% code — xem F36 trong traceability matrix) |
| Task Graph | DAG + BFS, AI decompose, grant 5-level, dispatch to Agent |
| Build | Vite + electron-builder + Docker |
| Testing | Vitest + Playwright (E2E) |

### Dev Server Agent

| Layer | Technology |
|-------|-----------|
| Runtime | Node.js 22+ (compiled single binary via `ncc`) |
| RPC Server | Binary-framed protocol (13-byte header) carrying JSON-RPC payloads over WebSocket. 🚧 Pure "JSON-RPC 2.0, no framing" (ADR-014) là Proposed, chưa triển khai |
| Terminal | node-pty (PTY management) + WebGL streams |
| Git | `child_process.execFile` + native git binary — chủ yếu qua `git.exec`/`git.execStream` (generic passthrough), không phải RPC theo từng operation |
| File System | Node.js `fs`, chokidar (watch), ripgrep (search) |
| AI Agent Spawn | node-pty + hook-based JSON status POST (không phải OSC-133 parsing như tài liệu gốc mô tả) |
| AI Credentials | AES-256-GCM files (`~/.orca/ai-providers/*.enc`) |
| Local DB | better-sqlite3 (worktrees, sessions, task runs) |
| SSH | ssh2 library (outbound tunneling) |
| Process Manager | systemd / launchd / Docker restart policies. 🚧 Lifecycle/update tooling theo CR-DS-004 (agent.requestUpdate, version negotiation) không tìm thấy trong `agent/src` |
| Health | Periodic metrics collection, event streaming |
| Deployment | Single binary + install.sh script / Docker image |

---

## Feature → HLD Traceability Matrix

> Cột **Status** đã được đối chiếu lại với `audit/backend/backend-vs-design-review.md` §5 (Vòng 2, F22–F40). Các hàng F22–F32 đã sửa theo kết quả audit; xem [Ghi chú kiểm định](#ghi-chú-kiểm-định-2026-08-08) bên dưới bảng để biết lý do từng thay đổi. F33–F39 giữ nguyên 🚧 (đã đúng thực trạng — vẫn chưa triển khai đầy đủ theo audit Vòng 2).

| Feature | Priority | Status | C2 Container | C3 Component | C4 Module | ADR |
|---------|----------|--------|--------------|--------------|-----------|-----|
| F01 Parallel Worktrees | P0 | ✅ | Main Process | C3.1 | C4.1 | — |
| F02 Terminal Splits | P0 | ✅ | Main Process, Daemon | C3.1, C3.2 | C4.1 | — |
| F03 Mobile Companion | P0 | ✅ | Mobile | C3.4 | — | — |
| F04 AI Agent Support | P0 | ✅ | Main Process | C3.1, C3.8 | C4.1 | — |
| F07 SSH Worktrees | P1 | ✅ | Relay | C3.3, C3.5 | C4.4 | — |
| F09 Orca CLI | P1 | ✅ | CLI | C2 | — | — |
| F14 Automations | P2 | 🚧 | Main Process | C3.1 | C4.1 | — |
| F22 Web Server Mode | P0 | ✅ | Orca Web Server | C3.6 | C4.2 | ADR-001 |
| F23 Multi-User Auth | P0 | ⚠️ | Orca Web Server | C3.1 | C4.1 | ADR-003 |
| F24 Per-User Sandbox | P0 | ❌ | Orca Web Server | C3.1 | C4.1 | ADR-003 |
| F25 Admin Panel | P1 | ❌ | Orca Web Server | C3.1 | — | — |
| F26 Multi-Database | P1 | ✅ | Server DB | C3.7 | C4.3 | ADR-002 |
| F27 Fleet Health Monitoring | P1 | ❌ | Dev Servers | C3.5 | C4.4 | ADR-004 |
| F28 Dev Server Onboarding | P1 | ⚠️ | Dev Servers | C3.5 | C4.4 | ADR-004 |
| F29 Agent WebSocket | P1 | ❌ | Agent WS | C3.8 | C4.5 | ADR-005 |
| F30 Remote Integrations | P1 | ❌ | Orca Web Server | C3.9 | C4.6 | ADR-006 |
| F31 Fleet Provisioning | P1 | ❌ | Dev Servers | C3.5 | C4.4 | ADR-004 |
| F32 Team RBAC | P2 | 🚧 | Orca Web Server | C3.1 | — | — |
| **F33 Profile Hierarchy** | **P0** | **🚧** | **Profile/Project Svc** | **C3.10** | **C4.7** | **ADR-007** |
| **F34 Project Binding** | **P0** | **🚧** | **Profile/Project Svc** | **C3.10** | **C4.8** | **ADR-007, ADR-011** |
| **F35 AI Provider Mgmt** | **P0** | **🚧** | **AI Provider Svc** | **C3.11** | **C4.9** | **ADR-008** |
| **F36 Workflow Orchestration** | **P1** | **🚧** | **Workflow Orchestrator** | **C3.11b** | **C4.9** | **ADR-009** |
| **F37 Task Graph** | **P0** | **🚧** | **Task Graph Svc** | **C3.11c** | **C4.9** | **ADR-010** |
| **F38 Project Workspace** | **P0** | **🚧** | **Profile/Project Svc** | **C3.12** | **C4.10** | **ADR-011** |
| **F39 Remote Git UI** | **P0** | **🚧** | **Dev Servers (relay)** | **C3.12** | **C4.10** | **ADR-012** |

### Ghi chú kiểm định (2026-08-08)

Thay đổi so với bản trước (theo `audit/backend/backend-vs-design-review.md` §5.0 và các mục §5.x tương ứng):

- **F23 Multi-User Auth** ✅→⚠️: `GET /auth/me` thiếu field `name`/`provider`; cookie `SameSite=strict` (không phải `Lax` như tài liệu), `Secure` chỉ bật khi `NODE_ENV=production` (§5.2).
- **F24 Per-User Sandbox** ✅→❌: hai tiêu chí đánh dấu "đã xong" thực ra chưa cài đặt — auto-respawn khi crash (max 3 lần) và idle-timeout qua env var đều không có logic thực thi (§5.3).
- **F25 Admin Panel** ✅→❌: `GET /admin/api/sessions` là stub rỗng (`note: 'Full listing not yet implemented'`); toàn bộ backend API cho Access Policies (PoliciesPage) không tồn tại dù DB schema đã sẵn (§5.4).
- **F27 Fleet Health Monitoring** ✅→❌: không thu thập CPU/RAM/disk/latency thật — các khái niệm này không tồn tại trong data model, chỉ poll SSH connection status (§5.6).
- **F28 Dev Server Onboarding** ✅→⚠️: chức năng cốt lõi có nhưng tên hàm/class/endpoint sai khác nhiều so với tài liệu (§5.7).
- **F29 Agent WebSocket** ✅→❌: keepalive/timeout thật là 5s/20s (không phải 30s/90s), close code là WS chuẩn 1008/1005 (không phải 4001-4003), `AGENT_MIN_VERSION` là hằng số chết chưa dùng để check version-mismatch (§5.8).
- **F30 Remote Integrations** ✅→❌: Category A (GitHub/GitLab) hầu hết chạy trực tiếp trên Backend thay vì relay tới Dev Server Agent; `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` không bao giờ được truyền; biến env đúng là `ORCA_SERVER_SECRET`, không phải `ORCA_CREDENTIAL_KEY` (§5.9, §2.12b).
- **F31 Fleet Provisioning** ✅→❌: CLI `orca fleet provision` hoàn toàn chưa implement; bootstrap thiếu disk-space check và SHA256 verify cho relay/agent binary (§5.10).
- **F32 Team RBAC** (giữ 🚧, bổ sung cảnh báo): `requireAdmin(ctx)` trong `profile-rpc-handler.ts` là stub — chỉ check đã login, không check role — đây là **phát hiện bảo mật nghiêm trọng nhất toàn bộ audit** (§5.11, Top 10 #1).

---

## DB Migration → Feature Mapping

> Đối chiếu lại với `audit/backend/backend-vs-design-review.md` §2.7. Bản trước chỉ liệt kê 0001–0010 và hầu hết tên bảng 0001–0004, 0007 sai — migration runner thật có **13 file** (0001→0013). Bảng dưới đây dùng tên bảng thật theo code; cột Features cho 0001–0004 là suy luận hợp lý theo nội dung bảng (audit không gán feature 1:1 cho từng migration cũ, chỉ xác nhận tên bảng).

| Migration | Tables (theo code thật) | Features | Ghi chú |
|-----------|--------------------------|----------|---------|
| 0001 initial | `settings`, `projects`, `repos`, `ssh_targets` | Core Desktop bootstrap | Không phải `orca_worktrees, orca_sessions` như tài liệu cũ ghi |
| 0002 automations | `automations` | F14 | Không phải `terminal_scrollback_snapshots` như tài liệu cũ ghi |
| 0003 sessions | `workspace_sessions` | F22, F23 | Không phải `ssh_hosts, saved_port_forwards` như tài liệu cũ ghi |
| 0004 app_tables | `orca_projects`, `orca_repos`, `orca_ssh_targets`, `orca_global_settings` | F23, F24 | Không phải `automations, automation_runs, notifications, rate_limits` như tài liệu cũ ghi |
| 0005 auth | `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies` | F23, F25 | ✅ khớp chính xác với tài liệu |
| **0006** (v5.0) | `orca_companies`, `orca_departments`, `orca_user_profiles` | **F33** | Tên bảng số nhiều (`companies`), khác đề xuất `orca_company` trong ADR-016 |
| **0007** (v5.0) | `orca_v5_projects`, `orca_v5_project_members` | **F34** | Đổi tên `v5_` để tránh đụng độ với bảng `orca_projects` đã bị chiếm bởi migration 0004 |
| **0008** (v5.0) | `orca_ai_provider_accounts`, `orca_provider_usage` | **F35** | ✅ khớp chính xác |
| **0009** (v5.0) | `orca_workflow_templates`, `orca_workflow_executions`, `orca_workflow_step_executions` | **F36** | Khớp gần đúng — bảng thứ 3 có thêm tiền tố `workflow_` |
| **0010** (v5.0) | `orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments`, `orca_team_members` | **F37** | Khớp + có thêm `orca_team_members` ngoài tài liệu gốc |
| 0011–0012 | `terminal_sessions`, `port_forwards`, `push_subscriptions` | F02, F07, F03 (suy đoán theo tên bảng) | Không có trong tài liệu gốc; audit chỉ xác nhận nhóm bảng này tồn tại trong khoảng 0011–0012, chưa tách chi tiết migration nào tạo bảng nào |
| 0013 | Thêm cột `root_trace_id` (không tạo bảng mới) | F40 | Cơ chế trace-correlation-qua-restart cho Workflow — dùng API `resume:{id}` để tái tạo parent span sau khi Backend restart; audit đánh giá đây là **điểm khớp thiết kế tốt nhất toàn bộ audit (~95%)** — xem `WorkflowOrchestrator.ts:110,120,249-252,359-360` |
