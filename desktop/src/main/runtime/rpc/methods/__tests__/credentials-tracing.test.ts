/**
 * Tests for `credentials.set`/`credentials.revoke` RPC handler tracing (TASK-BE-014.3).
 *
 * Covers `src/main/runtime/rpc/methods/credentials.ts` — instrumented with
 * `Tracers.remoteIntegrationCredentialStoreFlow`.
 *
 * Security focus: `params.token` must NEVER appear in any TraceFields of any
 * event emitted on `remoteIntegration:credentialStore`. `credentials.status`/
 * `credentials.list` must never emit a span on this flow (out of scope,
 * over-instrumentation risk per CR-TRACE-000 §5).
 *
 * @module main/runtime/rpc/methods/__tests__/credentials-tracing.test
 */

import { describe, it, expect, vi } from 'vitest'

const { mockIsWebCredentialMode, mockSetToken, mockDeleteToken, mockHasToken, mockGetConfig, mockListServices } =
  vi.hoisted(() => ({
    mockIsWebCredentialMode: vi.fn().mockReturnValue(true),
    mockSetToken: vi.fn().mockResolvedValue(undefined),
    mockDeleteToken: vi.fn().mockResolvedValue(undefined),
    mockHasToken: vi.fn().mockResolvedValue(false),
    mockGetConfig: vi.fn().mockResolvedValue(null),
    mockListServices: vi.fn().mockResolvedValue([])
  }))

vi.mock('../../../../credentials', () => ({
  isWebCredentialMode: mockIsWebCredentialMode,
  getWebCredentialStore: () => ({
    setToken: mockSetToken,
    deleteToken: mockDeleteToken,
    hasToken: mockHasToken,
    getConfig: mockGetConfig,
    listServices: mockListServices
  })
}))

import { CREDENTIAL_METHODS } from '../credentials'
import type { RpcContext, RpcMethod } from '../../core'
import { registerTraceSink, type TraceEvent } from '../../../../../shared/trace'

function findMethod(name: string): RpcMethod {
  const m = CREDENTIAL_METHODS.find((m) => m.name === name)
  if (!m) {throw new Error(`Method ${name} not found`)}
  return m
}

function makeCtx(userId = 'user-1'): RpcContext {
  return { userId } as RpcContext
}

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

// Asserts a real secret value never appears in any field of any captured event.
function expectNoTokenLeak(events: TraceEvent[], secret: string): void {
  for (const event of events) {
    expect(JSON.stringify(event.fields)).not.toContain(secret)
  }
}

describe('credentials.set tracing', () => {
  const SECRET_TOKEN = 'super-secret-pat-value-should-never-leak'

  it('success → start({service,userId}), step(encryptWrite), ok({service})', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    const { events, stop } = captureTraceEvents()

    const result = await findMethod('credentials.set').handler(
      { service: 'bitbucket', token: SECRET_TOKEN },
      makeCtx('user-1')
    )
    stop()

    expect(result).toEqual({ success: true })
    expect(mockSetToken).toHaveBeenCalledWith('bitbucket', SECRET_TOKEN, undefined)

    const flowEvents = events.filter((e) => e.flow === 'remoteIntegration:credentialStore')
    expect(flowEvents.some((e) => e.level === 'start' && e.fields.service === 'bitbucket' && e.fields.userId === 'user-1')).toBe(true)
    expect(flowEvents.some((e) => e.level === 'step' && e.label === 'encryptWrite' && e.fields.service === 'bitbucket')).toBe(true)
    expect(flowEvents.some((e) => e.level === 'ok' && e.fields.service === 'bitbucket')).toBe(true)

    expectNoTokenLeak(flowEvents, SECRET_TOKEN)
  })

  it('not in web credential mode → fail(err, {service}) before throw', async () => {
    mockIsWebCredentialMode.mockReturnValue(false)
    const { events, stop } = captureTraceEvents()

    await expect(
      findMethod('credentials.set').handler({ service: 'bitbucket', token: SECRET_TOKEN }, makeCtx())
    ).rejects.toThrow('Web Server mode')
    stop()

    const failEvent = events.find((e) => e.flow === 'remoteIntegration:credentialStore' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.service).toBe('bitbucket')
    expectNoTokenLeak(events, SECRET_TOKEN)
  })

  it('setToken throws → fail(err, {service}) before re-throw', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    mockSetToken.mockRejectedValueOnce(new Error('disk write failed'))
    const { events, stop } = captureTraceEvents()

    await expect(
      findMethod('credentials.set').handler({ service: 'gitea', token: SECRET_TOKEN }, makeCtx())
    ).rejects.toThrow('disk write failed')
    stop()

    const failEvent = events.find((e) => e.flow === 'remoteIntegration:credentialStore' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.service).toBe('gitea')
    expect(failEvent?.fields.err).toContain('disk write failed')
    expectNoTokenLeak(events, SECRET_TOKEN)
  })

  it('config object is never placed into a trace field', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    const { events, stop } = captureTraceEvents()

    await findMethod('credentials.set').handler(
      { service: 'jira', token: SECRET_TOKEN, config: { activeSiteId: 'site-1', siteUrl: 'https://jira.example.com' } },
      makeCtx()
    )
    stop()

    const flowEvents = events.filter((e) => e.flow === 'remoteIntegration:credentialStore')
    for (const event of flowEvents) {
      expect(event.fields).not.toHaveProperty('config')
    }
  })
})

describe('credentials.revoke tracing', () => {
  it('success → start({service,userId}), ok({service})', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    const { events, stop } = captureTraceEvents()

    const result = await findMethod('credentials.revoke').handler({ service: 'gitea' }, makeCtx('user-1'))
    stop()

    expect(result).toEqual({ success: true })
    expect(mockDeleteToken).toHaveBeenCalledWith('gitea')

    const flowEvents = events.filter((e) => e.flow === 'remoteIntegration:credentialStore')
    expect(flowEvents.some((e) => e.level === 'start' && e.fields.service === 'gitea' && e.fields.userId === 'user-1')).toBe(true)
    expect(flowEvents.some((e) => e.level === 'ok' && e.fields.service === 'gitea')).toBe(true)
  })

  it('not in web credential mode → fail(err, {service}) before throw', async () => {
    mockIsWebCredentialMode.mockReturnValue(false)
    const { events, stop } = captureTraceEvents()

    await expect(
      findMethod('credentials.revoke').handler({ service: 'gitea' }, makeCtx())
    ).rejects.toThrow('Web Server mode')
    stop()

    const failEvent = events.find((e) => e.flow === 'remoteIntegration:credentialStore' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.service).toBe('gitea')
  })

  it('deleteToken throws → fail(err, {service}) before re-throw', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    mockDeleteToken.mockRejectedValueOnce(new Error('unlink failed'))
    const { events, stop } = captureTraceEvents()

    await expect(
      findMethod('credentials.revoke').handler({ service: 'azure-devops' }, makeCtx())
    ).rejects.toThrow('unlink failed')
    stop()

    const failEvent = events.find((e) => e.flow === 'remoteIntegration:credentialStore' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.service).toBe('azure-devops')
    expect(failEvent?.fields.err).toContain('unlink failed')
  })
})

describe('credentials.status / credentials.list — regression: no tracer', () => {
  it('credentials.status/list do not emit remoteIntegration:credentialStore spans', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    const { events, stop } = captureTraceEvents()

    await findMethod('credentials.status').handler({ service: 'bitbucket' }, makeCtx())
    await findMethod('credentials.list').handler(undefined, makeCtx())
    stop()

    expect(events.filter((e) => e.flow === 'remoteIntegration:credentialStore')).toHaveLength(0)
  })
})
