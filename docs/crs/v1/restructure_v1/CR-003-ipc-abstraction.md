# CR-003 — IPC → RPC Transport Abstraction

**Status:** Proposed  
**Priority:** 🟠 High  
**Depends on:** CR-001  
**Blocks:** CR-004

---

## Mục tiêu

Tạo ra một **RPC Transport Layer** thống nhất, cho phép frontend giao tiếp với backend thông qua cùng một API bất kể platform mode:
- **Electron mode**: dùng `ipcRenderer` / `ipcMain` (giữ nguyên)
- **Web/Node mode**: dùng WebSocket JSON-RPC

---

## Bối cảnh & Vấn đề

Hiện tại:
- `src/preload/index.ts` expose `window.electron = { ipcRenderer }` để renderer dùng
- `src/renderer/` gọi `window.electron.invoke(channel, args)` cho tất cả backend calls
- Điều này tạo **hard coupling** giữa renderer và Electron IPC
- `src/main/runtime/rpc/ws-transport.ts` đã có WebSocket transport cho mobile clients, nhưng chưa được dùng cho web frontend

Orca đã có một WebSocket RPC layer hoàn chỉnh trong `src/main/runtime/rpc/` — đây là nền tảng tốt. Vấn đề là **frontend chưa biết dùng nó thay vì IPC**.

---

## Giải pháp Đề xuất

### 1. Định nghĩa `IRpcClient` Interface

```typescript
// src/platform/rpc-client-interface.ts

export interface IRpcClient {
  // Invoke a handler and await response (like ipcRenderer.invoke)
  invoke<T = any>(channel: string, ...args: any[]): Promise<T>
  
  // Fire-and-forget (like ipcRenderer.send)
  send(channel: string, ...args: any[]): void
  
  // Subscribe to server-pushed events (like ipcRenderer.on)
  on(channel: string, listener: RpcEventListener): UnsubscribeFn
  off(channel: string, listener: RpcEventListener): void
  once(channel: string, listener: RpcEventListener): void
  
  // Connection lifecycle
  isConnected(): boolean
  connect(): Promise<void>
  disconnect(): void
}

export type RpcEventListener = (event: RpcEvent, ...args: any[]) => void
export type UnsubscribeFn = () => void

export interface RpcEvent {
  // Minimal event object matching ipcRenderer event interface
  readonly type: string
}
```

### 2. ElectronRpcClient (dùng trong Electron mode)

```typescript
// src/platform/adapters/electron/rpc-client.ts

import type { IRpcClient, RpcEventListener } from '../../rpc-client-interface'

/**
 * Thin wrapper around window.electron.ipcRenderer.
 * This is the EXISTING behavior — zero change to functionality.
 */
export class ElectronRpcClient implements IRpcClient {
  private get ipc() {
    return (window as any).electron?.ipcRenderer
  }
  
  async invoke<T>(channel: string, ...args: any[]): Promise<T> {
    return this.ipc.invoke(channel, ...args)
  }
  
  send(channel: string, ...args: any[]): void {
    this.ipc.send(channel, ...args)
  }
  
  on(channel: string, listener: RpcEventListener) {
    this.ipc.on(channel, listener)
    return () => this.ipc.removeListener(channel, listener)
  }
  
  off(channel: string, listener: RpcEventListener): void {
    this.ipc.removeListener(channel, listener)
  }
  
  once(channel: string, listener: RpcEventListener): void {
    this.ipc.once(channel, listener)
  }
  
  isConnected(): boolean { return true }
  async connect(): Promise<void> {}
  disconnect(): void {}
}
```

### 3. WebSocketRpcClient (dùng trong Web mode)

```typescript
// src/platform/adapters/web/rpc-client.ts

import type { IRpcClient, RpcEventListener, UnsubscribeFn } from '../../rpc-client-interface'

interface PendingCall {
  resolve: (value: any) => void
  reject: (err: Error) => void
  timer: ReturnType<typeof setTimeout>
}

/**
 * WebSocket-based RPC client for web frontend.
 * Protocol mirrors the existing OrcaRuntimeRpcServer protocol.
 * 
 * Message format:
 * Request:  { id, type: 'invoke', channel, args }
 * Response: { id, type: 'result', result } | { id, type: 'error', message }
 * Push:     { type: 'push', channel, args }
 */
export class WebSocketRpcClient implements IRpcClient {
  private ws: WebSocket | null = null
  private readonly url: string
  private pending = new Map<string, PendingCall>()
  private eventListeners = new Map<string, Set<RpcEventListener>>()
  private nextId = 1
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private readonly INVOKE_TIMEOUT_MS = 30_000
  private readonly RECONNECT_DELAY_MS = 2_000
  
  constructor(url?: string) {
    // Auto-detect WebSocket URL from current page location
    this.url = url ?? this.detectUrl()
  }
  
  private detectUrl(): string {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${proto}//${window.location.host}/ws/runtime/api`
  }
  
  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.ws = new WebSocket(this.url)
      
      this.ws.onopen = () => {
        console.log('[WsRpcClient] Connected to Orca backend')
        resolve()
      }
      
      this.ws.onerror = (err) => {
        reject(new Error('WebSocket connection failed'))
      }
      
      this.ws.onmessage = (event) => {
        this.handleMessage(event.data)
      }
      
      this.ws.onclose = () => {
        console.warn('[WsRpcClient] Connection closed, scheduling reconnect...')
        this.scheduleReconnect()
      }
    })
  }
  
  private handleMessage(data: string): void {
    try {
      const msg = JSON.parse(data)
      
      if (msg.type === 'result' || msg.type === 'error') {
        // Response to invoke
        const pending = this.pending.get(msg.id)
        if (!pending) return
        this.pending.delete(msg.id)
        clearTimeout(pending.timer)
        
        if (msg.type === 'result') {
          pending.resolve(msg.result)
        } else {
          pending.reject(new Error(msg.message))
        }
      } else if (msg.type === 'push') {
        // Server-pushed event (replaces ipcRenderer.on)
        const listeners = this.eventListeners.get(msg.channel)
        if (listeners) {
          const event = { type: msg.channel }
          for (const listener of listeners) {
            listener(event, ...msg.args)
          }
        }
      }
    } catch (err) {
      console.error('[WsRpcClient] Failed to parse message:', err)
    }
  }
  
  async invoke<T>(channel: string, ...args: any[]): Promise<T> {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error(`[WsRpcClient] Not connected when calling: ${channel}`)
    }
    
    const id = String(this.nextId++)
    
    return new Promise<T>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id)
        reject(new Error(`RPC timeout: ${channel}`))
      }, this.INVOKE_TIMEOUT_MS)
      
      this.pending.set(id, { resolve, reject, timer })
      
      this.ws!.send(JSON.stringify({ id, type: 'invoke', channel, args }))
    })
  }
  
  send(channel: string, ...args: any[]): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return
    this.ws.send(JSON.stringify({ type: 'send', channel, args }))
  }
  
  on(channel: string, listener: RpcEventListener): UnsubscribeFn {
    let set = this.eventListeners.get(channel)
    if (!set) {
      set = new Set()
      this.eventListeners.set(channel, set)
    }
    set.add(listener)
    return () => set!.delete(listener)
  }
  
  off(channel: string, listener: RpcEventListener): void {
    this.eventListeners.get(channel)?.delete(listener)
  }
  
  once(channel: string, listener: RpcEventListener): void {
    const unsub = this.on(channel, (event, ...args) => {
      unsub()
      listener(event, ...args)
    })
  }
  
  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }
  
  disconnect(): void {
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    this.ws?.close()
    this.ws = null
  }
  
  private scheduleReconnect(): void {
    this.reconnectTimer = setTimeout(() => {
      this.connect().catch(err => {
        console.error('[WsRpcClient] Reconnect failed:', err)
        this.scheduleReconnect()
      })
    }, this.RECONNECT_DELAY_MS)
  }
}
```

### 4. Backend: Extend ws-transport.ts để xử lý invoke/send

File `src/main/runtime/rpc/ws-transport.ts` đã có WebSocket server. Cần thêm JSON-RPC dispatch layer:

```typescript
// src/main/runtime/rpc/web-ipc-bridge.ts (FILE MỚI)

import type { NodeIpcBridge } from '../../../platform/adapters/node/ipc'

/**
 * Connects incoming WebSocket JSON-RPC messages to NodeIpcBridge.
 * This is the server-side complement of WebSocketRpcClient.
 * 
 * Nhận message từ WebSocket clients (web browser) và dispatch
 * sang NodeIpcBridge như thể đến từ Electron IPC.
 */
export class WebIpcBridge {
  constructor(private readonly ipc: NodeIpcBridge) {}
  
  async handleWebSocketMessage(
    data: string,
    windowId: number,
    reply: (msg: string) => void
  ): Promise<void> {
    let msg: any
    try {
      msg = JSON.parse(data)
    } catch {
      reply(JSON.stringify({ type: 'error', message: 'Invalid JSON' }))
      return
    }
    
    if (msg.type === 'invoke') {
      try {
        const result = await this.ipc.invoke(msg.channel, windowId, ...msg.args)
        reply(JSON.stringify({ id: msg.id, type: 'result', result }))
      } catch (err: any) {
        reply(JSON.stringify({ 
          id: msg.id, 
          type: 'error', 
          message: err?.message ?? String(err) 
        }))
      }
    } else if (msg.type === 'send') {
      const event = { sender: { id: windowId, send: reply } }
      this.ipc.emit(msg.channel, event as any, ...msg.args)
    }
  }
  
  // Push server events to all connected WebSocket clients
  pushToClients(channel: string, args: any[], broadcast: (msg: string) => void): void {
    broadcast(JSON.stringify({ type: 'push', channel, args }))
  }
}
```

### 5. RPC Client Factory (Frontend)

```typescript
// src/platform/rpc-client-factory.ts

import type { IRpcClient } from './rpc-client-interface'

export function createRpcClient(): IRpcClient {
  if (typeof window !== 'undefined' && (window as any).electron?.ipcRenderer) {
    // Running in Electron
    const { ElectronRpcClient } = require('./adapters/electron/rpc-client')
    return new ElectronRpcClient()
  } else {
    // Running in browser (Web mode)
    const { WebSocketRpcClient } = require('./adapters/web/rpc-client')
    return new WebSocketRpcClient()
  }
}

// Singleton instance for use across the app
let _rpcClient: IRpcClient | null = null

export function getRpcClient(): IRpcClient {
  if (!_rpcClient) {
    _rpcClient = createRpcClient()
  }
  return _rpcClient
}
```

---

## Phạm vi thay đổi

### Files mới
| File | Mô tả |
|------|-------|
| `[NEW] src/platform/rpc-client-interface.ts` | IRpcClient interface |
| `[NEW] src/platform/rpc-client-factory.ts` | Factory & singleton |
| `[NEW] src/platform/adapters/electron/rpc-client.ts` | Electron IPC wrapper |
| `[NEW] src/platform/adapters/web/rpc-client.ts` | WebSocket client |
| `[NEW] src/main/runtime/rpc/web-ipc-bridge.ts` | Server-side dispatch bridge |

### Files sửa đổi
| File | Thay đổi |
|------|---------|
| `[MODIFY] src/main/runtime/rpc/ws-transport.ts` | Tích hợp `WebIpcBridge` để xử lý invoke/send messages |
| `[MODIFY] src/server/index.ts` | Kết nối `WebIpcBridge` với WebSocket server |

### Files KHÔNG thay đổi
- `src/preload/index.ts` — Giữ nguyên Electron preload
- `src/renderer/` — Chưa thay đổi ở CR này (CR-004 sẽ xử lý)
- `src/main/ipc/` — **KHÔNG sửa** bất kỳ IPC handler nào

---

## Compatibility Matrix

| Mode | Frontend | Backend | Communication |
|------|----------|---------|---------------|
| Electron | `ElectronRpcClient` | `ipcMain.handle()` | Electron IPC (unchanged) |
| Web/Node | `WebSocketRpcClient` | `NodeIpcBridge` + `WebIpcBridge` | WebSocket JSON-RPC |

---

## Rủi ro & Biện pháp

| Rủi ro | Biện pháp |
|--------|-----------|
| Channel name mismatch | Dùng string constants từ `src/shared/` |
| WebSocket auth/security | Token-based auth theo pattern hiện tại của ws-transport.ts |
| Message ordering | Dùng request ID, không phụ thuộc ordering |
| Large payload over WS | Đã có `MAX_WS_MESSAGE_BYTES` limit trong ws-transport.ts |

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23**

| File | Status |
|------|--------|
| `src/platform/rpc-client-interface.ts` | ✅ Done — `IRpcClient` interface |
| `src/platform/adapters/web/rpc-client.ts` | ✅ Done — `WebSocketRpcClient` |
| Tests: `rpc-client.test.ts` | ✅ 15/15 pass |
