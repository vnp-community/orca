# TASK-009: Tạo `src/main/auth/auth-middleware.ts`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 1 — Auth Foundation
**Solution:** [SOL-LG-001](../solutions/SOL-LG-001-auth-session.md) §4.6
**Depends on:** TASK-008
**Blocks:** TASK-010, TASK-012, TASK-029 (admin middleware)

---

## Mục tiêu

Tạo Express middleware xác thực session cookie và attach `req.orcaSession` vào request. Thêm `requireAuth` guard cho protected routes.

---

## File cần tạo

**Path:** `src/main/auth/auth-middleware.ts`

---

## Nội dung

```typescript
// src/main/auth/auth-middleware.ts
import type { Request, Response, NextFunction } from 'express'
import type { AuthManager } from './auth-manager'
import type { OrcaSession } from './auth-types'

// Extend Express Request type để TypeScript hiểu req.orcaSession
declare module 'express-serve-static-core' {
  interface Request {
    orcaSession?: OrcaSession
  }
}

/**
 * Middleware: populate req.orcaSession nếu cookie hợp lệ.
 * Không reject nếu không có session (cho phép public routes).
 * Mount sớm nhất có thể trong middleware chain.
 */
export function createAuthMiddleware(auth: AuthManager) {
  return (req: Request, _res: Response, next: NextFunction): void => {
    const session = auth.validateRequest(req.headers.cookie)
    if (session) req.orcaSession = session
    next()
  }
}

/**
 * Guard: reject 401 nếu req.orcaSession chưa được populate.
 * Dùng sau createAuthMiddleware() trên các routes cần auth.
 */
export function requireAuth(req: Request, res: Response, next: NextFunction): void {
  if (!req.orcaSession) {
    res.status(401).json({ error: 'unauthenticated', message: 'Login required' })
    return
  }
  next()
}
```

---

## Acceptance Criteria

- [x] File tồn tại, TypeScript compile sạch
- [x] `declare module 'express-serve-static-core'` augment `Request.orcaSession` đúng type
- [x] `createAuthMiddleware()` không block request khi không có cookie (next() luôn được gọi)
- [x] `requireAuth` trả về 401 JSON khi không có `req.orcaSession`
- [x] `requireAuth` gọi `next()` khi có session
