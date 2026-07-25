# TASK-007: Tạo file `src/main/ipc/dev-server-ipc.ts`

**Phase:** 1 — Foundation  
**Solution:** [SOL-002](../solutions/SOL-002-dev-server-manager.md) §8  
**Depends on:** TASK-001, TASK-004  
**Blocks:** TASK-008

---

## Mục tiêu

Tạo IPC handlers cho tất cả `devServer.*` channels theo pattern TDD-09.

---

## File cần tạo

**Path:** `src/main/ipc/dev-server-ipc.ts`

---

## Nội dung cần implement

```typescript
// Pattern: theo TDD-09 IPC Handler convention
import type { DevServerManager } from '../dev-server/dev-server-manager'
import type { DevServerInput, DevServerStatus } from '../../shared/dev-server-types'

// Dùng đúng type IpcMain/WebIpcBridge từ codebase hiện tại
// (tra cứu bằng grep "IpcMain\|WebIpcBridge" trong src/main/ipc/)
type IpcBridge = import('electron').IpcMain | import('../web-ipc-bridge').WebIpcBridge

export function registerDevServerIpcHandlers(
  ipc: IpcBridge,
  manager: DevServerManager
): void {

  // List all dev servers
  ipc.handle('devServer.list', async () => manager.list())

  // Add new dev server
  ipc.handle('devServer.add', async (_, input: DevServerInput) => {
    return manager.add(input)
  })

  // Remove dev server
  ipc.handle('devServer.remove', async (_, id: string) => {
    await manager.remove(id)
  })

  // Test connection (ephemeral — không save)
  ipc.handle('devServer.testConnection', async (_, input: DevServerInput) => {
    return manager.testConnection(input)
  })

  // Connect dev server
  ipc.handle('devServer.connect', async (_, id: string) => {
    await manager.connect(id)
    return manager.get(id)
  })

  // Disconnect dev server
  ipc.handle('devServer.disconnect', async (_, id: string) => {
    await manager.disconnect(id)
  })

  // Get single dev server info
  ipc.handle('devServer.get', async (_, id: string) => {
    return manager.get(id)
  })

  // Push status changes to frontend:
  manager.on('devServer:statusChanged', (id: string, status: DevServerStatus) => {
    // Dùng đúng method emit của IpcBridge (tra cứu pattern trong src/main/ipc/)
    ipc.emit('devServer:statusChanged', { id, status })
  })

  manager.on('devServer:added', (id: string) => {
    ipc.emit('devServer:added', { id })
  })

  manager.on('devServer:removed', (id: string) => {
    ipc.emit('devServer:removed', { id })
  })
}
```

---

## Acceptance Criteria

- [x] File tồn tại tại `src/main/ipc/dev-server-ipc.ts`
- [x] `registerDevServerIpcHandlers()` được export
- [x] Đầy đủ 6 handle channels: `list`, `add`, `remove`, `testConnection`, `connect`, `disconnect`
- [x] Bonus: `devServer.get` handler
- [x] Push events `devServer:statusChanged`, `devServer:added`, `devServer:removed` được forward về frontend
- [x] Dùng đúng IPC type pattern từ codebase (không để `any`)
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

1. Tra cứu pattern IPC handler trong `src/main/ipc/` để dùng đúng type cho `ipc` parameter
2. Nếu `ipc.emit` không tồn tại trong interface, tìm method tương đương (e.g., `ipc.sendToAll`, `webContents.send`)
3. Đối với web mode (`WebIpcBridge`), cách emit sự kiện về frontend có thể khác với Electron mode
