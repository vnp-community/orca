import { describe, it, expect, beforeEach, vi } from 'vitest'
import { WebIpcBridge } from '../web-ipc-bridge'
import { NodeIpcBridge } from '../ipc'
import { NodeWindowManager } from '../window'
import type { IpcEvent } from '../../../ipc-interface'

describe('WebIpcBridge', () => {
  let manager: NodeWindowManager
  let ipc: NodeIpcBridge
  let bridge: WebIpcBridge
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let reply: any // vi.fn() — typed as any for test convenience

  beforeEach(() => {
    manager = new NodeWindowManager()
    ipc = new NodeIpcBridge(manager)
    bridge = new WebIpcBridge(ipc)
    reply = vi.fn()
  })

  // ── type: invoke — success ──────────────────────────────────────────────────

  describe('invoke — success', () => {
    it('routes to handler and replies with result', async () => {
      ipc.handle('test:echo', async (_e: IpcEvent, val: any) => val)

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'r1', type: 'invoke', channel: 'test:echo', args: ['hello'] }),
        1,
        reply
      )

      const msg = JSON.parse(reply.mock.calls[0][0])
      expect(msg).toMatchObject({ id: 'r1', type: 'result', result: 'hello' })
    })

    it('passes multiple args correctly', async () => {
      ipc.handle('math:sum', async (_e: IpcEvent, a: number, b: number, c: number) => a + b + c)

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'r2', type: 'invoke', channel: 'math:sum', args: [1, 2, 3] }),
        1,
        reply
      )

      const msg = JSON.parse(reply.mock.calls[0][0])
      expect(msg.result).toBe(6)
    })

    it('passes windowId as sender.id to handler', async () => {
      let capturedId: number | undefined
      ipc.handle('test:who', async (event: IpcEvent) => {
        capturedId = event.sender.id
      })

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'r3', type: 'invoke', channel: 'test:who', args: [] }),
        42,
        reply
      )

      expect(capturedId).toBe(42)
    })

    it('handles missing/empty args array', async () => {
      ipc.handle('test:noargs', async () => 'ok')

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'r4', type: 'invoke', channel: 'test:noargs' }),
        1,
        reply
      )

      const msg = JSON.parse(reply.mock.calls[0][0])
      expect(msg.result).toBe('ok')
    })

    it('handles null result correctly', async () => {
      ipc.handle('test:null', async () => null)

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'r5', type: 'invoke', channel: 'test:null', args: [] }),
        1,
        reply
      )

      const msg = JSON.parse(reply.mock.calls[0][0])
      expect(msg).toMatchObject({ type: 'result', result: null })
    })
  })

  // ── type: invoke — errors ──────────────────────────────────────────────────

  describe('invoke — errors', () => {
    it('replies with error when handler throws', async () => {
      ipc.handle('test:boom', async () => {
        throw new Error('handler exploded')
      })

      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'e1', type: 'invoke', channel: 'test:boom', args: [] }),
        1,
        reply
      )

      const msg = JSON.parse(reply.mock.calls[0][0])
      expect(msg).toMatchObject({ id: 'e1', type: 'error', message: 'handler exploded' })
    })

    it('replies with error for unknown channel', async () => {
      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'e2', type: 'invoke', channel: 'no:handler', args: [] }),
        1,
        reply
      )

      const msg = JSON.parse(reply.mock.calls[0][0])
      expect(msg.type).toBe('error')
      expect(msg.id).toBe('e2')
    })

    it('includes id in error response', async () => {
      await bridge.handleWebSocketMessage(
        JSON.stringify({ id: 'err-id-99', type: 'invoke', channel: 'missing', args: [] }),
        1,
        reply
      )

      const msg = JSON.parse(reply.mock.calls[0][0])
      expect(msg.id).toBe('err-id-99')
    })
  })

  // ── type: send — fire-and-forget ───────────────────────────────────────────

  describe('send — fire-and-forget', () => {
    it('emits event without sending reply', async () => {
      const listener = vi.fn()
      ipc.on('test:ff', listener)

      await bridge.handleWebSocketMessage(
        JSON.stringify({ type: 'send', channel: 'test:ff', args: ['data'] }),
        1,
        reply
      )

      expect(listener).toHaveBeenCalledOnce()
      expect(reply).not.toHaveBeenCalled() // no reply for fire-and-forget
    })

    it('passes args to listener', async () => {
      let capturedArgs: any[] | undefined
      ipc.on('test:args-send', (_event: IpcEvent, ...args: any[]) => {
        capturedArgs = args
      })

      await bridge.handleWebSocketMessage(
        JSON.stringify({ type: 'send', channel: 'test:args-send', args: [1, 'two', true] }),
        1,
        reply
      )

      expect(capturedArgs).toEqual([1, 'two', true])
    })
  })

  // ── Malformed input ────────────────────────────────────────────────────────

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
        1,
        reply
      )

      expect(reply).not.toHaveBeenCalled()
    })
  })

  // ── pushToClients ──────────────────────────────────────────────────────────

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

    it('handles multiple args', () => {
      const broadcast = vi.fn()
      bridge.pushToClients('multi:args', ['a', 'b', 123], broadcast)
      const msg = JSON.parse(broadcast.mock.calls[0][0])
      expect(msg.args).toEqual(['a', 'b', 123])
    })
  })
})

// ── Integration: window.send → WebSocket push ─────────────────────────────────

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
