# Orca Backend — Technical Design Document
## Index & Overview

**Version:** 4.0 (restructure_v1 + onboarding + remote-server + sql-server + login CRs)
**Date:** 2026-07-23  
**Updated:** 2026-07-24 (v4.0 — login CRs: auth, multi-user sandbox, SSH isolation, admin panel)  
**Source:** `src/main/` + `src/shared/` + `src/relay/` + `src/platform/` + `src/server/`  
**Change Requests:** [restructure_v1](../../../docs/crs/v1/restructure_v1/) | [onboarding](../crs/v1/onboarding/) | [remote-server](../crs/v1/remote-server/) | [sql-server](../crs/v1/sql-server/) | [login](../crs/v1/login/)  
**Solutions:** [restructure_v1 ✅](../crs/v1/restructure_v1/solutions/) | [onboarding ✅](../crs/v1/onboarding/solutions/) | [remote-server ✅](../crs/v1/remote-server/solutions/) | [sql-server ✅](../crs/v1/sql-server/solutions/) | [login ✅](../crs/v1/login/solutions/)  

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
