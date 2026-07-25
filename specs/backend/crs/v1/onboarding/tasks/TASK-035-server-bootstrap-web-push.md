# TASK-035: Tích hợp `WebPushManager` vào `server-bootstrap.ts` + `src/server/index.ts`

**Phase:** 3 — Web Push Notifications  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) §B.6  
**Depends on:** TASK-032, TASK-034  
**Blocks:** TASK-036

---

## Mục tiêu

Khởi tạo `WebPushManager` trong `server-bootstrap.ts` và đăng ký push API routes trong `src/server/index.ts`.

---

## Files cần sửa

1. `src/main/server-bootstrap.ts`
2. `src/server/index.ts`

---

## Thay đổi cho `server-bootstrap.ts`

```typescript
import { WebPushManager } from './notifications/web-push-manager'

// Trong initializeOrcaServices():
const pushManager = new WebPushManager(store)    // NEW — sau khi store khởi tạo

// Expose trong return value:
return {
  // ...existing exports...
  pushManager,     // NEW
}
```

---

## Thay đổi cho `src/server/index.ts`

```typescript
import { registerPushApiRoutes } from './push-api-routes'
import { existsSync } from 'node:fs'

// Sau khi khởi tạo:
const { shutdown, pushManager, /* ...other */ } = await initializeOrcaServices(...)

if (existsSync(webRoot)) {
  const httpServer = await startHttpServer(httpPort, webRoot)
  registerPushApiRoutes(httpServer, pushManager)   // NEW
}
```

---

## Acceptance Criteria

- [x] `WebPushManager` được khởi tạo sau `store` trong bootstrap
- [x] `pushManager` được expose trong return object của `initializeOrcaServices()`
- [x] `registerPushApiRoutes()` được gọi với đúng `httpServer` và `pushManager`
- [x] Chỉ đăng ký push routes khi web mode (`existsSync(webRoot)`)
- [x] Server vẫn khởi động thành công
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

1. Đọc `server-bootstrap.ts` và `src/server/index.ts` để hiểu pattern hiện tại
2. Tìm đúng điểm inject `pushManager` vào http server setup
3. Đảm bảo không phá vỡ luồng khởi tạo hiện có
