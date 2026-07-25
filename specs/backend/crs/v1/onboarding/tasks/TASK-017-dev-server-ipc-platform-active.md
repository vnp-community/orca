# TASK-017: Sửa `src/main/ipc/dev-server-ipc.ts` — Thêm `getPlatform` + `setActiveDevServer`

**Phase:** 2 — Platform Wizard  
**Solution:** [SOL-004-005-006](../solutions/SOL-004-005-006-platform-preflight-repo.md) §A.1, §A.2  
**Depends on:** TASK-007  
**Blocks:** TASK-022

---

## Mục tiêu

Thêm 2 IPC handlers mới vào `dev-server-ipc.ts`:
1. `devServer.getPlatform` — trả về platform của một dev server
2. `settings.setActiveDevServer` — lưu `activeDevServerId` vào GlobalSettings

---

## File cần sửa

**Path:** `src/main/ipc/dev-server-ipc.ts`

---

## Thay đổi cần thực hiện

Trong hàm `registerDevServerIpcHandlers()`, thêm:

```typescript
// Get platform của 1 dev server
ipc.handle('devServer.getPlatform', async (_, devServerId: string): Promise<NodeJS.Platform | null> => {
  return devServerManager.get(devServerId)?.platform ?? null
})

// Set active dev server (lưu vào GlobalSettings)
ipc.handle('settings.setActiveDevServer', async (_, devServerId: string | null) => {
  await store.updateGlobalSettings({ activeDevServerId: devServerId ?? null })
  devServerManager.emit('activeDevServerChanged', devServerId)
})
```

> **Lưu ý:** Cần inject `store` vào `registerDevServerIpcHandlers()` nếu chưa có, hoặc dùng setter pattern.

---

## Acceptance Criteria

- [x] `devServer.getPlatform(id)` trả về đúng `platform` của dev server
- [x] `devServer.getPlatform(id)` trả về `null` nếu server không tồn tại hoặc chưa connected
- [x] `settings.setActiveDevServer(id)` lưu vào store
- [x] `settings.setActiveDevServer(null)` clear active server (lưu `null`)
- [x] Emit `'activeDevServerChanged'` event sau khi update
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

1. Kiểm tra signature hiện tại của `registerDevServerIpcHandlers()` — có thể cần thêm `store` param
2. Tìm `updateGlobalSettings` hoặc tương đương trong `store` interface
