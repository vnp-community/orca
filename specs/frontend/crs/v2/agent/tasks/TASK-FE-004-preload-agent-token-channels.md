# TASK-FE-004 — `preload.ts`: Expose `onAgentToken` / `offAgentToken` IPC channels

**Solution:** [SOL-FE-AG-003](../solutions/SOL-FE-AG-003-ipc-agent-token-event.md)  
**File:** `src/preload/preload.ts` [MODIFY] _(Electron mode only)_  
**Depends on:** TASK-FE-003 (IPC broadcasts `devServer:agentToken`)  
**Status:** ✅ DONE (2026-07-26) — preload/index.ts N/A (web-only project); api-types.ts updated  

---

## Mục tiêu

Expose `onAgentToken` và `offAgentToken` channels trong Electron preload script để renderer có thể subscribe/unsubscribe nhận `AgentTokenInfo` khi backend generate token.

---

## Context hiện tại

Cần tìm section `devServer` trong `src/preload/preload.ts`. Nếu file không có section devServer (Web-only project), bỏ qua task này.

```typescript
// Pattern hiện tại trong preload.ts cho push events:
// (ví dụ từ ssh section)
ssh: {
  onConnectionStateChanged: (handler) =>
    ipcRenderer.on('ssh:connectionStateChanged', (_, event) => handler(event)),
  offConnectionStateChanged: (handler) =>
    ipcRenderer.off('ssh:connectionStateChanged', (_, event) => handler(event)),
}
```

---

## Thay đổi cần thực hiện

### Bước 1: Check xem preload.ts có devServer section không

```bash
grep -n "devServer\|onStatusChanged" src/preload/preload.ts | head -20
```

### Bước 2A: Nếu CÓ devServer section trong preload.ts

**Thêm vào devServer object:**
```typescript
import type { AgentTokenInfo } from '../../shared/dev-server-types'

// Trong devServer section:
onAgentToken: (handler: (info: AgentTokenInfo) => void) => {
  const listener = (_: Electron.IpcRendererEvent, info: AgentTokenInfo) => handler(info)
  ipcRenderer.on('devServer:agentToken', listener)
  // Store listener reference so offAgentToken can remove exact same fn
  ;(handler as unknown as { _ipcListener?: unknown })._ipcListener = listener
},

offAgentToken: (handler: (info: AgentTokenInfo) => void) => {
  const listener = (handler as unknown as { _ipcListener?: unknown })._ipcListener
  if (listener) {
    ipcRenderer.off('devServer:agentToken', listener as Electron.IpcRendererListener)
  }
},
```

### Bước 2B: Nếu KHÔNG CÓ devServer section (web-only project)

Preload có thể không export devServer — SKIP task này, chỉ cần làm TASK-FE-005 (web-preload-api.ts).

---

## Acceptance Criteria

- [x] `window.api.devServer.onAgentToken` hoạt động trong Electron mode (nếu applicable)
- [x] `window.api.devServer.offAgentToken` cleanup được handler
- [x] Không memory leak: `off` phải remove đúng listener function
- [x] TypeScript type: `handler: (info: AgentTokenInfo) => void`
- [x] TypeScript compile không lỗi
