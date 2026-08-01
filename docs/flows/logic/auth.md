# Luồng Dữ liệu — Authentication & User Management

**Domain:** Auth & User Management  
**Nghiệp vụ:** BL-AUTH-01 → BL-AUTH-05  
**Kiến trúc tham chiếu:** HLD v1 — Orca Web Server, C3.1, C4.2, ADR-001/003, F22/F23/F24

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Browser (Web Client) | UI | Login form, Admin SPA |
| Orca Web Server (Express) | Backend | HTTP :6769, REST API |
| AuthManager | Business Logic | bcrypt verify, session create |
| SessionManager | Business Logic | Per-user child process management |
| WsSessionRouter | Business Logic | WebSocket → user workspace routing |
| Server Database | Persistence | orca_users, orca_sessions, orca_audit_log |
| Child Process (per user) | Runtime | Isolated user workspace |

---

## BL-AUTH-01 — Local Login (email + password)

```
Người dùng (bất kỳ)
    │
    ▼
[Browser] POST /auth/local
    Body: { email: "alex@co.com", password: "secret123" }
    │
    ▼
[Orca Web Server — AuthRouter.localLogin()]
    ├─ SELECT user FROM orca_users WHERE email=? AND is_active=1  ← Server DB
    ├─ bcrypt.compare(password, user.passwordHash)  [12 rounds]
    ├─ IF fail: INSERT orca_audit_log { action: 'login.fail' }  ← DB
    │           return 401 Unauthorized
    ├─ Generate session token: crypto.randomBytes(32).toString('hex')
    ├─ INSERT orca_sessions { token, userId, expiresAt: now+8h, lastSeenAt }  ← DB
    ├─ INSERT orca_audit_log { action: 'login.success' }  ← DB
    └─ Set-Cookie: orca_session=<token>; HttpOnly; Secure; SameSite=Strict
    │
    ▼
[Browser] redirect to /app
    └─ WebSocket connect: ws://orca:6768 (với cookie orca_session)

Luồng:
Browser → POST /auth/local → Express → bcrypt verify
                           → Server DB (SELECT user)
                           → Server DB (INSERT session + audit)
                           → Set-Cookie → Browser
Browser → WebSocket :6768 (cookie auth) → WsSessionRouter
```

---

## BL-AUTH-02 — Session Management & Isolation

```
[Browser] WebSocket connect ws://orca:6768
    Cookie: orca_session=<token>
    │
    ▼
[WsSessionRouter.route()]
    ├─ SELECT session FROM orca_sessions WHERE token=? AND expiresAt > now  ← DB
    ├─ IF invalid: close WebSocket 401
    ├─ UPDATE orca_sessions SET lastSeenAt=now  ← DB
    ├─ Lookup SessionManager: childProcess[userId]?
    │   IF not found: fork new child process
    │       child = fork('orca-user-worker', [], {
    │           env: { USER_ID: userId, DATA_PATH: ~/.orca/users/<userId>/ }
    │       })
    │       Open Unix Socket: ~/.orca/users/<userId>/orca.sock
    └─ Proxy WebSocket ↔ Child Process (Unix Socket)
    │
    ▼
[Child Process] nhận WebSocket messages → xử lý trong isolated workspace

Per-User Data:
~/.orca/users/<userId>/
    orca.sock      ← Unix socket
    orca.db        ← Per-user SQLite
    credentials.enc ← AES-256-GCM tokens
    worktrees/     ← Git worktrees

SESSION EXPIRY:
    SessionManager timer: check idle > 4h → graceful kill child process
    Admin kill:       DELETE /admin/api/sessions/:id → SIGTERM child

Luồng:
Browser WS → WsSessionRouter → Server DB (verify session)
           → SessionManager → fork()/reuse child process
           → proxy: WS ↔ Unix Socket ↔ Child Process
```

---

## BL-AUTH-03 — Per-User Process Sandbox

```
[SessionManager.fork(userId)]
    │
    ▼
[Child Process bootstrap]
    ├─ Set USER_ID, DATA_PATH environment
    ├─ Open per-user SQLite: DATA_PATH/orca.db
    ├─ Create Unix Socket: DATA_PATH/orca.sock
    ├─ Initialize: WorktreeManager, AgentManager, TerminalManager
    │   (all scoped to DATA_PATH/worktrees/)
    └─ Emit: child:ready

[Isolation guarantees]
    ├─ Memory: separate V8 heap (fork = copy-on-write)
    ├─ File system: all ops scoped to DATA_PATH
    ├─ SSH connections: isolated SSH connection pool per child
    └─ Database: separate SQLite file per user

[Child crash handling]
    process.on('exit') → SessionManager detects
    IF exit code != 0: respawn (max 3 attempts)
    IF persistent crash: alert admin + mark session broken

Luồng:
SessionManager → fork(userId) → Child Process (isolated Node.js)
                              → per-user SQLite + Unix Socket
                              → WorktreeManager (scoped ops)
```

---

## BL-AUTH-04 — Admin User CRUD & Session Kill

```
Admin
    │
    ▼
[Browser Admin SPA] POST /admin/api/users
    Headers: Cookie: orca_session=<admin_token>
    Body: { email, name, role, password }
    │
    ▼
[Orca Web Server — AdminRouter]
    ├─ requireAdmin() guard:
    │   SELECT user.role WHERE token=? → must be 'admin'
    ├─ Validate input (Zod schema)
    ├─ Check email uniqueness
    ├─ bcrypt.hash(password, 12)
    ├─ INSERT orca_users { id, email, name, role, passwordHash }  ← DB
    └─ INSERT orca_audit_log { action: 'user.create' }  ← DB

DEACTIVATE USER:
    PATCH /admin/api/users/:id { is_active: false }
    ├─ UPDATE orca_users SET is_active=0
    ├─ DELETE orca_sessions WHERE userId=?  ← DB (kick all sessions)
    ├─ SessionManager: SIGTERM child process
    └─ audit_log: 'user.deactivate'

KILL SESSION:
    DELETE /admin/api/sessions/:id
    ├─ DELETE orca_sessions WHERE id=?  ← DB
    ├─ WsSessionRouter: drop WebSocket connection
    └─ audit_log: 'session.kill'

Luồng:
Admin → Browser → POST /admin/api/* → AdminRouter (requireAdmin guard)
                                    → Server DB (CRUD)
                                    → SessionManager (kill child if needed)
                                    → AuditLog (INSERT)
```

---

## BL-AUTH-05 — Audit Log

```
[writeAudit() helper] — called internally by AuthManager / AdminRouter
    │
    ▼
INSERT orca_audit_log {
    id: uuid(),
    actor_id: <userId hoặc null>,
    action: 'login.success' | 'user.create' | ...,
    target_type: 'user' | 'session' | 'ssh_host',
    target_id: <id>,
    metadata: JSON.stringify({ ip, extra }),
    ip_address: req.ip,
    created_at: new Date().toISOString()
}

Admin Query:
    GET /admin/api/audit?action=login.fail&from=2026-07-01&page=1
    ├─ requireAdmin() guard
    ├─ SELECT * FROM orca_audit_log WHERE action=? AND created_at>=? ORDER BY created_at DESC
    └─ Return paginated JSON

Export:
    GET /admin/api/audit/export?format=csv
    └─ Stream CSV response (no memory buffering)

Luồng:
Internal event → writeAudit() → Server DB (INSERT orca_audit_log)
Admin → GET /admin/api/audit → AdminRouter → Server DB (SELECT paginated)
                             → JSON/CSV response
```

---

## Sơ đồ tổng quan — Auth & User Management

```
┌─────────────┐   HTTP/WS   ┌────────────────────────────────────┐
│  Browser    │◄───────────►│  Orca Web Server (Express)         │
│  Login form │             │  AuthRouter: POST /auth/local       │
│  Admin SPA  │             │  AdminRouter: /admin/api/*         │
│  App UI     │             │  WsSessionRouter: ws://orca:6768   │
└─────────────┘             └──────────┬─────────────────────────┘
                                       │
                            ┌──────────▼──────────────────────────┐
                            │  Server Database                     │
                            │  orca_users                         │
                            │  orca_sessions                      │
                            │  orca_audit_log                     │
                            │  orca_access_policies               │
                            └──────────┬──────────────────────────┘
                                       │
                            ┌──────────▼──────────────────────────┐
                            │  SessionManager                     │
                            │  Per-user Child Processes           │
                            │  ├── userId-A → Child(A) ← sock A  │
                            │  ├── userId-B → Child(B) ← sock B  │
                            │  └── userId-C → Child(C) ← sock C  │
                            └─────────────────────────────────────┘

WsSessionRouter proxies:
Browser WS ↔ WsSessionRouter ↔ Unix Socket ↔ Child Process
```
