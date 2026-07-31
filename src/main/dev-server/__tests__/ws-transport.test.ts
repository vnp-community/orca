// src/main/dev-server/__tests__/ws-transport.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createWebSocketTransport, WS_READY_STATE } from '../ws-transport'
import { runOrcaInitiatorHandshake, runOrcaReceiverHandshake } from '../ws-handshake'
import { encodeJsonRpcFrame } from '../../ssh/relay-protocol'
import { AGENT_HANDSHAKE_METHOD } from '../../../shared/agent-wire-protocol'

// ─── Mock WsLike factory ─────────────────────────────────────────────────────

type MockListener = (data?: unknown) => void

function createMockWs(readyState = WS_READY_STATE.OPEN) {
  const listeners: Record<string, MockListener[]> = {}
  const sent: Buffer[] = []

  const ws = {
    readyState,
    send: vi.fn((data: Buffer) => {
      sent.push(data)
    }),
    close: vi.fn(),
    on: vi.fn((event: string, cb: MockListener) => {
      if (!listeners[event]) listeners[event] = []
      listeners[event].push(cb)
    }),
    // Test helpers
    _emit(event: string, data?: unknown) {
      for (const cb of listeners[event] ?? []) cb(data)
    },
    _sent: sent,
  }
  return ws
}

// ─── WebSocket Transport ─────────────────────────────────────────────────────

describe('createWebSocketTransport', () => {
  it('forwards incoming message Buffer to onData handlers', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    const received: Buffer[] = []
    transport.onData((data) => received.push(data))

    const payload = Buffer.from('test-payload')
    ws._emit('message', payload)

    expect(received).toHaveLength(1)
    expect(received[0]).toEqual(payload)
  })

  it('write() calls ws.send() when readyState is OPEN', () => {
    const ws = createMockWs(WS_READY_STATE.OPEN)
    const transport = createWebSocketTransport(ws)
    const data = Buffer.from('hello')
    transport.write(data)
    expect(ws.send).toHaveBeenCalledWith(data)
  })

  it('write() is a no-op when readyState is CLOSED', () => {
    const ws = createMockWs(WS_READY_STATE.CLOSED)
    const transport = createWebSocketTransport(ws)
    transport.write(Buffer.from('should-not-send'))
    expect(ws.send).not.toHaveBeenCalled()
  })

  it('write() is a no-op when readyState is CLOSING', () => {
    const ws = createMockWs(WS_READY_STATE.CLOSING)
    const transport = createWebSocketTransport(ws)
    transport.write(Buffer.from('should-not-send'))
    expect(ws.send).not.toHaveBeenCalled()
  })

  it('ws "close" event triggers all onClose handlers', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    const h1 = vi.fn()
    const h2 = vi.fn()
    transport.onClose(h1)
    transport.onClose(h2)
    ws._emit('close')
    expect(h1).toHaveBeenCalledOnce()
    expect(h2).toHaveBeenCalledOnce()
  })

  it('ws "error" event triggers onClose handlers (errors always close WS)', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    const onClose = vi.fn()
    transport.onClose(onClose)
    ws._emit('error', new Error('network error'))
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('close() calls ws.close()', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    transport.close?.()
    expect(ws.close).toHaveBeenCalledOnce()
  })

  it('onData handler error is isolated (other handlers still called)', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    const badHandler = vi.fn(() => {
      throw new Error('handler crash')
    })
    const goodHandler = vi.fn()
    transport.onData(badHandler)
    transport.onData(goodHandler)
    expect(() => ws._emit('message', Buffer.from('x'))).not.toThrow()
    expect(goodHandler).toHaveBeenCalledOnce()
  })

  it('multiple onData handlers each receive the message', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    const h1 = vi.fn()
    const h2 = vi.fn()
    transport.onData(h1)
    transport.onData(h2)
    const data = Buffer.from('multi')
    ws._emit('message', data)
    expect(h1).toHaveBeenCalledWith(data)
    expect(h2).toHaveBeenCalledWith(data)
  })
})

// ─── Initiator Handshake (relay-websocket) ───────────────────────────────────

describe('runOrcaInitiatorHandshake', () => {
  beforeEach(() => {
    vi.useRealTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })
  it('sends agent.handshake request immediately after setup', async () => {
    const ws = createMockWs()
    const promise = runOrcaInitiatorHandshake(ws, '1.4.0')

    // Should have sent exactly one frame
    expect(ws.send).toHaveBeenCalledOnce()
    expect(ws._sent[0]).toBeInstanceOf(Buffer)

    // Settle the promise to avoid unhandled rejection
    const response = encodeJsonRpcFrame(
      {
        jsonrpc: '2.0',
        id: 1,
        result: {
          ok: true,
          platform: 'linux',
          arch: 'x64',
          agentVersion: '1.0.0',
          sessionId: 'sess-settle',
        },
      },
      1,
      0
    )
    ws._emit('message', response)
    await promise
  })

  it('resolves with correct platform/arch/sessionId from agent response', async () => {
    const ws = createMockWs()
    const promise = runOrcaInitiatorHandshake(ws, '1.4.0')

    const response = encodeJsonRpcFrame(
      {
        jsonrpc: '2.0',
        id: 1,
        result: {
          ok: true,
          platform: 'linux',
          arch: 'arm64',
          nodeVersion: 'v20.11.0',
          agentVersion: '1.0.0',
          sessionId: 'sess-test-123',
        },
      },
      1,
      0
    )
    ws._emit('message', response)

    const info = await promise
    expect(info.platform).toBe('linux')
    expect(info.arch).toBe('arm64')
    expect(info.nodeVersion).toBe('v20.11.0')
    expect(info.agentVersion).toBe('1.0.0')
    expect(info.sessionId).toBe('sess-test-123')
  })

  it('rejects on error response from agent', async () => {
    const ws = createMockWs()
    const promise = runOrcaInitiatorHandshake(ws, '1.4.0')

    const errorResponse = encodeJsonRpcFrame(
      {
        jsonrpc: '2.0',
        id: 1,
        error: { code: -33100, message: 'Version too old' },
      },
      1,
      0
    )
    ws._emit('message', errorResponse)

    await expect(promise).rejects.toThrow('Agent rejected handshake')
    expect(ws.close).toHaveBeenCalled()
  })

  it('rejects and closes ws on timeout', async () => {
    vi.useFakeTimers()
    const ws = createMockWs()
    const promise = runOrcaInitiatorHandshake(ws, '1.4.0').catch((e: Error) => e)
    await vi.advanceTimersByTimeAsync(21_000)
    const result = await promise
    expect(result).toBeInstanceOf(Error)
    expect((result as Error).message).toMatch('timed out')
    expect(ws.close).toHaveBeenCalled()
  })

  it('ignores keepalive frames (0x09) without resolving', async () => {
    const ws = createMockWs()
    const promise = runOrcaInitiatorHandshake(ws, '1.4.0')

    // Keepalive frame: type=0x09, 13 bytes header only
    const keepalive = Buffer.alloc(13, 0)
    keepalive[0] = 0x09
    ws._emit('message', keepalive)

    // Promise should still be pending — settle with real response
    const response = encodeJsonRpcFrame(
      {
        jsonrpc: '2.0',
        id: 1,
        result: {
          ok: true,
          platform: 'darwin',
          arch: 'arm64',
          agentVersion: '1.0.0',
          sessionId: 'sess-after-ka',
        },
      },
      2,
      1
    )
    ws._emit('message', response)
    const info = await promise
    expect(info.sessionId).toBe('sess-after-ka')
  })
})

// ─── Receiver Handshake (direct-websocket) ───────────────────────────────────

describe('runOrcaReceiverHandshake', () => {
  const VALID_TOKEN = 'valid-token-abc'
  const validateToken = (t: string) => t === VALID_TOKEN

  beforeEach(() => {
    vi.useRealTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })
  function makeHandshakeFrame(overrides: Record<string, unknown> = {}) {
    return encodeJsonRpcFrame(
      {
        jsonrpc: '2.0',
        id: 1,
        method: AGENT_HANDSHAKE_METHOD,
        params: {
          agentVersion: '1.0.0',
          agentToken: VALID_TOKEN,
          platform: 'linux',
          arch: 'x64',
          nodeVersion: 'v20.11.0',
          capabilities: ['fs', 'git'],
          ...overrides,
        },
      },
      1,
      0
    )
  }

  it('resolves and sends handshake-ok on valid token', async () => {
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')

    ws._emit('message', makeHandshakeFrame())
    const info = await promise

    expect(info.platform).toBe('linux')
    expect(info.arch).toBe('x64')
    expect(info.agentToken).toBe(VALID_TOKEN)
    expect(ws.send).toHaveBeenCalledOnce() // sent handshake-ok
    expect(ws.close).not.toHaveBeenCalled() // did NOT close on success
  })

  it('resolved info contains correct nodeVersion and agentVersion', async () => {
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')
    ws._emit('message', makeHandshakeFrame())
    const info = await promise
    expect(info.nodeVersion).toBe('v20.11.0')
    expect(info.agentVersion).toBe('1.0.0')
  })

  it('sessionId in resolved info starts with sess-', async () => {
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')
    ws._emit('message', makeHandshakeFrame())
    const info = await promise
    expect(info.sessionId).toMatch(/^sess-\d+/)
  })

  it('rejects and closes ws on invalid token', async () => {
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')

    ws._emit('message', makeHandshakeFrame({ agentToken: 'wrong-token' }))

    await expect(promise).rejects.toThrow('authentication failed')
    expect(ws.send).toHaveBeenCalledOnce() // sent error frame
    expect(ws.close).toHaveBeenCalled()
  })

  it('rejects and closes ws if first message is not agent.handshake', async () => {
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')

    const wrongMsg = encodeJsonRpcFrame(
      { jsonrpc: '2.0', id: 1, method: 'preflight.check', params: {} },
      1,
      0
    )
    ws._emit('message', wrongMsg)

    await expect(promise).rejects.toThrow('Protocol violation')
    expect(ws.close).toHaveBeenCalled()
  })

  it('rejects and closes ws on timeout (no message received)', async () => {
    vi.useFakeTimers()
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0').catch((e: Error) => e)
    await vi.advanceTimersByTimeAsync(21_000)
    const result = await promise
    expect(result).toBeInstanceOf(Error)
    expect((result as Error).message).toMatch('did not send handshake')
    expect(ws.close).toHaveBeenCalled()
  })

  it('rejects on first message being a response (not a request)', async () => {
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')

    // A response frame (has result, no method)
    const responseFrame = encodeJsonRpcFrame(
      { jsonrpc: '2.0', id: 1, result: { ok: true } },
      1,
      0
    )
    ws._emit('message', responseFrame)

    await expect(promise).rejects.toThrow('Protocol violation')
    expect(ws.close).toHaveBeenCalled()
  })
})
