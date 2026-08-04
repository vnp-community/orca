import { describe, it, expect, vi } from 'vitest'
import { DevServerRelayBridge } from '../dev-server-relay-bridge'
import type { PersistedDevServer } from '../../../shared/dev-server-types'
import type { SshConnectionManager } from '../../ssh/ssh-connection-manager'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'

function makeConfig(): PersistedDevServer {
  return {
    id: 'dev-01',
    name: 'dev-01',
    connectionType: 'direct-websocket',
    addedAt: 0
  } as PersistedDevServer
}

function makeBridgeWithSession(requestImpl: (method: string, params?: unknown) => Promise<unknown>): DevServerRelayBridge {
  const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)
  bridge.session = { request: vi.fn(requestImpl) } as never
  return bridge
}

describe('DevServerRelayBridge.call — timeoutMs null guard', () => {
  // Regression: a call proxied from a per-user child process
  // (GatewayDevServerManagerProxy.getRelay().call → sendRequest → IPC) arrives
  // here with timeoutMs coerced from undefined to null by process.send's
  // array serialization. Without a guard, setTimeout(..., null) fires at 0ms,
  // rejecting every relay call almost instantly regardless of the real
  // 10s default — this is what "Relay call 'x' timed out after nullms" meant.

  it('does not reject almost-instantly when timeoutMs is explicitly null', async () => {
    const bridge = makeBridgeWithSession(
      () => new Promise((resolve) => setTimeout(() => resolve({ ok: true }), 50))
    )

    const result = await bridge.call('test.method', {}, null as unknown as number)
    expect(result).toEqual({ ok: true })
  })

  it('still uses the 10s default when timeoutMs is omitted (undefined)', async () => {
    const bridge = makeBridgeWithSession(
      () => new Promise((resolve) => setTimeout(() => resolve({ ok: true }), 50))
    )

    const result = await bridge.call('test.method', {})
    expect(result).toEqual({ ok: true })
  })

  it('still honors an explicit valid timeoutMs and rejects on a real timeout', async () => {
    const bridge = makeBridgeWithSession(() => new Promise(() => {})) // never resolves

    await expect(bridge.call('test.method', {}, 20)).rejects.toThrow(
      "Relay call 'test.method' timed out after 20ms"
    )
  })

  it('still throws the not-connected error when session is null, regardless of timeoutMs', async () => {
    const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)

    await expect(bridge.call('test.method', {}, null as unknown as number)).rejects.toThrow(
      /Dev Server agent not connected/
    )
  })
})

// ── CR-TRACE-001: callWithTimeout() relay:agentCall resume (TASK-BE-001.4) ─────

describe('DevServerRelayBridge.callWithTimeout — relay:agentCall span resume', () => {
  function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    return { events, stop: unregister }
  }

  it('resumes the relay:agentCall span id from params.traceId', async () => {
    const bridge = makeBridgeWithSession(() => Promise.resolve({ ok: true }))
    const { events, stop } = captureTraceEvents()

    await bridge.call('test.method', { traceId: 'resume-relay-1' })
    stop()

    const relayEvents = events.filter((e) => e.flow === 'relay:agentCall')
    expect(relayEvents.length).toBeGreaterThan(0)
    expect(relayEvents.every((e) => e.id === 'resume-relay-1')).toBe(true)
  })

  it('starts a new relay:agentCall span id when params.traceId is absent (backward compatible)', async () => {
    const bridge = makeBridgeWithSession(() => Promise.resolve({ ok: true }))
    const { events, stop } = captureTraceEvents()

    await bridge.call('test.method', {})
    stop()

    const relayEvents = events.filter((e) => e.flow === 'relay:agentCall')
    expect(relayEvents.length).toBeGreaterThan(0)
    expect(relayEvents[0]?.id).toBeTruthy()
  })

  it('emits ok() on the relay:agentCall span with the same resumed id on success', async () => {
    const bridge = makeBridgeWithSession(() => Promise.resolve({ ok: true }))
    const { events, stop } = captureTraceEvents()

    await bridge.call('test.method', { traceId: 'resume-relay-ok' })
    stop()

    const okEvent = events.find((e) => e.flow === 'relay:agentCall' && e.level === 'ok')
    expect(okEvent?.id).toBe('resume-relay-ok')
  })

  it('emits fail() with AGENT_NOT_CONNECTED before throwing, still resuming params.traceId', async () => {
    // Why: exercising callWithTimeout()'s own no-session guard directly — the
    // public call() wrapper throws its own "not connected" error before ever
    // reaching callWithTimeout when session is null from the start, so this
    // branch inside callWithTimeout is only reached via the reconnect-wait
    // path. Calling the private method directly isolates that guard's
    // resume behavior without needing to simulate the full reconnect timing.
    const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)
    const { events, stop } = captureTraceEvents()

    const callWithTimeout = (
      bridge as unknown as {
        callWithTimeout: (method: string, params: Record<string, unknown>, timeoutMs: number) => Promise<unknown>
      }
    ).callWithTimeout.bind(bridge)

    await expect(
      callWithTimeout('test.method', { traceId: 'resume-relay-fail' }, 1_000)
    ).rejects.toThrow(/Dev Server agent not connected/)
    stop()

    const failEvent = events.find((e) => e.flow === 'relay:agentCall' && e.level === 'fail')
    expect(failEvent?.id).toBe('resume-relay-fail')
    expect(failEvent?.fields.err).toContain('AGENT_NOT_CONNECTED')
  })

  it('emits fail() on the relay:agentCall span when the underlying request rejects', async () => {
    const bridge = makeBridgeWithSession(() => Promise.reject(new Error('boom')))
    const { events, stop } = captureTraceEvents()

    await expect(bridge.call('test.method', { traceId: 'resume-relay-reject' })).rejects.toThrow('boom')
    stop()

    const failEvent = events.find((e) => e.flow === 'relay:agentCall' && e.level === 'fail')
    expect(failEvent?.id).toBe('resume-relay-reject')
    expect(failEvent?.fields.err).toContain('boom')
  })

  // ── CR-TRACE-002 regression: agent.exec-style dual-field params (TASK-BE-002.4) ──
  // ProfileAwareAgentSpawner.spawn() sends agent.exec params with BOTH a flat
  // `traceId` (read here) and a nested `_trace: { id }` (read only by
  // agent-rpc-dispatch.ts on the Dev Server side). The nested field must not
  // confuse or break this bridge's flat-field resume.

  it('resumes relay:agentCall from the flat traceId on an agent.exec-style call that also carries a nested _trace.id', async () => {
    const bridge = makeBridgeWithSession(() => Promise.resolve({ sessionId: 'agent-sess-1' }))
    const { events, stop } = captureTraceEvents()

    await bridge.call('agent.exec', {
      binary: 'claude',
      args: [],
      cwd: '/repo',
      env: {},
      traceId: 'resume-agent-exec-1',
      _trace: { id: 'resume-agent-exec-1' }
    })
    stop()

    const relayEvents = events.filter((e) => e.flow === 'relay:agentCall')
    expect(relayEvents.length).toBeGreaterThan(0)
    expect(relayEvents.every((e) => e.id === 'resume-agent-exec-1')).toBe(true)
  })

  it('does not choke or leak the nested _trace field into span fields for an agent.exec-style call', async () => {
    const bridge = makeBridgeWithSession(() => Promise.resolve({ sessionId: 'agent-sess-2' }))
    const { events, stop } = captureTraceEvents()

    await bridge.call('agent.exec', {
      binary: 'claude',
      args: [],
      cwd: '/repo',
      env: {},
      traceId: 'resume-agent-exec-2',
      _trace: { id: 'resume-agent-exec-2' }
    })
    stop()

    const okEvent = events.find((e) => e.flow === 'relay:agentCall' && e.level === 'ok')
    expect(okEvent?.id).toBe('resume-agent-exec-2')
    expect(Object.keys(okEvent?.fields ?? {})).not.toContain('_trace')
  })
})

// ── onNotification regression ──────────────────────────────────────────────
// A prior version wired `bridge.onNotification(handler)` in DevServerManager
// against this class before it ever defined that method — every call threw
// synchronously (`bridge.onNotification is not a function`), aborting
// connect()/connectDaemonAgent() before the agent token was ever wired.
// These tests lock in a real, reconnect-safe implementation: DevServerManager
// depends on this to forward pty.data/pty.exit/fs.changed pushes from the
// agent up through devServer:notification to every per-user child process.

function makeMockMux(): { onNotification: ReturnType<typeof vi.fn>; fire: (method: string, params: Record<string, unknown>) => void } {
  let handler: ((method: string, params: Record<string, unknown>) => void) | null = null
  const unsubscribe = vi.fn(() => {
    handler = null
  })
  return {
    onNotification: vi.fn((h: (method: string, params: Record<string, unknown>) => void) => {
      handler = h
      return unsubscribe
    }),
    fire: (method: string, params: Record<string, unknown>) => handler?.(method, params)
  }
}

describe('DevServerRelayBridge.onNotification', () => {
  function wireForwarding(bridge: DevServerRelayBridge, mux: unknown): void {
    ;(bridge as unknown as { wireNotificationForwarding: (m: unknown) => void }).wireNotificationForwarding(mux)
  }

  it('does not throw when registered — the original regression', () => {
    const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)
    expect(() => bridge.onNotification(() => {})).not.toThrow()
  })

  it('forwards a notification pushed by the wired mux to the registered handler', () => {
    const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)
    const received: Array<[string, Record<string, unknown>]> = []
    bridge.onNotification((method, params) => received.push([method, params]))

    const mux = makeMockMux()
    wireForwarding(bridge, mux)
    mux.fire('pty.data', { id: 'pty-1', data: 'hello' })

    expect(received).toEqual([['pty.data', { id: 'pty-1', data: 'hello' }]])
  })

  it('keeps forwarding to a handler registered before any mux ever connected', () => {
    const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)
    const received: string[] = []
    bridge.onNotification((method) => received.push(method))

    // Handler registered first, mux "connects" later — mirrors connectWithExternalToken.
    const mux = makeMockMux()
    wireForwarding(bridge, mux)
    mux.fire('fs.changed', {})

    expect(received).toEqual(['fs.changed'])
  })

  it('re-wires to a new mux on reconnect without duplicating delivery from the old one', () => {
    const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)
    const received: string[] = []
    bridge.onNotification((method) => received.push(method))

    const firstMux = makeMockMux()
    wireForwarding(bridge, firstMux)
    firstMux.fire('pty.data', {})

    const secondMux = makeMockMux()
    wireForwarding(bridge, secondMux)
    firstMux.fire('pty.data', {}) // old mux — must not still be wired
    secondMux.fire('pty.exit', {})

    expect(received).toEqual(['pty.data', 'pty.exit'])
  })

  it('unsubscribe stops further delivery to that handler', () => {
    const bridge = new DevServerRelayBridge(makeConfig(), {} as SshConnectionManager, null)
    const received: string[] = []
    const unsubscribe = bridge.onNotification((method) => received.push(method))

    const mux = makeMockMux()
    wireForwarding(bridge, mux)
    mux.fire('pty.data', {})
    unsubscribe()
    mux.fire('pty.data', {})

    expect(received).toEqual(['pty.data'])
  })
})
