# TASK-030: Sửa `src/server/http-server.ts` — Mount /admin/api + first-run setup

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 4 — Admin Panel
**Solution:** [SOL-LG-004](../solutions/SOL-LG-004-admin-ui.md) §5
**Depends on:** TASK-024, TASK-025, TASK-026, TASK-027, TASK-028, TASK-029
**Blocks:** (Phase 4 complete — toàn bộ 38 tasks xong)

---

## Mục tiêu

Mount admin router vào HTTP server, khởi tạo tất cả handlers, chạy first-run setup. Đây là task kết thúc Phase 4 và toàn bộ project.

---

## Cài đặt dependency

Không cần dependency mới — dùng Express đã có.

---

## File cần sửa

**Path:** `src/server/http-server.ts`

---

## Thay đổi cần thực hiện

### 1. Thêm imports

```typescript
import { createAdminRouter }      from '../main/admin/admin-router'
import { AdminUserHandlers }      from '../main/admin/admin-user-handlers'
import { AdminSessionHandlers }   from '../main/admin/admin-session-handlers'
import { AdminStatsHandler }      from '../main/admin/admin-stats-handler'
import { AdminAuditHandlers }     from '../main/admin/admin-audit-handlers'
import { AuditLogger }            from '../main/admin/audit-logger'
import { ensureFirstAdminUser }   from '../main/admin/first-run-setup'
import type { AuthManager }       from '../main/auth/auth-manager'
```

### 2. Thêm `authManager` vào HttpServerOptions (nếu chưa có từ TASK-012)

```typescript
export interface HttpServerOptions {
  port:        number
  webRoot:     string
  dbMonitor?:  DatabaseHealthMonitor
  authManager: AuthManager    // từ TASK-012 — đã có
}
```

### 3. Khởi tạo và mount admin routes

Trong `startHttpServer()`, sau khi `/auth` routes đã mount (từ TASK-012):

```typescript
// ── Admin Panel ──────────────────────────────────────────────
const db         = options.authManager.sessionStore['db'] as IDatabase  // Access db từ authManager
const auditLogger = new AuditLogger(db)

const adminRouter = createAdminRouter({
  userHandlers: new AdminUserHandlers({
    userStore:    options.authManager.userStore,
    sessionStore: options.authManager.sessionStore,
    auditLogger
  }),
  sessionHandlers: new AdminSessionHandlers({
    sessionStore: options.authManager.sessionStore,
    auditLogger
  }),
  statsHandler: new AdminStatsHandler(db),
  auditHandlers: new AdminAuditHandlers(auditLogger)
})

app.use('/admin/api', adminRouter)

// First-run: seed admin user if none exists
await ensureFirstAdminUser(db, options.authManager.userStore)

// Audit: server start
auditLogger.log({ action: 'server.start', detail: { port: options.port } })
```

---

## Lưu ý về db access

`AuthManager` hiện không expose `db` trực tiếp. Có 2 cách:

**Option A (preferred):** Truyền `db` xuống `startHttpServer()` như là `HttpServerOptions.db`:
```typescript
export interface HttpServerOptions {
  port:        number
  webRoot:     string
  dbMonitor?:  DatabaseHealthMonitor
  authManager: AuthManager
  db:          IDatabase    // ← THÊM
}
```

**Option B:** Expose `db` từ `AuthManager`:
```typescript
// Trong AuthManager constructor:
public readonly db: IDatabase
```

→ Chọn **Option A** (cleaner separation of concerns).

---

## Final Endpoint Map

```
HTTP :6769
  GET  /              → static SPA
  POST /auth/local    → login
  POST /auth/logout   → logout
  GET  /auth/me       → current user
  GET  /admin/api/stats           → dashboard (admin only)
  GET  /admin/api/users           → list users (admin only)
  POST /admin/api/users           → create user (admin only)
  DEL  /admin/api/users/:id       → deactivate user (admin only)
  GET  /admin/api/sessions        → list sessions (admin only)
  DEL  /admin/api/sessions/:id    → kill session (admin only)
  DEL  /admin/api/users/:id/sessions → kill all user sessions (admin only)
  GET  /admin/api/audit           → audit log (admin only)
  GET  /health                    → health check
  GET  /health/ready
  GET  /health/metrics
```

---

## Acceptance Criteria

- [x] Admin router mount tại `/admin/api`
- [x] `ensureFirstAdminUser()` được gọi khi server start
- [x] `auditLogger.log({ action: 'server.start' })` được gọi
- [x] `GET /admin/api/stats` với non-admin cookie → 403
- [x] `GET /admin/api/stats` với admin cookie → 200 JSON `{totalUsers, activeUsers, activeSessions, pairedDevices}`
- [x] `GET /admin/api/audit` với filter `?action=login.success` → filtered events
- [x] TypeScript compile không có lỗi mới
- [x] PairCode / E2EE connections vẫn hoạt động bình thường (không regression)
