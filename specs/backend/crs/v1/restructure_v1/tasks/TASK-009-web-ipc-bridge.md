# TASK-009: Tạo `WebIpcBridge` (server-side IPC dispatch)

**Source:** SOL-BE-003  
**Phase:** 2 | **Effort:** S (45–60 min)  
**Depends on:** TASK-006 (NodeIpcBridge)

---

## Objective

Tạo `src/platform/adapters/node/web-ipc-bridge.ts` — lớp trung gian nhận WebSocket messages từ web frontend và dispatch chúng qua `NodeIpcBridge`. Đây là cầu nối giữa WebSocket server và hệ thống IPC handlers của Orca.

---

## Protocol

```
Web Frontend → WebSocket → WebIpcBridge → NodeIpcBridge → IPC Handler
                                                 ↓
Web Frontend ← WebSocket ← WebIpcBridge ← sendToWindow()
```

**Message types:**
| Type | Hướng | Ý nghĩa |
|------|-------|---------|
| `invoke` | Client→Server | Gọi IPC handler, chờ kết quả |
| `result` | Server→Client | Kết quả thành công |
| `error` | Server→Client | Kết quả lỗi |
| `send` | Client→Server | Fire-and-forget event |
| `push` | Server→Client | Push event từ backend |

---

## Files to create

### 1. `src/platform/adapters/node/web-ipc-bridge.ts`

```typescript
import type { NodeIpcBridge } from './ipc'
import type { IpcEvent } from '../../ipc-interface'

export type BroadcastFn = (msg: string) => void

/**
 * WebIpcBridge — server-side handler for WebSocket IPC messages.
 *
 * Each WebSocket connection (representing one browser tab/client)
 * sends messages that are routed through this bridge into NodeIpcBridge.
 *
 * The `reply` function (from WebSocket.send) sends responses back.
 * The `broadcast` function (in pushToClients) sends server-push events.
 */
export class WebIpcBridge {
  constructor(private readonly ipc: NodeIpcBridge) {}

  /**
   * Handle an incoming WebSocket message from a client.
   *
   * @param data - Raw message string (expected JSON)
   * @param windowId - ID of the NodeWindow/connection this message came from
   * @param reply - Function to send response back to this specific client
   */
  async handleWebSocketMessage(
    data: string,
    windowId: number,
    reply: BroadcastFn
  ): Promise<void> {
    let msg: any
    try {
      msg = JSON.parse(data)
    } catch {
      reply(JSON.stringify({ type: 'error', message: 'Invalid JSON' }))
      return
    }

    if (msg.type === 'invoke') {
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
      // Fire-and-forget: route to ipc.emit() without expecting a reply
      const args: unknown[] = Array.isArray(msg.args) ? msg.args : []
      const event: IpcEvent = {
        sender: {
          id: windowId,
          send: (_ch: string, ..._args: any[]) => {} // intentional no-op for send events
        }
      }
      this.ipc.emit(msg.channel, event, ...args)
      // No reply for fire-and-forget
    }
    // Unknown types are silently ignored
  }

  /**
   * Push a server-side event to connected clients.
   * Called when backend code wants to notify the web frontend.
   *
   * @param channel - Event channel name
   * @param args - Event payload args
   * @param broadcast - Function to send to the target client(s)
   */
  pushToClients(channel: string, args: any[], broadcast: BroadcastFn): void {
    broadcast(JSON.stringify({ type: 'push', channel, args }))
  }
}
```

### 2. `src/platform/adapters/node/__tests__/web-ipc-bridge.test.ts`

```typescript
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { WebIpcBridge } from '../web-ipc-bridge'
import { NodeIpcBridge } from '../ipc'
import { NodeWindowManager } from '../window'

describe('WebIpcBridge', () => {
  let manager: NodeWindowManager
  let ipc: NodeIpcBridge
  let bridge: WebIpcBridge
  let reply: ReturnType<typeof vi.fn>

  beforeEach(() => {
    manager = new NodeWindowManager()
    ipc = new NodeIpcBridge(manager)
    bridge = new WebIpcBridge(ipc)
    reply = vi.fn()
  })

  // ── type: invoke ────────────────────────────────────────────────────────────

  describe('invoke — success', () => {
    it('routes to handler and replies with result', async () => {
      ipc.handle('test:echo', async (_e, val) => val)

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'r1', type: 'invoke', channel: 'test:echo', args: ['hello'] }),
        1, reply
      )

      const msg = JSON.parse(reply.mock.calls[0][0])
      expect(msg).toMatchObject({ id: 'r1', type: 'result', result: 'hello' })
    })

    it('passes multiple args correctly', async () => {
      ipc.handle('math:sum', async (_e, a, b, c) => a + b + c)

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'r2', type: 'invoke', channel: 'math:sum', args: [1, 2, 3] }),
        1, reply
      )

      const msg = JSON.parse(reply.mock.calls[0][0])
      expect(msg.result).toBe(6)
    })

    it('passes windowId as sender.id to handler', async () => {
      let capturedId: number | undefined
      ipc.handle('test:who', async (event) => { capturedId = event.sender.id })

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'r3', type: 'invoke', channel: 'test:who', args: [] }),
        42, reply
      )

      expect(capturedId).toBe(42)
    })

    it('handles empty args array', async () => {
      ipc.handle('test:noargs', async () => 'ok')

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'r4', type: 'invoke', channel: 'test:noargs' }),
        1, reply
      )

      const msg = JSON.parse(reply.mock.calls[0][0])
      expect(msg.result).toBe('ok')
    })
  })

  describe('invoke — errors', () => {
    it('replies with error when handler throws', async () => {
      ipc.handle('test:boom', async () => { throw new Error('handler exploded') })

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'e1', type: 'invoke', channel: 'test:boom', args: [] }),
        1, reply
      )

      const msg = JSON.parse(reply.mock.calls[0][0])
      expect(msg).toMatchObject({ id: 'e1', type: 'error', message: 'handler exploded' })
    })

    it('replies with error for unknown channel', async () => {
      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'e2', type: 'invoke', channel: 'no:handler', args: [] }),
        1, reply
      )

      const msg = JSON.parse(reply.mock.calls[0][0])
      expect(msg.type).toBe('error')
      expect(msg.id).toBe('e2')
    })
  })

  // ── type: send ──────────────────────────────────────────────────────────────

  describe('send — fire-and-forget', () => {
    it('emits event without sending reply', async () => {
      const listener = vi.fn()
      ipc.on('test:ff', listener)

      await bridge.handleWebSocketMessage(
        JSON.stringify({ type: 'send', channel: 'test:ff', args: ['data'] }),
        1, reply
      )

      expect(listener).toHaveBeenCalledOnce()
      expect(reply).not.toHaveBeenCalled()  // no reply for fire-and-forget
    })
  })

  // ── Malformed input ─────────────────────────────────────────────────────────

  describe('malformed JSON', () => {
    it('replies with error', async () => {
      await bridge.handleWebSocketMessage('not-json', 1, reply)

      const msg = JSON.parse(reply.mock.calls[0][0])
      expect(msg.type).toBe('error')
      expect(msg.message).toContain('Invalid JSON')
    })
  })

  describe('unknown type', () => {
    it('is silently ignored (no reply)', async () => {
      await bridge.handleWebSocketMessage(
        JSON.stringify({ type: 'unknown-type', data: 'x' }),
        1, reply
      )

      expect(reply).not.toHaveBeenCalled()
    })
  })

  // ── pushToClients ───────────────────────────────────────────────────────────

  describe('pushToClients()', () => {
    it('sends correct push message format', () => {
      const broadcast = vi.fn()
      bridge.pushToClients('ssh:state', [{ connected: true }], broadcast)

      const msg = JSON.parse(broadcast.mock.calls[0][0])
      expect(msg).toMatchObject({
        type: 'push',
        channel: 'ssh:state',
        args: [{ connected: true }]
      })
    })

    it('handles empty args', () => {
      const broadcast = vi.fn()
      bridge.pushToClients('event:empty', [], broadcast)
      const msg = JSON.parse(broadcast.mock.calls[0][0])
      expect(msg.args).toEqual([])
    })
  })
})
```

### 3. Integration test: `window.send` → WS push

```typescript
// src/platform/adapters/node/__tests__/window-to-ws-integration.test.ts
import { describe, it, expect, vi } from 'vitest'
import { NodeWindowManager } from '../window'
import { NodeIpcBridge } from '../ipc'
import { WebIpcBridge } from '../web-ipc-bridge'

describe('window.send → WebSocket push integration', () => {
  it('backend send() propagates to broadcast function', () => {
    const manager = new NodeWindowManager()
    const ipc = new NodeIpcBridge(manager)
    const bridge = new WebIpcBridge(ipc)

    const win = manager.createWindow({})
    manager.setMainWindow(win)
    const broadcast = vi.fn()

    // Simulate WebSocket client subscription to this window's messages
    win.onSend('rateLimits:update', (args) => {
      bridge.pushToClients('rateLimits:update', args, broadcast)
    })

    // Backend pushes a message
    ipc.sendToWindow(win.id, 'rateLimits:update', { remaining: 50 })

    const msg = JSON.parse(broadcast.mock.calls[0][0])
    expect(msg).toMatchObject({
      type: 'push',
      channel: 'rateLimits:update',
      args: [{ remaining: 50 }]
    })
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep "web-ipc-bridge" | head -10
npx vitest run src/platform/adapters/node/__tests__/web-ipc-bridge.test.ts
npx vitest run src/platform/adapters/node/__tests__/window-to-ws-integration.test.ts
```

Expected: **20+ tests pass**, 0 errors.

---

## Done criteria

- [x] `src/platform/adapters/node/web-ipc-bridge.ts` tạo thành công
- [x] `handleWebSocketMessage()` xử lý đúng 4 type: invoke/send/error/unknown
- [x] `invoke` trả về `{ id, type: 'result', result }` khi thành công
- [x] `invoke` trả về `{ id, type: 'error', message }` khi fail
- [x] `send` không gửi reply
- [x] `pushToClients()` format: `{ type: 'push', channel, args }`
- [x] Integration test pass: `window.send → broadcast`
