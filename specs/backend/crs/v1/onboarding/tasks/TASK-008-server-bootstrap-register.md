# TASK-008: Sửa `src/main/server-bootstrap.ts` — Đăng ký DevServerManager + IPC

**Phase:** 1 — Foundation  
**Solution:** [SOL-002](../solutions/SOL-002-dev-server-manager.md) Checklist  
**Depends on:** TASK-004, TASK-007  
**Blocks:** (không — endpoint cuối cùng của Phase 1 wiring)

---

## Mục tiêu

Khởi tạo `DevServerManager` trong `server-bootstrap.ts` và đăng ký toàn bộ IPC handlers, đảm bảo manager được inject đúng dependencies.

---

## File cần sửa

**Path:** `src/main/server-bootstrap.ts`

---

## Thay đổi cần thực hiện

### 1. Import

```typescript
import { DevServerManager } from './dev-server/dev-server-manager'
import { registerDevServerIpcHandlers } from './ipc/dev-server-ipc'
```

### 2. Khởi tạo DevServerManager

Trong hàm `initializeOrcaServices()` (hoặc hàm bootstrap tương đương), sau khi `store` và `sshManager` đã khởi tạo:

```typescript
// Khởi tạo DevServerManager (sau store + sshManager)
const devServerManager = new DevServerManager(store, sshManager)

// Đăng ký IPC handlers
registerDevServerIpcHandlers(ipc, devServerManager)
```

### 3. Expose trong return value (nếu cần)

```typescript
return {
  // ...existing exports...
  devServerManager,     // NEW — expose để http-server và các service khác dùng
}
```

---

## Acceptance Criteria

- [x] `DevServerManager` được instantiate với `store` và `sshManager`
- [x] `registerDevServerIpcHandlers()` được gọi với đúng `ipc` instance
- [x] Thứ tự khởi tạo đúng: `store` → `sshManager` → `devServerManager`
- [x] `devServerManager` được expose trong return object (để TASK-014, TASK-022 sau này dùng)
- [x] Server vẫn khởi động thành công (không crash)
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

1. Đọc toàn bộ `src/main/server-bootstrap.ts` để hiểu cấu trúc hiện tại
2. Tìm đúng vị trí `sshManager` được khởi tạo để đặt `devServerManager` sau
3. Không làm thay đổi các service khác đang hoạt động
