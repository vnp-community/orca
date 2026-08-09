// src/main/dev-server/__tests__/agent-ws-server-tracing.test.ts
//
// Tests for `handleConnection()` tracing (TASK-BE-013.3/013.4) — covers
// `Tracers.agentWsTokenVerifyFlow` (`agentWs:tokenVerify`), including the
// orphaned-span bugfix: the span must now open at socket-upgrade time (top
// of handleConnection()) and be REUSED (same id) in the handshake-reject
// `.catch()` branch, instead of a brand-new random-id span created only
// after the fact. `Tracers.agentWsFlow` (`agentWs:lifecycle`) remains an
// independent span for the connect→disconnect lifecycle, untouched.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { AgentWebSocketServer } from '../agent-ws-server'
import type { WsHandshakeInfo } from '../ws-handshake'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'

// ─── Mocks ───────────────────────────────────────────────────────────────────

// vi.hoisted() runs before vi.mock() factories, making handshakeCtrl available
const handshakeCtrl = vi.hoisted(() => ({
  resolve: null as ((info: WsHandshakeInfo) => void) | null,
  reject: null as ((err: Error) => void) | null,
}))

vi.mock('../ws-handshake', () => ({
  runOrcaReceiverHandshake: vi.fn(
    (_ws: unknown, validate: (t: string) => boolean, _ver: string) =>
      new Promise<WsHandshakeInfo>((res, rej) => {
        // Mimic the real handshake: `validate(token)` fires as the agent's
        // token arrives over the wire, BEFORE the handshake promise resolves
        // — this is what makes handleConnection()'s span.step('tokenLookup')
        // observable in these tests.
        handshakeCtrl.resolve = (info: WsHandshakeInfo) => {
          if (info.agentToken) {validate(info.agentToken)}
          res(info)
        }
        handshakeCtrl.reject = rej
      })
  ),
}))

vi.mock('../ws-transport', () => ({
  createWebSocketTransport: vi.fn(() => ({
    write: vi.fn(),
    onData: vi.fn(),
    onClose: vi.fn(),
    close: vi.fn(),
  })),
}))

vi.mock('../../ssh/ssh-channel-multiplexer', () => ({
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  SshChannelMultiplexer: vi.fn(function (this: any) {
    this._isMock = true
  }),
}))

// ─── Helpers ─────────────────────────────────────────────────────────────────

function makeHandshakeInfo(overrides: Partial<WsHandshakeInfo> = {}): WsHandshakeInfo {
  return {
    platform: 'linux',
    arch: 'x64',
    nodeVersion: 'v20.11.0',
    agentVersion: '1.0.0',
    sessionId: 'sess-test-123',
    agentToken: 'test-token-abc',
    ...overrides,
  }
}

// A more complete WS mock than the plain `{ close: vi.fn() }` used by
// agent-ws-server.test.ts — this suite exercises the success path, which
// calls `ws.once('close', ...)`, so `.once`/`.emit` must actually work.
function makeMockWs() {
  const listeners: Record<string, ((...args: unknown[]) => void)[]> = {}
  return {
    close: vi.fn(),
    once(event: string, cb: (...args: unknown[]) => void) {
      ;(listeners[event] ??= []).push(cb)
    },
    emit(event: string, ...args: unknown[]) {
      for (const cb of listeners[event] ?? []) {cb(...args)}
    },
  }
}

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

function triggerConnection(server: AgentWebSocketServer, ws: unknown): void {
  ;(server as unknown as { handleConnection(ws: unknown): void }).handleConnection(ws)
}

// ─── Tests ───────────────────────────────────────────────────────────────────

describe('AgentWebSocketServer.handleConnection — agentWs:tokenVerify tracing', () => {
  let server: AgentWebSocketServer

  beforeEach(() => {
    vi.clearAllMocks()
    handshakeCtrl.resolve = null
    handshakeCtrl.reject = null
    server = new AgentWebSocketServer('1.4.0')
  })

  afterEach(() => {
    server.stop()
    vi.useRealTimers()
  })

  it('span starts (start()) synchronously before the handshake resolves or rejects', () => {
    const { events, stop } = captureTraceEvents()

    triggerConnection(server, makeMockWs())
    stop()

    // handleConnection() returns synchronously without ever calling
    // resolve/reject — the span must already have a 'start' event.
    const startEvents = events.filter((e) => e.flow === 'agentWs:tokenVerify' && e.level === 'start')
    expect(startEvents).toHaveLength(1)
  })

  it('valid token, slot exists: step(tokenLookup) then ok({devServerId, sessionId}); tokenPrefix never contains the full token', async () => {
    const { events, stop } = captureTraceEvents()
    server.registerSlot('test-token-abc-full-secret', vi.fn(), vi.fn())

    triggerConnection(server, makeMockWs())
    handshakeCtrl.resolve!(makeHandshakeInfo({ agentToken: 'test-token-abc-full-secret', devServerId: 'dev-42' }))
    await new Promise((r) => setTimeout(r, 0))
    stop()

    const spanEvents = events.filter((e) => e.flow === 'agentWs:tokenVerify')
    const stepEvent = spanEvents.find((e) => e.level === 'step' && e.label === 'tokenLookup')
    expect(stepEvent).toBeDefined()
    const tokenPrefix = String(stepEvent?.fields.tokenPrefix ?? '')
    expect(tokenPrefix.length).toBeLessThanOrEqual(15) // 12 chars + '...'
    expect(tokenPrefix).not.toContain('test-token-abc-full-secret')

    const okEvent = spanEvents.find((e) => e.level === 'ok')
    expect(okEvent).toBeDefined()
    expect(okEvent?.fields).toMatchObject({ devServerId: 'dev-42', sessionId: 'sess-test-123' })

    // Ordering: tokenLookup step precedes ok()
    const stepIdx = spanEvents.findIndex((e) => e.level === 'step')
    const okIdx = spanEvents.findIndex((e) => e.level === 'ok')
    expect(stepIdx).toBeLessThan(okIdx)
  })

  it('slot expired (race): handshake resolves but slot already gone → fail("slot-expired", {devServerId})', async () => {
    const { events, stop } = captureTraceEvents()
    const disposer = server.registerSlot('race-token', vi.fn(), vi.fn())
    // Simulate the slot expiring/being removed between validate-check and resolve.
    disposer()

    triggerConnection(server, makeMockWs())
    handshakeCtrl.resolve!(makeHandshakeInfo({ agentToken: 'race-token', devServerId: 'dev-race' }))
    await new Promise((r) => setTimeout(r, 0))
    stop()

    const failEvent = events.find((e) => e.flow === 'agentWs:tokenVerify' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.err).toBe('slot-expired')
    expect(failEvent?.fields.devServerId).toBe('dev-race')
  })

  it('invalid token (orphaned-span bugfix): fail(err, {reason:"invalid-token"}) reuses the SAME span id from start()', async () => {
    const { events, stop } = captureTraceEvents()

    triggerConnection(server, makeMockWs())
    handshakeCtrl.reject!(new Error('Auth failed — invalid token'))
    await new Promise((r) => setTimeout(r, 0))
    stop()

    const spanEvents = events.filter((e) => e.flow === 'agentWs:tokenVerify')
    const startEvent = spanEvents.find((e) => e.level === 'start')
    const failEvent = spanEvents.find((e) => e.level === 'fail')

    expect(startEvent).toBeDefined()
    expect(failEvent).toBeDefined()
    // The core bugfix assertion: fail() must carry the SAME span id that
    // start() emitted — not a fresh, unrelated random id.
    expect(failEvent!.id).toBe(startEvent!.id)
    expect(failEvent?.fields.reason).toBe('invalid-token')
    expect(failEvent?.fields.err).toContain('Auth failed')
  })

  it('agentWsFlow (lifecycle) is unaffected: still starts on connect and steps on disconnect, with an id independent from agentWsTokenVerifyFlow', async () => {
    const { events, stop } = captureTraceEvents()
    const onConnected = vi.fn()
    server.registerSlot('lifecycle-token', onConnected, vi.fn())

    const ws = makeMockWs()
    triggerConnection(server, ws)
    handshakeCtrl.resolve!(makeHandshakeInfo({ agentToken: 'lifecycle-token', devServerId: 'dev-lc' }))
    await new Promise((r) => setTimeout(r, 0))

    ws.emit('close', 1000, Buffer.from('bye'))
    stop()

    const tokenVerifyEvents = events.filter((e) => e.flow === 'agentWs:tokenVerify')
    const lifecycleEvents = events.filter((e) => e.flow === 'agentWs:lifecycle')

    expect(tokenVerifyEvents.some((e) => e.level === 'start')).toBe(true)
    expect(lifecycleEvents.some((e) => e.level === 'start')).toBe(true)
    expect(lifecycleEvents.some((e) => e.level === 'step' && e.label === 'connected')).toBe(true)
    expect(lifecycleEvents.some((e) => e.level === 'step' && e.label === 'disconnect')).toBe(true)

    // Independent spans — different ids for the same connection.
    const tokenVerifyId = tokenVerifyEvents.find((e) => e.level === 'start')?.id
    const lifecycleId = lifecycleEvents.find((e) => e.level === 'start')?.id
    expect(tokenVerifyId).toBeDefined()
    expect(lifecycleId).toBeDefined()
    expect(tokenVerifyId).not.toBe(lifecycleId)

    expect(onConnected).toHaveBeenCalledOnce()
  })
})
