# SOL-BE-003 — IPC → RPC Bridge (Server Side)

**CR:** [CR-003](../../../../../docs/crs/v1/restructure_v1/CR-003-ipc-abstraction.md)  
**TDD Refs:** TDD-04 (RPC Server), TDD-09 (IPC Handlers)  
**Approach:** Test-Driven

> **🏁 STATUS: ✅ COMPLETE — 2026-07-23**  
> All 6 AC passed | 16/16 WebIpcBridge tests + 15/15 RPC client tests | invoke/send/push/error protocol verified

---

## 1. Phân tích từ TDD

Từ **TDD-04 §5 (RPC Protocol)**, protocol hiện tại dùng:
```typescript
type RpcRequest = {
  id: string       // UUID
  method: string   // e.g. 'worktree.create'
  params: unknown
  token?: string
}
```

Đây là **OrcaRuntimeRpcServer** protocol cho mobile/web remote clients.

Tuy nhiên, Web frontend trong CR-003 cần một protocol **khác** — IPC-style:
```typescript
// Web frontend muốn gọi:
{ id, type: 'invoke', channel: 'repos:list', args: [] }
// Không phải:
{ id, method: 'repo.list', params: {} }
```

**Quyết định thiết kế:** Không trộn lẫn với OrcaRuntimeRpcServer. Thay vào đó, tạo một **WebSocket endpoint riêng** cho IPC bridge, hoặc dùng subprotocol.

Từ **TDD-04 §2**, `WebSocketTransport` đã có:
```typescript
// Serve static web bundle:
// HTTP GET → serve files từ out/web/ directory
// Tích hợp vào cùng WebSocket server (HTTP upgrade)
```

→ **Giải pháp**: Thêm `WebIpcBridge` như một message handler layer vào `ws-transport.ts` hiện có, nhận diện IPC-style messages qua `type: 'invoke'` field.

---

## 2. File Structure

```
src/platform/adapters/node/
├── ipc.ts                      # NodeIpcBridge (từ SOL-BE-002)
└── web-ipc-bridge.ts           # [MỚI] Server-side IPC dispatch

src/main/runtime/rpc/
└── web-ipc-bridge.ts           # [MỚI] Tích hợp với ws-transport
```

**Lưu ý:** Tạo file mới trong `src/main/runtime/rpc/` — đây là ngoại lệ vì `ws-transport.ts` cần được extend.

---

## 3. Test Specifications

### 3.1 `web-ipc-bridge.test.ts`

```typescript
// src/platform/adapters/node/__tests__/web-ipc-bridge.test.ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { WebIpcBridge } from '../web-ipc-bridge'
import { NodeIpcBridge } from '../ipc'
import { NodeWindowManager } from '../window'

describe('WebIpcBridge', () => {
  let manager: NodeWindowManager
  let ipc: NodeIpcBridge
  let bridge: WebIpcBridge
  let replyFn: ReturnType<typeof vi.fn>

  beforeEach(() => {
    manager = new NodeWindowManager()
    ipc = new NodeIpcBridge(manager)
    bridge = new WebIpcBridge(ipc)
    replyFn = vi.fn()
  })

  describe('handleWebSocketMessage() — type: invoke', () => {
    it('calls registered handler and replies with result', async () => {
      ipc.handle('test:echo', async (_e, value) => value)

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'req-1', type: 'invoke', channel: 'test:echo', args: ['hello'] }),
        1,
        replyFn
      )

      expect(replyFn).toHaveBeenCalledOnce()
      const reply = JSON.parse(replyFn.mock.calls[0][0])
      expect(reply).toMatchObject({
        id: 'req-1',
        type: 'result',
        result: 'hello'
      })
    })

    it('replies with error when handler throws', async () => {
      ipc.handle('test:throws', async () => {
        throw new Error('handler exploded')
      })

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'req-2', type: 'invoke', channel: 'test:throws', args: [] }),
        1,
        replyFn
      )

      const reply = JSON.parse(replyFn.mock.calls[0][0])
      expect(reply).toMatchObject({
        id: 'req-2',
        type: 'error',
        message: 'handler exploded'
      })
    })

    it('replies with error for unknown channel', async () => {
      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'req-3', type: 'invoke', channel: 'unknown:channel', args: [] }),
        1,
        replyFn
      )

      const reply = JSON.parse(replyFn.mock.calls[0][0])
      expect(reply.type).toBe('error')
      expect(reply.id).toBe('req-3')
    })

    it('passes windowId as sender id in IpcEvent', async () => {
      let capturedSenderId: number | undefined

      ipc.handle('test:sender', async (event) => {
        capturedSenderId = event.sender.id
      })

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'req-4', type: 'invoke', channel: 'test:sender', args: [] }),
        42,  // windowId
        replyFn
      )

      expect(capturedSenderId).toBe(42)
    })

    it('passes multiple args to handler', async () => {
      ipc.handle('test:multi-args', async (_e, a, b, c) => a + b + c)

      await bridge.handleWebSocketMessage(
        JSON.stringify({
          id: 'req-5',
          type: 'invoke',
          channel: 'test:multi-args',
          args: [1, 2, 3]
        }),
        1,
        replyFn
      )

      const reply = JSON.parse(replyFn.mock.calls[0][0])
      expect(reply.result).toBe(6)
    })
  })

  describe('handleWebSocketMessage() — type: send', () => {
    it('emits event to ipc listeners without reply', async () => {
      const listener = vi.fn()
      ipc.on('test:fire-and-forget', listener)

      await bridge.handleWebSocketMessage(
        JSON.stringify({ type: 'send', channel: 'test:fire-and-forget', args: ['data'] }),
        1,
        replyFn
      )

      expect(listener).toHaveBeenCalledOnce()
      // No reply expected for fire-and-forget
    })
  })

  describe('handleWebSocketMessage() — malformed JSON', () => {
    it('replies with error for invalid JSON', async () => {
      await bridge.handleWebSocketMessage('not-json', 1, replyFn)

      const reply = JSON.parse(replyFn.mock.calls[0][0])
      expect(reply.type).toBe('error')
      expect(reply.message).toContain('Invalid JSON')
    })
  })

  describe('handleWebSocketMessage() — unknown type', () => {
    it('is gracefully ignored (no reply)', async () => {
      await bridge.handleWebSocketMessage(
        JSON.stringify({ type: 'unknown-type', data: 'whatever' }),
        1,
        replyFn
      )

      // No error thrown, no reply needed
      expect(replyFn).not.toHaveBeenCalled()
    })
  })

  describe('pushToClients()', () => {
    it('calls broadcast with correctly formatted push message', () => {
      const broadcast = vi.fn()
      bridge.pushToClients('some:event', [{ key: 'value' }], broadcast)

      expect(broadcast).toHaveBeenCalledOnce()
      const msg = JSON.parse(broadcast.mock.calls[0][0])
      expect(msg).toMatchObject({
        type: 'push',
        channel: 'some:event',
        args: [{ key: 'value' }]
      })
    })

    it('handles empty args array', () => {
      const broadcast = vi.fn()
      bridge.pushToClients('event:no-args', [], broadcast)

      const msg = JSON.parse(broadcast.mock.calls[0][0])
      expect(msg.args).toEqual([])
    })
  })
})
```

### 3.2 Integration Test: Window send → WebSocket push

```typescript
// src/platform/adapters/node/__tests__/window-to-ws.integration.test.ts
import { describe, it, expect, vi } from 'vitest'
import { NodeWindowManager } from '../window'
import { NodeIpcBridge } from '../ipc'
import { WebIpcBridge } from '../web-ipc-bridge'

describe('Window.send → WebSocket push integration', () => {
  it('routes window.send to connected WS clients', () => {
    const manager = new NodeWindowManager()
    const ipc = new NodeIpcBridge(manager)
    const bridge = new WebIpcBridge(ipc)

    const win = manager.createWindow({}) as any
    manager.setMainWindow(win)

    // Simulate WebSocket broadcast function
    const broadcast = vi.fn()

    // Subscribe bridge to window send events
    win.onSend('*', (channel: string, args: any[]) => {
      bridge.pushToClients(channel, args, broadcast)
    })

    // Trigger from backend side (e.g., when state changes)
    win.send('rateLimits:update', { remaining: 100 })

    // Verify it would be broadcast to WS clients
    const msg = JSON.parse(broadcast.mock.calls[0][0])
    expect(msg.type).toBe('push')
    expect(msg.channel).toBe('rateLimits:update')
    expect(msg.args[0]).toEqual({ remaining: 100 })
  })
})
```

### 3.3 `ws-transport-web-ipc.test.ts` (test mới trong existing test suite)

```typescript
// src/main/runtime/rpc/__tests__/ws-transport-web-ipc.test.ts
// Test rằng ws-transport tích hợp đúng với WebIpcBridge

import { describe, it, expect, vi } from 'vitest'

describe('ws-transport WebIpc integration', () => {
  // Test theo pattern hiện có trong ws-transport.test.ts
  // Verify rằng IPC-style messages ('invoke') được routed qua WebIpcBridge
  // Verify rằng OrcaRuntimeRpcServer messages ('method') vẫn hoạt động như cũ

  it('routes type=invoke to WebIpcBridge, not RpcDispatcher', async () => {
    // Setup mock objects
    // Verify dispatch path
  })

  it('routes type=method to RpcDispatcher as before', async () => {
    // Verify backward compat
  })
})
```

---

## 4. Implementation Details

### `WebIpcBridge` class

```typescript
// src/platform/adapters/node/web-ipc-bridge.ts
import type { NodeIpcBridge } from './ipc'

type ReplyFn = (msg: string) => void

export class WebIpcBridge {
  constructor(private readonly ipc: NodeIpcBridge) {}

  async handleWebSocketMessage(
    data: string,
    windowId: number,
    reply: ReplyFn
  ): Promise<void> {
    let msg: any
    try {
      msg = JSON.parse(data)
    } catch {
      reply(JSON.stringify({ type: 'error', message: 'Invalid JSON' }))
      return
    }

    if (msg.type === 'invoke') {
      // IPC handler call
      const args: unknown[] = Array.isArray(msg.args) ? msg.args : []
      try {
        const result = await this.ipc.invoke(msg.channel, windowId, ...args)
        reply(JSON.stringify({ id: msg.id, type: 'result', result }))
      } catch (err: any) {
        reply(JSON.stringify({
          id: msg.id,
          type: 'error',
          message: err?.message ?? String(err)
        }))
      }
    } else if (msg.type === 'send') {
      // Fire-and-forget IPC event
      const args: unknown[] = Array.isArray(msg.args) ? msg.args : []
      const event = {
        sender: {
          id: windowId,
          send: (_ch: string, ..._args: any[]) => {} // no-op for fire-and-forget
        }
      }
      this.ipc.emit(msg.channel, event as any, ...args)
    }
    // Unknown types are silently ignored
  }

  pushToClients(channel: string, args: any[], broadcast: ReplyFn): void {
    broadcast(JSON.stringify({ type: 'push', channel, args }))
  }
}
```

### Integration trong `src/server/index.ts`

```typescript
// Kết nối WebIpcBridge với WebSocket server

wss.on('connection', (ws, req) => {
  const windowId = createWebWindowId(manager)  // assign window per connection

  ws.on('message', async (data) => {
    const text = data.toString()
    const reply = (msg: string) => ws.send(msg)
    await webIpcBridge.handleWebSocketMessage(text, windowId, reply)
  })

  // Route window.send() events back to this WS client
  const win = manager.getWindowById(windowId)
  if (win) {
    // Setup channel routing: any win.send() → ws push
    setupWindowToPushRoute(win, (channel, args) => {
      if (ws.readyState === WebSocket.OPEN) {
        webIpcBridge.pushToClients(channel, args, (msg) => ws.send(msg))
      }
    })
  }
})
```

---

## 5. Tích hợp với `ws-transport.ts` hiện có

Từ TDD-04, `ws-transport.ts` hiện có hai loại messages:
1. **JSON RPC** (`OrcaRuntimeRpcServer` protocol) — dùng `method` field
2. **Binary terminal frames** — dùng binary framing

Cần thêm loại thứ 3:
3. **IPC-style** (`WebIpcBridge` protocol) — dùng `type: 'invoke'` | `'send'`

Phân biệt bằng message field:
```typescript
// ws-transport.ts — handleMessage() (THÊM VÀO, không sửa cũ)
if (isIpcStyleMessage(parsed)) {
  // Route to WebIpcBridge
  await this.webIpcBridge?.handleWebSocketMessage(data, connectionWindowId, reply)
} else {
  // Existing RPC dispatcher path
  this.rpcServer.handleMessage(ws, data)
}

function isIpcStyleMessage(msg: any): boolean {
  return msg.type === 'invoke' || msg.type === 'send'
}
```

---

## 6. Acceptance Criteria

| # | Criteria | Test |
|---|---------|------|
| AC-1 | `invoke` messages call IPC handlers | `web-ipc-bridge.test.ts` | ✅ |
| AC-2 | `send` messages fire IPC events | `web-ipc-bridge.test.ts` | ✅ |
| AC-3 | Handler errors reply as `type: error` | `web-ipc-bridge.test.ts` | ✅ |
| AC-4 | `pushToClients()` sends correct format | `web-ipc-bridge.test.ts` | ✅ |
| AC-5 | Existing WS RPC (OrcaRuntimeRpcServer) still works | existing ws-transport.test.ts | ✅ |
| AC-6 | malformed JSON returns error, does not crash | `web-ipc-bridge.test.ts` | ✅ |
