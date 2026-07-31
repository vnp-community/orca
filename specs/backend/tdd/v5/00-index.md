# Orca Backend — Technical Design Document
## Index & Overview

**Version:** 5.0 (restructure_v1 + onboarding + remote-server + sql-server + login + v5.0 workspace CRs)
**Date:** 2026-07-23  
**Updated:** 2026-07-30 (v5.0 — cross-references with HLD v1 C3, C4 + backend-server-architecture.md)  
**Source:** `src/main/` + `src/shared/` + `src/relay/` + `src/platform/` + `src/server/`  
**Change Requests:** [restructure_v1](../../../docs/crs/v1/restructure_v1/) | [onboarding](../crs/v1/onboarding/) | [remote-server](../crs/v1/remote-server/) | [sql-server](../crs/v1/sql-server/) | [login](../crs/v1/login/)  
**Solutions:** [restructure_v1 ✅](../crs/v1/restructure_v1/solutions/) | [onboarding ✅](../crs/v1/onboarding/solutions/) | [remote-server ✅](../crs/v1/remote-server/solutions/) | [sql-server ✅](../crs/v1/sql-server/solutions/) | [login ✅](../crs/v1/login/solutions/)  
**HLD Reference:** [backend-server-architecture.md](../../../docs/hld/backend-server-architecture.md) | [HLD v1 C3](../../../docs/hld/v1/C3-components.md) | [HLD v1 C4](../../../docs/hld/v1/C4-code.md)  

---

## Tài liệu trong bộ TDD này

| # | File | Nội dung | Cập nhật |
|---|------|---------|----------|
| 1 | [01-architecture-overview.md](./01-architecture-overview.md) | Kiến trúc tổng thể, process model, data flow | ✅ v2.0 |
| 2 | [02-main-process.md](./02-main-process.md) | Electron Main Process — startup, lifecycle, services | ✅ v2.0 |
| 3 | [03-daemon-layer.md](./03-daemon-layer.md) | PTY Daemon — out-of-process terminal engine | v1.x (unchanged) |
| 4 | [04-rpc-server.md](./04-rpc-server.md) | WebSocket/Unix RPC Server — API surface, auth, E2EE | ✅ v4.0 |
| 5 | [05-ssh-relay.md](./05-ssh-relay.md) | SSH & Relay — remote connection, relay deploy, relay protocol | ✅ v4.0 |
| 6 | [06-persistence.md](./06-persistence.md) | Persistence Layer — JSON store, schema, migrations | ✅ v4.0 |
| 7 | [07-runtime-service.md](./07-runtime-service.md) | OrcaRuntimeService — core business logic, worktrees, git | v1.x (unchanged) |
| 8 | [08-agent-orchestration.md](./08-agent-orchestration.md) | Agent Orchestration — multi-agent fan-out, hooks | v1.x (unchanged) |
| 9 | [09-ipc-handlers.md](./09-ipc-handlers.md) | IPC Handlers — Electron + Web IPC, filesystem, PTY, SSH | ✅ v2.0 |
| 10 | [10-platform-layer.md](./10-platform-layer.md) | Platform Abstraction Layer — interfaces, adapters, stubs | ✅ v2.0 |
| 11 | [11-web-server-mode.md](./11-web-server-mode.md) | Web Server Mode — Node.js server, HTTP, WebSocket | ✅ v4.0 |
| 12 | [12-database-layer.md](./12-database-layer.md) | **[NEW]** Database Abstraction — provider, pool, migration, repository, health | ✅ v3.0 |
| 13 | [13-dev-server-onboarding.md](./13-dev-server-onboarding.md) | **[NEW]** Dev Server Management, Onboarding, Fleet, Web Push | ✅ v3.0 |

---

## Tóm tắt kiến trúc v2.0

```
┌──────────────────────────────────────────────────────────────────────────┐
│                   PLATFORM ABSTRACTION LAYER (NEW)                       │
│                         src/platform/                                    │
│                                                                          │
│  IPlatformServices = { app, ipc, windowManager, storage, system }        │
│                                                                          │
│  setPlatform() / getPlatform()  ← singleton, init trước tất cả          │
│                                                                          │
│  ┌──────────────────────┐   ┌──────────────────────────────────────┐    │
│  │   ElectronAdapter    │   │   NodeAdapter (web/server mode)      │    │
│  │   (Electron mode)    │   │   src/platform/adapters/node/        │    │
│  │   [future]           │   │   NodeApp + NodeWindow +             │    │
│  └──────────────────────┘   │   NodeIpcBridge + NodeSecureStorage  │    │
│                             └──────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────────────┘
                    ↑ electron-node-wrapper.ts delegates here

┌──────────────────┐    ┌──────────────────────────────────────────────┐
│  ELECTRON MODE   │    │  WEB SERVER MODE (NEW)                       │
│  src/main/       │    │  src/server/index.ts                         │
│  index.ts        │    │                                              │
│                  │    │  1. createNodeAdapter()                      │
│  Electron app    │    │  2. setPlatform(adapter)                     │
│  lifecycle       │    │  3. initializeOrcaServices()                 │
│  BrowserWindow   │    │     (server-bootstrap.ts)                    │
│  ipcMain         │    │  4. startHttpServer()  ← serves out/web/     │
│  safeStorage     │    │  5. OrcaRuntimeRpcServer (WebSocket :6768)   │
└──────────────────┘    └──────────────────────────────────────────────┘
        │                               │
        └──────────────┬────────────────┘
                       ▼
        src/main/runtime/orca-runtime.ts
        src/main/persistence.ts        (JSON file store — Electron compat)
        src/main/db/                   [NEW v3.0] Database abstraction layer
        src/main/repositories/         [NEW v3.0] IStateRepository + adapters
        src/main/daemon/ (PTY)
        src/main/ipc/ (handlers — shared!)
        src/main/dev-server/           [NEW v3.0] DevServerManager
        src/main/notifications/        [NEW v3.0] WebPushManager
        src/main/ssh/fleet-*.ts        [NEW v3.0] Fleet management
```

---

## Deployment Targets v3.0

| Target | Entry | Platform | Transport | DB |
|--------|-------|----------|-----------|----|
| Electron Desktop | `src/main/index.ts` | ElectronAdapter (via Electron) | IPC + WS + Unix | JSON file |
| Web Server | `src/server/index.ts` | NodeAdapter | WS :6768 + HTTP :6769 | SQLite / MySQL / PG / TiDB |
| Docker | `out/server/index.js` | NodeAdapter (built) | WS + HTTP | SQLite (default) or `ORCA_DB_URL` |

---

## Build Targets v2.0

| Script | Config | Output |
|--------|--------|--------|
| `pnpm build` | `electron.vite.config.ts` | `out/main/`, `out/renderer/`, `out/preload/` |
| `pnpm build:backend` | `vite.server.config.ts` | `out/server/` (Node.js) |
| `pnpm build:frontend:web` | `vite.web-spa.config.ts` | `out/web/` (Browser SPA) |
| `pnpm build:server` | All 3 | relay + backend + frontend:web |
| `pnpm dev:web-spa` | `vite.web-spa.config.ts` | Dev server :5174 |

---

## Nguyên tắc thiết kế v3.0

1. **Process isolation**: PTY daemon chạy riêng, crash không ảnh hưởng server
2. **Security boundary**: WebSocket RPC server là ranh giới duy nhất — mọi request đều qua auth-token + E2EE
3. **Headless-first**: Server mode chạy hoàn toàn không cần Electron/display
4. **Idempotent relay**: Relay binary deploy tự động, versioned, GC old versions
5. **DB-agnostic persistence** _(sql-server CRs)_: `IStateRepository` / `IConnectionPool` abstraction → SQLite, MySQL, PostgreSQL, TiDB
6. **Platform abstraction** _(restructure_v1)_: `src/platform/` — không có `import 'electron'` ngoài adapter
7. **Additive only** _(restructure_v1)_: `src/main/` không bị sửa
8. **Test isolation**: Platform + DB adapters test trong Node.js vitest, không cần Electron/real DB
9. **Multi-server fleet** _(remote-server CRs)_: `DevServerManager` quản lý N remote dev servers
10. **Observable health** _(sql-server CRs)_: `/health`, `/health/ready`, `/health/metrics` endpoints

---

## Addendum A: restructure_v1 — COMPLETE ✅

> **Status:** 19/19 tasks | 166/166 tests | 5/5 solutions

Key new files: `src/platform/` (all adapters + stubs), `src/main/server-bootstrap.ts`, `src/server/http-server.ts`, `vite.server.config.ts`, `vite.web-spa.config.ts`, `deploy/prod/` (Dockerfile, compose, deploy.sh).

---

## Addendum B: onboarding CRs (CR-OB-001~009) — COMPLETE ✅

> **Status:** 9/9 solutions | Phase 1, 2, 3 all done  
> **TDD:** [TDD-13: Dev Server Onboarding](./13-dev-server-onboarding.md)

Key new files:
- `src/shared/dev-server-types.ts` — DevServer, WindowsTerminalCapabilities
- `src/main/dev-server/` — DevServerManager, relay bridge, preflight
- `src/main/ipc/dev-server-ipc.ts`, `onboarding-ipc.ts`, `repo-remote-ipc.ts`
- `src/main/notifications/web-push-manager.ts` — VAPID + push subscriptions
- `src/server/push-api-routes.ts` — Push API HTTP routes

Changes to `server-bootstrap.ts`: `devServerManager`, `pushManager`, `dbMonitor` returned.

---

## Addendum C: remote-server CRs (CR-001~006) — COMPLETE ✅ (Phase 1+2)

> **Status:** 25/25 tasks | Phase 1 + 2 done | Phase 3 (OIDC) deferred  
> **TDD:** [TDD-13: Dev Server Onboarding §8](./13-dev-server-onboarding.md#8-fleet-management)

Key new files:
- `src/main/ssh/fleet-config-parser.ts` — YAML fleet config + Zod
- `src/main/ssh/fleet-bootstrap-service.ts` — 7-step bootstrap pipeline
- `src/main/ssh/fleet-health-monitor.ts` — Periodic health + webhook
- `src/shared/fleet-types.ts`, `rbac-types.ts`
- `src/main/audit/audit-log.ts` — NDJSON audit trail
- `src/cli/handlers/fleet.ts`, `src/cli/specs/fleet.ts` — Fleet CLI

---

## Addendum D: sql-server CRs (CR-001~006) — COMPLETE ✅

> **Status:** 6/6 solutions | 16 test files | 205 tests  
> **TDD:** [TDD-12: Database Layer](./12-database-layer.md)

Key new files:
- `src/main/db/` — types, registry, pool, migrations (3), sqlite + mysql + pg adapters
- `src/main/db/health.ts`, `health-monitor.ts` — HealthChecker
- `src/main/db/config.ts`, `dsn-parser.ts`, `config-loader.ts` — Config + DSN
- `src/main/repositories/` — IStateRepository, JsonFileRepository, SqlStateRepository, factory
- `src/server/health-endpoint.ts` — `/health`, `/health/ready`, `/health/metrics`

Changes to `server-bootstrap.ts`: connection pool init, auto-migration, health monitor, `dbMonitor` returned.  
Changes to `src/server/http-server.ts`: `HttpServerOptions.dbMonitor` → routes `/health/*`.  
Changes to `deploy/prod/docker-compose.yml`: `ORCA_DB_URL` env comment, healthcheck → `/health/ready`.

---

## Addendum E: login CRs (CR-LOGIN-001~004) — COMPLETE ✅

> **Status:** 30/30 tasks | 4/4 solutions | 134/134 tests | 0 TS errors | 163/163 Acceptance Criteria  
> **Date:** 2026-07-24  
> **Solutions:** [login solutions ✅](../crs/v1/login/solutions/)  
> **TDD refs:** TDD-04 (RPC/Auth), TDD-05 (SSH), TDD-06 (Persistence), TDD-11 (Web Server), TDD-12 (DB)

### CR-LOGIN-001: Auth Layer (Login + SSO + Session)

Key new files:
- `src/main/auth/auth-types.ts` — `OrcaSession`, `OrcaSessionUser`, `SESSION_TTL_MS` (8h)
- `src/main/auth/auth-session-store.ts` — CRUD sessions trong SQLite, cleanup expired
- `src/main/auth/auth-user-store.ts` — bcrypt hash/verify, `upsertSsoUser()`, deactivate
- `src/main/auth/auth-local-handler.ts` — email/password login với email format validation
- `src/main/auth/auth-manager.ts` — Facade: `validateRequest()`, `login()`, `logout()`, 30-min cleanup interval
- `src/main/auth/auth-middleware.ts` — Express middleware, `requireAuth()` guard, `req.orcaSession` populated
- `src/main/auth/auth-router.ts` — `POST /auth/local`, `POST /auth/logout`, `GET /auth/me`, `GET /auth/sso/:provider`

Changes to `src/main/db/migrations/`: Migration **0005_add_auth_schema** — tables `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies`.

Changes to `src/server/http-server.ts`: `cookie-parser` → `AuthMiddleware` → `AuthRouter` on `/auth`. `authManager` passed in options.

Changes to `src/main/server-bootstrap.ts`: `AuthManager` initialized (dedicated SQLite connection), `ensureFirstAdminUser` on first boot, `authManager` returned in result.

### CR-LOGIN-002: Per-User Sandbox (Process Isolation)

Key new files:
- `src/main/session/session-types.ts` — `UserProcess`, `SessionManagerConfig`, `ORCA_MULTI_USER` env flag
- `src/main/session/session-manager.ts` — Fork per userId, Unix socket isolation, idle timeout 4h, 30s spawn timeout, `--max-old-space-size=512`
- `src/main/session/ws-session-router.ts` — WS proxy supervisor → user socket keyed by `req.orcaSession.userId`
- `src/main/session/user-process-entry.ts` — Fork entry point: reads `ORCA_USER_ID`, `ORCA_SOCKET_PATH`, starts per-user OrcaRuntimeRpcServer

Isolation model:
```
Supervisor process (server-bootstrap)
  └── fork() per user → user-process-entry.ts
        ├── ORCA_USER_ID=<uuid>
        ├── ORCA_SOCKET_PATH=userData/users/<userId>/orca.sock
        └── OrcaRuntimeRpcServer (Unix socket only)
```

### CR-LOGIN-003: SSH Dev Server Isolation

Key new files:
- `src/main/ssh/ssh-user-resolver.ts` — `toLinuxUsername(email, uid)` → `'orca-<prefix>'`, collision-safe hashing
- `src/main/ssh/dev-server-provisioner.ts` — Idempotent `provision()`: create linux user, deploy relay, authorize SSH key; logs `ssh.connect` to audit

Changes to `src/main/ssh/ssh-connection-store.ts`: per-user connection store resolution with `userId` routing.

### CR-LOGIN-004: Admin Panel

Key new files:
- `src/main/admin/admin-types.ts` — `AdminStats`, `AuditEvent`, role types
- `src/main/admin/audit-logger.ts` — Sync SQLite writes to `orca_audit_log`; `logAction(userId, email, action, detail, ip)`
- `src/main/admin/admin-middleware.ts` — `requireAdmin` guard (role === 'admin'); 403 if not
- `src/main/admin/admin-user-handlers.ts` — `GET/POST/DELETE /admin/api/users` (list, create, deactivate)
- `src/main/admin/admin-session-handlers.ts` — `DELETE /admin/api/sessions/:id`, `DELETE /admin/api/users/:id/sessions`
- `src/main/admin/admin-stats-handler.ts` — `GET /admin/api/stats` → `{ totalUsers, activeUsers, activeSessions }`
- `src/main/admin/admin-audit-handlers.ts` — `GET /admin/api/audit?userId=&action=` with pagination
- `src/main/admin/admin-router.ts` — `/admin/api/*` router; all routes behind `requireAdmin`
- `src/main/admin/first-run-setup.ts` — `ensureFirstAdminUser()`: seeds admin if no admin in `orca_users`; idempotent

Changes to `src/server/http-server.ts`: Mount `/admin/api` router; `AuditLogger`, admin handlers wired.

### Updated Environment Variables (v4.0)

| Variable | Default | Description |
|----------|---------|-------------|
| `ORCA_MULTI_USER` | `0` | **[NEW]** Enable per-user process isolation (`1` = on) |

### Updated `ServerBootstrapResult` (v4.0)

```typescript
export interface ServerBootstrapResult {
  shutdown(): Promise<void>
  devServerManager: DevServerManager   // onboarding CRs
  dbMonitor: HealthChecker            // sql-server CRs
  pushManager: WebPushManager         // onboarding CRs
  authManager: AuthManager            // [NEW v4.0] login CRs
}
```

### Test Coverage (v4.0 additions)

| Module | Tests |
|--------|-------|
| `src/main/auth/__tests__/` | 40 tests |
| `src/main/session/__tests__/` | 21 tests |
| `src/main/ssh/__tests__/` | 29 tests |
| `src/main/admin/__tests__/` | 44 tests |
| **Total new** | **134 tests** |

---

## v5.0 — New TDDs

| TDD | Domain | Feature(s) | ADR | HLD Ref | Status |
|-----|--------|-----------|-----|---------|--------|
| [TDD-14](./14-profile-hierarchy.md) | User Profile Hierarchy | F33, F34 | ADR-007 | C3.10, C4.7 | 🚧 In-Progress |
| [TDD-15](./15-project-binding.md) | Project-Dev Server Binding | F34 | ADR-007, ADR-011 | C3.10, C4.8 | 🚧 In-Progress |
| [TDD-16](./16-ai-provider-management.md) | AI Provider Account Management | F35 | ADR-008 | C3.11a, C4.9 | 🚧 In-Progress |
| [TDD-17](./17-workflow-orchestration.md) | Multi-Server Workflow Orchestration | F36 | ADR-009 | C3.11c, C4.9 | 🚧 In-Progress |
| [TDD-18](./18-task-graph.md) | Task Graph Management | F37 | ADR-010 | C3.11b, C4.9 | 🚧 In-Progress |
| [TDD-19](./19-project-workspace.md) | Project Workspace | F38 | ADR-011 | C3.12, C4.10 | 🚧 In-Progress |
| [TDD-20](./20-remote-git-ui.md) | Remote Git UI | F39 | ADR-012 | C3.12, C4.10 | 🚧 In-Progress |

---

## Addendum F: v5.0 HLD Cross-References (2026-07-30)

> **Nguồn:** [backend-server-architecture.md](../../../docs/hld/backend-server-architecture.md), [HLD v1 C3](../../../docs/hld/v1/C3-components.md), [HLD v1 C4](../../../docs/hld/v1/C4-code.md)

### F.1 Backend — Vai trò và vị trí trong hệ thống

Orca Backend Server là **Control Plane** — không tự thực thi code, chỉ orchestrate:

| Vai trò | Module | HLD Ref |
|---------|--------|---------|
| Auth & Session Gateway | `AuthManager`, `WsSessionRouter` | C3.9 |
| RPC Router | `WsSessionRouter` → per-user process | C3.4 |
| Fleet Manager | `FleetHealthMonitor` (60s poll) | C3.6 |
| Profile Resolver | `ProfileResolver` (3-layer, cache 60s) | C3.10 |
| Project Registry | `ProjectService` + `ProjectServerRouter` | C3.10 |
| AI Provider Manager | `AIProviderService` + `ProviderCredentialRelay` | C3.11a |
| Workflow Orchestrator | `WorkflowOrchestrator` + `DAGBuilder` | C3.11c |
| Task Graph | `TaskService` + `TaskAgentExecutor` | C3.11b |
| Admin Panel Host | Express `/admin/api/*` | C3.9 |
| Agent WebSocket Hub | `AgentWebSocketServer` (direct-ws mode) | C3.8 |

### F.2 Web Server Bootstrap Flow (từ HLD §4)

```
src/server/index.ts
    │
    ├── new NodeAdapter({ userDataPath: ~/.orca })
    ├── bootstrapWebApp(nodeAdapter)
    │        ├── AuthManager.init(db)
    │        ├── SessionManager.init()          ← fork per user
    │        ├── AgentWebSocketServer.init()    ← direct-ws mode
    │        └── FleetHealthMonitor.start()     ← 60s poll
    │
    ├── Express HTTP :6769
    │        ├── POST /auth/local
    │        ├── GET  /admin/api/*
    │        ├── GET  /health/ready
    │        └── GET  /health/metrics
    │
    └── WebSocket Server :6768
             ├── /           → WsSessionRouter (per-user Unix socket)
             └── /agent      → AgentWebSocketServer

// ServerBootstrapResult (v5.0):
export interface ServerBootstrapResult {
  shutdown(): Promise<void>
  devServerManager: DevServerManager   // onboarding CRs
  dbMonitor: HealthChecker             // sql-server CRs
  pushManager: WebPushManager          // onboarding CRs
  authManager: AuthManager             // login CRs
  profileService: ProfileService       // [v5.0]
  projectService: ProjectService       // [v5.0]
  aiProviderService: AIProviderService // [v5.0]
  workflowOrchestrator: WorkflowOrchestrator // [v5.0]
  taskService: TaskService             // [v5.0]
}
```

### F.3 Per-User Session Isolation (từ HLD §7)

```
User → POST /auth/local → orca_session cookie
    → WS connect → WsSessionRouter validates cookie → userId
    → SessionManager.getOrCreate(userId)
              → fork() user-process-entry.ts
                    ORCA_USER_ID=<uuid>
                    ORCA_SOCKET_PATH=~/.orca/users/<userId>/orca.sock
                    OrcaRuntimeRpcServer (Unix socket only)
    → Idle timeout: 4h → kill
    → Max respawns: 3 per hour
```

### F.4 Backend → Dev Server: 3 Connection Modes (từ HLD §6.2)

```
Mode 1: relay-ssh
    Backend → ssh2 library → SSH exec channel
              → SshChannelMultiplexer → JSON-RPC 2.0

Mode 2: relay-websocket
    Backend → HTTP Upgrade ws://agent:PORT/orca-relay
              Header: Authorization: Bearer <agentToken>
              → WsTransport → JSON-RPC 2.0

Mode 3: direct-websocket (Agent chủ động)
    Dev Server Agent → wss://backend:6768/agent
              Handshake: { agentToken } → { sessionId }
              → AgentWebSocketServer → JSON-RPC 2.0
```

### F.5 DB Schema — Full Migration List (từ HLD §10)

| Migration | Tables mới | v5.0 TDD |
|-----------|-----------|----------|
| 0001 `init` | projects, worktrees, agent_sessions, settings | existing |
| 0002 `sessions` | terminal_scrollback_snapshots | existing |
| 0003 `ssh_targets` | ssh_hosts, saved_port_forwards | existing |
| 0004 `automations` | automations, automation_runs, notifications, rate_limits | existing |
| 0005 `auth_schema` | orca_users, orca_sessions, orca_audit_log, orca_access_policies | login CRs |
| **0006** `profile` | **orca_company, orca_departments** + ALTER orca_users | **TDD-14** |
| **0007** `project` | **orca_projects, orca_project_members** | **TDD-15** |
| **0008** `ai_providers` | **orca_ai_provider_accounts, orca_provider_usage** | **TDD-16** |
| **0009** `workflow` | **orca_workflow_templates, orca_workflow_executions, orca_step_executions** | **TDD-17** |
| **0010** `task_graph` | **orca_tasks, orca_task_edges, orca_task_grants, orca_task_comments** | **TDD-18** |

### F.6 RPC Method Namespaces (từ HLD §9)

| Namespace | Tổng methods | Xử lý tại | TDD |
|-----------|-------------|-----------|-----|
| `profile.*` | 8 | ProfileService (Backend) | TDD-14 |
| `projects.*` | 9 | ProjectService (Backend) | TDD-15 |
| `ai-providers.*` | 5 | AIProviderService + relay | TDD-16 |
| `workflows.*` | 7 | WorkflowOrchestrator | TDD-17 |
| `tasks.*` | 11 | TaskService + relay | TDD-18 |
| `credentials.*` | 4 | WebCredentialStore (Backend) | TDD-11 |
| `preflight.*` | 1 | local + relay merge | TDD-11 |
| `git.*` | ~10 | Relay → Dev Server GitEngine | TDD-20 |
| `fs.*` | ~5 | Relay → Dev Server FsEngine | TDD-19 |
| `pty.*` | ~5 | Relay → Dev Server PtyManager | TDD-04 |

### F.7 Communication Matrix (từ HLD §8)

| From | To | Protocol | Port/Path | Format |
|------|----|----------|-----------|--------|
| Browser | Orca HTTP | HTTPS | `:6769` | REST JSON |
| Browser | Orca WS | WebSocket | `:6768/` | JSON-RPC (binary frames) |
| Dev Agent | Orca WS | WebSocket | `:6768/agent` | Binary + JSON-RPC |
| Orca | Dev Server | SSH exec | `:22` | relay protocol |
| Orca | Dev Server | WebSocket | `agent:PORT/orca-relay` | Binary + JSON-RPC |
| Orca | AI Agent CLIs | PTY stdin/stdout | — | Text + OSC sequences |
| Orca | GitHub/GitLab | HTTPS | `:443` | REST/GraphQL JSON |
| Orca | Linear/Jira | HTTPS | `:443` | REST JSON |
| Orca | Database | SQL | DSN | IConnectionPool |
| Orca | Mobile | WebSocket | dynamic | TweetNaCl encrypted |
| CLI | Daemon | Unix Socket | `~/.orca/daemon.sock` | NDJSON |

### F.8 RBAC — hasPermission() (từ HLD C4.6)

```typescript
// Roles:
type UserRole = 'admin' | 'lead' | 'developer'
type ProjectRole = 'developer' | 'lead' | 'admin'

// hasPermission(user, resource, action) policy:
// admin   → ALL resources, ALL actions
// lead    → project management + dept profile
// developer → own profile + assigned tasks

// Per-resource checks:
// profile.updateCompany  → requireAdmin
// profile.updateDept     → requireLead(deptId)
// profile.updateUser     → self OR admin
// projects.create        → requireLead
// ai-providers.*         → requireAdmin | requireLead(serverId)
// tasks.grant            → requireManageGrant(taskId)
// workflows.run          → requireProjectMember(projectId)
```

### F.9 ProfileResolver — Deep Merge Logic (từ HLD C4.7)

```typescript
// ProfileResolver.resolve(userId): ResolvedProfile
// 1. ProfileCache.get(userId) → HIT? return (TTL 60s)
// 2. MISS:
//    companyProfile = db.getCompanyProfile()
//    deptProfile    = db.getDeptProfile(user.deptId)
//    userProfile    = db.getUserProfile(userId)
// 3. deepMergeProfiles(company, dept, user):
//    - Scalars: user > dept > company (user wins)
//    - Arrays (pathAdditions): CONCATENATE [company, dept, user]
//    - Maps (envVars): user > dept > company
//    - LOCKED (security section): ALWAYS company value, user/dept ignored
// 4. Validate: preferredModel ∈ approvedModels (company whitelist)
// 5. Build _sources map: { 'agent.preferredModel': 'dept', 'shell.defaultShell': 'company' }
// 6. Cache (60s) + return ResolvedProfile

// Cache invalidation events:
// 'profile.company.updated' → clear ALL cached profiles
// 'profile.dept.updated'    → clear profiles of dept members
// 'profile.user.updated'    → clear userId profile
```

### F.10 WorkflowOrchestrator — DAG Dispatch (từ HLD C4.9)

```typescript
// WorkflowOrchestrator.run(templateId, inputs, projectId)
// 1. Load template → resolve inheritance (parent template merge)
// 2. DAGBuilder.build(steps) → validate no cycles
// 3. Topological sort → execution waves
// 4. Wave execution (parallel):
//      wave[0]: steps with no dependencies
//      wave[1]: steps whose deps in wave[0] completed
// 5. Per step:
//      StepType='agent' → relay.call('agent.spawn', { model, initFile, cwd })
//      StepType='shell' → relay.call('shell.exec', { command, cwd, env })
//      StepType='action' → built-in actions (git.commit, github.createPR...)
// 6. StreamStepOutput: websocket event 'workflow.step.output' { executionId, stepId, chunk }
// 7. On step complete → record orca_step_executions, advance to next wave
```

### F.11 TaskService — Grant Resolution (từ HLD C4.9)

```typescript
// hasTaskAccess(userId, taskId, level: TaskGrantLevel)
// Priority order: owner > admin > user > team > company
// apply_tree: grant propagates to ALL subtasks (BFS)
// Expiry: grant.expires_at check

// tasks.runAgent(taskId, worktreeId?)
// 1. TaskGrantService.hasTaskAccess(userId, taskId, 'execute')
// 2. FleetHealthMonitor.check(devServerId) → healthy
// 3. AIProviderResolver.resolve(userId, projectId) → accountId
// 4. ProfileResolver.resolve(userId) → profile
// 5. TaskAgentExecutor.buildPreamble(task, project, user, deps)
//    → "# Task Context\nTask: <title>\nDeps: <dep-titles>\n..."
// 6. relay.call('agent.spawn', { model, trustPreset, env, cwd, initFile })
//    env: { ORCA_TASK_ID, ORCA_USER_ID, ORCA_PROJECT_ID, ...profile.shell.envVars }
// 7. UPDATE orca_tasks SET agent_session_id, status='in_progress'
// 8. Push 'task.status.updated' → all subscribers
```
