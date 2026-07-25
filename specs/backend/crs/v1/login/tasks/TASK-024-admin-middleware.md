# TASK-024: Tạo `src/main/admin/admin-middleware.ts`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 4 — Admin Panel
**Solution:** [SOL-LG-004](../solutions/SOL-LG-004-admin-ui.md) §4.3
**Depends on:** TASK-009 (auth-middleware — đã có `req.orcaSession`)
**Blocks:** TASK-031 (admin-router)

---

## Mục tiêu

Tạo `requireAdmin` middleware — guard các routes `/admin/api/*`, chỉ cho phép users có `role = 'admin'`.

---

## File cần tạo

**Path:** `src/main/admin/admin-middleware.ts`

---

## Nội dung

```typescript
// src/main/admin/admin-middleware.ts
import type { Request, Response, NextFunction } from 'express'

/**
 * Guard: chỉ cho phép requests từ user có role = 'admin'.
 * Phải đặt SAU createAuthMiddleware() (vì cần req.orcaSession).
 *
 * Trả về:
 * - 401 nếu không có session (chưa login)
 * - 403 nếu có session nhưng không phải admin
 */
export function requireAdmin(req: Request, res: Response, next: NextFunction): void {
  const session = req.orcaSession  // Populated bởi createAuthMiddleware()

  if (!session) {
    res.status(401).json({
      error:   'unauthenticated',
      message: 'Login required'
    })
    return
  }

  if (session.role !== 'admin') {
    res.status(403).json({
      error:         'forbidden',
      message:       'Admin role required',
      required_role: 'admin',
      your_role:     session.role
    })
    return
  }

  next()
}
```

---

## Acceptance Criteria

- [x] File tồn tại, TypeScript compile sạch
- [x] `requireAdmin` trả về 401 khi không có `req.orcaSession`
- [x] `requireAdmin` trả về 403 khi `session.role !== 'admin'`
- [x] `requireAdmin` gọi `next()` khi `session.role === 'admin'`
- [x] Response JSON có `error` field trong cả 2 error cases
