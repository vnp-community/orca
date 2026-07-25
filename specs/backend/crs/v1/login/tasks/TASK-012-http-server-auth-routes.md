# TASK-012: Sửa `src/server/http-server.ts` — Mount auth routes + cookie-parser

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 1 — Auth Foundation
**Solution:** [SOL-LG-001](../solutions/SOL-LG-001-auth-session.md) §6
**Depends on:** TASK-009, TASK-010, TASK-011
**Blocks:** (Phase 1 complete)

---

## Mục tiêu

Mount `cookie-parser`, `auth-middleware`, `auth-router` vào HTTP server. Đây là task kết thúc Phase 1.

---

## Cài đặt dependency

```bash
pnpm add cookie-parser
pnpm add -D @types/cookie-parser
```

---

## File cần sửa

**Path:** `src/server/http-server.ts`

---

## Thay đổi cần thực hiện

### 1. Thêm imports

```typescript
import cookieParser from 'cookie-parser'
import { createAuthMiddleware } from '../main/auth/auth-middleware'
import { createAuthRouter }     from '../main/auth/auth-router'
import type { AuthManager }     from '../main/auth/auth-manager'
```

### 2. Thêm `authManager` vào `HttpServerOptions`

```typescript
// TRƯỚC:
export interface HttpServerOptions {
  port:    number
  webRoot: string
  dbMonitor?: DatabaseHealthMonitor
}

// SAU:
export interface HttpServerOptions {
  port:        number
  webRoot:     string
  dbMonitor?:  DatabaseHealthMonitor
  authManager: AuthManager           // ← THÊM
}
```

### 3. Mount middleware và routes

Trong `startHttpServer()`, sau khi `app` được khởi tạo (express()):

```typescript
// Cookie parser — đặt trước mọi route
app.use(cookieParser())

// Auth middleware — populate req.orcaSession nếu có valid cookie
app.use(createAuthMiddleware(options.authManager))

// Auth routes
app.use('/auth', createAuthRouter(options.authManager))
```

### 4. Cập nhật call site trong `src/server/index.ts`

```typescript
// Tìm chỗ gọi startHttpServer() và thêm authManager:
const httpServer = await startHttpServer({
  port:        httpPort,
  webRoot,
  dbMonitor,
  authManager,   // ← THÊM (lấy từ initializeOrcaServices result)
})
```

---

## Endpoint Map sau khi hoàn thành Phase 1

```
HTTP :6769
  GET  /           → static SPA (out/web/)
  POST /auth/local → login
  POST /auth/logout → logout
  GET  /auth/me    → current user info
  GET  /auth/sso/* → 501 stub
  GET  /health     → health check
  GET  /health/ready
  GET  /health/metrics
```

---

## Acceptance Criteria

- [x] `cookie-parser` được mount trước `auth-middleware` và `auth-router`
- [x] `GET /auth/me` với `curl -b 'orca_session=<invalid>'` → 401
- [x] `POST /auth/local` với valid credentials → 200 + `Set-Cookie: orca_session=...`
- [x] TypeScript compile không có lỗi mới
- [x] Server khởi động và serve static files như trước (không regression)
