# Orca Backend — Technical Design Document v4
## Index & Overview

**Version:** 4.0 (as-implemented — login CRs complete)  
**Date:** 2026-07-28  
**Source:** `src/main/` + `src/shared/` + `src/relay/` + `src/platform/` + `src/server/`  
**Baseline TDD:** [`../00-index.md`](../00-index.md) (v1–v4 history)

---

## Tài liệu trong bộ TDD v4

| # | File | Nội dung | Status |
|---|------|---------|--------|
| 1 | [01-architecture-overview.md](./01-architecture-overview.md) | Kiến trúc tổng thể v4 — process model, platform abstraction, data flow | ✅ |
| 2 | [02-server-entry.md](./02-server-entry.md) | Server Entry Point — startup sequence, env vars, graceful shutdown | ✅ |
| 3 | [03-server-bootstrap.md](./03-server-bootstrap.md) | Server Bootstrap — `initializeOrcaServices()`, `ServerBootstrapResult` | ✅ |
| 4 | [04-http-server.md](./04-http-server.md) | HTTP Server — Express, static files, auth routes, admin routes | ✅ |
| 5 | [05-auth-layer.md](./05-auth-layer.md) | Auth Layer — `AuthManager`, session store, local login, middleware | ✅ |
| 6 | [06-multi-user-sandbox.md](./06-multi-user-sandbox.md) | Multi-User Sandbox — `SessionManager`, fork isolation, `WsSessionRouter` | ✅ |
| 7 | [07-admin-panel.md](./07-admin-panel.md) | Admin Panel — admin-router, user/session CRUD, audit log, first-run | ✅ |
| 8 | [08-dev-server-manager.md](./08-dev-server-manager.md) | Dev Server Manager — CRUD, connection lifecycle, `AgentWebSocketServer` | ✅ |
| 9 | [09-agent-token-api.md](./09-agent-token-api.md) | Agent Token API — `POST /api/agent-token`, auth, `connectDaemonAgent` | ✅ |
| 10 | [10-database-layer.md](./10-database-layer.md) | Database Layer — IConnectionPool, migrations, health, SQLite/MySQL/PG | ✅ |
| 11 | [11-persistence.md](./11-persistence.md) | Persistence — JSON Store, IStateRepository, JsonFileRepository | ✅ |
| 12 | [12-ssh-relay.md](./12-ssh-relay.md) | SSH & Relay — relay deploy, SshChannelMultiplexer, fleet config | ✅ |
| 13 | [13-health-endpoints.md](./13-health-endpoints.md) | Health Endpoints — /health, /health/ready, /health/metrics | ✅ |
| 14 | [14-web-push.md](./14-web-push.md) | Web Push — WebPushManager, VAPID, Push API routes | ✅ |
| 15 | [15-platform-abstraction.md](./15-platform-abstraction.md) | Platform Abstraction — IPlatformServices, NodeAdapter, interfaces | ✅ |

---

## Kiến trúc tổng thể v4

```
┌───────────────────────────────────────────────────────────────────────────┐
│  src/server/index.ts  (Node.js Server Entry)                              │
│                                                                           │
│  1. createNodeAdapter()  →  setPlatform(adapter)                          │
│  2. initializeOrcaServices()  [server-bootstrap.ts]                       │
│     └─ Store, DevServerManager, AgentWebSocketServer,                     │
│        WebPushManager, AuthManager, DB pool, migrations                   │
│  3. startHttpServer(httpPort, webRoot, { authManager, dbMonitor,          │
│                        apiHandler: createAgentTokenApiHandler(...) })     │
│     └─ Express: cookie-parser → AuthMiddleware                            │
│                  /auth/*       → AuthRouter                               │
│                  /admin/api/*  → AdminRouter (requireAdmin)               │
│                  /health/*     → HealthEndpoint                           │
│                  /api/*        → apiHandler (agent-token-routes)          │
│                  (static)      → out/web/ SPA files                       │
│  4. registerPushApiRoutes(httpServer, pushManager)                        │
│  5. agentWsServer.attach(httpServer)   <- ws://:6769/agent                │
│  6. [MULTI_USER=1]  SessionManager + WsSessionRouter                     │
│                     └─ fork per userId → user-process-entry.ts           │
│                        └─ OrcaRuntimeRpcServer (Unix socket)              │
└───────────────────────────────────────────────────────────────────────────┘
                    │                     │
             RPC port 6768         HTTP port 6769
             OrcaRuntimeRpcServer  Express + static files
```

---

## Deployment Targets v4

| Target | Entry | Platform | Transport | DB |
|--------|-------|----------|-----------|-----|
| Electron Desktop | `src/main/index.ts` | ElectronAdapter | IPC + WS | JSON file |
| Web Server | `src/server/index.ts` | NodeAdapter | WS :6768 + HTTP :6769 | SQLite / MySQL / PG / TiDB |
| Docker | `out/server/index.js` | NodeAdapter (built) | WS + HTTP | SQLite (default) or `ORCA_DB_URL` |
| Multi-User | `src/server/index.ts` (ORCA_MULTI_USER=1) | NodeAdapter | HTTP + Unix sockets | SQLite per user |

---

## Environment Variables v4

| Variable | Default | Description |
|----------|---------|-------------|
| `ORCA_PORT` | `6768` | WebSocket/RPC port |
| `ORCA_HTTP_PORT` | `6769` | HTTP port for static files + API |
| `ORCA_USER_DATA_PATH` | `~/.orca` | Override userData directory |
| `ORCA_WEB_ROOT` | `out/web` | Path to web bundle dir |
| `ORCA_VERSION` | — | App version string |
| `ORCA_MULTI_USER` | `0` | Enable per-user process isolation (1 = on) |
| `ORCA_ADMIN_EMAIL` | `admin@localhost` | Initial admin email |
| `ORCA_ADMIN_PASSWORD` | (auto-generated) | Initial admin password |
| `ORCA_DB_URL` | — | Database connection DSN (MySQL/PG/TiDB) |
| `ORCA_DB_DIALECT` | `sqlite` | DB dialect when no DSN given |
| `ORCA_AGENT_API_SECRET` | — | Bearer token bảo vệ POST /api/agent-token |
| `ORCA_SERVER_SECRET` | — | Secret cho SessionManager (HMAC) |

---

## `ServerBootstrapResult` (v4)

```typescript
export interface ServerBootstrapResult {
  shutdown():         Promise<void>
  devServerManager:  DevServerManager      // CRUD + connection lifecycle
  dbMonitor:         HealthChecker         // /health endpoint integration
  pushManager:       WebPushManager        // Web Push VAPID
  authManager:       AuthManager           // Auth facade (login, validate, logout)
  sessionManager:    SessionManager | null // null nếu ORCA_MULTI_USER=0
  agentWsServer:     AgentWebSocketServer  // attach to HTTP server for /agent WS
}
```

---

## Test Coverage Summary (v4)

| Module | Tests |
|--------|-------|
| `src/main/auth/__tests__/` | 40 tests |
| `src/main/session/__tests__/` | 21 tests |
| `src/main/ssh/__tests__/` | 29 tests |
| `src/main/admin/__tests__/` | 44 tests |
| `src/server/__tests__/` | 20 tests |
| `src/main/db/` | 205 tests |
| `src/main/dev-server/__tests__/` | ~60 tests |
| **Total (v4 additions)** | **~419 tests** |

---

## Nguyên tắc thiết kế v4

1. **Process isolation**: PTY daemon riêng, crash không ảnh hưởng server
2. **Security boundary**: WS RPC server là ranh giới duy nhất — mọi request qua auth-token + E2EE
3. **Headless-first**: Server mode chạy hoàn toàn không cần Electron/display
4. **Auth-first**: `AuthManager.validateRequest()` gắn vào mọi WebSocket upgrade và HTTP request
5. **Per-user sandbox**: `ORCA_MULTI_USER=1` → fork() per userId, Unix socket isolation
6. **DB-agnostic**: `IStateRepository` / `IConnectionPool` → SQLite, MySQL, PostgreSQL, TiDB
7. **Platform abstraction**: `src/platform/` — không có `import 'electron'` ngoài adapter
8. **Additive-only**: `src/main/` core không bị sửa — chỉ thêm mới
9. **Observable health**: `/health`, `/health/ready`, `/health/metrics` endpoints
10. **Agent direct-WS**: `AgentWebSocketServer` nhận agents kết nối trực tiếp qua `/agent` path
