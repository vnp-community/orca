# TASK-006: Tạo `NodeIpcBridge`

**Source:** SOL-BE-002  
**Phase:** 1 | **Effort:** S (45–60 min)  
**Depends on:** TASK-002, TASK-003, TASK-005

---

## Objective

Tạo `src/platform/adapters/node/ipc.ts` — implementation của `IIpcBridge` chạy in-process trong Node.js. Đây là thành phần quan trọng nhất vì toàn bộ IPC handlers của Orca sẽ route qua nó.

---

## Files to create

### 1. `src/platform/adapters/node/ipc.ts`

```typescript
import { EventEmitter } from 'node:events'
import type {
  IIpcBridge,
  IpcHandler,
  IpcListener,
  IpcEvent
} from '../../ipc-interface'
import type { IWindowManager } from '../../window-interface'

/**
 * NodeIpcBridge — IIpcBridge implementation for Node.js server mode.
 *
 * Replaces Electron's ipcMain in the server context:
 * - handle(channel, handler): registers async IPC handler
 * - invoke(channel, windowId, ...args): dispatches a call (used by WebIpcBridge)
 * - on/off/emit: fire-and-forget events
 * - sendToWindow/sendToAll: push events to connected clients
 *
 * When WebIpcBridge receives a WebSocket message, it calls invoke() here.
 * When backend code calls sendToWindow(), it routes to NodeWindow.send()
 * which then propagates to WebSocket clients.
 */
export class NodeIpcBridge extends EventEmitter implements IIpcBridge {
  private readonly _handlers = new Map<string, IpcHandler>()
  private readonly _listeners = new Map<string, Set<IpcListener>>()
  private readonly _windowManager: IWindowManager

  constructor(windowManager: IWindowManager) {
    super()
    this._windowManager = windowManager
  }

  // ── Handler registration ───────────────────────────────────────────────────

  handle(channel: string, listener: IpcHandler): void {
    if (this._handlers.has(channel)) {
      console.warn(
        `[NodeIpcBridge] Overwriting existing handler for channel: "${channel}"`
      )
    }
    this._handlers.set(channel, listener)
  }

  removeHandler(channel: string): void {
    this._handlers.delete(channel)
  }

  // ── Event subscription ─────────────────────────────────────────────────────

  on(channel: string, listener: IpcListener): this {
    let set = this._listeners.get(channel)
    if (!set) {
      set = new Set()
      this._listeners.set(channel, set)
    }
    set.add(listener)
    return this
  }

  off(channel: string, listener: IpcListener): this {
    this._listeners.get(channel)?.delete(listener)
    return this
  }

  // ── Dispatch (called by WebIpcBridge) ──────────────────────────────────────

  /**
   * Invoke a registered handler.
   * Called by WebIpcBridge when a WebSocket 'invoke' message arrives.
   *
   * @param channel - The IPC channel name
   * @param windowId - The ID of the window/connection that sent the request
   * @param args - Arguments to pass to the handler
   */
  async invoke(channel: string, windowId: number, ...args: any[]): Promise<any> {
    const handler = this._handlers.get(channel)
    if (!handler) {
      throw new Error(
        `[NodeIpcBridge] No IPC handler registered for channel: "${channel}"`
      )
    }

    const event: IpcEvent = {
      sender: {
        id: windowId,
        send: (replyChannel: string, ...replyArgs: any[]) => {
          this.sendToWindow(windowId, replyChannel, ...replyArgs)
        }
      }
    }

    return handler(event, ...args)
  }

  /**
   * Emit a fire-and-forget event.
   * Notifies all listeners registered via on().
   * Called by WebIpcBridge when a WebSocket 'send' message arrives.
   */
  emit(channel: string, event: IpcEvent, ...args: any[]): boolean {
    const set = this._listeners.get(channel)
    if (set) {
      for (const listener of set) {
        try {
          listener(event, ...args)
        } catch (err) {
          console.error(`[NodeIpcBridge] Listener error on "${channel}":`, err)
        }
      }
    }
    return super.emit(channel, event, ...args)
  }

  // ── Push notifications ─────────────────────────────────────────────────────

  sendToWindow(windowId: number, channel: string, ...args: any[]): void {
    const windows = this._windowManager.getAllWindows()
    const win = windows.find(w => w.id === windowId)
    if (win) {
      win.send(channel, ...args)
    }
  }

  sendToAll(channel: string, ...args: any[]): void {
    for (const win of this._windowManager.getAllWindows()) {
      win.send(channel, ...args)
    }
  }
}
```

### 2. `src/platform/adapters/node/__tests__/ipc.test.ts`

```typescript
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { NodeIpcBridge } from '../ipc'
import { NodeWindowManager } from '../window'
import { runIIpcBridgeConformanceTests } from '../../../__tests__/interface-conformance'
import type { IpcEvent } from '../../../ipc-interface'

// ── Conformance ──────────────────────────────────────────────────────────────
runIIpcBridgeConformanceTests(() => {
  const manager = new NodeWindowManager()
  return new NodeIpcBridge(manager)
})

// ── NodeIpcBridge-specific ───────────────────────────────────────────────────
describe('NodeIpcBridge — specific behavior', () => {
  let manager: NodeWindowManager
  let ipc: NodeIpcBridge

  beforeEach(() => {
    manager = new NodeWindowManager()
    ipc = new NodeIpcBridge(manager)
  })

  describe('invoke() — handler dispatch', () => {
    it('passes all args to handler correctly', async () => {
      ipc.handle('math:add', async (_e, a: number, b: number) => a + b)
      expect(await ipc.invoke('math:add', 0, 3, 4)).toBe(7)
    })

    it('supports synchronous handler', async () => {
      ipc.handle('sync:handler', (_e) => 'sync-result')
      expect(await ipc.invoke('sync:handler', 0)).toBe('sync-result')
    })

    it('propagates handler errors', async () => {
      ipc.handle('bad:handler', async () => { throw new Error('boom') })
      await expect(ipc.invoke('bad:handler', 0)).rejects.toThrow('boom')
    })

    it('error message contains channel name for unknown channel', async () => {
      await expect(ipc.invoke('totally:unknown', 0))
        .rejects.toThrow('"totally:unknown"')
    })
  })

  describe('IpcEvent.sender', () => {
    it('sender.id matches windowId', async () => {
      let capturedId: number | undefined
      ipc.handle('test:sender-id', async (event) => {
        capturedId = event.sender.id
      })
      await ipc.invoke('test:sender-id', 42)
      expect(capturedId).toBe(42)
    })

    it('sender.send() routes to the correct window', async () => {
      const win = manager.createWindow({})
      const received: any[] = []
      win.onSend('reply:channel', (args) => received.push(args))

      ipc.handle('test:reply', async (event) => {
        event.sender.send('reply:channel', 'pong')
      })
      await ipc.invoke('test:reply', win.id)

      expect(received).toHaveLength(1)
      expect(received[0]).toEqual(['pong'])
    })
  })

  describe('sendToWindow()', () => {
    it('delivers message to correct window', () => {
      const win = manager.createWindow({})
      const received: any[] = []
      win.onSend('push:event', (args) => received.push(args))

      ipc.sendToWindow(win.id, 'push:event', 'hello', 99)

      expect(received).toHaveLength(1)
      expect(received[0]).toEqual(['hello', 99])
    })

    it('is silent when window not found', () => {
      expect(() => ipc.sendToWindow(9999, 'any:channel')).not.toThrow()
    })
  })

  describe('sendToAll()', () => {
    it('sends to every window', () => {
      const w1 = manager.createWindow({})
      const w2 = manager.createWindow({})
      const r1: any[] = []
      const r2: any[] = []
      w1.onSend('broadcast', (a) => r1.push(a))
      w2.onSend('broadcast', (a) => r2.push(a))

      ipc.sendToAll('broadcast', 'data')

      expect(r1).toHaveLength(1)
      expect(r2).toHaveLength(1)
    })

    it('is a no-op when no windows', () => {
      expect(() => ipc.sendToAll('broadcast', 'data')).not.toThrow()
    })
  })

  describe('emit() — fire-and-forget', () => {
    it('calls registered on() listeners', () => {
      const listener = vi.fn()
      const event: IpcEvent = { sender: { id: 0, send: vi.fn() } }
      ipc.on('test:emit', listener)
      ipc.emit('test:emit', event, 'arg1')
      expect(listener).toHaveBeenCalledWith(event, 'arg1')
    })

    it('does not crash when listener throws', () => {
      const event: IpcEvent = { sender: { id: 0, send: vi.fn() } }
      ipc.on('test:throw', () => { throw new Error('listener crash') })
      expect(() => ipc.emit('test:throw', event)).not.toThrow()
    })
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep "adapters/node/ipc" | head -10
npx vitest run src/platform/adapters/node/__tests__/ipc.test.ts
```

Expected: **20+ tests pass**, 0 TypeScript errors.

---

## Done criteria

- [x] `src/platform/adapters/node/ipc.ts` tạo thành công
- [x] `invoke()` method tồn tại (NodeIpcBridge-specific, không có trong interface)
- [x] `emit()` method không crash khi listener throw
- [x] `sender.send()` route đúng đến window
- [x] 20+ unit tests pass
