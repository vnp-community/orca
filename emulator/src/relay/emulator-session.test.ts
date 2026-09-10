// src/relay/emulator-session.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { EventEmitter } from 'node:events'
import { createWireState, encodeDataFrame, encodeKeepaliveFrame, decodeFrame } from 'orca-dev-agent-transport'
import { createEmulatorSession } from './emulator-session'
import type { EmulatorConfig } from './emulator-config'
import type { EmulatorLogger } from './emulator-logger'
import type { EmulatorRpcDispatcher, JsonRpcRequest, JsonRpcResponse } from './emulator-rpc-dispatch'

const HEADER_SIZE = 13

// ─── MockWs ──────────────────────────────────────────────────────────────────
class MockWs extends EventEmitter {
  readyState = 1 // WebSocket.OPEN
  send = vi.fn()
  close = vi.fn()
}

// ─── Test helpers ────────────────────────────────────────────────────────────
function extractJson(ws: MockWs, callIndex = 0): Record<string, unknown> {
  const buf = ws.send.mock.calls[callIndex]![0] as Buffer
  return JSON.parse(buf.subarray(HEADER_SIZE).toString('utf8'))
}

function buildResponseFrame(payloadObj: object): Buffer {
  const state = createWireState()
  return encodeDataFrame(state, JSON.stringify(payloadObj))
}

function buildKeepaliveFrame(): Buffer {
  return encodeKeepaliveFrame(createWireState())
}

const mockConfig: EmulatorConfig = {
  backendUrl: 'wss://test.example/agent',
  agentToken: 'tok-test',
  logLevel: 'info'
}

const mockLog: EmulatorLogger = { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() }

function makeDispatcher(handler: (rpc: JsonRpcRequest) => Promise<JsonRpcResponse>): EmulatorRpcDispatcher {
  return { dispatch: handler }
}

const noopDispatcher = makeDispatcher(async (rpc) => ({ jsonrpc: '2.0', id: rpc.id, result: {} }))

describe('createEmulatorSession().start()', () => {
  it('sends handshake immediately when ws.readyState=1 (OPEN)', () => {
    const ws = new MockWs()
    createEmulatorSession(mockConfig, mockLog, noopDispatcher).start(ws as any)
    expect(ws.send).toHaveBeenCalledOnce()
    const rpc = extractJson(ws, 0)
    expect(rpc.method).toBe('agent.handshake')
    expect(rpc.jsonrpc).toBe('2.0')
  })

  it('handshake params carry only the device capability — no tools/git/pty', () => {
    const ws = new MockWs()
    createEmulatorSession(mockConfig, mockLog, noopDispatcher).start(ws as any)
    const params = extractJson(ws, 0).params as Record<string, unknown>
    expect(params.capabilities).toEqual(['device'])
    expect(params.tools).toBeUndefined()
  })

  it('handshake params include agentVersion, platform, arch', () => {
    const ws = new MockWs()
    createEmulatorSession(mockConfig, mockLog, noopDispatcher).start(ws as any)
    const params = extractJson(ws, 0).params as Record<string, unknown>
    expect(params.agentVersion).toBeTruthy()
    expect(params.platform).toBe(process.platform)
    expect(params.arch).toBe(process.arch)
  })

  it('handshake params include agentToken when set', () => {
    const ws = new MockWs()
    createEmulatorSession(mockConfig, mockLog, noopDispatcher).start(ws as any)
    expect((extractJson(ws, 0).params as any).agentToken).toBe('tok-test')
  })

  it('handshake params omit agentToken when unset', () => {
    const ws = new MockWs()
    const cfg = { ...mockConfig, agentToken: undefined }
    createEmulatorSession(cfg, mockLog, noopDispatcher).start(ws as any)
    expect((extractJson(ws, 0).params as any).agentToken).toBeUndefined()
  })

  it('waits for open event when ws.readyState != 1', () => {
    const ws = new MockWs()
    ws.readyState = 0 // CONNECTING
    createEmulatorSession(mockConfig, mockLog, noopDispatcher).start(ws as any)
    expect(ws.send).not.toHaveBeenCalled()
    ws.readyState = 1
    ws.emit('open')
    expect(ws.send).toHaveBeenCalledOnce()
  })
})

describe('handshake response handling', () => {
  it('fires onHandshakeOk callback when result.ok === true', () => {
    const ws = new MockWs()
    const session = createEmulatorSession(mockConfig, mockLog, noopDispatcher)
    const onOk = vi.fn()
    session.onHandshakeOk(onOk)
    session.start(ws as any)

    ws.emit('message', buildResponseFrame({ jsonrpc: '2.0', id: 1, result: { ok: true, sessionId: 's1', orcaVersion: '1.0.0' } }))
    expect(onOk).toHaveBeenCalledOnce()
  })

  it('closes the connection with 1008 when handshake response carries an error', () => {
    const ws = new MockWs()
    const session = createEmulatorSession(mockConfig, mockLog, noopDispatcher)
    session.start(ws as any)

    ws.emit('message', buildResponseFrame({ jsonrpc: '2.0', id: 1, error: { code: -33101, message: 'bad token' } }))
    expect(ws.close).toHaveBeenCalledWith(1008, 'Handshake failed')
  })

  it('ignores non-handshake frames until handshake completes', () => {
    const ws = new MockWs()
    const dispatch = vi.fn(async (rpc: JsonRpcRequest) => ({ jsonrpc: '2.0' as const, id: rpc.id, result: {} }))
    const session = createEmulatorSession(mockConfig, mockLog, makeDispatcher(dispatch))
    session.start(ws as any)

    ws.emit('message', buildResponseFrame({ jsonrpc: '2.0', id: 2, method: 'device.list', params: {} }))
    expect(dispatch).not.toHaveBeenCalled()
  })
})

describe('post-handshake RPC dispatch', () => {
  async function startAndHandshake(dispatcher: EmulatorRpcDispatcher): Promise<MockWs> {
    const ws = new MockWs()
    const session = createEmulatorSession(mockConfig, mockLog, dispatcher)
    session.start(ws as any)
    ws.emit('message', buildResponseFrame({ jsonrpc: '2.0', id: 1, result: { ok: true, sessionId: 's1', orcaVersion: '1.0.0' } }))
    return ws
  }

  it('dispatches device.* requests to the EmulatorRpcDispatcher and sends the response back', async () => {
    const dispatch = vi.fn(async (rpc: JsonRpcRequest) => ({
      jsonrpc: '2.0' as const,
      id: rpc.id,
      result: { devices: [] }
    }))
    const ws = await startAndHandshake(makeDispatcher(dispatch))
    ws.send.mockClear()

    ws.emit('message', buildResponseFrame({ jsonrpc: '2.0', id: 7, method: 'device.list', params: {} }))
    await vi.waitFor(() => expect(ws.send).toHaveBeenCalledOnce())

    expect(dispatch).toHaveBeenCalledWith(expect.objectContaining({ method: 'device.list', id: 7 }))
    const response = extractJson(ws, 0)
    expect(response.id).toBe(7)
    expect((response.result as any).devices).toEqual([])
  })

  it('does not crash the session when the dispatcher throws', async () => {
    const dispatch = vi.fn(async () => {
      throw new Error('boom')
    })
    const ws = await startAndHandshake(makeDispatcher(dispatch))
    ws.send.mockClear()

    ws.emit('message', buildResponseFrame({ jsonrpc: '2.0', id: 8, method: 'device.list', params: {} }))
    await vi.waitFor(() => expect(mockLog.error).toHaveBeenCalled())
    expect(ws.send).not.toHaveBeenCalled()
  })
})

describe('keepalive', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('sends a keepalive frame every 5000ms', () => {
    const ws = new MockWs()
    createEmulatorSession(mockConfig, mockLog, noopDispatcher).start(ws as any)
    ws.send.mockClear() // clear handshake

    vi.advanceTimersByTime(5001)

    expect(ws.send).toHaveBeenCalledOnce()
    const frame = ws.send.mock.calls[0]![0] as Buffer
    expect(frame.readUInt8(0)).toBe(9) // TYPE_KEEPALIVE
    expect(frame.length).toBe(HEADER_SIZE)
  })

  it('stop() prevents further keepalive sends', () => {
    const ws = new MockWs()
    const session = createEmulatorSession(mockConfig, mockLog, noopDispatcher)
    session.start(ws as any)
    session.stop()
    ws.send.mockClear()
    vi.advanceTimersByTime(10000)
    expect(ws.send).not.toHaveBeenCalled()
  })

  it('responds to an incoming keepalive frame with a keepalive pong', () => {
    const ws = new MockWs()
    createEmulatorSession(mockConfig, mockLog, noopDispatcher).start(ws as any)
    ws.send.mockClear()

    ws.emit('message', buildKeepaliveFrame())

    expect(ws.send).toHaveBeenCalledOnce()
    const frame = ws.send.mock.calls[0]![0] as Buffer
    expect(frame.readUInt8(0)).toBe(9) // TYPE_KEEPALIVE
  })
})

describe('frame decode sanity (uses the real orca-dev-agent-transport decoder)', () => {
  it('a handshake frame this session sends round-trips through decodeFrame', () => {
    const ws = new MockWs()
    createEmulatorSession(mockConfig, mockLog, noopDispatcher).start(ws as any)
    const sentFrame = ws.send.mock.calls[0]![0] as Buffer
    const decoded = decodeFrame(createWireState(), sentFrame)!
    expect(decoded.type).toBe(1) // TYPE_REGULAR
    const payload = JSON.parse(decoded.payload.toString('utf8'))
    expect(payload.method).toBe('agent.handshake')
  })
})
