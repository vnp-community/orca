# TDD-11: Web Server Mode

**Document:** TDD-11 (NEW — restructure_v1)  
**Version:** 1.0  
**Date:** 2026-07-23  
**Domain:** Node.js Web Server — HTTP static files, WebSocket IPC, Docker deployment  
**Source files:**
- `src/server/index.ts`
- `src/server/http-server.ts`
- `src/main/server-bootstrap.ts`
- `vite.server.config.ts`
- `vite.web-spa.config.ts`
- `deploy/prod/`

> **Status: ✅ IMPLEMENTED** — build verified | 20/20 http-server tests | docker-compose valid

---

## 1. Mục tiêu

Web Server Mode cho phép chạy Orca **không cần Electron**:

```
Trước:  orca serve → Electron process (headless) → node-pty, keytar, sqlite
Sau:    docker run → Node.js process              → node-pty, keytar, sqlite
                     (NO Electron dependency)
```

Use cases:
- Deploy lên server VPS/cloud (Docker)
- Truy cập từ Browser bất kỳ qua Web SPA
- CI/CD headless testing
- Lightweight deployment (không cần 200MB+ Electron binaries)

---

## 2. Entry Point (`src/server/index.ts`)

```typescript
// Init sequence — THỨ TỰ QUAN TRỌNG:

// Step 1: Parse config từ CLI args / env
const userDataPath = process.env.ORCA_USER_DATA_PATH ?? os.homedir() + '/.orca'
const rpcPort = parseInt(process.env.ORCA_PORT ?? '6768')
const httpPort = parseInt(process.env.ORCA_HTTP_PORT ?? '6769')
const webRoot = join(__dirname, '../web')  // out/web/

// Step 2: Platform init TRƯỚC KHI import bất kỳ module nào dùng electron alias
import { createNodeAdapter } from '../platform/adapters/node'
import { setPlatform } from '../platform/context'

const adapter = createNodeAdapter(userDataPath ? { userDataPath } : {})
setPlatform(adapter)

// Step 3: Boot services (dynamic import để tránh circular)
const { initializeOrcaServices } = await import('../main/server-bootstrap')
const { shutdown } = await initializeOrcaServices({
  platform: getPlatform(),
  port: rpcPort
})

// Step 4: HTTP server (nếu web bundle tồn tại)
if (existsSync(webRoot)) {
  const { startHttpServer } = await import('./http-server')
  const httpServer = await startHttpServer(httpPort, webRoot)
  console.log(`[Server] Web UI: http://localhost:${httpPort}`)
}

// Step 5: Graceful shutdown
const handleShutdown = async (signal: string) => {
  await shutdown()
  process.exit(0)
}
process.on('SIGINT', () => handleShutdown('SIGINT'))
process.on('SIGTERM', () => handleShutdown('SIGTERM'))
```

---

## 3. Server Bootstrap (`src/main/server-bootstrap.ts`)

```typescript
export interface ServerBootstrapOptions {
  platform: IPlatformServices
  port?: number  // default 6768
}

export interface ServerBootstrapResult {
  shutdown(): Promise<void>
}

export async function initializeOrcaServices(
  options: ServerBootstrapOptions
): Promise<ServerBootstrapResult>
```

**Init sequence bên trong:**

```
1. initDataPath()                  → set userData dir (no-arg, đọc từ app stub)
2. new Store()                     → SQLite/JSON store
3. initStatsPath() + new StatsCollector()
4. initOrcaProfilePaths()          → non-fatal nếu fail
5. initDaemonPtyProvider()         → non-fatal nếu fail (terminal features degrade)
6. new OrcaRuntimeService(store, stats)
7. new OrcaRuntimeRpcServer({
     runtime, userDataPath,
     enableWebSocket: true,
     wsPort: port
   })
8. rpcServer.start()               → WebSocket listening
```

**Non-fatal pattern:**
```typescript
try {
  await initOrcaProfilePaths()
} catch (err) {
  console.warn('[ServerBootstrap] initOrcaProfilePaths failed (non-fatal):', err.message)
  // Server tiếp tục — chỉ profile features bị ảnh hưởng
}
```

---

## 4. HTTP Server (`src/server/http-server.ts`)

### 4.1 Tính năng

```typescript
export async function startHttpServer(
  port: number,
  webRoot: string
): Promise<http.Server>
```

- **Static file serving** — serve `out/web/` với correct MIME types
- **SPA fallback** — mọi route không match file → `web-index.html`
- **Path traversal protection** — reject `/../`, `%2e%2e`, etc. → 403
- **Empty webRoot** — 404 nếu file không tồn tại
- **MIME types** từ file extension:

| Extension | MIME Type |
|-----------|-----------|
| `.js`, `.mjs` | `application/javascript` |
| `.css` | `text/css` |
| `.html` | `text/html; charset=utf-8` |
| `.json` | `application/json` |
| `.png`, `.jpg`, `.gif`, `.svg`, `.ico`, `.webp` | image/* |
| `.woff`, `.woff2` | font/woff(2) |
| `.wasm` | `application/wasm` |

### 4.2 Security

```typescript
// Path traversal check
const resolved = path.resolve(webRoot, decoded)
if (!resolved.startsWith(path.resolve(webRoot))) {
  // 403 Forbidden
}

// URL decode TRƯỚC KHI check
// fetch() normalize '..' client-side, nhưng raw curl có thể gửi encoded
const decoded = decodeURIComponent(urlPath)
```

### 4.3 SPA Fallback Logic

```typescript
// Request: /some/react/route
// 1. Check if file exists: webRoot + /some/react/route → NOT FOUND
// 2. Check if .html variant exists → NOT FOUND
// 3. Serve web-index.html với status 200 (SPA routing)

// Request: /assets/app.js
// 1. Check if file exists: webRoot + /assets/app.js → FOUND
// 2. Serve với correct MIME type
```

---

## 5. Vite Build Configs

### 5.1 `vite.server.config.ts` — Node.js Backend

```typescript
// Target: node22 (production server)
// Input: src/server/index.ts + src/main/daemon/daemon-entry.ts
// Output: out/server/
// Key alias: 'electron' → src/platform/stubs/electron-node-wrapper.ts
// External: node-pty, better-sqlite3, keytar, ssh2, etc.

defineConfig({
  build: {
    target: 'node22',
    lib: { entry: { index: 'src/server/index.ts', 'daemon-entry': '...' } },
  },
  resolve: {
    alias: {
      'electron': resolve('./src/platform/stubs/electron-node-wrapper.ts')
    }
  },
  define: {
    'import.meta.env.ORCA_PLATFORM': '"server"',
    'ORCA_FEATURE_WALL_ENABLED': 'false'
  }
})
```

**Outputs:**
- `out/server/index.js` — entry point (không có `require('electron')`)
- `out/server/daemon-entry.js` — PTY daemon process
- `out/server/*.js` — các chunk khác

### 5.2 `vite.web-spa.config.ts` — Browser SPA

```typescript
// Target: browser (modern)
// Input: src/renderer/web-index.html
// Output: out/web/
// Key alias: 'electron' → src/platform/stubs/electron-web-stub.ts
// Dev server: :5174 với WS proxy → localhost:6768

defineConfig({
  root: 'src/renderer',
  base: './',  // relative paths cho reverse proxy compatibility
  build: {
    outDir: 'out/web',
    rollupOptions: {
      input: { 'web-index': 'src/renderer/web-index.html' }
    }
  },
  resolve: {
    alias: {
      'electron': resolve('./src/platform/stubs/electron-web-stub.ts')
    }
  },
  define: {
    'import.meta.env.ORCA_PLATFORM': '"web"'
  },
  server: {
    port: 5174,
    proxy: {
      '/ws': { target: 'ws://localhost:6768', ws: true },
      '/api': { target: 'http://localhost:6768' }
    }
  }
})
```

**Output:** `out/web/web-index.html` + `out/web/assets/`

---

## 6. Build Scripts (`package.json`)

| Script | Command | Output |
|--------|---------|--------|
| `build:backend` | `vite build -c vite.server.config.ts` | `out/server/` |
| `build:frontend:web` | `vite build -c vite.web-spa.config.ts` | `out/web/` |
| `build:server` | `build:relay + build:backend + build:frontend:web` | All |
| `dev:web-spa` | `vite --config vite.web-spa.config.ts` | Dev server :5174 |
| `dev:web` | `vite --config vite.web.config.ts --host 127.0.0.1` | Electron web dev |

---

## 7. Docker Deployment (`deploy/prod/`)

### 7.1 Dockerfile — Multi-stage

```dockerfile
# Stage 1: builder
FROM node:22-alpine AS builder
# Install pnpm + native build tools (python3, make, g++)
# pnpm install --frozen-lockfile (ALL deps including dev)
# pnpm run build:relay || true
# pnpm run build:backend
# pnpm run build:frontend:web

# Stage 2: runtime
FROM node:22-alpine AS runtime
# Install pnpm + runtime deps (git, openssh-client, bash)
# pnpm install --frozen-lockfile --prod
# pnpm rebuild node-pty better-sqlite3  ← native modules cho Linux/Alpine
# COPY --from=builder /app/out/server ./out/server
# COPY --from=builder /app/out/web ./out/web
```

**Key points:**
- Native modules phải rebuild cho Linux/Alpine platform
- `node-pty` và `better-sqlite3` cần `python3`, `make`, `g++`
- Dev deps không có trong production image

### 7.2 docker-compose.yml

```yaml
services:
  orca-server:
    image: vnpblc/orca-server:${ORCA_VERSION:-latest}
    environment:
      - ORCA_PORT=6768       # WebSocket/RPC
      - ORCA_HTTP_PORT=6769  # HTTP static
      - ORCA_USER_DATA_PATH=/data/orca
    volumes:
      - orca-data:/data/orca   # SQLite + encryption keys
    ports:
      - "${ORCA_RPC_PORT:-6768}:6768"
      - "${ORCA_WEB_PORT:-6769}:6769"
    healthcheck:
      test: wget -qO- http://localhost:6769/
```

### 7.3 Environment Variables

| Variable | Default | Mô tả |
|----------|---------|-------|
| `ORCA_PORT` | `6768` | WebSocket/RPC port |
| `ORCA_HTTP_PORT` | `6769` | HTTP static files port |
| `ORCA_USER_DATA_PATH` | `~/.orca` | Data directory (SQLite, keys) |
| `NODE_ENV` | `production` | Node environment |
| `ORCA_VERSION` | `0.0.0` | Image version tag |

---

## 8. Port Architecture

```
:6768  — WebSocket/RPC
         Client (Browser/App) ←→ OrcaRuntimeRpcServer
         Protocol: JSON-RPC (OrcaRuntimeRpcServer) +
                   IPC-style (WebIpcBridge)
         Auth: token required

:6769  — HTTP
         Browser ←→ startHttpServer()
         Serves: out/web/ static files
         SPA fallback: all routes → web-index.html
         No auth (public static files)
```

---

## 9. Dev Workflow

### Local development

```bash
# Terminal 1: Start backend
pnpm dev:server  # hoặc: node out/server/index.js

# Terminal 2: Start web SPA dev server  
pnpm dev:web-spa
# → http://localhost:5174
# → WS proxy: /ws → localhost:6768

# Production build test
pnpm build:server
node out/server/index.js
# → RPC: ws://localhost:6768
# → Web UI: http://localhost:6769
```

### Docker local test

```bash
docker build -f deploy/prod/Dockerfile -t orca-test .
docker run -p 6768:6768 -p 6769:6769 orca-test
# → http://localhost:6769
```

---

## 10. Test Coverage

| Module | Test File | Tests |
|--------|-----------|-------|
| `http-server.ts` | `src/server/__tests__/http-server.test.ts` | 20/20 |
| `web-ipc-bridge.ts` | `src/platform/adapters/node/__tests__/web-ipc-bridge.test.ts` | 16/16 |
| `context.ts` | `src/platform/__tests__/context.test.ts` | 6/6 |
| NodeAdapter | `src/platform/adapters/node/__tests__/` | 109/109 |
| RPC client | `src/platform/adapters/web/__tests__/rpc-client.test.ts` | 15/15 |

### http-server.test.ts — test cases

```typescript
describe('startHttpServer', () => {
  it('/ → web-index.html')
  it('/some/route → web-index.html (SPA fallback)')
  it('/file.css → correct MIME type text/css')
  it('/file.js → application/javascript')
  it('/../secret → 403 (path traversal)')
  it('/%2e%2e/secret → 403 (encoded path traversal)')
  it('missing webRoot → 404')
  it('startHttpServer(0, webRoot) → resolves with Server')
  // ... 12 total
})
```

---

## Addendum v3.0: HTTP Server Extensions (sql-server + onboarding CRs) — IMPLEMENTED ✅

> **Date:** 2026-07-23

### `startHttpServer()` — Updated Signature

```typescript
export interface HttpServerOptions {
  /** If provided, /health/* routes are exposed before static file serving */
  dbMonitor?: HealthChecker
}

export async function startHttpServer(
  port: number,
  webRoot: string,
  options: HttpServerOptions = {}  // NEW
): Promise<Server>
```

### Health Endpoints (khi `dbMonitor` provided)

Routing priority trong HTTP server:
```
1. /health/*  → healthHandler (nếu dbMonitor được pass)
2. /*          → static files (SPA fallback)
```

| Endpoint | Response | Purpose |
|----------|----------|---------|
| `GET /health` | JSON: `{ status, dialect, latencyMs, poolStats }` | Monitoring dashboard |
| `GET /health/ready` | `200 OK` hoặc `503 Unavailable` | Kubernetes readiness probe |
| `GET /health/metrics` | Prometheus format | Grafana / Prometheus |

Health handler được lazy-created:
```typescript
if (options.dbMonitor) {
  const { createHealthEndpoint } = await import('./health-endpoint')
  healthHandler = createHealthEndpoint(options.dbMonitor, { includePoolStats: true })
}
```

### Push API Routes (onboarding CRs)

```typescript
// src/server/push-api-routes.ts
export function registerPushApiRoutes(
  server: http.Server,
  pushManager: WebPushManager
): void
// Routes:
// POST   /push/subscribe    ← Browser registers push subscription
// DELETE /push/subscribe    ← Browser unregisters
// GET    /push/vapid-key    ← Get server VAPID public key
```

Đăng ký trong `server/index.ts` sau `startHttpServer()`:
```typescript
const { registerPushApiRoutes } = await import('./push-api-routes')
registerPushApiRoutes(httpServer, pushManager)
```

### Updated Server Bootstrap Return Type

```typescript
export interface ServerBootstrapResult {
  shutdown(): Promise<void>
  devServerManager: DevServerManager   // NEW — onboarding CRs
  dbMonitor: HealthChecker            // NEW — sql-server CRs
  pushManager: WebPushManager         // NEW — onboarding CRs
}
```

### Updated Docker Healthcheck

```yaml
# deploy/prod/docker-compose.yml:
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://localhost:6769/health/ready"]
  interval: 30s
  timeout: 5s       # reduced from 10s
  start_period: 15s  # reduced from 20s
  retries: 3

# Optional DB environment:
# - ORCA_DB_URL=mysql://orca_user:${DB_PASSWORD}@db:3306/orca_prod
# - ORCA_DB_URL=postgresql://orca_user:${DB_PASSWORD}@db:5432/orca_prod
# - ORCA_DB_URL=tidb://orca_user:${DB_PASSWORD}@tidb:4000/orca_prod
```

### Updated Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ORCA_PORT` | `6768` | WebSocket/RPC |
| `ORCA_HTTP_PORT` | `6769` | HTTP static + health |
| `ORCA_USER_DATA_PATH` | `~/.orca` | Data dir |
| `ORCA_DB_URL` | _(none)_ | **[NEW]** Full DSN (auto-detect dialect) |
| `ORCA_DB_DIALECT` | `sqlite` | **[NEW]** Dialect override |
| `NODE_ENV` | `production` | Node environment |
| `ORCA_VERSION` | `0.0.0` | Image tag |

Tham khảo:
- [TDD-12: Database Layer](./12-database-layer.md) — health endpoint, dbMonitor
- [TDD-13: Dev Server Onboarding](./13-dev-server-onboarding.md) — pushManager, push API

---

## Addendum v4.0: Auth, Admin Panel & Multi-User Mode (login CRs) — IMPLEMENTED ✅

> **Date:** 2026-07-24 | **Status:** Complete | **Tests:** 134 (auth+session+ssh+admin)

### HTTP Server Route Map (v4.0 complete)

```
HTTP :6769 (Express)
  ├── cookie-parser                           ← parse orca_session cookie
  ├── AuthMiddleware                          ← req.orcaSession populated from DB
  │
  ├── GET  /                                  ← serve out/web/index.html (SPA)
  ├── GET  /assets/*                          ← static assets
  │
  ├── POST /auth/local                        ← email+password → Set-Cookie: orca_session
  ├── POST /auth/logout                       ← clear cookie + revokeSession
  ├── GET  /auth/me                           ← { id, email, name, role } | 401
  ├── GET  /auth/sso/:provider                ← 501 (Phase 2 deferred)
  │
  ├── GET  /admin/api/stats                   ← requireAdmin → { totalUsers, activeUsers, activeSessions }
  ├── GET  /admin/api/users                   ← requireAdmin → OrcaSessionUser[]
  ├── POST /admin/api/users                   ← requireAdmin → create user
  ├── DELETE /admin/api/users/:id             ← requireAdmin → deactivate + kill sessions
  ├── DELETE /admin/api/sessions/:id          ← requireAdmin → kill session
  ├── DELETE /admin/api/users/:id/sessions    ← requireAdmin → kill all user sessions
  ├── GET  /admin/api/audit                   ← requireAdmin → AuditEvent[] (filtered)
  │
  ├── GET  /health                            ← { status, dialect, latencyMs }
  ├── GET  /health/ready                      ← 200 | 503
  └── GET  /health/metrics                    ← Prometheus text
```

### HttpServerOptions (v4.0)

```typescript
interface HttpServerOptions {
  port?: number           // default 6769
  staticRoot?: string     // out/web/
  dbMonitor?: HealthChecker
  authManager?: AuthManager   // [NEW v4.0] — mounts /auth + /admin/api when provided
}
```

### Auth Cookie

```
Set-Cookie: orca_session=<64-hex>; HttpOnly; Secure; SameSite=Lax; Max-Age=28800
```

- TTL: 8h (28800s)
- HttpOnly: không accessible từ JS
- SameSite=Lax: CSRF protection
- Validated per-request bởi `AuthMiddleware` qua `AuthSessionStore.validateSession()`

### Admin Panel Security

```
requireAdmin middleware:
  ├── Check req.orcaSession exists (401 if not)
  └── Check req.orcaSession.role === 'admin' (403 if not)
```

Mọi admin action được log vào `orca_audit_log`:

```typescript
// Action types:
'user.create'     // tạo user mới
'user.deactivate' // deactivate user
'session.kill'    // kill single session
'sessions.kill_all' // kill all sessions của user
'login.success'   // login thành công
'login.failure'   // sai credentials
```

### First Run Setup

```typescript
// server-bootstrap.ts — gọi khi khởi động:
await ensureFirstAdminUser(authDb, authManager.userStore)
// → Nếu không có admin nào → tạo 1 admin với random password
// → In credentials ra stdout (one-time)
// → Idempotent: không làm gì nếu admin đã tồn tại
```

### Multi-User WebSocket Routing (ORCA_MULTI_USER=1)

```
WebSocket request → WsSessionRouter
  ├── ORCA_MULTI_USER=0 → delegate to rpcServer (backward compat)
  └── ORCA_MULTI_USER=1 → SessionManager.getOrSpawn(userId)
        ├── userId từ req.orcaSession (cookie auth)
        ├── Spawn isolated process → userData/users/<userId>/orca.sock
        └── Proxy WS ↔ Unix socket
```

### Updated Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ORCA_PORT` | `6768` | WebSocket/RPC |
| `ORCA_HTTP_PORT` | `6769` | HTTP static + health + auth + admin |
| `ORCA_USER_DATA_PATH` | `~/.orca` | Data dir |
| `ORCA_DB_URL` | _(none)_ | Full DSN (auto-detect dialect) |
| `ORCA_DB_DIALECT` | `sqlite` | Dialect override |
| `ORCA_MULTI_USER` | `0` | **[NEW v4.0]** Per-user process isolation |
| `NODE_ENV` | `production` | Node environment |

Tham khảo:
- [TDD-04: RPC Server](./04-rpc-server.md) — auth flow, session model
- [TDD-05: SSH Relay](./05-ssh-relay.md) — SSH user isolation
- [TDD-06: Persistence](./06-persistence.md) — migration 0005, auth schema
- `src/server/http-server.ts`, `src/main/auth/`, `src/main/admin/`, `src/main/session/`
