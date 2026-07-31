# SOL-FE-AG-003 — IPC Bridge: agentTokenGenerated Event

**CR:** [CR-AG-004](../../../../../docs/crs/v2/agent/CR-AG-004-direct-websocket-mode.md)  
**TDD Refs:** TDD-FE-07 §2 (useIpcEvents), TDD-FE-09 §9 (IPC API Surface)  
**Depends on:** SOL-AG-004 (Backend: `DevServerRelayBridge.emit('agentTokenGenerated')`)  
**Approach:** IPC channel extension — thêm mới, không sửa existing  
**Status:** ✅ IMPLEMENTED (2026-07-26)  

---

## 1. Phân tích luồng sự kiện

### 1.1 Backend side (đã implement)

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts
this.emit('agentTokenGenerated', {
  devServerId: this.config.id,
  agentToken,
  orcaUrl: `ws://<orca-host>:6768${AGENT_WS_PATH}`,
})
```

Sự kiện này được `DevServerRelayBridge` emit nhưng **chưa có IPC channel** để forward lên renderer.

### 1.2 Luồng cần xây dựng

```
DevServerRelayBridge.emit('agentTokenGenerated', payload)
    │
    ▼
DevServerManager listens on bridge.on('agentTokenGenerated')
    │
    ▼ (Electron mode)
ipcMain.emit / mainWindow.webContents.send('devServer:agentToken', payload)
    │
    ▼ (Web mode)  
WebSocket RPC → Orca Server push event
    │
    ▼
window.api.devServer.onAgentToken(handler)
    │
    ▼
useAddDevServer hook receives { agentToken, orcaUrl }
```

---

## 2. Giải pháp

### 2.1 Backend: `dev-server-ipc.ts` — Forward bridge event → IPC

```typescript
// src/main/ipc/dev-server-ipc.ts (MODIFY)
// Thêm vào registerDevServerIpcHandlers():

// When any bridge emits agentTokenGenerated, forward to renderer
devServerManager.on('devServer:agentToken', (payload) => {
  // Electron mode: push to renderer window
  sendToRenderer('devServer:agentToken', payload)
})
```

**Hoặc nếu DevServerManager không re-emit:** Bridge event cần được forwarded:

```typescript
// src/main/dev-server/dev-server-manager.ts (MODIFY)
// Trong connect(), sau khi tạo bridge:
const bridge = new DevServerRelayBridge(persisted, this.sshManager, this.agentWsServer)

// Forward bridge's agentTokenGenerated to manager-level event
bridge.on('agentTokenGenerated', (payload) => {
  this.emit('devServer:agentToken', payload)
})
```

### 2.2 IPC channel: Electron preload

```typescript
// src/preload/preload.ts (MODIFY)
// Thêm vào devServer API:
devServer: {
  // ... existing channels ...
  onAgentToken: (handler: (info: AgentTokenInfo) => void) =>
    ipcRenderer.on('devServer:agentToken', (_, info) => handler(info)),
  offAgentToken: (handler: (info: AgentTokenInfo) => void) =>
    ipcRenderer.off('devServer:agentToken', (_, info) => handler(info)),
}

type AgentTokenInfo = {
  devServerId: string
  agentToken: string
  orcaUrl: string
}
```

### 2.3 Web mode: `web-preload-api.ts`

```typescript
// src/renderer/src/web/web-preload-api.ts (MODIFY)
// Thêm vào devServer section (lines ~820-860):

// agentToken event handlers (web mode)
// Web mode: Backend push via WebSocket RPC (devServer.agentToken event)
// hoặc poll approach nếu push chưa support

const agentTokenHandlers = new Set<(info: AgentTokenInfo) => void>()

// Subscribe via WebSocket push event 'devServer.agentToken'
// (OrcaRuntimeRpcClient support push events via 'event:' prefix)
const unsubAgentToken = onRuntimeEvent('devServer.agentToken', (payload) => {
  for (const h of agentTokenHandlers) h(payload as AgentTokenInfo)
})

onAgentToken: (handler) => { agentTokenHandlers.add(handler) },
offAgentToken: (handler) => { agentTokenHandlers.delete(handler) },
```

> **Web mode alternative**: Nếu WebSocket push event chưa support, dùng cách đơn giản hơn: `testConnection` RPC trả về `{ agentToken, orcaUrl }` trong response khi `connectionType === 'direct-websocket'`.

### 2.4 Shared type: `AgentTokenInfo`

```typescript
// src/shared/dev-server-types.ts (MODIFY — thêm type)
export type AgentTokenInfo = {
  devServerId: string
  agentToken: string
  orcaUrl: string
}
```

### 2.5 `useIpcEvents.ts` — Global subscription (optional)

```typescript
// src/renderer/src/hooks/useIpcEvents.ts (MODIFY — optional)
// Nếu cần store agentToken trong global Zustand state:

window.api.devServer.onAgentToken?.((info) => {
  // Store can keep last token per devServerId if needed
  store.setAgentTokenInfo(info)
})

// Cleanup
return () => {
  window.api.devServer.offAgentToken?.(...)
}
```

> **Tradeoff**: `useAddDevServer` local state vs global Zustand.
> Recommendation: Local state trong `useAddDevServer` đủ — không cần global store.
> Chỉ cần global nếu `DevServerCard` cũng phải hiển thị token.

---

## 3. Alternative: Embed token trong testConnection response

**Simpler approach** — Thay vì event, trả về token trong `testConnection` result:

```typescript
// src/shared/dev-server-types.ts
export type ConnectionTestResult = {
  ok: boolean
  platform?: NodeJS.Platform
  nodeVersion?: string
  error?: string
  agentToken?: string    // NEW: chỉ có với direct-websocket trước khi agent connect
  orcaUrl?: string       // NEW
}
```

```typescript
// src/main/dev-server/dev-server-manager.ts
// direct-websocket testConnection: ngay khi emit agentTokenGenerated, trả về partial result
// (chứa token) để frontend hiển thị, sau đó chờ agent connect để trả về ok: true

// Pattern: 2-phase response
//   Phase 1: return { ok: false, agentToken, orcaUrl } — agent chưa connect nhưng token đã sẵn
//   Phase 2: actual promise resolves/rejects sau 60s
```

**Vấn đề**: IPC không hỗ trợ 2-phase response native. Cần streaming hoặc event.

**Recommendation**: Dùng **Event approach** (phần 2.1-2.3) cho đúng architecture.

---

## 4. Files thay đổi

### [MODIFY] `src/main/dev-server/dev-server-manager.ts`
- Bridge events được re-emit tại manager level: `this.emit('devServer:agentToken', payload)`

### [MODIFY] `src/main/ipc/dev-server-ipc.ts`
- `registerDevServerIpcHandlers()`: lắng nghe `devServerManager.on('devServer:agentToken')`
- Forward qua `mainWindow.webContents.send('devServer:agentToken', payload)`

### [MODIFY] `src/preload/preload.ts`
- Expose `onAgentToken` / `offAgentToken` IPC channels

### [MODIFY] `src/renderer/src/web/web-preload-api.ts`
- Expose `onAgentToken` / `offAgentToken` qua WebSocket runtime events

### [MODIFY] `src/shared/dev-server-types.ts`
- Export `AgentTokenInfo` type

---

## 5. Acceptance Criteria

- [x] `DevServerRelayBridge.emit('agentTokenGenerated')` được forward lên renderer
- [x] `window.api.devServer.onAgentToken(handler)` hoạt động trong Electron mode
- [x] `window.api.devServer.onAgentToken(handler)` hoạt động trong Web mode
- [x] Handler cleanup (`offAgentToken`) ngăn memory leak
- [x] `AgentTokenInfo` type được share giữa main và renderer
- [x] TypeScript compile không lỗi
