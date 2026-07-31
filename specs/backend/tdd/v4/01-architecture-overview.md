# TDD-BE-01: Kiến Trúc Tổng Thể Backend v4

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/server/index.ts`, `src/main/server-bootstrap.ts`, `src/platform/`

---

## 1. Process Model

### 1.1 Single-User Mode (ORCA_MULTI_USER=0)

```
Node.js Process (pid=1)
│
├─ HTTP Server (:6769)
│   ├─ Express middleware (cookie-parser → AuthMiddleware)
│   ├─ /auth/*          → AuthRouter (POST /auth/local, /auth/logout, GET /auth/me)
│   ├─ /admin/api/*     → AdminRouter (requireAdmin guard)
│   ├─ /health/*        → HealthEndpoint
│   ├─ /api/agent-token → AgentTokenApiHandler
│   └─ (static)         → out/web/ SPA + index.html fallback
│
├─ WebSocket /agent (:6769) → AgentWebSocketServer
│   └─ SshChannelMultiplexer (binary frame protocol)
│
├─ OrcaRuntimeRpcServer (:6768)
│   └─ OrcaRuntimeService (worktrees, git, PTY, agent hooks)
│
├─ PTY Daemon (out-of-process)
│   └─ IPC via Unix socket / TCP
│
└─ Web Push (VAPID)
    └─ push-api-routes.ts (POST /push/subscribe, etc.)
```

### 1.2 Multi-User Mode (ORCA_MULTI_USER=1)

```
Supervisor Process (pid=1)
│
├─ HTTP Server (:6769)  [shared]
│   ├─ Express (auth, admin, health, agent-token)
│   └─ WebSocket upgrade → WsSessionRouter
│       └─ Validates session cookie (AuthManager)
│           └─ Proxies to per-user Unix socket
│
├─ AgentWebSocketServer (/agent) [shared]
│
└─ SessionManager
    ├─ User A process: users/<userId-A>/orca.sock
    │   └─ OrcaRuntimeRpcServer (Unix socket only)
    ├─ User B process: users/<userId-B>/orca.sock
    │   └─ OrcaRuntimeRpcServer (Unix socket only)
    └─ Idle GC: 4h timeout, 5min check interval
```

---

## 2. Platform Abstraction Layer

### 2.1 Interface (src/platform/types.ts)

```typescript
export interface IPlatformServices {
  app:           IAppInterface      // getPath(), getVersion(), quit()
  ipc:           IIpcInterface      // on(), handle(), send()
  windowManager: IWindowInterface   // showWindow(), hideWindow(), openExternal()
  storage:       IStorageInterface  // getItem(), setItem(), deleteItem()
  system:        ISystemInterface   // platform, homedir, env
}
```

### 2.2 NodeAdapter (src/platform/adapters/node/)

```typescript
// NodeApp — app.getPath() resolves userData, logs, cache
// NodeIpcBridge — no-op IPC (server mode không cần Electron IPC)
// NodeSecureStorage — encrypt với crypto.createCipheriv (không dùng keytar)
// NodeWindowManager — stub (no BrowserWindow in server mode)
```

### 2.3 Singleton pattern

```typescript
// src/platform/context.ts
let platform: IPlatformServices | null = null

export function setPlatform(p: IPlatformServices): void {
  platform = p
}

export function getPlatform(): IPlatformServices {
  if (!platform) throw new Error('Platform not initialized')
  return platform
}
```

> **Quy tắc bất biến:** `setPlatform()` PHẢI được gọi TRƯỚC bất kỳ `import '../main/'` nào.

---

## 3. Data Flow

```
Browser/Agent                    HTTP Server (:6769)           OrcaRuntimeService
    │                                   │                            │
    │  POST /auth/local                 │                            │
    ├──────────────────────────────────▶│                            │
    │  Set-Cookie: session=<token>      │                            │
    │◀──────────────────────────────────│                            │
    │                                   │                            │
    │  WS Upgrade (cookie=session)      │                            │
    ├──────────────────────────────────▶│                            │
    │  [MULTI_USER] WsSessionRouter     │                            │
    │  validates → proxy to Unix socket │                            │
    │                                   │         OrcaRpc            │
    │  WS JSON-RPC calls               ═══════════════════════════▶│
    │◀══════════════════════════════════════════════════════════════│
```

---

## 4. Build Targets

| Script | Config | Output |
|--------|--------|--------|
| `pnpm build` | `electron.vite.config.ts` | `out/main/`, `out/renderer/`, `out/preload/` |
| `pnpm build:backend` | `vite.server.config.ts` | `out/server/` (Node.js) |
| `pnpm build:frontend:web` | `vite.web-spa.config.ts` | `out/web/` (Browser SPA) |
| `pnpm build:server` | All 3 | relay + backend + frontend:web |
| `pnpm dev:web-spa` | `vite.web-spa.config.ts` | Dev server :5174 |

---

## 5. Dependency Graph (tóm tắt)

```
src/server/index.ts
  └── src/platform/adapters/node/   (STEP 1: setPlatform)
  └── src/main/server-bootstrap.ts  (STEP 2: initializeOrcaServices)
        ├── src/main/persistence.ts          (Store, initDataPath)
        ├── src/main/dev-server/             (DevServerManager, AgentWebSocketServer)
        ├── src/main/notifications/          (WebPushManager)
        ├── src/main/auth/                   (AuthManager)
        ├── src/main/db/                     (IConnectionPool, migrations)
        ├── src/main/repositories/           (IStateRepository)
        └── src/main/runtime/               (OrcaRuntimeService)
  └── src/server/http-server.ts             (STEP 3: startHttpServer)
        ├── src/main/auth/auth-middleware.ts
        ├── src/main/auth/auth-router.ts
        ├── src/main/admin/admin-router.ts
        └── src/server/health-endpoint.ts
  └── src/server/agent-token-routes.ts      (STEP 4)
  └── src/server/push-api-routes.ts         (STEP 5)
```
