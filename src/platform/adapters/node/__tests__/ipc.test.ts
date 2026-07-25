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
      ipc.handle('math:add', async (_e: IpcEvent, a: number, b: number) => a + b)
      expect(await ipc.invoke('math:add', 0, 3, 4)).toBe(7)
    })

    it('supports synchronous handler', async () => {
      ipc.handle('sync:handler', (_e: IpcEvent) => 'sync-result')
      expect(await ipc.invoke('sync:handler', 0)).toBe('sync-result')
    })

    it('propagates handler errors', async () => {
      ipc.handle('bad:handler', async () => {
        throw new Error('boom')
      })
      await expect(ipc.invoke('bad:handler', 0)).rejects.toThrow('boom')
    })

    it('error message contains channel name for unknown channel', async () => {
      await expect(ipc.invoke('totally:unknown', 0)).rejects.toThrow('"totally:unknown"')
    })
  })

  describe('IpcEvent.sender', () => {
    it('sender.id matches windowId', async () => {
      let capturedId: number | undefined
      ipc.handle('test:sender-id', async (event: IpcEvent) => {
        capturedId = event.sender.id
      })
      await ipc.invoke('test:sender-id', 42)
      expect(capturedId).toBe(42)
    })

    it('sender.send() routes to the correct window', async () => {
      const win = manager.createWindow({})
      const received: any[] = []
      win.onSend('reply:channel', (args) => received.push(args))

      ipc.handle('test:reply', async (event: IpcEvent) => {
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
      ipc.on('test:throw', () => {
        throw new Error('listener crash')
      })
      expect(() => ipc.emit('test:throw', event)).not.toThrow()
    })
  })
})
