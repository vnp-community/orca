# TASK-029: Tạo `src/main/admin/admin-router.ts`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 4 — Admin Panel
**Solution:** [SOL-LG-004](../solutions/SOL-LG-004-admin-ui.md) §4.8
**Depends on:** TASK-024, TASK-025, TASK-026, TASK-027
**Blocks:** TASK-030 (http-server integration)

---

## Mục tiêu

Tạo Express Router mount tất cả admin routes tại `/admin/api/*`. Guard tất cả bằng `requireAdmin`.

---

## File cần tạo

**Path:** `src/main/admin/admin-router.ts`

---

## Nội dung

```typescript
// src/main/admin/admin-router.ts
import { Router } from 'express'
import { requireAdmin } from './admin-middleware'
import type { AdminUserHandlers }    from './admin-user-handlers'
import type { AdminSessionHandlers } from './admin-session-handlers'
import type { AdminStatsHandler }    from './admin-stats-handler'
import type { AdminAuditHandlers }   from './admin-audit-handlers'

export function createAdminRouter(deps: {
  userHandlers:    AdminUserHandlers
  sessionHandlers: AdminSessionHandlers
  statsHandler:    AdminStatsHandler
  auditHandlers:   AdminAuditHandlers
}): Router {
  const router = Router()

  // Apply requireAdmin guard to ALL admin routes
  router.use(requireAdmin)

  // ── Stats Dashboard ──────────────────────────────────────────
  router.get('/stats', deps.statsHandler.getStats)

  // ── Users ────────────────────────────────────────────────────
  router.get('/users',        deps.userHandlers.listUsers)
  router.post('/users',       deps.userHandlers.createUser)
  router.delete('/users/:id', deps.userHandlers.deactivateUser)

  // ── Sessions ─────────────────────────────────────────────────
  router.get('/sessions',                       deps.sessionHandlers.listAllSessions)
  router.delete('/sessions/:sessionId',         deps.sessionHandlers.killSession)
  router.delete('/users/:userId/sessions',      deps.sessionHandlers.killAllUserSessions)

  // ── Audit Log ────────────────────────────────────────────────
  router.get('/audit', deps.auditHandlers.queryAuditLog)

  return router
}
```

---

## HTTP API Reference

| Method | Path | Handler | Role required |
|--------|------|---------|---------------|
| `GET` | `/admin/api/stats` | `getStats` | admin |
| `GET` | `/admin/api/users` | `listUsers` | admin |
| `POST` | `/admin/api/users` | `createUser` | admin |
| `DELETE` | `/admin/api/users/:id` | `deactivateUser` | admin |
| `GET` | `/admin/api/sessions` | `listAllSessions` | admin |
| `DELETE` | `/admin/api/sessions/:sessionId` | `killSession` | admin |
| `DELETE` | `/admin/api/users/:userId/sessions` | `killAllUserSessions` | admin |
| `GET` | `/admin/api/audit` | `queryAuditLog` | admin |

---

## Acceptance Criteria

- [x] File tồn tại, TypeScript compile sạch
- [x] `requireAdmin` được dùng bằng `router.use()` (áp dụng cho tất cả routes)
- [x] Tất cả 8 routes đã mount
- [x] `GET /admin/api/stats` → 403 nếu không phải admin
- [x] `GET /admin/api/stats` → 200 JSON nếu là admin
