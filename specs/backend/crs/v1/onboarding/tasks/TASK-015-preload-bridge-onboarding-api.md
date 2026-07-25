# TASK-015: Expose `window.api.onboarding` trong Preload / WebIpcBridge

**Phase:** 1 — Remote Agent Detection  
**Solution:** [SOL-003](../solutions/SOL-003-remote-agent-detection.md) §6  
**Depends on:** TASK-014  
**Blocks:** (không — frontend-facing endpoint)

---

## Mục tiêu

Expose các IPC channels `onboarding.detectAgents` và `onboarding.detectAgentsAllServers` trong preload script (Electron mode) và/hoặc `WebIpcBridge` (Web mode) để frontend có thể gọi qua `window.api.onboarding`.

---

## Files cần sửa

Tùy codebase, có thể là 1 hoặc 2 files:
- **Electron mode:** `src/preload/index.ts` (hoặc `src/preload/api.ts`)
- **Web mode:** `src/main/web-ipc-bridge.ts` (hoặc tương đương)

---

## Thay đổi cần thực hiện

```typescript
// Thêm vào object window.api (hoặc contextBridge.exposeInMainWorld):

onboarding: {
  detectAgents: (params: { devServerId: string | null }) =>
    ipcRenderer.invoke('onboarding.detectAgents', params),

  detectAgentsAllServers: () =>
    ipcRenderer.invoke('onboarding.detectAgentsAllServers'),
},
```

---

## Acceptance Criteria

- [x] `window.api.onboarding.detectAgents({ devServerId })` gọi được từ renderer
- [x] `window.api.onboarding.detectAgentsAllServers()` gọi được từ renderer
- [x] TypeScript types cho `window.api.onboarding` được cập nhật (nếu có type file)
- [x] Electron preload compile thành công
- [x] Web mode cũng expose nếu WebIpcBridge tồn tại

---

## Lưu ý cho AI

1. Đọc preload file hiện tại để xem pattern expose API (contextBridge hay global assign)
2. Tìm type definition cho `window.api` (thường trong `src/shared/` hoặc `src/renderer/src/types/`)
3. Cập nhật cả type definition nếu có
4. Đảm bảo theo pattern security: không expose `ipcRenderer` trực tiếp
