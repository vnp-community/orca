// src/relay/__tests__/agent-connection-direct.test.ts
//
// Covers the outer reconnect loop in agent-connection-direct.ts — previously
// untested (see specs/agent/api/gaps-and-findings.md #8's own caveat: the
// idle-watchdog/reconnect wiring was "theoretically" in place but had no
// end-to-end coverage proving a terminate()-triggered close actually drives
// a fresh reconnect). `ws` and `./agent-session` are mocked so this file
// isolates the loop's own close-code branching/backoff logic, not
// agent-session.ts's internals (already covered by agent-session.test.ts).
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { EventEmitter } from 'node:events'
import type { AgentConfig } from '../agent-config'
import type { ToolDefinition } from '../agent-tool-registry'
import type { AgentLogger } from '../agent-logger'

// ─── Mock `ws` — connectDirect does `new WebSocket(url, opts)` directly ────
// A plain constructable class (not vi.fn(() => ...)) — connectDirect's
// `new WebSocket(...)` requires the mock itself to support `new`, which an
// arrow-function mock implementation does not.
class MockWs extends EventEmitter {
  readyState = 1 // WebSocket.OPEN
  close = vi.fn()
  terminate = vi.fn()
  ping = vi.fn()
  constructor() {
    super()
    mockWsInstances.push(this)
  }
}
const mockWsInstances: MockWs[] = []
vi.mock('ws', () => ({ default: MockWs }))

// ─── Mock ./agent-session — isolate the reconnect loop, not session internals ──
type FakeSession = {
  start: ReturnType<typeof vi.fn>
  stop: ReturnType<typeof vi.fn>
  onHandshakeOk: ReturnType<typeof vi.fn>
  fireHandshakeOk: () => void
}
const fakeSessions: FakeSession[] = []
vi.mock('../agent-session', () => ({
  createSession: vi.fn().mockImplementation(() => {
    let handshakeOkCb: (() => void) | null = null
    const session: FakeSession = {
      start: vi.fn(),
      stop: vi.fn(),
      onHandshakeOk: vi.fn((cb: () => void) => { handshakeOkCb = cb }),
      fireHandshakeOk: () => handshakeOkCb?.()
    }
    fakeSessions.push(session)
    return session
  })
}))

const { connectDirect } = await import('../agent-connection-direct')

const mockConfig: AgentConfig = {
  mode: 'direct-websocket',
  orcaUrl: 'wss://test.example/agent',
  orcaHttpUrl: 'https://test.example',
  agentToken: 'tok-test',
  apiSecret: '', // falsy -> legacy AGENT_TOKEN path, no tokenManager/HTTP calls
  agentPort: 6799,
  devServerId: 'test-server',
  logLevel: 'info',
  workDir: '/tmp',
  toolPath: '/usr/bin',
  toolEnv: {},
  credentialDir: '/tmp/.creds',
  tlsRejectUnauthorized: true
}
const mockTools: ToolDefinition[] = []
const mockLog: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

describe('connectDirect — reconnect loop', () => {
  let exitSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    vi.useFakeTimers()
    mockWsInstances.length = 0
    fakeSessions.length = 0
    exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never)
  })

  afterEach(() => {
    vi.useRealTimers()
    exitSpy.mockRestore()
  })

  it('a clean close (code 1000) exits without reconnecting', async () => {
    void connectDirect(mockConfig, mockTools, mockLog)
    await vi.waitFor(() => expect(mockWsInstances.length).toBe(1))

    mockWsInstances[0]!.emit('close', 1000, Buffer.from(''))
    await vi.advanceTimersByTimeAsync(200) // the 100ms setTimeout before process.exit(0)

    expect(exitSpy).toHaveBeenCalledWith(0)
    expect(mockWsInstances.length).toBe(1) // never reconnected
  })

  it('a terminate()-triggered close (code 1006) after handshake-ok reconnects with backoff', async () => {
    void connectDirect(mockConfig, mockTools, mockLog)
    await vi.waitFor(() => expect(mockWsInstances.length).toBe(1))

    fakeSessions[0]!.fireHandshakeOk()
    // Simulate the liveness monitor's ws.terminate() — terminate() fires
    // 'close' with no clean-close code (1006 is the ws library's own
    // "abnormal closure" default for a locally-terminated socket).
    mockWsInstances[0]!.emit('close', 1006, Buffer.from(''))

    // Not reconnected yet — still waiting out RECONNECT_DELAYS_MS[0] (1s).
    await vi.advanceTimersByTimeAsync(500)
    expect(mockWsInstances.length).toBe(1)

    await vi.advanceTimersByTimeAsync(600)
    expect(mockWsInstances.length).toBe(2)
    expect(exitSpy).not.toHaveBeenCalled()
  })

  it('a close before handshake-ok also reconnects (auth-failed path)', async () => {
    void connectDirect(mockConfig, mockTools, mockLog)
    await vi.waitFor(() => expect(mockWsInstances.length).toBe(1))

    // No fireHandshakeOk() — connection never authenticated.
    mockWsInstances[0]!.emit('close', 1005, Buffer.from(''))

    await vi.advanceTimersByTimeAsync(1500)
    expect(mockWsInstances.length).toBe(2)
  })

  it('backoff escalates across repeated drops that never stay up', async () => {
    void connectDirect(mockConfig, mockTools, mockLog)
    await vi.waitFor(() => expect(mockWsInstances.length).toBe(1))

    // Cycle 1: handshake, then immediate drop (well under STABLE_CONNECTION_MS) — backoff should NOT reset.
    fakeSessions[0]!.fireHandshakeOk()
    mockWsInstances[0]!.emit('close', 1006, Buffer.from(''))
    await vi.advanceTimersByTimeAsync(1500) // RECONNECT_DELAYS_MS[0] = 1s
    expect(mockWsInstances.length).toBe(2)

    // Cycle 2: same — immediate drop again. Next delay should be
    // RECONNECT_DELAYS_MS[1] = 2s, not reset back to 1s.
    fakeSessions[1]!.fireHandshakeOk()
    mockWsInstances[1]!.emit('close', 1006, Buffer.from(''))
    await vi.advanceTimersByTimeAsync(1500) // only 1.5s elapsed — 2s delay not yet due
    expect(mockWsInstances.length).toBe(2)
    await vi.advanceTimersByTimeAsync(1000) // now past the 2s mark
    expect(mockWsInstances.length).toBe(3)
  })

  it('backoff resets to the shortest delay after a connection stays up past STABLE_CONNECTION_MS', async () => {
    void connectDirect(mockConfig, mockTools, mockLog)
    await vi.waitFor(() => expect(mockWsInstances.length).toBe(1))

    // First cycle: immediate drop, escalates backoff once (delay becomes 2s next).
    fakeSessions[0]!.fireHandshakeOk()
    mockWsInstances[0]!.emit('close', 1006, Buffer.from(''))
    await vi.advanceTimersByTimeAsync(1500)
    expect(mockWsInstances.length).toBe(2)

    // Second cycle: handshake stays up for STABLE_CONNECTION_MS (10s) before
    // dropping — this should reset the backoff back to RECONNECT_DELAYS_MS[0] (1s).
    fakeSessions[1]!.fireHandshakeOk()
    await vi.advanceTimersByTimeAsync(10_001)
    mockWsInstances[1]!.emit('close', 1006, Buffer.from(''))
    await vi.advanceTimersByTimeAsync(1_100) // would still be short of the escalated 2s/5s delays
    expect(mockWsInstances.length).toBe(3)
  })
})
