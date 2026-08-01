// src/relay/__tests__/agent-session.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { EventEmitter } from 'node:events'
import { createSession } from '../agent-session'
import { HEADER_SIZE } from '../agent-wire'
import type { AgentConfig } from '../agent-config'
import type { ToolDefinition } from '../agent-tool-registry'
import type { AgentLogger } from '../agent-logger'

// ─── MockWs ──────────────────────────────────────────────────────────────────
class MockWs extends EventEmitter {
  readyState = 1  // WebSocket.OPEN
  send = vi.fn()
  close = vi.fn()
}

// ─── Test helpers ─────────────────────────────────────────────────────────────
function buildFrame(payloadObj: object): Buffer {
  const payload = Buffer.from(JSON.stringify(payloadObj), 'utf8')
  const header  = Buffer.allocUnsafe(HEADER_SIZE)
  header.writeUInt8(1, 0)               // TYPE Regular
  header.writeUInt32BE(1, 1)            // SEQ=1
  header.writeUInt32BE(0, 5)            // ACK=0
  header.writeUInt32BE(payload.length, 9)
  return Buffer.concat([header, payload])
}

function buildKeepaliveFrame(): Buffer {
  const header = Buffer.allocUnsafe(HEADER_SIZE)
  header.writeUInt8(9, 0)   // TYPE KeepAlive
  header.writeUInt32BE(5, 1)
  header.writeUInt32BE(0, 5)
  header.writeUInt32BE(0, 9)
  return header
}

function extractJson(ws: MockWs, callIndex = 0): Record<string, unknown> {
  const buf = ws.send.mock.calls[callIndex]![0] as Buffer
  return JSON.parse(buf.subarray(HEADER_SIZE).toString('utf8'))
}

// ─── Config & stubs ───────────────────────────────────────────────────────────
const mockConfig: AgentConfig = {
  mode: 'direct-websocket',
  orcaUrl: 'wss://test',
  agentToken: 'tok-test',
  agentPort: 6799,
  devServerId: 'test-server',
  logLevel: 'info',
  workDir: '/tmp',
  toolPath: '/usr/bin',
  toolEnv: {},
  credentialDir: '/tmp/.creds',
  tlsRejectUnauthorized: true,
}

const mockTool: ToolDefinition = {
  name: 'tool1',
  binary: null,
  description: 'Test tool',
  inputSchema: { type: 'object', properties: {} },
  async handler() { return { stdout: 'ok', stderr: '', exitCode: 0 } },
}

const mockLog: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

// ─── Tests ────────────────────────────────────────────────────────────────────

/** Pre-built capabilities for tests — avoids async git/pty checks in most tests */
const MOCK_CAPS = ['fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees', 'pty'] as const

/** Create session with pre-built capabilities (sync handshake path — no git/pty I/O) */
function createTestSession(
  cfg: AgentConfig = mockConfig,
  tools: typeof mockTool[] = [],
  log: typeof mockLog = mockLog
) {
  return createSession(cfg, tools, log, MOCK_CAPS)
}

/**
 * Wait until the async sendHandshake (which runs buildCapabilities including git check)
 * has completed and ws.send has been called at least once.
 * Only used in tests that explicitly test dynamic capability detection.
 */
async function waitForHandshake(ws: MockWs): Promise<void> {
  await vi.waitFor(() => {
    if (ws.send.mock.calls.length === 0) {
      throw new Error('waitForHandshake: ws.send not called yet')
    }
  }, { timeout: 5000 })
}

describe('createSession().start()', () => {
  it('sends handshake immediately when ws.readyState=1 (OPEN)', () => {
    const ws = new MockWs()
    createTestSession(mockConfig, [mockTool]).start(ws as any)
    expect(ws.send).toHaveBeenCalledOnce()
    const rpc = extractJson(ws, 0)
    expect(rpc.method).toBe('agent.handshake')
  })

  it('handshake jsonrpc version is "2.0"', () => {
    const ws = new MockWs()
    createTestSession(mockConfig, [mockTool]).start(ws as any)
    expect(extractJson(ws, 0).jsonrpc).toBe('2.0')
  })

  it('handshake params include devServerId', () => {
    const ws = new MockWs()
    createTestSession(mockConfig, [mockTool]).start(ws as any)
    expect((extractJson(ws, 0).params as any).devServerId).toBe('test-server')
  })

  it('handshake params include tools list', () => {
    const ws = new MockWs()
    createTestSession(mockConfig, [mockTool]).start(ws as any)
    expect((extractJson(ws, 0).params as any).tools).toContain('tool1')
  })

  it('handshake params include agentToken when non-empty', () => {
    const ws = new MockWs()
    createTestSession(mockConfig).start(ws as any)
    expect((extractJson(ws, 0).params as any).agentToken).toBe('tok-test')
  })

  it('handshake does NOT include agentToken when empty string', () => {
    const ws = new MockWs()
    const cfg = { ...mockConfig, agentToken: '' }
    createTestSession(cfg).start(ws as any)
    expect((extractJson(ws, 0).params as any).agentToken).toBeUndefined()
  })

  it('waits for open event when ws.readyState != 1', () => {
    const ws = new MockWs()
    ws.readyState = 0  // CONNECTING
    createTestSession(mockConfig).start(ws as any)
    expect(ws.send).not.toHaveBeenCalled()
    ws.readyState = 1
    ws.emit('open')
    expect(ws.send).toHaveBeenCalledOnce()
  })

  it('handshake params include capabilities array', () => {
    const ws = new MockWs()
    createTestSession(mockConfig).start(ws as any)
    const caps = (extractJson(ws, 0).params as any).capabilities
    expect(Array.isArray(caps)).toBe(true)
    expect(caps).toContain('fs')
    expect(caps).toContain('preflight')
    expect(caps).toContain('agent.spawn')
  })

  it('dynamic buildCapabilities runs when no prebuilt caps provided', async () => {
    const ws = new MockWs()
    // No MOCK_CAPS — triggers real async buildCapabilities()
    createSession(mockConfig, [], mockLog).start(ws as any)
    await waitForHandshake(ws)
    const caps = (extractJson(ws, 0).params as any).capabilities
    expect(Array.isArray(caps)).toBe(true)
    expect(caps).toContain('fs')
  })
})

describe('keepalive', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('sends keepalive frame every 5000ms', async () => {
    const ws = new MockWs()
    // createTestSession uses MOCK_CAPS → sync handshake path (no I/O)
    // doHandshake() uses .then() → drain microtask queue with await Promise.resolve()
    createTestSession(mockConfig).start(ws as any)
    await Promise.resolve()  // drain .then() chain so startKeepalive is registered
    ws.send.mockClear()  // clear handshake

    vi.advanceTimersByTime(5001)

    expect(ws.send).toHaveBeenCalledOnce()
    const frame = ws.send.mock.calls[0]![0] as Buffer
    expect(frame.readUInt8(0)).toBe(9)   // TYPE_KEEPALIVE
    expect(frame.length).toBe(HEADER_SIZE)
  })

  it('sends multiple keepalives over 10s', async () => {
    const ws = new MockWs()
    createTestSession(mockConfig).start(ws as any)
    await Promise.resolve()  // drain microtask queue
    ws.send.mockClear()
    vi.advanceTimersByTime(10001)
    expect(ws.send.mock.calls.length).toBeGreaterThanOrEqual(2)
  })

  it('stop() prevents further keepalive sends', async () => {
    const ws = new MockWs()
    const session = createTestSession(mockConfig)
    session.start(ws as any)
    await Promise.resolve()  // drain microtask queue
    session.stop()
    ws.send.mockClear()
    vi.advanceTimersByTime(10000)
    expect(ws.send).not.toHaveBeenCalled()
  })
})

describe('message handling — handshake phase', () => {
  it('fires onHandshakeOk callback on result.ok=true', () => {
    const ws = new MockWs()
    const session = createSession(mockConfig, [], mockLog)
    const cb = vi.fn()
    session.onHandshakeOk(cb)
    session.start(ws as any)

    ws.emit('message', buildFrame({ jsonrpc: '2.0', id: 1, result: { ok: true, sessionId: 's1', orcaVersion: '1.0' } }))
    expect(cb).toHaveBeenCalledOnce()
  })

  it('calls ws.close(1008) on handshake error', () => {
    const ws = new MockWs()
    createSession(mockConfig, [], mockLog).start(ws as any)
    ws.emit('message', buildFrame({ jsonrpc: '2.0', id: 1, error: { code: -33101, message: 'AuthFailed' } }))
    expect(ws.close).toHaveBeenCalledWith(1008, expect.any(String))
  })

  it('ignores non-Buffer message data', () => {
    const ws = new MockWs()
    createSession(mockConfig, [], mockLog).start(ws as any)
    ws.send.mockClear()
    ws.emit('message', 'text data')
    expect(ws.send).not.toHaveBeenCalled()
  })
})

describe('KeepAlive frame response', () => {
  it('responds to KeepAlive frame with a keepalive frame', () => {
    const ws = new MockWs()
    createSession(mockConfig, [], mockLog).start(ws as any)
    ws.send.mockClear()

    ws.emit('message', buildKeepaliveFrame())

    expect(ws.send).toHaveBeenCalledOnce()
    const resp = ws.send.mock.calls[0]![0] as Buffer
    expect(resp.readUInt8(0)).toBe(9)  // KeepAlive type
    expect(resp.length).toBe(HEADER_SIZE)
  })
})

describe('onHandshakeOk', () => {
  it('supports multiple callbacks', () => {
    const ws = new MockWs()
    const session = createSession(mockConfig, [], mockLog)
    const cb1 = vi.fn()
    const cb2 = vi.fn()
    session.onHandshakeOk(cb1)
    session.onHandshakeOk(cb2)
    session.start(ws as any)

    ws.emit('message', buildFrame({ jsonrpc: '2.0', id: 1, result: { ok: true } }))
    expect(cb1).toHaveBeenCalledOnce()
    expect(cb2).toHaveBeenCalledOnce()
  })
})
