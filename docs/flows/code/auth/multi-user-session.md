# Multi-User Web Server Mode — Session & Sandbox Flow

> **Scope**: Luồng xác thực và cô lập người dùng khi Orca chạy ở chế độ Web Server (F22 Web Server Mode, F23 Multi-User Auth, F24 Per-User Sandbox)
>
> **Key files**:
> - [`src/server/index.ts`](../../src/server/index.ts) — Bootstrap web server
> - [`src/main/auth/auth-manager.ts`](../../src/main/auth/auth-manager.ts) — AuthManager: bcrypt + session cookie
> - [`src/main/auth/session-manager.ts`](../../src/main/auth/session-manager.ts) — SessionManager: HTTP-only session cookie
> - [`src/main/runtime/ws-session-router.ts`](../../src/main/runtime/ws-session-router.ts) — WsSessionRouter: per-user WS fork
> - [`src/main/runtime/runtime-rpc.ts`](../../src/main/runtime/runtime-rpc.ts) — OrcaRuntimeRpcServer
> - **Business Logic**: [BL-AUTH-01](../logic/auth/BL-AUTH-01-local-login.md), [BL-AUTH-02](../logic/auth/BL-AUTH-02-session-management.md), [BL-AUTH-03](../logic/auth/BL-AUTH-03-per-user-sandbox.md)

---

## 1. Tổng quan kiến trúc

```
Browser                   Orca Web Server (NodeAdapter)
  │                               │
  │  GET /                        │
  │ ─────────────────────────────►│ → serve web SPA (out/web/index.html)
  │                               │
  │  POST /auth/local             │
  │  { email, password }          │
  │ ─────────────────────────────►│ AuthManager.login()
  │                               │ → bcrypt.compare(pw, hash, 12r)
  │                               │ → INSERT orca_sessions (token, userId, expires_at +8h)
  │◄──────────────────────────────│ Set-Cookie: orca_session=<token>; HttpOnly; SameSite=Strict
  │                               │
  │  WS ws://:6768/ + Cookie      │
  │ ─────────────────────────────►│ WsSessionRouter.onConnection()
  │                               │ → validateSessionCookie(cookie)
  │                               │ → getScopedRuntimeForUser(userId) → Fork process
  │                               │
  │  JSON-RPC (per-user scope)    │
  │◄─────────────────────────────►│ OrcaRuntimeRpcServer (user-isolated)
```

---

## 2. Bootstrap: Web Server Startup

### 2.1 Entry Point

```typescript
// src/server/index.ts
import { NodeAdapter } from '../platform/adapters/node'
import { bootstrapWebApp } from '../main/server-bootstrap'

const nodeAdapter = new NodeAdapter({
  userDataPath: process.env.ORCA_USER_DATA_PATH ?? path.join(os.homedir(), '.orca')
})

await bootstrapWebApp(nodeAdapter)
```

### 2.2 bootstrapWebApp() Sequence

```
bootstrapWebApp(nodeAdapter)
    │
    ├── DatabaseProvider.createPool(parseDsn(ORCA_DB_URL))
    │   → MigrationRunner.run(db)         [0001 → 0010]
    │   → Verify all migrations applied
    │
    ├── AuthManager.init(db)
    │   → Load bcrypt config (12 rounds)
    │   → Load session TTL (8h default)
    │
    ├── SessionManager.init(db)
    │   → Prune expired sessions (background job mỗi 1h)
    │
    ├── AgentWebSocketServer.init()       [F29 Agent WS]
    │   → WS path: /agent
    │
    ├── FleetHealthMonitor.start()        [F27 Fleet]
    │   → Background health ping mỗi 30s
    │
    ├── Express HTTP server :6769
    │   ├── POST /auth/local
    │   ├── POST /auth/logout
    │   ├── GET  /auth/me
    │   ├── GET  /admin/api/* (requireAdmin middleware)
    │   ├── GET  /health/ready
    │   ├── GET  /health/metrics (Prometheus)
    │   └── GET  /* → serve SPA
    │
    └── WebSocket server :6768
        ├── /         ← browser RPC (WsSessionRouter)
        └── /agent    ← agent connections (AgentWebSocketServer)
```

---

## 3. Authentication Flow (F23 Multi-User Auth)

### 3.1 Local Login

```
Browser                    Express /auth/local          AuthManager          DB (orca_users/sessions)
  │                              │                           │                        │
  │ POST /auth/local             │                           │                        │
  │ { email, password }          │                           │                        │
  │──────────────────────────────►                           │                        │
  │                              │ AuthManager.login(email, pw)                       │
  │                              │──────────────────────────►│                        │
  │                              │                           │ SELECT * FROM orca_users
  │                              │                           │ WHERE email = ? AND is_active = 1
  │                              │                           │──────────────────────────────────►│
  │                              │                           │◄──────────────────────────────────│
  │                              │                           │ { id, email, role, password_hash } │
  │                              │                           │                        │
  │                              │                           │ bcrypt.compare(pw, hash, 12r)
  │                              │                           │ [CPU-bound ~100ms]
  │                              │                           │
  │                              │                           │ if FAIL → return { error: 'invalid_credentials' }
  │                              │                           │
  │                              │                           │ token = randomBytes(32).toString('hex') // 64-hex
  │                              │                           │ expires_at = now + 8h
  │                              │                           │ INSERT orca_sessions (token, userId, expires_at, ...)
  │                              │                           │──────────────────────────────────►│
  │                              │◄──────────────────────────│                        │
  │                              │ { success: true, user: { id, email, name, role } } │
  │◄─────────────────────────────│                           │                        │
  │ Set-Cookie: orca_session=<token>; HttpOnly; Secure; SameSite=Strict; Max-Age=28800
  │ { success: true, user: { ... } }
```

### 3.2 Session Validation Middleware

```typescript
// src/main/auth/session-manager.ts
async validateSession(token: string): Promise<OrcaUser | null> {
  const session = await db.queryOne(
    'SELECT s.token, s.user_id, s.expires_at, u.email, u.name, u.role, u.is_active ' +
    'FROM orca_sessions s JOIN orca_users u ON s.user_id = u.id ' +
    'WHERE s.token = ? AND s.expires_at > ? AND u.is_active = 1',
    [token, Date.now()]
  )
  if (!session) return null

  // Slide expiry: reset expires_at mỗi request (sliding window)
  await db.run(
    'UPDATE orca_sessions SET last_seen_at = ?, expires_at = ? WHERE token = ?',
    [Date.now(), Date.now() + SESSION_TTL_MS, token]
  )

  return { id: session.user_id, email: session.email, name: session.name, role: session.role }
}
```

### 3.3 DB Schema (Migration 0003–0005)

```sql
-- Migration 0003: Auth sessions
CREATE TABLE orca_workspace_sessions (
  token       TEXT PRIMARY KEY,
  user_id     TEXT NOT NULL REFERENCES orca_users(id),
  device_name TEXT,
  created_at  INTEGER NOT NULL,
  expires_at  INTEGER NOT NULL,
  last_seen_at INTEGER DEFAULT 0
);
CREATE INDEX idx_sessions_user ON orca_workspace_sessions(user_id);
CREATE INDEX idx_sessions_expires ON orca_workspace_sessions(expires_at);

-- Migration 0004: Users table
CREATE TABLE orca_users (
  id           TEXT PRIMARY KEY,
  email        TEXT UNIQUE NOT NULL,
  name         TEXT NOT NULL,
  role         TEXT DEFAULT 'developer',   -- developer | lead | admin
  is_active    INTEGER DEFAULT 1,
  password_hash TEXT,                       -- bcrypt 12r (null nếu SSO only)
  department_id TEXT,                       -- [v5.0] REFERENCES orca_departments(id)
  profile_json  TEXT DEFAULT '{}',          -- [v5.0] Per-user OrcaProfile JSON
  created_at   INTEGER,
  updated_at   INTEGER
);

-- Migration 0005: Audit log + RBAC policies
CREATE TABLE orca_audit_log (
  id        TEXT PRIMARY KEY,
  actor_id  TEXT REFERENCES orca_users(id),
  action    TEXT NOT NULL,
  target    TEXT,
  metadata  TEXT DEFAULT '{}',
  timestamp INTEGER NOT NULL,
  ip        TEXT
);
-- append-only: no UPDATE, no DELETE
```

---

## 4. Per-User Sandbox (F24)

### 4.1 WsSessionRouter — WebSocket Fork

```typescript
// src/main/runtime/ws-session-router.ts

class WsSessionRouter {
  // Map: userId → OrcaRuntimeRpcServer (isolated runtime per user)
  private userRuntimes = new Map<string, OrcaRuntimeRpcServer>()

  onConnection(ws: WebSocket, req: IncomingMessage): void {
    const token = extractCookie(req, 'orca_session')
    const user = await this.sessionManager.validateSession(token)

    if (!user) {
      ws.close(4401, 'Unauthorized')
      return
    }

    const runtime = this.getOrCreateUserRuntime(user.id)
    runtime.handleWebSocketConnection(ws, user)
  }

  private getOrCreateUserRuntime(userId: string): OrcaRuntimeRpcServer {
    if (this.userRuntimes.has(userId)) {
      return this.userRuntimes.get(userId)!
    }
    // Fork: tạo isolated runtime cho user này
    const runtime = new OrcaRuntimeRpcServer({
      userId,
      userDataPath: path.join(this.baseDataPath, 'users', userId),
      db: this.db,
    })
    this.userRuntimes.set(userId, runtime)
    return runtime
  }
}
```

### 4.2 Per-User Isolation Boundaries

```
┌─── OrcaRuntimeRpcServer (User: alice) ──────────────────────────┐
│  PTY sessions: Map { ptyId → PtyHandle }  ← alice's terminals only
│  Worktrees: from projects where alice is member                 │
│  Orchestration tasks: alice's agent runs                        │
│  SSH connections: via RelayConnectionPool (shared pool, scoped) │
│  File path: ~/.orca/users/alice-uuid/                           │
└──────────────────────────────────────────────────────────────────┘

┌─── OrcaRuntimeRpcServer (User: bob) ────────────────────────────┐
│  PTY sessions: Map { ptyId → PtyHandle }  ← bob's terminals only
│  Worktrees: from projects where bob is member                   │
│  File path: ~/.orca/users/bob-uuid/                             │
└──────────────────────────────────────────────────────────────────┘

Shared:
  ├── DB pool (IConnectionPool) — queries filtered by userId/projectId
  ├── RelayConnectionPool — SSH relay bridges (shared, userId injected in context)
  ├── FleetHealthMonitor — fleet state cache (shared)
  └── AgentWebSocketServer — agent WS connections (shared, per devServerId)
```

---

## 5. Admin Panel (F25)

### 5.1 Admin Routes

```typescript
// src/server/admin-routes.ts
function requireAdmin(req, res, next) {
  const user = req.session?.user
  if (!user || user.role !== 'admin') {
    return res.status(403).json({ error: 'admin_required' })
  }
  next()
}

router.use('/admin/api', requireAdmin)

// Users CRUD
router.get('/admin/api/users',          listUsers)
router.post('/admin/api/users',         createUser)
router.put('/admin/api/users/:id',      updateUser)
router.delete('/admin/api/users/:id',   deactivateUser)

// Sessions management
router.get('/admin/api/sessions',       listActiveSessions)
router.delete('/admin/api/sessions/:id', revokeSession)

// Audit log
router.get('/admin/api/audit',          getAuditLog)

// Server stats
router.get('/admin/api/stats',          getServerStats)
```

### 5.2 Admin SPA

```
Browser → GET /admin
        → serve admin-spa/index.html (separate Vite bundle)
        → React SPA với routes:
           /admin/users
           /admin/sessions
           /admin/audit
           /admin/fleet
           /admin/ai-providers    [v5.0]
           /admin/departments     [v5.0]
```

---

## 6. Session Lifecycle

```
[Login]
  POST /auth/local { email, pw }
    → bcrypt verify → INSERT orca_sessions
    → Set-Cookie: orca_session=<token>; HttpOnly; 8h

[Active Usage]
  WS connect với cookie
    → validateSession(token) → user found
    → WsSessionRouter → getOrCreateUserRuntime(userId)
    → sliding expiry: expires_at += 8h mỗi request

[Logout]
  POST /auth/logout
    → DELETE FROM orca_sessions WHERE token = ?
    → Set-Cookie: orca_session=; Max-Age=0
    → WsSessionRouter: close WS connections cho user này

[Expiry]
  Background job (mỗi 1h):
    DELETE FROM orca_sessions WHERE expires_at < ?
    → Purge expired → free resources

[Admin Revoke]
  DELETE /admin/api/sessions/:id
    → DELETE FROM orca_sessions
    → Close active WS connections
    → Audit log: { action: 'session.revoke', actor: adminId, target: sessionId }
```

---

## 7. Audit Log

```typescript
// Mọi action quan trọng đều ghi vào orca_audit_log
// Pattern: append-only, không update/delete

async function auditLog(
  actor: OrcaUser,
  action: string,
  target?: string,
  metadata?: object
): Promise<void> {
  await db.run(
    'INSERT INTO orca_audit_log (id, actor_id, action, target, metadata, timestamp, ip) VALUES (?,?,?,?,?,?,?)',
    [randomUUID(), actor.id, action, target ?? null, JSON.stringify(metadata ?? {}), Date.now(), getClientIp()]
  )
}

// Các action được audit:
// auth.login.success | auth.login.failed
// auth.logout
// user.create | user.update | user.deactivate
// session.revoke
// server.connect | server.disconnect
// agent.spawn | agent.stop
// worktree.create | worktree.delete
// ai_provider.credential.write
```

---

## 8. Security Properties

| Property | Cơ chế | Level |
|---------|--------|-------|
| **Password hashing** | bcrypt 12 rounds | Mạnh |
| **Session token** | randomBytes(32) = 64-hex | Mạnh |
| **Cookie security** | HttpOnly, SameSite=Strict, Secure | Mạnh |
| **Session TTL** | 8h sliding window | Vừa |
| **User isolation** | Per-user runtime fork | Mạnh |
| **RBAC** | Role: developer/lead/admin | Cơ bản |
| **Audit trail** | append-only orca_audit_log | Có |
| **Brute force** | ❌ Chưa có rate limiting login | Thiếu |
| **2FA** | ❌ Chưa implement (profile security.require2FA) | Thiếu |

---

## 9. Cross-References

| Flow liên quan | Mô tả |
|---|---|
| [profile-resolution.md](./profile-resolution.md) | Resolve profile sau khi user login |
| [project-workspace-switch.md](./project-workspace-switch.md) | User switch project sau khi authenticated |
| [authentication.md](./authentication.md) | E2EE auth (desktop mode) |
| [relay-management.md](./relay-management.md) | SSH relay reuse per user context |
| **HLD C1 Flow 5** | Web Server Multi-User flow |
| **BL-AUTH-01** | Local login business logic |
| **BL-AUTH-02** | Session management business logic |
| **BL-AUTH-03** | Per-user sandbox business logic |
| **BL-AUTH-04** | Admin user CRUD |
| **BL-AUTH-05** | Audit log |
