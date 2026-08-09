/**
 * Tests for GitProviderCredentialService tracing (TASK-BE-014.2).
 *
 * Covers `getGitHubPAT`/`getGitLabPAT` in
 * `src/main/project/GitProviderCredentialService.ts` — instrumented with
 * `Tracers.remoteIntegrationCredentialDecryptFlow`.
 *
 * Security focus: the decrypted token value must NEVER appear in any
 * TraceFields of any event emitted on `remoteIntegration:credentialDecrypt`.
 *
 * @module main/project/__tests__/GitProviderCredentialService-tracing.test
 */

import { describe, it, expect, vi } from 'vitest'
import { GitProviderCredentialService } from '../GitProviderCredentialService'
import type { WebCredentialStore } from '../../credentials/web-credential-store'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

function makeStore(overrides: Partial<WebCredentialStore> = {}): WebCredentialStore {
  return {
    getToken: vi.fn(),
    ...overrides
  } as unknown as WebCredentialStore
}

// Asserts a real secret value never appears in any field of any captured event.
function expectNoTokenLeak(events: TraceEvent[], secret: string): void {
  for (const event of events) {
    expect(JSON.stringify(event.fields)).not.toContain(secret)
  }
}

describe('GitProviderCredentialService tracing', () => {
  // ── getGitHubPAT ─────────────────────────────────────────────────────────────

  it('getGitHubPAT() — found → start/step(decrypt)/ok({provider:"github", found:true})', async () => {
    const secretToken = 'ghp_super-secret-token-value'
    const store = makeStore({ getToken: vi.fn().mockResolvedValue(secretToken) })
    const service = new GitProviderCredentialService(() => store)
    const { events, stop } = captureTraceEvents()

    const result = await service.getGitHubPAT('user-1')
    stop()

    expect(result).toBe(secretToken)
    expect(store.getToken).toHaveBeenCalledWith('bitbucket')

    const flowEvents = events.filter((e) => e.flow === 'remoteIntegration:credentialDecrypt')
    expect(flowEvents.some((e) => e.level === 'start' && e.fields.provider === 'github' && e.fields.userId === 'user-1')).toBe(true)
    expect(flowEvents.some((e) => e.level === 'step' && e.label === 'decrypt' && e.fields.provider === 'github')).toBe(true)
    const okEvent = flowEvents.find((e) => e.level === 'ok')
    expect(okEvent?.fields).toMatchObject({ provider: 'github', found: true })

    expectNoTokenLeak(flowEvents, secretToken)
  })

  it('getGitHubPAT() — not found → ok({found:false})', async () => {
    const store = makeStore({ getToken: vi.fn().mockResolvedValue(null) })
    const service = new GitProviderCredentialService(() => store)
    const { events, stop } = captureTraceEvents()

    const result = await service.getGitHubPAT('user-1')
    stop()

    expect(result).toBeNull()
    const okEvent = events.find((e) => e.flow === 'remoteIntegration:credentialDecrypt' && e.level === 'ok')
    expect(okEvent?.fields).toMatchObject({ provider: 'github', found: false })
  })

  it('getGitHubPAT() — store throws → fail(err, {provider:"github"}) before re-throw', async () => {
    const store = makeStore({ getToken: vi.fn().mockRejectedValue(new Error('decrypt failed')) })
    const service = new GitProviderCredentialService(() => store)
    const { events, stop } = captureTraceEvents()

    await expect(service.getGitHubPAT('user-1')).rejects.toThrow('decrypt failed')
    stop()

    const failEvent = events.find((e) => e.flow === 'remoteIntegration:credentialDecrypt' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.provider).toBe('github')
    expect(failEvent?.fields.err).toContain('decrypt failed')
  })

  // ── getGitLabPAT ─────────────────────────────────────────────────────────────

  it('getGitLabPAT() — found → start/step(decrypt)/ok({provider:"gitlab", found:true})', async () => {
    const secretToken = 'glpat-super-secret-token-value'
    const store = makeStore({ getToken: vi.fn().mockResolvedValue(secretToken) })
    const service = new GitProviderCredentialService(() => store)
    const { events, stop } = captureTraceEvents()

    const result = await service.getGitLabPAT('user-1', 'proj-1')
    stop()

    expect(result).toBe(secretToken)
    expect(store.getToken).toHaveBeenCalledWith('gitea')

    const flowEvents = events.filter((e) => e.flow === 'remoteIntegration:credentialDecrypt')
    expect(flowEvents.some((e) => e.level === 'start' && e.fields.provider === 'gitlab' && e.fields.userId === 'user-1')).toBe(true)
    expect(flowEvents.some((e) => e.level === 'step' && e.label === 'decrypt' && e.fields.provider === 'gitlab')).toBe(true)
    const okEvent = flowEvents.find((e) => e.level === 'ok')
    expect(okEvent?.fields).toMatchObject({ provider: 'gitlab', found: true })

    expectNoTokenLeak(flowEvents, secretToken)
  })

  it('getGitLabPAT() — not found → ok({found:false})', async () => {
    const store = makeStore({ getToken: vi.fn().mockResolvedValue(null) })
    const service = new GitProviderCredentialService(() => store)
    const { events, stop } = captureTraceEvents()

    const result = await service.getGitLabPAT('user-1', 'proj-1')
    stop()

    expect(result).toBeNull()
    const okEvent = events.find((e) => e.flow === 'remoteIntegration:credentialDecrypt' && e.level === 'ok')
    expect(okEvent?.fields).toMatchObject({ provider: 'gitlab', found: false })
  })

  it('getGitLabPAT() — store throws → fail(err, {provider:"gitlab"}) before re-throw', async () => {
    const store = makeStore({ getToken: vi.fn().mockRejectedValue(new Error('decrypt failed')) })
    const service = new GitProviderCredentialService(() => store)
    const { events, stop } = captureTraceEvents()

    await expect(service.getGitLabPAT('user-1', 'proj-1')).rejects.toThrow('decrypt failed')
    stop()

    const failEvent = events.find((e) => e.flow === 'remoteIntegration:credentialDecrypt' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.provider).toBe('gitlab')
    expect(failEvent?.fields.err).toContain('decrypt failed')
  })

  // ── Regression: setGitHubPAT/deleteGitHubPAT/setGitLabPAT/deleteGitLabPAT are not instrumented ──

  it('write/delete methods do not emit remoteIntegration:credentialDecrypt spans', async () => {
    const store = makeStore({
      getToken: vi.fn().mockResolvedValue(null),
      setToken: vi.fn().mockResolvedValue(undefined),
      deleteToken: vi.fn().mockResolvedValue(undefined)
    })
    const service = new GitProviderCredentialService(() => store)
    const { events, stop } = captureTraceEvents()

    await service.setGitHubPAT('user-1', 'tok')
    await service.deleteGitHubPAT('user-1')
    await service.setGitLabPAT('user-1', 'proj-1', 'tok')
    await service.deleteGitLabPAT('user-1', 'proj-1')
    stop()

    expect(events.filter((e) => e.flow === 'remoteIntegration:credentialDecrypt')).toHaveLength(0)
  })
})
