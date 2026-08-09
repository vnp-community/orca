/**
 * Tests for `preflight.check` RPC handler tracing (TASK-BE-014.3).
 *
 * Covers `src/main/runtime/rpc/methods/preflight.ts` — instrumented with
 * `Tracers.remoteIntegrationPreflightFlow`, forwarding `traceId` to the
 * relay-delegated call and resuming a Browser-originated span id.
 *
 * @module main/runtime/rpc/methods/__tests__/preflight-tracing.test
 */

import { describe, it, expect, vi } from 'vitest'

const {
  detectInstalledAgentsWithShellPathHydrationMock,
  detectRemoteAgentsMock,
  detectRemoteWindowsTerminalCapabilitiesMock,
  refreshShellPathAndDetectAgentsMock,
  runPreflightCheckMock
} = vi.hoisted(() => ({
  detectInstalledAgentsWithShellPathHydrationMock: vi.fn(),
  detectRemoteAgentsMock: vi.fn(),
  detectRemoteWindowsTerminalCapabilitiesMock: vi.fn(),
  refreshShellPathAndDetectAgentsMock: vi.fn(),
  runPreflightCheckMock: vi.fn()
}))

vi.mock('../../../../ipc/preflight', () => ({
  detectInstalledAgentsWithShellPathHydration: detectInstalledAgentsWithShellPathHydrationMock,
  detectRemoteAgents: detectRemoteAgentsMock,
  detectRemoteWindowsTerminalCapabilities: detectRemoteWindowsTerminalCapabilitiesMock,
  refreshShellPathAndDetectAgents: refreshShellPathAndDetectAgentsMock,
  runPreflightCheck: runPreflightCheckMock
}))

import { PREFLIGHT_METHODS } from '../preflight'
import type { RpcContext, RpcMethod } from '../../core'
import { registerTraceSink, type TraceEvent } from '../../../../../shared/trace'

function findMethod(name: string): RpcMethod {
  const m = PREFLIGHT_METHODS.find((m) => m.name === name)
  if (!m) {throw new Error(`Method ${name} not found`)}
  return m
}

function makeRelay(callImpl: (...args: unknown[]) => unknown) {
  return { call: vi.fn().mockImplementation(callImpl) }
}

function makeCtx(devServerManager?: { getRelay: ReturnType<typeof vi.fn> }): RpcContext {
  return { devServerManager } as unknown as RpcContext
}

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

describe('preflight.check tracing', () => {
  it('local mode (no devServerId) → step(localCheck), ok({mode:"local"})', async () => {
    runPreflightCheckMock.mockResolvedValueOnce({ git: { installed: true }, gh: { installed: true, authenticated: true } })
    const { events, stop } = captureTraceEvents()

    const result = await findMethod('preflight.check').handler({}, makeCtx())
    stop()

    expect(result).toMatchObject({ git: { installed: true } })
    const flowEvents = events.filter((e) => e.flow === 'remoteIntegration:preflight')
    expect(flowEvents.some((e) => e.level === 'step' && e.label === 'localCheck')).toBe(true)
    expect(flowEvents.some((e) => e.level === 'ok' && e.fields.mode === 'local')).toBe(true)
  })

  it('remote mode success → step(relayDelegate,{devServerId}), ok({devServerId}), relay.call receives traceId===span.id', async () => {
    const relay = makeRelay(() => Promise.resolve({ gh: { installed: true } }))
    const devServerManager = { getRelay: vi.fn().mockReturnValue(relay) }
    const { events, stop } = captureTraceEvents()

    const result = await findMethod('preflight.check').handler(
      { devServerId: 'ds-1' },
      makeCtx(devServerManager)
    )
    stop()

    expect(result).toMatchObject({ gh: { installed: true } })
    const flowEvents = events.filter((e) => e.flow === 'remoteIntegration:preflight')
    expect(flowEvents.some((e) => e.level === 'step' && e.label === 'relayDelegate' && e.fields.devServerId === 'ds-1')).toBe(true)
    const okEvent = flowEvents.find((e) => e.level === 'ok')
    expect(okEvent?.fields.devServerId).toBe('ds-1')

    const startEvent = flowEvents.find((e) => e.level === 'start')
    const callArgs = relay.call.mock.calls[0]
    expect(callArgs?.[0]).toBe('preflight.check')
    expect((callArgs?.[1] as { traceId?: string }).traceId).toBe(startEvent?.id)
    expect(callArgs?.[2]).toBe(30_000)
  })

  it('relay not connected → fail("relay-not-connected", {devServerId}) BEFORE throw, no double-fail', async () => {
    const devServerManager = { getRelay: vi.fn().mockReturnValue(undefined) }
    const { events, stop } = captureTraceEvents()

    await expect(
      findMethod('preflight.check').handler({ devServerId: 'ds-missing' }, makeCtx(devServerManager))
    ).rejects.toThrow(/not connected/)
    stop()

    const failEvents = events.filter((e) => e.flow === 'remoteIntegration:preflight' && e.level === 'fail')
    expect(failEvents).toHaveLength(1)
    expect(failEvents[0]?.fields.devServerId).toBe('ds-missing')
  })

  it('relay.call rejects (different error) → fail(err, {devServerId}) called exactly once', async () => {
    const relay = makeRelay(() => Promise.reject(new Error('relay timeout')))
    const devServerManager = { getRelay: vi.fn().mockReturnValue(relay) }
    const { events, stop } = captureTraceEvents()

    await expect(
      findMethod('preflight.check').handler({ devServerId: 'ds-1' }, makeCtx(devServerManager))
    ).rejects.toThrow('relay timeout')
    stop()

    const failEvents = events.filter((e) => e.flow === 'remoteIntegration:preflight' && e.level === 'fail')
    expect(failEvents).toHaveLength(1)
    expect(failEvents[0]?.fields.devServerId).toBe('ds-1')
    expect(failEvents[0]?.fields.err).toContain('relay timeout')
  })

  it('traceId resume → span.id === params.traceId', async () => {
    runPreflightCheckMock.mockResolvedValueOnce({ git: { installed: true } })
    const { events, stop } = captureTraceEvents()

    await findMethod('preflight.check').handler({ traceId: 'xyz' }, makeCtx())
    stop()

    const startEvent = events.find((e) => e.flow === 'remoteIntegration:preflight' && e.level === 'start')
    expect(startEvent?.id).toBe('xyz')
  })
})

describe('preflight.detectAgents / detectRemoteAgents / detectRemoteWindowsTerminalCapabilities / refreshAgents — regression: no tracer', () => {
  it('do not emit remoteIntegration:preflight spans', async () => {
    detectInstalledAgentsWithShellPathHydrationMock.mockResolvedValueOnce(['codex'])
    detectRemoteAgentsMock.mockResolvedValueOnce(['claude'])
    detectRemoteWindowsTerminalCapabilitiesMock.mockResolvedValueOnce({
      wslAvailable: true, wslDistros: [], pwshAvailable: true, gitBashAvailable: true, hostPlatform: 'win32'
    })
    refreshShellPathAndDetectAgentsMock.mockResolvedValueOnce({
      agents: ['codex'], addedPathSegments: [], shellHydrationOk: true, pathSource: 'shell_hydrate', pathFailureReason: 'none'
    })
    const { events, stop } = captureTraceEvents()

    await findMethod('preflight.detectAgents').handler(undefined, makeCtx())
    await findMethod('preflight.detectRemoteAgents').handler({ connectionId: 'ssh-1' }, makeCtx())
    await findMethod('preflight.detectRemoteWindowsTerminalCapabilities').handler({ connectionId: 'ssh-1' }, makeCtx())
    await findMethod('preflight.refreshAgents').handler(undefined, makeCtx())
    stop()

    expect(events.filter((e) => e.flow === 'remoteIntegration:preflight')).toHaveLength(0)
  })
})
