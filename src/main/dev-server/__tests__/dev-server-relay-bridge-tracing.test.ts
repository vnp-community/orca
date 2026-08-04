// src/main/dev-server/__tests__/dev-server-relay-bridge-tracing.test.ts
//
// Tests for `connectRelayWebSocket()` tracing (TASK-BE-013.2/013.4).
// Covers `Tracers.agentWsHandshakeFlow` (`agentWs:handshake`) — the
// relay-websocket mode (Orca-as-WS-client) handshake span, distinct from
// `relay:agentCall` (post-connect RPC calls, untouched) and
// `agentWs:tokenVerify`/`agentWs:lifecycle` (direct-websocket mode, tested
// in agent-ws-server-tracing.test.ts).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import WebSocket from 'ws'
import { DevServerRelayBridge, type RelayHandshakeInfo } from '../dev-server-relay-bridge'
import type { PersistedDevServer } from '../../../shared/dev-server-types'
import type { SshConnectionManager } from '../../ssh/ssh-connection-manager'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'
import type { WsHandshakeInfo } from '../ws-handshake'

// ─── Mocks ───────────────────────────────────────────────────────────────────

// Minimal self-contained fake WebSocket (no 'node:events' import needed inside
// the vi.mock factory — factories are hoisted above imports).
vi.mock('ws', () => {
  class MockWebSocket {
    static instances: MockWebSocket[] = []
    binaryType = ''
    terminate = vi.fn()
    close = vi.fn()
    private listeners: Record<string, Array<(...args: unknown[]) => void>> = {}

    constructor(public url: string, public opts?: unknown) {
      MockWebSocket.instances.push(this)
    }

    on(event: string, cb: (...args: unknown[]) => void): this {
      ;(this.listeners[event] ??= []).push(cb)
      return this
    }

    emit(event: string, ...args: unknown[]): void {
      for (const cb of this.listeners[event] ?? []) cb(...args)
    }
  }
  return { default: MockWebSocket }
})

// vi.hoisted() runs before vi.mock() factories, making handshakeCtrl available
// inside the factory below.
const handshakeCtrl = vi.hoisted(() => ({
  pending: [] as Array<{ resolve: (info: WsHandshakeInfo) => void; reject: (err: Error) => void }>,
}))

vi.mock('../ws-handshake', () => ({
  runOrcaInitiatorHandshake: vi.fn(
    () =>
      new Promise<WsHandshakeInfo>((resolve, reject) => {
        handshakeCtrl.pending.push({ resolve, reject })
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
    this.onNotification = vi.fn()
    this.onDispose = vi.fn()
  }),
}))

vi.mock('../../../platform/context', () => ({
  getPlatform: vi.fn(() => ({ app: { getVersion: () => '1.4.0' } })),
}))

// ─── Helpers ─────────────────────────────────────────────────────────────────

function makeConfig(): PersistedDevServer {
  return {
    id: 'dev-01',
    name: 'dev-01',
    connectionType: 'relay-websocket',
    addedAt: 0,
  } as PersistedDevServer
}

function makeHandshakeInfo(overrides: Partial<WsHandshakeInfo> = {}): WsHandshakeInfo {
  return {
    platform: 'linux',
    arch: 'x64',
    nodeVersion: 'v20.11.0',
    agentVersion: '1.0.0',
    sessionId: 'sess-test-123',
    ...overrides,
  }
}

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

function connectRelayWebSocket(
  bridge: DevServerRelayBridge,
  rawUrl: string,
  opts: { testOnly?: boolean } = {}
): Promise<RelayHandshakeInfo> {
  return (
    bridge as unknown as {
      connectRelayWebSocket(rawUrl: string, opts: { testOnly?: boolean }): Promise<RelayHandshakeInfo>
    }
  ).connectRelayWebSocket(rawUrl, opts)
}

function lastWs(): InstanceType<typeof WebSocket> & { emit: (event: string, ...args: unknown[]) => void } {
  const instances = (WebSocket as unknown as { instances: unknown[] }).instances
  return instances[instances.length - 1] as InstanceType<typeof WebSocket> & {
    emit: (event: string, ...args: unknown[]) => void
  }
}

// ─── Tests ───────────────────────────────────────────────────────────────────

describe('DevServerRelayBridge.connectRelayWebSocket — agentWs:handshake tracing', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    handshakeCtrl.pending.length = 0
    ;(WebSocket as unknown as { instances: unknown[] }).instances.length = 0
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('success: start() → step(tcpConnected) → ok() with platform/nodeVersion', async () => {
    const { events, stop } = captureTraceEvents()
    const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)

    const promise = connectRelayWebSocket(bridge, 'ws://host:1/agent?token=tok-1', {})
    lastWs().emit('open')
    handshakeCtrl.pending[0]!.resolve(makeHandshakeInfo({ platform: 'darwin', nodeVersion: 'v22.0.0', agentVersion: '2.0.0' }))
    await promise
    stop()

    const spanEvents = events.filter((e) => e.flow === 'agentWs:handshake')
    expect(spanEvents.filter((e) => e.level === 'start')).toHaveLength(1)
    expect(spanEvents.filter((e) => e.level === 'step' && e.label === 'tcpConnected')).toHaveLength(1)
    const okEvent = spanEvents.find((e) => e.level === 'ok')
    expect(okEvent).toBeDefined()
    expect(okEvent?.fields).toMatchObject({ platform: 'darwin', nodeVersion: 'v22.0.0', agentVersion: '2.0.0' })

    // step must precede handshake — assert ordering via elapsedMs/array order
    const stepIdx = spanEvents.findIndex((e) => e.level === 'step')
    const okIdx = spanEvents.findIndex((e) => e.level === 'ok')
    expect(stepIdx).toBeLessThan(okIdx)
  })

  it('TCP timeout: fail() with phase:"tcpConnect" after 10s with no open event', async () => {
    vi.useFakeTimers()
    const { events, stop } = captureTraceEvents()
    const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)

    const promise = connectRelayWebSocket(bridge, 'ws://host:1/agent?token=tok-2', {})
    const rejection = expect(promise).rejects.toThrow(/TCP connection timed out/)

    await vi.advanceTimersByTimeAsync(10_001)
    await rejection
    stop()

    const failEvent = events.find((e) => e.flow === 'agentWs:handshake' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.phase).toBe('tcpConnect')
  })

  it('WS error before open: fail() with phase:"tcpConnect"', async () => {
    const { events, stop } = captureTraceEvents()
    const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)

    const promise = connectRelayWebSocket(bridge, 'ws://host:1/agent?token=tok-3', {})
    const rejection = expect(promise).rejects.toThrow(/WebSocket error/)
    lastWs().emit('error', new Error('ECONNREFUSED'))
    await rejection
    stop()

    const failEvent = events.find((e) => e.flow === 'agentWs:handshake' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.phase).toBe('tcpConnect')
    expect(failEvent?.fields.err).toContain('ECONNREFUSED')
  })

  it('handshake reject: fail() with phase:"handshake"', async () => {
    const { events, stop } = captureTraceEvents()
    const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)

    const promise = connectRelayWebSocket(bridge, 'ws://host:1/agent?token=tok-4', {})
    lastWs().emit('open')
    const rejection = expect(promise).rejects.toThrow('agent rejected handshake')
    handshakeCtrl.pending[0]!.reject(new Error('agent rejected handshake'))
    await rejection
    stop()

    const failEvent = events.find((e) => e.flow === 'agentWs:handshake' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.phase).toBe('handshake')
    expect(failEvent?.fields.err).toContain('agent rejected handshake')
  })

  it('reconnect after close creates a brand-new span (different id, not reused)', async () => {
    vi.useFakeTimers()
    const { events, stop } = captureTraceEvents()
    const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)

    // Not testOnly — enables the reconnect loop.
    const promise = connectRelayWebSocket(bridge, 'ws://host:1/agent?token=tok-5', {})
    const ws1 = lastWs()
    ws1.emit('open')
    handshakeCtrl.pending[0]!.resolve(makeHandshakeInfo())
    await promise

    // Simulate the agent WS dropping — bridge schedules a reconnect via
    // setTimeout(attempt, delayMs) (exponential backoff, ~2-3s for attempt 0).
    ws1.emit('close')
    await vi.advanceTimersByTimeAsync(3_100)

    const ws2 = lastWs()
    expect(ws2).not.toBe(ws1)
    ws2.emit('open')
    handshakeCtrl.pending[1]!.resolve(makeHandshakeInfo())
    await vi.advanceTimersByTimeAsync(0)
    stop()

    const startEvents = events.filter((e) => e.flow === 'agentWs:handshake' && e.level === 'start')
    expect(startEvents).toHaveLength(2)
    expect(startEvents[0]!.id).not.toBe(startEvents[1]!.id)
  })

  it('never puts the Authorization token value into agentWs:handshake TraceFields', async () => {
    const { events, stop } = captureTraceEvents()
    const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)

    const promise = connectRelayWebSocket(bridge, 'ws://host:1/agent?token=super-secret-token', {})
    lastWs().emit('open')
    handshakeCtrl.pending[0]!.resolve(makeHandshakeInfo())
    await promise
    stop()

    const spanEvents = events.filter((e) => e.flow === 'agentWs:handshake')
    for (const e of spanEvents) {
      expect(Object.values(e.fields)).not.toContain('super-secret-token')
      expect(JSON.stringify(e.fields)).not.toContain('super-secret-token')
    }
  })
})
