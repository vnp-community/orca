# Session Management — Orca Server

> **Scope**: Quản lý WebSocket connections, Tab state, Client isolation
> **Key files**:
> - [`src/main/runtime/runtime-rpc.ts`](../../src/main/runtime/runtime-rpc.ts) — WS session lifecycle
> - [`src/main/runtime/orca-runtime.ts`](../../src/main/runtime/orca-runtime.ts) — `onClientDisconnected`
> - [`src/shared/ws-transport.ts`](../../src/shared/ws-transport.ts) — `WebSocketTransport`
> - [`src/renderer/src/runtime/web-runtime-session.ts`](../../src/renderer/src/runtime/web-runtime-session.ts) — Client session state

---

## 1. Tổng quan

Orca dùng khái niệm "session" ở hai tầng khác nhau:

| Tầng | Khái niệm | Scope | Lifetime |
|------|-----------|-------|---------|
| **WS Connection** | Một WebSocket connection | 1 physical connection | Tồn tại đến khi disconnect |
| **Device Session** | 1 paired device (deviceToken) | 1 logical user device | Persist qua reconnects |
| **Tab Session** | 1 terminal/workspace tab | Chia sẻ giữa các connections cùng device | Persist trong runtime |

---

## 2. WebSocket Connection Lifecycle

### 2.1 Connection State

Mỗi WS connection được track bởi:

```typescript
// src/main/runtime/runtime-rpc.ts
private e2eeChannels   = new Map<WebSocket, E2EEChannel>()   // E2EE state
private wsConnectionIds = new Map<WebSocket, string>()         // random hex ID
// connectionId: 8 bytes hex (randomBytes(8).toString('hex'))
// → dùng để route binary streams (PTY output)
```

### 2.2 Connection Events

```typescript
wsTransport.onMessage((msg, _reply, ws) => {
  // Phase 1: trước E2EE ready — xử lý handshake messages
  let channel = this.e2eeChannels.get(ws)
  if (!channel) {
    // Tạo E2EEChannel mới
    channel = new E2EEChannel(ws, {
      serverSecretKey: this.e2eeKeypair!.secretKey,
      validateToken: (token) => !!this.deviceRegistry?.validateToken(token),
      onReady: (ch) => {
        // E2EE complete → update deviceToken → start handling RPCs
        if (ch.deviceToken) {
          wsTransport.setClientId(ws, ch.deviceToken)
          // updateLastSeen → persists deviceToken usage
          const device = this.deviceRegistry?.validateToken(ch.deviceToken)
          if (device) this.deviceRegistry?.updateLastSeen(device.deviceId)
        }
        this.wsConnectionIds.set(ws, randomBytes(8).toString('hex'))
      },
      onError: (code, reason) => { ws.close(code, reason) }
    })
    this.e2eeChannels.set(ws, channel)
  }

  // Phase 2: sau E2EE ready — forward đến RPC handler
  const authenticatedDeviceToken = this.e2eeChannels.get(ws)?.deviceToken ?? null
  channel.onMessage(decryptedMsg, encryptedReply => {
    this.handleRequest(decryptedMsg, encryptedReply, { authenticatedDeviceToken, wsTransport })
  })
})

wsTransport.onConnectionClose((clientId, ws, hasOtherConnections) => {
  const connectionId = this.wsConnectionIds.get(ws)
  this.wsConnectionIds.delete(ws)
  this.binaryStreamHandlers.delete(connectionId)

  const channel = this.e2eeChannels.get(ws)
  const deviceToken = channel?.deviceToken
  this.e2eeChannels.delete(ws)
  channel?.destroy()

  // Notify runtime về disconnect
  if (deviceToken) this.runtime.onClientDisconnected(deviceToken)
})
```

---

## 3. Device Session — Multi-connection per Device

Một device (deviceToken) có thể có **nhiều WS connections** cùng lúc:

```
deviceToken: "bbc860..."
  ├── WS connection 1 (Chrome tab 1) — connectionId: "a1b2c3d4"
  ├── WS connection 2 (Chrome tab 2) — connectionId: "e5f6a7b8"
  └── WS connection 3 (mobile app)   — connectionId: "c9d0e1f2"
```

`WebSocketTransport.setClientId(ws, deviceToken)` maps ws → deviceToken.

Khi revoke device:
```typescript
wsTransport.terminateClientConnections(device.token)
// → đóng TẤT CẢ WS connections có clientId === device.token
```

---

## 4. Tab Session — Workspace State

### 4.1 Tab là gì?

Tab trong Orca = một workspace item:
- Terminal tab (PTY session)
- Git worktree tab
- Agent output tab

```typescript
// session.tabs.* RPC methods
'session.tabs.list'        → Tab[]           // list tabs của connection này
'session.tabs.listAll'     → Tab[]           // list tất cả tabs (multi-connection)
'session.tabs.activate'    → void            // focus tab
'session.tabs.close'       → void
'session.tabs.createTerminal' → Tab
'session.tabs.move'        → void
'session.tabs.subscribe'   → stream          // events khi tabs thay đổi
'session.tabs.subscribeAll' → stream         // events từ tất cả connections
'session.tabs.unsubscribe'
'session.tabs.unsubscribeAll'
```

### 4.2 Tab State Machine (client-side)

```typescript
// src/renderer/src/runtime/web-runtime-session.ts
type TabState = {
  id:       string
  title:    string
  type:     'terminal' | 'worktree' | 'agent'
  isActive: boolean
  // ...
}
```

### 4.3 Tab Ownership

Mỗi terminal tab owned bởi 1 `connectionId`:
```typescript
type PtyHandle = {
  ptyId:        string
  connectionId: string   // connectionId của WS khi terminal được tạo
  // ...
}
```

Khi connection đóng → `onClientDisconnected(clientId)`:
```typescript
// src/main/runtime/orca-runtime.ts
onClientDisconnected(clientId: string): void {
  for (const handle of this.ptyHandles.values()) {
    if (handle.connectionId === clientId) {
      // Resize terminals to remove this client's size preference
      this.resizeForClient(handle, clientId, null)
      // Terminal tự nó không bị kill — vẫn chạy, client khác có thể reattach
    }
  }
  // Cancel pending requests, clear subscriptions
}
```

Terminal **không bị kill** khi disconnect → có thể reattach sau.

---

## 5. WebSocketTransport — Multi-client Abstraction

```typescript
// src/shared/ws-transport.ts
class WebSocketTransport {
  // Map: clientId (deviceToken) → Set<WebSocket>
  private clients: Map<string, Set<WebSocket>>

  setClientId(ws: WebSocket, clientId: string): void
  // → thêm ws vào clients[clientId]

  broadcast(clientId: string, message: string): void
  // → gửi message đến TẤT CẢ WS của clientId

  terminateClientConnections(clientId: string): void
  // → ws.terminate() cho tất cả WS của clientId

  onMessage(handler: (msg, reply, ws) => void): void
  onConnectionClose(handler: (clientId, ws, hasOtherConnections) => void): void
}
```

---

## 6. Binary Stream Sessions (PTY Output)

Binary streams được scoped theo `connectionId` (không phải `deviceToken`):

```typescript
private binaryStreamHandlers = new Map<
  string,                // connectionId
  Map<number, (frame: TerminalStreamFrame) => void>  // streamId → handler
>()

// Khi subscribe terminal output:
registerBinaryStreamHandler(connectionId, streamId, handler)
// → binaryStreamHandlers.get(connectionId).set(streamId, handler)

// Khi nhận binary WS frame:
const connectionId = this.wsConnectionIds.get(ws)
this.binaryStreamHandlers.get(connectionId)?.get(frame.streamId)?.(frame)

// Khi WS đóng:
this.binaryStreamHandlers.delete(connectionId)
// → tự động cleanup tất cả stream handlers của connection này
```

---

## 7. Client Session State (Web Client)

```typescript
// src/renderer/src/runtime/web-runtime-session.ts
class WebRuntimeSession {
  // Connection state
  private wsClient: WebRuntimeClient | null = null
  private connectionState: 'disconnected' | 'connecting' | 'connected'

  // Session data (sau khi connected)
  private runtimeId:  string
  private deviceToken: string
  private tabs:       Tab[]

  // Reconnect logic
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectAttempts: number = 0

  async connect(pairingOffer: PairingOffer): Promise<void> {
    this.wsClient = new WebRuntimeClient(pairingOffer.endpoint)
    await this.wsClient.connect(pairingOffer)  // E2EE handshake
    // → receive runtimeId, authToken từ bootstrap response
    // → sync tabs state
  }

  private async handleDisconnect(): Promise<void> {
    // Exponential backoff reconnect
    const delay = Math.min(1000 * 2 ** this.reconnectAttempts, 30_000)
    this.reconnectTimer = setTimeout(() => this.connect(savedOffer), delay)
    this.reconnectAttempts++
  }
}
```

---

## 8. Session Persistence

### 8.1 Server Side

Không có server-side session persistence (stateless per connection). State được lưu:
- **PTY processes**: vẫn chạy sau disconnect (daemon process), client có thể reattach
- **Orchestration tasks**: lưu trong SQLite → restore khi reconnect
- **SSH connections**: maintain qua disconnect nếu `gracePeriod > 0`

### 8.2 Client Side (Web)

```typescript
// localStorage hoặc sessionStorage (browser)
// Lưu:
// - pairing offer (để auto-reconnect)
// - runtimeId (để verify same server)
// - tab state / active tab
```

---

## 9. Session Isolation (Hiện tại vs Kế hoạch)

### Hiện tại — Single Runtime

```
deviceToken A ──┐
deviceToken B ──┤── OrcaRuntimeService (shared) ──► PTY, SSH, DB (shared)
deviceToken C ──┘
```

**Không có isolation**: mọi clients dùng cùng runtime, cùng DB.

### Kế hoạch (CR-LOGIN-002)

```
userId A ──► UserProcess A (fork) ──► PTY, SSH, DB (isolated in /data/orca/users/A/)
userId B ──► UserProcess B (fork) ──► PTY, SSH, DB (isolated in /data/orca/users/B/)
```

---

## 10. Tóm tắt Connection Lifecycle

```
Client                    OrcaRuntimeRpcServer          OrcaRuntimeService
  │                              │                              │
  │── WS connect ───────────────►│                              │
  │                              │ create E2EEChannel           │
  │── e2ee_hello ───────────────►│ ECDH sharedKey               │
  │◄─ e2ee_ready ────────────────│                              │
  │── e2ee_auth (encrypted) ────►│ validateToken → OK           │
  │                              │ setClientId(ws, deviceToken) │
  │                              │ wsConnectionIds.set(ws, connId)
  │                              │                              │
  │── { method: 'terminal.create', ... } ──────────────────────►│
  │◄─ { result: { ptyId } } ────────────────────────────────────│
  │                              │                              │
  │── { method: 'terminal.subscribe', ptyId } ─────────────────►│
  │◄─ binary PTY frames (encrypted) ────────────────────────────│
  │                              │                              │
  │  [disconnect / tab close]    │                              │
  │── WS close ─────────────────►│                              │
  │                              │ e2eeChannels.delete(ws)      │
  │                              │ binaryStreamHandlers.delete(connId)
  │                              │──────────────────────────────►│
  │                              │                   onClientDisconnected(deviceToken)
  │                              │                   resizeTerminals
  │                              │                   clearSubscriptions
  │                              │
  │  [reconnect sau 1-30s]       │
  │── WS connect ───────────────►│ repeat từ đầu...
```
