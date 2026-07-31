# TDD-BE-07: Admin Panel

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/main/admin/`

---

## 1. Module Map

| File | Role |
|------|------|
| `admin-types.ts` | `AdminStats`, `AuditEvent`, role types |
| `audit-logger.ts` | Sync SQLite writes tới `orca_audit_log` |
| `admin-middleware.ts` | `requireAdmin` guard (role === 'admin') |
| `admin-router.ts` | Express router `/admin/api/*` |
| `admin-user-handlers.ts` | CRUD users |
| `admin-session-handlers.ts` | Kill sessions |
| `admin-stats-handler.ts` | Aggregate stats |
| `admin-audit-handlers.ts` | Query audit log |
| `first-run-setup.ts` | `ensureFirstAdminUser()` — seed on first boot |

---

## 2. API Routes

```
GET    /admin/api/stats
  → { totalUsers, activeUsers, activeSessions, totalAuditEvents }

GET    /admin/api/users
  → OrcaUser[]

POST   /admin/api/users
  Body: { email, name, password, role }
  → Created OrcaUser

DELETE /admin/api/users/:id
  → Deactivate user (soft delete)

GET    /admin/api/sessions
  → OrcaSession[] (active)

DELETE /admin/api/sessions/:id
  → Kill session

DELETE /admin/api/users/:id/sessions
  → Kill all sessions of user

GET    /admin/api/audit?userId=&action=&page=&pageSize=
  → { events: AuditEvent[], total, page, pageSize }
```

---

## 3. `requireAdmin` Middleware

```typescript
function requireAdmin(req, res, next) {
  if (!req.orcaSession) return res.status(401).json({ error: 'unauthenticated' })
  if (req.orcaSession.role !== 'admin') return res.status(403).json({ error: 'forbidden' })
  next()
}
```

---

## 4. AuditLogger

```typescript
class AuditLogger {
  logAction(
    userId:  string,
    email:   string,
    action:  string,   // e.g., 'user.create', 'session.kill', 'user.login'
    detail:  string,
    ip:      string
  ): void              // SYNC write tới orca_audit_log
}
```

**Audit events:** `user.login`, `user.logout`, `user.create`, `user.deactivate`, `session.kill`, `ssh.connect`, `ssh.provision`

---

## 5. `ensureFirstAdminUser()`

```typescript
async function ensureFirstAdminUser(
  userStore: AuthUserStore,
  env: {
    adminEmail?:    string    // ORCA_ADMIN_EMAIL (default: admin@localhost)
    adminPassword?: string    // ORCA_ADMIN_PASSWORD (auto-generate if not set)
  }
): Promise<void>
```

Logic:
1. Kiểm tra có admin nào trong `orca_users` không
2. Nếu không có → tạo user với role='admin'
3. Nếu `ORCA_ADMIN_PASSWORD` không set → log generated password (warn)
4. Idempotent: gọi nhiều lần không tạo duplicate

---

## 6. AdminStats

```typescript
export type AdminStats = {
  totalUsers:       number
  activeUsers:      number   // active=true trong orca_users
  activeSessions:   number   // chưa expired
  totalAuditEvents: number
}
```

---

## 7. AuditEvent

```typescript
export type AuditEvent = {
  id:        number
  userId:    string | null
  email:     string | null
  action:    string
  detail:    string
  ip:        string | null
  createdAt: number          // Unix ms
}
```

---

## 8. Tests (44 tests)

| File | Tests |
|------|-------|
| `admin-user-handlers.test.ts` | 12 |
| `admin-session-handlers.test.ts` | 8 |
| `admin-stats-handler.test.ts` | 6 |
| `admin-audit-handlers.test.ts` | 8 |
| `audit-logger.test.ts` | 6 |
| `first-run-setup.test.ts` | 4 |
