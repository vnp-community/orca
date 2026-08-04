/**
 * Tests for ProfileResolver (TDD-14) — TASK-011
 *
 * Tests all merge rules, cache behavior, source tracking, and invalidation.
 * Follows spec from TASK-011-profile-resolver-tests.md (≥ 14 tests).
 *
 * @module main/profile/__tests__/ProfileResolver.test
 */

import { describe, it, expect, vi, afterEach } from 'vitest'
import { ProfileResolver } from '../ProfileResolver'
import type { ProfileService } from '../ProfileService'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'

// ── helper ──────────────────────────────────────────────────────────────────

function makeService(overrides: Partial<ProfileService> = {}): ProfileService {
  return {
    getCompanyProfileForUser: vi.fn().mockResolvedValue(null),
    getDeptProfileForUser: vi.fn().mockResolvedValue(null),
    getUserProfile: vi.fn().mockResolvedValue(null),
    ...overrides,
  } as unknown as ProfileService
}

describe('ProfileResolver', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  // ── 1. Cache miss fetches all 3 layers ────────────────────────────────────

  it('resolve: cache miss fetches 3 layers in parallel', async () => {
    const svc = makeService()
    const resolver = new ProfileResolver(svc)
    await resolver.resolve('u-1')
    expect(svc.getCompanyProfileForUser).toHaveBeenCalledWith('u-1')
    expect(svc.getDeptProfileForUser).toHaveBeenCalledWith('u-1')
    expect(svc.getUserProfile).toHaveBeenCalledWith('u-1')
  })

  // ── 2. Cache hit → does NOT call service again ────────────────────────────

  it('resolve: cache hit does not call service', async () => {
    const svc = makeService()
    const resolver = new ProfileResolver(svc)
    await resolver.resolve('u-1')
    await resolver.resolve('u-1')
    expect(svc.getUserProfile).toHaveBeenCalledTimes(1)
  })

  // ── 3. User > dept > company (scalar merge) ────────────────────────────────

  it('agent.preferredModel: user > dept > company', async () => {
    const svc = makeService({
      getCompanyProfileForUser: vi.fn().mockResolvedValue({ agent: { preferredModel: 'gpt-4' } }),
      getDeptProfileForUser: vi.fn().mockResolvedValue({ agent: { preferredModel: 'claude-3' } }),
      getUserProfile: vi.fn().mockResolvedValue({ agent: { preferredModel: 'gemini-pro' } }),
    })
    const resolver = new ProfileResolver(svc)
    const result = await resolver.resolve('u-1')
    expect(result.agent?.preferredModel).toBe('gemini-pro')
    expect(result._sources['agent.preferredModel']).toBe('user')
  })

  // ── 4. Dept wins when user has no value ───────────────────────────────────

  it('agent.preferredModel: dept wins when user has no value', async () => {
    const svc = makeService({
      getDeptProfileForUser: vi.fn().mockResolvedValue({ agent: { preferredModel: 'claude-3' } }),
      getUserProfile: vi.fn().mockResolvedValue({ editor: { tabSize: 2 } }),
    })
    const resolver = new ProfileResolver(svc)
    const result = await resolver.resolve('u-1')
    expect(result.agent?.preferredModel).toBe('claude-3')
    expect(result._sources['agent.preferredModel']).toBe('dept')
  })

  // ── 5. Security section locked to company ─────────────────────────────────

  it('security: always from company, user cannot override', async () => {
    const svc = makeService({
      getCompanyProfileForUser: vi.fn().mockResolvedValue({ security: { maxSessionHours: 8 } }),
      getUserProfile: vi.fn().mockResolvedValue({ security: { maxSessionHours: 999 } }),
    })
    const resolver = new ProfileResolver(svc)
    const result = await resolver.resolve('u-1')
    expect(result.security?.maxSessionHours).toBe(8)
    expect(result._sources['security']).toBe('company')
  })

  // ── 6. shell.pathAdditions: concatenate (not override) ───────────────────

  it('shell.pathAdditions: concatenates company + dept + user', async () => {
    const svc = makeService({
      getCompanyProfileForUser: vi.fn().mockResolvedValue({ shell: { pathAdditions: ['/company/bin'] } }),
      getDeptProfileForUser: vi.fn().mockResolvedValue({ shell: { pathAdditions: ['/dept/bin'] } }),
      getUserProfile: vi.fn().mockResolvedValue({ shell: { pathAdditions: ['/user/bin'] } }),
    })
    const resolver = new ProfileResolver(svc)
    const result = await resolver.resolve('u-1')
    expect(result.shell?.pathAdditions).toEqual(['/company/bin', '/dept/bin', '/user/bin'])
  })

  // ── 7. shell.envVars: merge with user wins ────────────────────────────────

  it('shell.envVars: user overrides dept overrides company', async () => {
    const svc = makeService({
      getCompanyProfileForUser: vi.fn().mockResolvedValue({ shell: { envVars: { FOO: 'company', BAR: 'company' } } }),
      getDeptProfileForUser: vi.fn().mockResolvedValue({ shell: { envVars: { FOO: 'dept' } } }),
      getUserProfile: vi.fn().mockResolvedValue({ shell: { envVars: { FOO: 'user' } } }),
    })
    const resolver = new ProfileResolver(svc)
    const result = await resolver.resolve('u-1')
    expect(result.shell?.envVars).toEqual({ FOO: 'user', BAR: 'company' })
  })

  // ── 8. mcp.servers: dedup by name, user wins on conflict ─────────────────

  it('mcp.servers: dedup by name', async () => {
    const svc = makeService({
      getCompanyProfileForUser: vi.fn().mockResolvedValue({ mcp: { servers: [{ name: 'fs', command: 'old-fs' }] } }),
      getUserProfile: vi.fn().mockResolvedValue({ mcp: { servers: [{ name: 'fs', command: 'new-fs' }] } }),
    })
    const resolver = new ProfileResolver(svc)
    const result = await resolver.resolve('u-1')
    expect(result.mcp?.servers).toHaveLength(1)
    expect(result.mcp?.servers?.[0].command).toBe('new-fs')
  })

  // ── 9. mcp.servers: union when different names ────────────────────────────

  it('mcp.servers: union when different names', async () => {
    const svc = makeService({
      getCompanyProfileForUser: vi.fn().mockResolvedValue({ mcp: { servers: [{ name: 'a', command: 'cmd-a' }] } }),
      getUserProfile: vi.fn().mockResolvedValue({ mcp: { servers: [{ name: 'b', command: 'cmd-b' }] } }),
    })
    const resolver = new ProfileResolver(svc)
    const result = await resolver.resolve('u-1')
    expect(result.mcp?.servers).toHaveLength(2)
  })

  // ── 10. resolve returns _resolvedAt timestamp ─────────────────────────────

  it('resolve sets _resolvedAt', async () => {
    const before = Date.now()
    const svc = makeService()
    const resolver = new ProfileResolver(svc)
    const result = await resolver.resolve('u-1')
    expect(result._resolvedAt).toBeGreaterThanOrEqual(before)
  })

  // ── 11. invalidate(userId) clears specific user cache ─────────────────────

  it('invalidate(userId) clears specific user', async () => {
    const svc = makeService()
    const resolver = new ProfileResolver(svc)
    await resolver.resolve('u-1')
    await resolver.resolve('u-2')
    resolver.invalidate('u-1')
    await resolver.resolve('u-1')
    // u-1 fetched twice, u-2 fetched once
    expect((svc.getUserProfile as ReturnType<typeof vi.fn>).mock.calls.filter((c: unknown[]) => c[0] === 'u-1')).toHaveLength(2)
    expect((svc.getUserProfile as ReturnType<typeof vi.fn>).mock.calls.filter((c: unknown[]) => c[0] === 'u-2')).toHaveLength(1)
  })

  // ── 12. invalidate() no args clears all cache ─────────────────────────────

  it('invalidate() no args clears all', async () => {
    const svc = makeService()
    const resolver = new ProfileResolver(svc)
    await resolver.resolve('u-1')
    await resolver.resolve('u-2')
    resolver.invalidate()
    await resolver.resolve('u-1')
    await resolver.resolve('u-2')
    expect(svc.getUserProfile).toHaveBeenCalledTimes(4)
  })

  // ── 13. Empty company + dept → user profile only ──────────────────────────

  it('resolves correctly with company + dept null', async () => {
    const svc = makeService({
      getUserProfile: vi.fn().mockResolvedValue({ editor: { tabSize: 4 } }),
    })
    const resolver = new ProfileResolver(svc)
    const result = await resolver.resolve('u-1')
    expect(result.editor?.tabSize).toBe(4)
  })

  // ── 14. All null → empty profile with sources ─────────────────────────────

  it('resolves empty profile when all layers null', async () => {
    const svc = makeService()
    const resolver = new ProfileResolver(svc)
    const result = await resolver.resolve('u-1')
    expect(result._sources).toBeDefined()
    expect(result._resolvedAt).toBeGreaterThan(0)
  })

  // ── 15. editor: scalar per-field merge ────────────────────────────────────

  it('editor: user tabSize wins over company, theme from company', async () => {
    const svc = makeService({
      getCompanyProfileForUser: vi.fn().mockResolvedValue({ editor: { tabSize: 4, theme: 'light' } }),
      getUserProfile: vi.fn().mockResolvedValue({ editor: { tabSize: 2 } }),
    })
    const resolver = new ProfileResolver(svc)
    const result = await resolver.resolve('u-1')
    expect(result.editor?.tabSize).toBe(2)
    expect(result.editor?.theme).toBe('light')
    expect(result._sources['editor.tabSize']).toBe('user')
    expect(result._sources['editor.theme']).toBe('company')
  })

  // ── 16. cache: different userIds independent ──────────────────────────────

  it('cache: different userIds are cached independently', async () => {
    const svc = makeService()
    const resolver = new ProfileResolver(svc)
    await resolver.resolve('u-1')
    await resolver.resolve('u-2')
    expect(svc.getCompanyProfileForUser).toHaveBeenCalledTimes(2)
    // Both still cached
    await resolver.resolve('u-1')
    await resolver.resolve('u-2')
    expect(svc.getCompanyProfileForUser).toHaveBeenCalledTimes(2)
  })
})

// ── CR-TRACE-015: ProfileResolver.resolve() tracing (TASK-BE-015.5) ────────

describe('ProfileResolver tracing (CR-TRACE-015)', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    return { events, stop: unregister }
  }

  it('resolve() cache miss → step(cacheCheck, { cacheHit: false }) then ok({ cacheHit: false })', async () => {
    const svc = makeService()
    const resolver = new ProfileResolver(svc)
    const { events, stop } = captureTraceEvents()

    await resolver.resolve('u-1')
    stop()

    const spanEvents = events.filter((e) => e.flow === 'profile:resolve')
    expect(spanEvents.map((e) => (e.level === 'step' ? e.label : e.level))).toEqual([
      'start',
      'cacheCheck',
      'ok'
    ])
    expect(spanEvents.find((e) => e.label === 'cacheCheck')?.fields).toMatchObject({ cacheHit: false })
    expect(spanEvents.find((e) => e.level === 'ok')?.fields).toMatchObject({ cacheHit: false })
  })

  it('resolve() cache hit → step(cacheCheck, { cacheHit: true }) then ok({ cacheHit: true }), no re-fetch', async () => {
    const svc = makeService()
    const resolver = new ProfileResolver(svc)
    await resolver.resolve('u-1') // warm the cache (not captured)

    const { events, stop } = captureTraceEvents()
    await resolver.resolve('u-1')
    stop()

    const spanEvents = events.filter((e) => e.flow === 'profile:resolve')
    expect(spanEvents.map((e) => (e.level === 'step' ? e.label : e.level))).toEqual([
      'start',
      'cacheCheck',
      'ok'
    ])
    expect(spanEvents.find((e) => e.label === 'cacheCheck')?.fields).toMatchObject({ cacheHit: true })
    expect(spanEvents.find((e) => e.level === 'ok')?.fields).toMatchObject({ cacheHit: true })
    expect(svc.getUserProfile).toHaveBeenCalledTimes(1)
  })

  it('resolve() does not emit any trace event with a flow other than profile:resolve (no nested span for merge())', async () => {
    const svc = makeService({
      getCompanyProfileForUser: vi.fn().mockResolvedValue({ agent: { preferredModel: 'gpt-4' } }),
      getDeptProfileForUser: vi.fn().mockResolvedValue({ shell: { pathAdditions: ['/x'] } }),
      getUserProfile: vi.fn().mockResolvedValue({ mcp: { servers: [{ name: 's1', command: 'run' }] } }),
    })
    const resolver = new ProfileResolver(svc)
    const { events, stop } = captureTraceEvents()

    await resolver.resolve('u-1')
    stop()

    expect(events.every((e) => e.flow === 'profile:resolve')).toBe(true)
  })

  it('resolve() ok() reports hasSecurityLock true when company profile has a security section', async () => {
    const svc = makeService({
      getCompanyProfileForUser: vi.fn().mockResolvedValue({ security: { approvedModels: ['gpt-4'] } }),
    })
    const resolver = new ProfileResolver(svc)
    const { events, stop } = captureTraceEvents()

    await resolver.resolve('u-1')
    stop()

    const okEvent = events.find((e) => e.flow === 'profile:resolve' && e.level === 'ok')
    expect(okEvent?.fields).toMatchObject({ hasSecurityLock: true })
  })
})
