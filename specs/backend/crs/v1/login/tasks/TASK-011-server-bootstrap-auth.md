# TASK-011: Sửa `src/main/server-bootstrap.ts` — Tích hợp AuthManager

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 1 — Auth Foundation
**Solution:** [SOL-LG-001](../solutions/SOL-LG-001-auth-session.md) §6
**Depends on:** TASK-008
**Blocks:** TASK-012

---

## Mục tiêu

Khởi tạo `AuthManager` trong `initializeOrcaServices()` và export nó trong `ServerBootstrapResult` để `http-server.ts` có thể dùng.

---

## File cần sửa

**Path:** `src/main/server-bootstrap.ts`

---

## Thay đổi cần thực hiện

### 1. Import AuthManager

```typescript
// Thêm vào đầu file
import { AuthManager } from './auth/auth-manager'
```

### 2. Cập nhật `ServerBootstrapResult`

```typescript
// TRƯỚC:
export interface ServerBootstrapResult {
  shutdown(): Promise<void>
}

// SAU:
export interface ServerBootstrapResult {
  shutdown():   Promise<void>
  authManager:  AuthManager    // ← THÊM
}
```

### 3. Khởi tạo `AuthManager` trong `initializeOrcaServices()`

Tìm đoạn code nơi `db` (IDatabase instance) được khởi tạo xong (sau migration runner), thêm:

```typescript
// Sau khi db đã initialized và migrations chạy xong:
const authManager = new AuthManager(db)
```

### 4. Trả về `authManager` trong result

```typescript
// TRƯỚC:
return { shutdown }

// SAU:
return {
  shutdown: async () => {
    authManager.destroy()   // Clear cleanup timer
    await existingShutdown()
  },
  authManager
}
```

---

## Lưu ý

- `AuthManager` cần `IDatabase` — chỉ available trong server mode (Node.js), không phải Electron mode
- Nếu `db` không available (Electron mode), `authManager` có thể là `null` hoặc không khởi tạo
- Đảm bảo `authManager.destroy()` được gọi trong shutdown sequence để clear timers

---

## Acceptance Criteria

- [x] `ServerBootstrapResult.authManager` có type `AuthManager`
- [x] `AuthManager` được khởi tạo sau khi DB migrations xong
- [x] `authManager.destroy()` được gọi trong `shutdown()`
- [x] Server vẫn start thành công với `pnpm dev:server` hoặc equivalent
- [x] TypeScript compile không có lỗi mới
