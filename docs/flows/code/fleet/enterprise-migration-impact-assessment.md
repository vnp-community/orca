# Đánh giá Tác động: Single-User Desktop → Enterprise Multi-Tenant Architecture

**Tài liệu:** Impact Assessment — Chuyển đổi kiến trúc  
**Phiên bản Orca:** v5.0 (HLD C4)  
**Ngày:** 2026-07-30  
**Tham chiếu:** `docs/hld/` (C1–C4, security), `docs/features/` (F01–F39)

---

## Tóm tắt điều hành

Việc chuyển đổi từ **Single-User Desktop (Electron, per-device)** sang **Enterprise Multi-Tenant Architecture (Orca Session Servers, multi-team, multi-user, multi-session)** tác động đến **tất cả 39 features** ở các mức độ khác nhau. Báo cáo này phân loại từng feature theo mức độ tác động, xác định các breaking changes, và đề xuất hướng xử lý.

### Bản chất thay đổi kiến trúc

| Chiều thay đổi | Trước (Single-User Desktop) | Sau (Enterprise Multi-Tenant) |
|---|---|---|
| **Runtime** | Electron (per-device) | Node.js Web Server (centralized) |
| **User scope** | 1 user / 1 machine | N users / 1 server instance |
| **Session model** | Luôn logged-in, no auth | bcrypt login + HTTP cookie session |
| **Data isolation** | Không cần (1 user) | Per-user `fork()` process + per-user DB |
| **Tenant** | 1 tenant (cá nhân) | Company → Department → User (3 tầng) |
| **Team** | Không có | Team-based RBAC + Fleet scoping |
| **Persistence** | Local SQLite (`~/.config/orca/`) | Server DB (SQLite/MySQL/PgSQL/TiDB) |
| **Sessions** | Local PTY, 1 instance | Orca Session Server → multi-session relay |
| **AI Providers** | `process.env` global (1 user) | Per-account AES-GCM + relay to Dev Server |
| **Automation scope** | Per-device cron | Multi-server DAG workflows |
| **Update mechanism** | electron-updater per machine | Centralized Docker/fleet deployment |

---

## Phân loại tác động tổng quan

```
🔴 BREAKING — Phải thiết kế lại hoàn toàn
🟠 MAJOR — Thay đổi đáng kể, cần refactor
🟡 MODERATE — Thay đổi có ý nghĩa nhưng tương thích backward
🟢 MINOR — Tác động nhỏ, thêm context/scope là đủ
⚪ NOT IMPACTED — Không thay đổi hoặc thay đổi không quan trọng
```

---

## Matrix Đánh giá theo Feature

| Feature | Tên | Priority | Impact | Ghi chú nhanh |
|---------|-----|----------|--------|--------------|
| F01 | Parallel Worktrees | P0 | 🟠 MAJOR | Scope by user + project |
| F02 | Terminal Splits | P0 | 🟡 MODERATE | PTY isolation per user |
| F03 | Mobile Companion | P0 | 🟠 MAJOR | Pairing model phải thay đổi |
| F04 | AI Agent Support | P0 | 🔴 BREAKING | Agent credential + trust thay đổi hoàn toàn |
| F05 | Design Mode | P1 | 🟢 MINOR | Không ảnh hưởng lớn |
| F06 | GitHub/Linear Integration | P1 | 🟠 MAJOR | Per-user token isolation |
| F07 | SSH Worktrees | P1 | 🟠 MAJOR | Fleet-managed, RBAC-gated |
| F08 | Annotate AI Diffs | P1 | 🟢 MINOR | User context annotation |
| F09 | Orca CLI | P1 | 🟠 MAJOR | Auth required, scope must change |
| F10 | Quick Open | P1 | 🟢 MINOR | Project-scope filtering |
| F11 | Notifications | P1 | 🟡 MODERATE | Per-user notification routing |
| F12 | File Explorer & Editor | P1 | 🟡 MODERATE | Project-scoped file tree |
| F13 | Text Search | P1 | 🟡 MODERATE | Project-scoped search |
| F14 | Automations | P2 | 🔴 BREAKING | Scope → multi-tenant, superseded by F36 |
| F15 | Computer Use | P2 | 🟠 MAJOR | Desktop-only → headless conflict |
| F16 | Rich Repo Previews | P2 | 🟢 MINOR | Không ảnh hưởng nhiều |
| F17 | Memory / AI Vault | P2 | 🟠 MAJOR | Vault phải per-user, server-side |
| F18 | Ephemeral VM | P2 | 🟠 MAJOR | Multi-user scheduling + quota |
| F19 | Localization | P2 | 🟢 MINOR | Tác động nhỏ |
| F20 | Speech Input | P3 | ⚪ NOT IMPACTED | Client-side only |
| F21 | Auto Update | P0 | 🔴 BREAKING | Electron update không áp dụng cho server |
| F22 | Web Server Mode | P0 | ✅ ENABLER | Đây chính là nền tảng kiến trúc mới |
| F23 | Multi-User Auth | P0 | ✅ FOUNDATION | Base layer cho toàn bộ |
| F24 | Per-User Sandbox | P0 | ✅ FOUNDATION | Đã thiết kế cho enterprise |
| F25 | Admin Panel | P1 | 🟡 MODERATE | Mở rộng thêm Company/Team admin |
| F26 | Multi-Database | P1 | 🟢 MINOR | Đã thiết kế đúng |
| F27 | Fleet Health Monitoring | P1 | 🟡 MODERATE | Scope theo team/project |
| F28 | Dev Server Onboarding | P1 | 🟡 MODERATE | Multi-team ownership |
| F29 | Agent WebSocket Protocol | P1 | 🟡 MODERATE | Per-user session context |
| F30 | Remote Integrations | P1 | 🟠 MAJOR | Per-user credentials |
| F31 | Fleet Provisioning | P1 | 🟡 MODERATE | Team ownership + RBAC |
| F32 | Team RBAC | P2 | ✅ FOUNDATION | Core của enterprise model |
| F33 | User Profile Hierarchy | P0 | ✅ PURPOSE-BUILT | Thiết kế cho enterprise |
| F34 | Project-Dev Server Binding | P0 | ✅ PURPOSE-BUILT | Thiết kế cho enterprise |
| F35 | AI Provider Account Mgmt | P0 | ✅ PURPOSE-BUILT | Thiết kế cho enterprise |
| F36 | Workflow Orchestration | P1 | ✅ PURPOSE-BUILT | Enterprise workflow engine |
| F37 | Task Graph Management | P0 | ✅ PURPOSE-BUILT | Multi-team task management |
| F38 | Project Workspace | P0 | ✅ PURPOSE-BUILT | Unified enterprise IDE context |
| F39 | Remote Git UI | P0 | ✅ PURPOSE-BUILT | Server-side git operations |

---

## Phân tích Chi tiết theo Mức độ Tác động

---

### 🔴 BREAKING — Phải thiết kế lại

#### F04 — AI Agent Support

**Vấn đề cốt lõi:** Hiện tại agent được spawn với credentials từ `process.env` global của desktop. Trong môi trường enterprise, mỗi user/project có AI provider account riêng, credentials không nằm trên Orca Server.

**Breaking changes:**
- `process.env.ANTHROPIC_API_KEY` (global) → `ProviderResolver.resolve(userId, projectId)` → đọc từ Dev Server
- Agent auto-detection (`PATH` scan) → phải scan từ Dev Server filesystem (relay)
- Session resume (`--resume <id>`) → session IDs phải per-user, per-tenant namespace
- Usage tracking → phải aggregate per account + per user + per team (quota enforcement)
- Trust presets → phải check Company profile `approvedModels` whitelist trước khi spawn
- Account switcher khi rate-limit → phải liệt kê accounts từ `F35 AIProviderResolver`

**Hướng xử lý:**
```
AgentSpawner.spawn(agentType, options) [cũ]
  → ProfileAwareAgentSpawner.spawn(userId, projectId, agentType, options) [mới]
      → resolveProfile(userId)  // F33
      → resolveProvider(userId, projectId, agentType)  // F35
      → validate model in profile.agent.approvedModels
      → relay.call('pty.spawn', { env: resolvedEnv, credentials: accountId })
```

**Estimate effort:** 🔴 High — refactor AgentSpawner + migration của agent-detection, session-resume, usage tracking

---

#### F14 — Automations

**Vấn đề cốt lõi:** Automations hiện tại là single-user, per-device cron scheduler lưu trong local SQLite. Với enterprise, automation phải được scope theo user/team/project, chạy trên Orca Server, và deploy actions tới Dev Servers qua relay.

**Breaking changes:**
- Cron scheduler từ desktop process → server-side cron (Orca Web Server)
- Automation scope: không có scope → `{userId, teamId?, projectId?}` required
- Actions: `create_worktree` local → `relay.call('worktree.create', {serverId})` 
- `run_agent` → phải qua `ProfileAwareAgentSpawner` với provider resolution (F35)
- `commit & push` → phải qua relay git ops (F39 pattern)
- Visibility: private / team / company (giống F36 template scoping)
- **F14 về bản chất bị superseded bởi F36** (Workflow Orchestration)

**Hướng xử lý:** F14 nên được merge/refactored thành F36 use-case đơn giản (single-step workflow). Legacy F14 automations cần migration path.

**Estimate effort:** 🔴 High — nên xem xét deprecate F14 và redirect sang F36

---

#### F21 — Auto Update

**Vấn đề cốt lõi:** `electron-updater` là Electron-specific, không áp dụng cho Node.js Server deployment.

**Breaking changes:**
- `electron-updater` → không có equivalent cho server mode
- Update mechanism: DMG/installer → Docker image pull + container restart
- Rollback: Electron watchdog → Docker `--previous-image` rollback
- Update channels (stable/pre-release) → Docker image tags (`latest`, `rc`, `beta`)
- Changelog display → có thể giữ trong UI, fetch từ API endpoint
- Per-device update → centralized fleet update (Admin triggers update cho tất cả nodes)

**Hướng xử lý:**
```
Desktop mode (Electron):    electron-updater [giữ nguyên]
Server mode (Enterprise):   Admin Panel → "Update Server" → pulls new Docker image
                            → rolling restart (nếu cluster)
                            → Fleet Health Monitor verify post-update
```

**Estimate effort:** 🟠 Medium — tách update logic hoàn toàn theo platform

---

### 🟠 MAJOR — Cần refactor đáng kể

#### F01 — Parallel Worktrees

**Tác động:** Worktrees hiện tại gắn với local filesystem user. Trong enterprise:
- Worktrees phải thuộc về `{userId, projectId, devServerId}` tuple
- Quota: admin có thể limit số worktrees per user / per project
- Visibility: user chỉ thấy worktrees của mình (trong project); lead thấy team; admin thấy all
- Fan-out prompt → phải check user's AI provider accounts và approved models (F33, F35)
- Cleanup: orphaned worktrees trên Dev Server phải được detect qua Fleet Health (F27)

**Breaking changes:**
- `orca_worktrees` table: thêm `userId`, `projectId`, `devServerId` columns
- WorktreeService: filter by `userId` + RBAC check for cross-user operations
- Fan-out limit phải respect `profile.agent.maxTokensPerSession` từ company policy

---

#### F03 — Mobile Companion

**Tác động:** Hiện tại mobile pair với **desktop instance** (peer-to-peer, local network). Trong enterprise, developer có thể không có desktop — họ access qua Web Server.

**Breaking changes:**
- Pairing target: `Desktop app` → `Orca Web Server instance` (có thể remote)
- Auth: QR one-time token → phải integrate với F23 session auth
- Notification routing: `Desktop → APNs/FCM` → `Orca Server → APNs/FCM` với userId routing
- Per-user push tokens: mỗi user cần push token riêng, không thể share device token
- E2E encryption model phải được revisit (TweetNaCl peer-to-peer vs. server-mediated)

---

#### F06 — GitHub/Linear Integration

**Tác động:** Hiện tại tokens lưu trong OS Keychain (local). Trong enterprise:
- Tokens phải lưu trong `WebCredentialStore` (per-user AES-256-GCM, server-side)
- GitHub org context: phải respect `profile.integrations.githubOrg` từ F33
- Linear workspace: phải respect `profile.integrations.linearWorkspace`
- PR creation: phải sử dụng per-user `GH_CONFIG_DIR` isolation (không share gh CLI config)
- Multi-org support: team có thể thuộc nhiều GitHub orgs khác nhau

---

#### F07 — SSH Worktrees

**Tác động:** SSH targets hiện tại là per-user personal SSH hosts. Trong enterprise:
- SSH targets được Fleet-managed (F27, F31) — không phải personal config nữa
- RBAC gating: developer chỉ thấy SSH targets của project mình (F32 policy)
- SSH keys: phải respect `profile.fleet.sshKeyPath` từ F33
- Connection type: phải respect `profile.fleet.defaultConnectionType` (relay-ssh vs direct-ws)
- ProxyJump: phải support fleet topology (jump host per cluster)

---

#### F09 — Orca CLI

**Tác động:** CLI hiện tại giao tiếp với Daemon qua Unix socket local. Trong enterprise:
- Target: daemon local socket → Orca Web Server HTTP/WS endpoint
- Auth: không cần → Bearer token (từ user login session hoặc API key)
- Scope: commands phải include `--project <id>` / `--server <id>` context
- CI/CD integration: service account tokens thay vì user sessions
- Headless mode (`orca serve`): phải support ORCA_MULTI_USER=1 từ đầu

---

#### F15 — Computer Use

**Tác động:**  
- Computer Use dựa trên screenshot của **local desktop display** (Electron)
- Trong server mode không có display → Computer Use không áp dụng
- Giải pháp: chỉ enable trong Desktop mode, hoặc implement VNC/remote display cho dev servers
- Cross-user isolation: `orca click/fill` chỉ affect session của user đó

---

#### F17 — Memory / AI Vault

**Tác động:** Session storage hiện tại là local SQLite. Trong enterprise:
- Vault phải per-user (isolate sessions của user A khỏi user B)
- Server-side storage: phải dùng Server DB, không phải local SQLite
- Visibility: user chỉ xem vault của mình; lead có thể xem team sessions (với permission)
- Cross-session search: phải scope theo `userId + projectId`
- Retention policy: phải respect company-level data retention policy

---

#### F18 — Ephemeral VM

**Tác động:**  
- VM scheduling: phải queue trên server infrastructure, không phải local machine
- Multi-user quota: tránh 1 user spawn quá nhiều VMs
- Recipe visibility: company → team → personal scoping (giống F36 templates)
- Output isolation: results per user, không accessible bởi other users
- Fleet integration: VMs nên được managed by Fleet Health (F27)

---

#### F30 — Remote Integrations

**Tác động:** Preflight proxy và GitHub/GitLab tokens phải per-user:
- Token storage: OS Keychain → `WebCredentialStore` per user
- API rate limits: phải track per user (không share company-wide limit quota)
- OAuth flows: phải redirect về user's browser session (không về desktop)
- Integration configs: phải respect `profile.integrations` hierarchy (F33)

---

### 🟡 MODERATE — Thay đổi có ý nghĩa, tương thích backward

#### F02 — Terminal Splits

**Tác động:**  
- PTY sessions phải per-user (đã handled bởi F24 process isolation)
- Scrollback persistence: phải lưu vào per-user DB (không global SQLite)
- Session router: WsSessionRouter đã handle, nhưng terminal session IDs phải namespace `userId`
- Multi-session: user có thể open browser từ 2 tabs → 2 terminal sessions → phải handle gracefully

---

#### F11 — Notifications

**Tác động:**  
- Notification routing: phải scope theo `userId` (không broadcast)
- Mobile push: phải match notification → user → push token
- Agent events: notification content phải include project context
- Admin override: admin có thể receive notifications cho all users (configurable)

---

#### F12 — File Explorer & Editor

**Tác động:**  
- File tree root: phải be project-scoped (F34, F38)
- Path traversal prevention: users không được navigate ngoài `project.repoPath`
- Write operations: phải check RBAC trước (F32)
- Editor tabs: phải restore per-user session (không shared state)

---

#### F13 — Text Search

**Tác động:**  
- Search scope: phải restricted to project files (F34 repoPath)
- Cross-project search: chỉ admin/lead với appropriate permissions
- Search index: phải be per-project, không phải global

---

#### F25 — Admin Panel

**Tác động:**  
- Hiện tại: User CRUD, Sessions, Audit Log
- Cần thêm: Company Profile Editor, Department Management, Team RBAC matrix
- Fleet: Admin panel cần aggregate data từ F33, F34, F35, F36, F37
- Role hierarchy: Company Admin > Team Lead > Developer (F33 security model)

---

#### F27 — Fleet Health Monitoring

**Tác động:**  
- Server visibility: developer chỉ thấy servers của project mình (F32 RBAC)
- Metrics aggregation: phải be per-team/per-project (không global dump)
- Webhook alerts: phải route đến team lead, không broadcast

---

#### F28 — Dev Server Onboarding

**Tác động:**  
- Server ownership: phải assigned to `{teamId, projectId}` (không personal)
- Onboarding permission: chỉ `lead` hoặc `admin` mới onboard servers
- SSH key provisioning: phải use fleet SSH key (từ F33 profile.fleet.sshKeyPath)

---

#### F29 — Agent WebSocket Protocol

**Tác động:**  
- Agent token validation: phải scope theo `devServerId + userId` context
- WS session routing: đã có WsSessionRouter, nhưng custom agent connections phải be validated
- Session ID namespace: `sessionId` phải include `userId` prefix để avoid collision

---

#### F31 — Fleet Provisioning

**Tác động:**  
- Provisioning ownership: phải assigned to team/project
- RBAC: chỉ `admin` hoặc `lead` có quyền provision
- Auto-tagging: servers phải be tagged với `teamId`, `projectId` khi provision

---

### 🟢 MINOR — Tác động nhỏ

#### F05 — Design Mode
Không thay đổi lớn. Chỉ cần đảm bảo annotations được lưu per-user, per-project.

#### F08 — Annotate AI Diffs
Annotation phải include `userId` và `projectId` context. Sharing annotations cần team visibility setting.

#### F10 — Quick Open
Quick Open phải be scoped to current project's file tree (F38 WorkspaceContext). No other major changes.

#### F16 — Rich Repo Previews
Repo metadata fetch phải use per-user GitHub credentials (F30 pattern). No structural change.

#### F19 — Localization
Profile hierarchy (F33) có thể include `locale` preference. No structural change needed.

#### F26 — Multi-Database
Đã thiết kế đúng với `IConnectionPool`, `MigrationRunner`. Minor: cần ensure migration runner handles multi-tenant schema separation nếu shared DB.

---

### ✅ FOUNDATION / PURPOSE-BUILT — Kiến trúc enterprise đã thiết kế đúng

#### F22 — Web Server Mode
Đây là **enabler chính** của toàn bộ enterprise migration. Platform abstraction (`IPlatformServices`, `NodeAdapter`) cho phép chuyển từ Electron sang Server mà không break business logic. ✅ Thiết kế tốt.

#### F23 — Multi-User Auth  
Foundation layer. Session management, bcrypt, HTTP-only cookies — đủ cho enterprise Phase 1. Phase 2 (SSO OIDC/SAML) là critical next step cho enterprise adoption. ✅ Thiết kế tốt.

#### F24 — Per-User Sandbox  
`fork()` per user process là đúng hướng. Process isolation, per-user SQLite, Unix socket routing đã giải quyết data isolation. ✅ Thiết kế tốt.

#### F32 — Team RBAC  
Phase 1 đã implement types + policy resolution. Phase 2 SSO là cần thiết cho enterprise (OIDC/SAML group mapping). ✅ Foundation tốt, cần SSO integration.

#### F33 — User Profile Hierarchy  
Thiết kế 3-tầng (Company → Dept → User) là đúng mô hình enterprise. Deep-merge algorithm, cache TTL 60s, security section lock — đầy đủ. ✅

#### F34 — Project-Dev Server Binding  
Project-centric architecture với `devServerId`, `repoPath`, `ProjectServerRouter` — backbone cho enterprise project isolation. ✅

#### F35 — AI Provider Account Management  
Security model (credentials trên Dev Server, không trên Orca Server), relay AES-256-GCM, 3 scopes, quota tracking — enterprise-ready. ✅

#### F36 — Workflow Orchestration  
Multi-server DAG, template inheritance 3-tầng, sharing model — thiết kế đúng cho enterprise. ✅ Supersedes F14.

#### F37 — Task Graph Management  
Grant system 5-level, AI decomposition, DAG validation — enterprise team management model đúng. ✅

#### F38 — Project Workspace  
WorkspaceContext + RelayConnectionPool — unified enterprise IDE context. ✅

#### F39 — Remote Git UI  
Git operations qua relay, per-user GH_CONFIG_DIR, command injection prevention — secure. ✅

---

## Các Rủi ro Kiến trúc Cần Giải quyết

### R1 — Session Scalability (HIGH RISK)

> Mô hình `fork()` per user (F24) không scale được với nhiều users đồng thời.

**Vấn đề:** Nếu 100 users online, 100 Node.js processes chạy. Với 1000 users, 1000 processes — không khả thi trên 1 server.

**Giải pháp đề xuất:**
- Session pooling: reuse user process nếu idle (hiện tại đã có 4h idle timeout)
- Horizontal scaling: multiple Orca Server instances + session affinity (sticky sessions)
- **Orca Session Server** concept (từ yêu cầu): dedicated session servers → xem xét kiến trúc cluster

---

### R2 — Database Contention (MEDIUM RISK)

> Per-user SQLite databases không scale tốt khi số lượng user lớn.

**Vấn đề:** Mỗi user có `orca.db` SQLite riêng — phù hợp khi ít users, nhưng problematic khi có hàng trăm users với nhiều concurrent operations.

**Giải pháp đề xuất:**
- Migrate heavy-traffic tables (sessions, audit log) sang shared MySQL/PostgreSQL
- Giữ SQLite chỉ cho user-specific local state (terminal scrollback, preferences)
- `IConnectionPool` đã abstract DB layer → migration feasible

---

### R3 — CLI Auth Gap (MEDIUM RISK)

> F09 CLI hiện tại dùng Unix socket (no auth). Enterprise cần auth token cho CLI.

**Giải pháp:**
- Service account API tokens (long-lived, revocable)
- `orca login --server https://orca.company.com --token <service-account-token>`
- Token stored in `~/.orca/credentials` (per-user, local)

---

### R4 — F21 Auto-Update Regression (HIGH RISK)

> Electron auto-update (electron-updater) không áp dụng cho server deployment.

**Giải pháp:** Tách biệt hoàn toàn 2 update paths:
1. **Desktop mode**: giữ nguyên electron-updater
2. **Server mode**: Admin-triggered Docker image update, Fleet Health Monitor verify

---

### R5 — F14 Automation Conflict với F36 (MEDIUM RISK)

> F14 (Automations) và F36 (Workflow Orchestration) có overlap. Cần deprecation plan.

**Giải pháp:**
- F14 simple automations → migrate to F36 single-step workflows
- Migration period: F14 automations auto-converted khi user opens server mode
- F14 document: marked as "Legacy — use F36 instead"

---

### R6 — F03 Mobile Pairing Model (MEDIUM RISK)

> Mobile companion hiện tại pair với Desktop (local peer-to-peer). Không áp dụng khi không có Desktop.

**Giải pháp:**
- Pairing với Orca Web Server (authenticated endpoint)
- Push token registration: `POST /api/mobile/register-push-token` với userId
- Notification routing: Orca Server → APNs/FCM per registered userId

---

### R7 — Agent Detection Cross-Platform (LOW RISK)

> F04 auto-detects agents bằng PATH scan trên local machine. Trong enterprise, agents chạy trên Dev Servers.

**Giải pháp:**
- Agent detection phải chạy via relay: `relay.call('system.detectAgents')` → scan PATH trên Dev Server
- Kết quả cache per `devServerId`

---

## Roadmap Thực hiện Đề xuất

### Phase 0 — Foundation (hiện tại v4.x → v5.0a)
F22 ✅, F23 ✅, F24 ✅, F26 ✅, F32 Phase 1 ✅

### Phase 1 — Core Enterprise (v5.0a)
- F33 Profile Hierarchy → **prerequisite cho tất cả Phase sau**
- F34 Project Binding → **prerequisite cho F38, F39**
- F32 Phase 2 (SSO OIDC/SAML)

### Phase 2 — AI & Credentials (v5.0b)
- F35 AI Provider Account Management → **fix F04 credential model**
- F04 refactor: ProfileAwareAgentSpawner → depends on F35
- F09 CLI auth tokens (service accounts)

### Phase 3 — Workspace (v5.0c)
- F38 Project Workspace (RelayConnectionPool)
- F39 Remote Git UI
- F12, F13 project-scoped file/search

### Phase 4 — Orchestration (v5.0d)
- F37 Task Graph Management
- F36 Workflow Orchestration (replaces F14)
- F01 multi-tenant worktrees
- F17 server-side AI Vault

### Phase 5 — Scale & Harden
- Session Server architecture (cluster mode)
- F21 server update mechanism
- F03 mobile → Web Server pairing
- F27, F28, F31 team-scoped fleet management

---

## Tổng kết theo Priority

| Mức độ ưu tiên | Số features | Action |
|---|---|---|
| 🔴 BREAKING (phải xử lý trước) | 3 (F04, F14, F21) | Redesign hoàn toàn |
| 🟠 MAJOR (refactor) | 9 (F01, F03, F06, F07, F09, F15, F17, F18, F30) | Refactor có thể backward-compat |
| 🟡 MODERATE (extend) | 9 (F02, F11, F12, F13, F25, F27, F28, F29, F31) | Thêm userId/projectId scope |
| 🟢 MINOR (nhỏ) | 6 (F05, F08, F10, F16, F19, F26) | Thêm context, không breaking |
| ✅ PURPOSE-BUILT | 10 (F22–F24, F32–F39) | Đã thiết kế đúng cho enterprise |
| ⚪ NOT IMPACTED | 2 (F20 Speech Input) | Không thay đổi |

**Tổng: 39 features — 100% bị tác động ở các mức độ khác nhau.**

---

*Tài liệu này dựa trên phân tích `docs/hld/` (C1–C4, security, deployment) và `docs/features/F01–F39`.*
