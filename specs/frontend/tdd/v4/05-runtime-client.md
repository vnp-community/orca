# TDD-FE-05: Runtime Client Layer

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/renderer/src/web/`, `src/platform/adapters/web/`

---

## 1. IRpcClient Interface

```typescript
// src/platform/rpc-client-interface.ts

export interface IRpcClient {
  connect(): Promise<void>
  disconnect(): void
  call<T>(method: string, params?: unknown): Promise<T>
  on(event: string, handler: (...args: unknown[]) => void): () => void
  connectionStatus: 'connected' | 'disconnected' | 'connecting' | 'error'
}
```

---

## 2. WebSocketRpcClient

```typescript
// src/platform/adapters/web/rpc-client.ts

class WebSocketRpcClient implements IRpcClient {
  constructor(url: string) {}

  async connect(): Promise<void>
  // Opens WebSocket, waits for 'open' event

  call<T>(method: string, params?: unknown): Promise<T>
  // Sends JSON-RPC 2.0 request, awaits response

  on(event: string, handler): () => void
  // Subscribe to server push events

  // Auto-reconnect: exponential backoff (1s, 2s, 4s, max 30s)
  // Heartbeat: ping every 15s, close if no pong within 5s
}
```

**JSON-RPC 2.0 format:**
```json
// Request
{ "jsonrpc": "2.0", "id": 1, "method": "runtime.getWorktrees", "params": {} }

// Response
{ "jsonrpc": "2.0", "id": 1, "result": [...] }

// Push event (no id)
{ "jsonrpc": "2.0", "method": "runtime.worktreeChanged", "params": {...} }
```

---

## 3. WebRuntimeClient (Desktop)

```typescript
// src/renderer/src/web/web-runtime-client.ts

// Dùng khi running in Browser (web mode)
// Wraps WebSocketRpcClient + provides App.tsx-compatible API

class WebRuntimeClient {
  // Mirror of ElectronRuntimeClient API surface
  // so App.tsx không biết đang chạy Desktop hay Web

  getWorktrees(): Promise<Worktree[]>
  createWorktree(params: CreateWorktreeParams): Promise<Worktree>
  deleteWorktree(id: string): Promise<void>
  // ... 50+ methods

  // PTY streaming
  createPtySession(worktreeId: string): Promise<string>  // returns sessionId
  writePty(sessionId: string, data: string): void
  onPtyData(sessionId: string, handler: (data: string) => void): () => void
}
```

---

## 4. ConnectionStatusProvider

```tsx
// src/renderer/src/web/ConnectionStatusProvider.tsx

// React context providing connection status (web-only)
const ConnectionStatusContext = createContext<ConnectionStatusValue>({
  status: 'disconnected',
  reconnecting: false,
  lastConnectedAt: null
})

export function ConnectionStatusProvider({ client, children }) {
  // Subscribes to client.connectionStatus changes
  // Provides 3 hooks:
}

// Hooks:
export function useConnectionStatus(): 'connected' | 'disconnected' | 'connecting' | 'error'
export function useIsConnected(): boolean
export function useIsReconnecting(): boolean
```

---

## 5. ConnectionStatusBanner

```tsx
// src/renderer/src/web/ConnectionStatusBanner.tsx

// Fixed-position overlay (bottom of screen) khi disconnected
// Shows: "Connection lost — reconnecting..." với spinner
// Auto-hides khi reconnected

function ConnectionStatusBanner() {
  const status = useConnectionStatus()
  if (status === 'connected') return null
  return (
    <div className="connection-banner">
      <Spinner />
      <span>Connection lost — reconnecting...</span>
    </div>
  )
}
```

---

## 6. web-session-client.ts

```typescript
// src/renderer/src/web/web-session-client.ts
// Xử lý workspace sessions qua WebSocket (thay Electron IPC)

class WebSessionClient {
  // Workspace session operations
  async openWorkspace(worktreeId: string): Promise<void>
  async closeWorkspace(worktreeId: string): Promise<void>

  // Terminal ops (via WebSocket streaming)
  async openTerminal(worktreeId: string, cols: number, rows: number): Promise<string>
  writeTerminal(sessionId: string, data: string): void
  resizeTerminal(sessionId: string, cols: number, rows: number): void
  onTerminalData(sessionId: string, cb: (data: string) => void): () => void
}
```

---

## 7. web-preload-api.ts (Không sửa)

```typescript
// src/renderer/src/web/web-preload-api.ts — 144KB
// Đây là "window.api" implementation cho web mode
// Mirrors Electron preload API surface (src/preload/index.ts)
// DO NOT MODIFY — additive extensions qua platform adapters chỉ

// Pattern: window.api.XXX = () => client.call('runtime.XXX', ...)
```

---

## 8. Tests (26 tests)

| File | Tests |
|------|-------|
| `web/__tests__/rpc-client.test.ts` | 15 |
| `web/__tests__/ConnectionStatusProvider.test.tsx` | 5 |
| `web/__tests__/ConnectionStatusBanner.test.tsx` | 6 |
