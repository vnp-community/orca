/**
 * Tests for ProviderResolver (TDD-16) — TASK-026
 *
 * Uses mocks for AIProviderService.
 * ≥ 15 tests.
 *
 * @module main/ai-providers/__tests__/ProviderResolver.test
 */

import { describe, it, expect, vi } from 'vitest'
import { ProviderResolver } from '../ProviderResolver'
import type { AIProviderService } from '../AIProviderService'
import type { AIProviderAccount } from '../../../shared/ai-provider-types'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

// ── helpers ─────────────────────────────────────────────────────────────────

function makeAccount(overrides: Partial<AIProviderAccount> = {}): AIProviderAccount {
  return {
    id: `acc-${  Math.random().toString(36).slice(2, 8)}`,
    devServerId: 'srv-1',
    provider: 'anthropic',
    scope: 'server',
    label: 'Test',
    status: 'active',
    quotaLimitDay: 0,
    createdBy: 'u-admin',
    createdAt: new Date(),
    updatedAt: new Date(),
    ...overrides,
  }
}

function makeService(
  accounts: AIProviderAccount[] = [],
  usageTokens: number = 0
): AIProviderService {
  return {
    listAccounts: vi.fn().mockResolvedValue(accounts),
    getUsageToday: vi.fn().mockResolvedValue({ tokens: usageTokens, requests: 0, costUsd: 0 }),
  } as unknown as AIProviderService
}

const BASE_OPTIONS = {
  devServerId: 'srv-1',
  projectId: 'proj-1',
  userId: 'user-1',
}

// ── tests ────────────────────────────────────────────────────────────────────

describe('ProviderResolver', () => {
  // ── 1. User-scope account returned first ──────────────────────────────────

  it('returns user-scope account over project-scope', async () => {
    const userAcc = makeAccount({ scope: 'user', scopeRefId: 'user-1', label: 'User' })
    const projAcc = makeAccount({ scope: 'project', scopeRefId: 'proj-1', label: 'Project' })
    const serverAcc = makeAccount({ scope: 'server', label: 'Server' })
    const resolver = new ProviderResolver(makeService([userAcc, projAcc, serverAcc]))
    const result = await resolver.resolve(BASE_OPTIONS)
    expect(result.label).toBe('User')
  })

  // ── 2. Project-scope when no user-scope ───────────────────────────────────

  it('returns project-scope when no matching user-scope account', async () => {
    const projAcc = makeAccount({ scope: 'project', scopeRefId: 'proj-1', label: 'Project' })
    const serverAcc = makeAccount({ scope: 'server', label: 'Server' })
    const resolver = new ProviderResolver(makeService([projAcc, serverAcc]))
    const result = await resolver.resolve(BASE_OPTIONS)
    expect(result.label).toBe('Project')
  })

  // ── 3. Server-scope when no user/project scope ────────────────────────────

  it('returns server-scope when no user or project scope accounts', async () => {
    const serverAcc = makeAccount({ scope: 'server', label: 'Server' })
    const resolver = new ProviderResolver(makeService([serverAcc]))
    const result = await resolver.resolve(BASE_OPTIONS)
    expect(result.label).toBe('Server')
  })

  // ── 4. ModelHint filter applied ───────────────────────────────────────────

  it('prefers account matching modelHint', async () => {
    const noModel = makeAccount({ scope: 'server', label: 'NoModel' })
    const withModel = makeAccount({ scope: 'server', label: 'WithModel', model: 'claude-3-5' })
    const resolver = new ProviderResolver(makeService([noModel, withModel]))
    const result = await resolver.resolve({ ...BASE_OPTIONS, modelHint: 'claude-3-5' })
    expect(result.label).toBe('WithModel')
  })

  // ── 5. Fallback without modelHint ─────────────────────────────────────────

  it('falls back to any account when modelHint does not match', async () => {
    const acc = makeAccount({ scope: 'server', label: 'NoModel' })
    const resolver = new ProviderResolver(makeService([acc]))
    const result = await resolver.resolve({ ...BASE_OPTIONS, modelHint: 'nonexistent-model' })
    expect(result.label).toBe('NoModel')
  })

  // ── 6. Inactive accounts excluded ─────────────────────────────────────────

  it('excludes inactive (pending) accounts', async () => {
    const pending = makeAccount({ scope: 'server', status: 'pending', label: 'Pending' })
    const active = makeAccount({ scope: 'server', status: 'active', label: 'Active' })
    const resolver = new ProviderResolver(makeService([pending, active]))
    const result = await resolver.resolve(BASE_OPTIONS)
    expect(result.label).toBe('Active')
  })

  it('excludes invalid accounts', async () => {
    const invalid = makeAccount({ scope: 'server', status: 'invalid', label: 'Invalid' })
    const active = makeAccount({ scope: 'server', status: 'active', label: 'Active' })
    const resolver = new ProviderResolver(makeService([invalid, active]))
    const result = await resolver.resolve(BASE_OPTIONS)
    expect(result.label).toBe('Active')
  })

  // ── 7. Quota exceeded accounts excluded ───────────────────────────────────

  it('excludes quota-exceeded accounts', async () => {
    const quotaAcc = makeAccount({ scope: 'server', label: 'Quota', quotaLimitDay: 1000 })
    const unlimited = makeAccount({ scope: 'server', label: 'Unlimited', quotaLimitDay: 0 })
    const service = {
      listAccounts: vi.fn().mockResolvedValue([quotaAcc, unlimited]),
      // quota account has 1500 tokens used today (exceeds 1000 limit)
      getUsageToday: vi.fn().mockImplementation((id: string) =>
        Promise.resolve({ tokens: id === quotaAcc.id ? 1500 : 0, requests: 0, costUsd: 0 })
      ),
    } as unknown as AIProviderService
    const resolver = new ProviderResolver(service)
    const result = await resolver.resolve(BASE_OPTIONS)
    expect(result.label).toBe('Unlimited')
  })

  // ── 8. Throws NO_PROVIDER_AVAILABLE when none found ──────────────────────

  it('throws NO_PROVIDER_AVAILABLE when no active accounts', async () => {
    const resolver = new ProviderResolver(makeService([]))
    await expect(resolver.resolve(BASE_OPTIONS)).rejects.toThrow('NO_PROVIDER_AVAILABLE')
  })

  it('throws NO_PROVIDER_AVAILABLE when all accounts inactive', async () => {
    const inactive = makeAccount({ scope: 'server', status: 'invalid' })
    const resolver = new ProviderResolver(makeService([inactive]))
    await expect(resolver.resolve(BASE_OPTIONS)).rejects.toThrow('NO_PROVIDER_AVAILABLE')
  })

  // ── 9. User scopeRefId must match userId ──────────────────────────────────

  it('user-scope account with wrong scopeRefId is not returned first', async () => {
    const wrongUser = makeAccount({ scope: 'user', scopeRefId: 'other-user', label: 'WrongUser' })
    const serverAcc = makeAccount({ scope: 'server', label: 'Server' })
    const resolver = new ProviderResolver(makeService([wrongUser, serverAcc]))
    const result = await resolver.resolve(BASE_OPTIONS) // userId = 'user-1'
    expect(result.label).toBe('Server')
  })

  // ── 10. Project scopeRefId must match projectId ───────────────────────────

  it('project-scope account with wrong scopeRefId is not returned first', async () => {
    const wrongProj = makeAccount({ scope: 'project', scopeRefId: 'other-proj', label: 'WrongProj' })
    const serverAcc = makeAccount({ scope: 'server', label: 'Server' })
    const resolver = new ProviderResolver(makeService([wrongProj, serverAcc]))
    const result = await resolver.resolve(BASE_OPTIONS) // projectId = 'proj-1'
    expect(result.label).toBe('Server')
  })

  // ── 11. Unlimited quota (quotaLimitDay=0) always passes quota check ────────

  it('unlimited account (quotaLimitDay=0) always available regardless of tokens used', async () => {
    const unlimited = makeAccount({ scope: 'server', quotaLimitDay: 0, label: 'Unlimited' })
    // Simulate many tokens used
    const service = makeService([unlimited], 9_999_999)
    const resolver = new ProviderResolver(service)
    const result = await resolver.resolve(BASE_OPTIONS)
    expect(result.label).toBe('Unlimited')
  })

  // ── 12. Within-quota account available ───────────────────────────────────

  it('account within quota (tokens < quotaLimitDay) is available', async () => {
    const acc = makeAccount({ scope: 'server', quotaLimitDay: 1000 })
    const service = makeService([acc], 500) // 500 < 1000
    const resolver = new ProviderResolver(service)
    const result = await resolver.resolve(BASE_OPTIONS)
    expect(result.id).toBe(acc.id)
  })

  // ── 13. ModelHint in user scope beats server scope with modelHint ─────────

  it('user-scope with modelHint beats server-scope with modelHint', async () => {
    const userWithModel = makeAccount({ scope: 'user', scopeRefId: 'user-1', label: 'UserModel', model: 'gpt-4' })
    const serverWithModel = makeAccount({ scope: 'server', label: 'ServerModel', model: 'gpt-4' })
    const resolver = new ProviderResolver(makeService([serverWithModel, userWithModel]))
    const result = await resolver.resolve({ ...BASE_OPTIONS, modelHint: 'gpt-4' })
    expect(result.label).toBe('UserModel')
  })

  // ── 14. quota_exceeded status excluded ────────────────────────────────────

  it('excludes quota_exceeded status accounts', async () => {
    const exceeded = makeAccount({ scope: 'server', status: 'quota_exceeded', label: 'Exceeded' })
    const active = makeAccount({ scope: 'user', scopeRefId: 'user-1', status: 'active', label: 'Active' })
    const resolver = new ProviderResolver(makeService([exceeded, active]))
    const result = await resolver.resolve(BASE_OPTIONS)
    expect(result.label).toBe('Active')
  })

  // ── 15. Multiple active accounts, first in priority order wins ───────────

  it('returns the first account in priority order when multiple active', async () => {
    const acc1 = makeAccount({ scope: 'server', label: 'First', model: 'claude-3' })
    const acc2 = makeAccount({ scope: 'server', label: 'Second', model: 'claude-3' })
    const resolver = new ProviderResolver(makeService([acc1, acc2]))
    const result = await resolver.resolve({ ...BASE_OPTIONS })
    // Both are server-scope with no modelHint — first one in the list wins
    expect(result.label).toBe('First')
  })

  // ── TASK-BE-016.2: resolve() tracing (aiProvider:resolve) ─────────────────────

  describe('resolve tracing', () => {
    it('no accounts within quota → span.fail("NO_PROVIDER_AVAILABLE", {reason:"quota-or-inactive"})', async () => {
      const resolver = new ProviderResolver(makeService([]))
      const { events, stop } = captureTraceEvents()

      await expect(resolver.resolve(BASE_OPTIONS)).rejects.toThrow('NO_PROVIDER_AVAILABLE')
      stop()

      const flowEvents = events.filter((e) => e.flow === 'aiProvider:resolve')
      const failEvent = flowEvents.find((e) => e.level === 'fail')
      expect(failEvent?.fields.reason).toBe('quota-or-inactive')
      // Only 1 fail() emitted for this branch — no double-fail.
      expect(flowEvents.filter((e) => e.level === 'fail')).toHaveLength(1)
    })

    it('accounts exist but none match any scope → span.fail("NO_PROVIDER_AVAILABLE", {reason:"no-scope-match"})', async () => {
      // Active, within quota, but scope='user' with a scopeRefId that isn't userId —
      // and no server-scope account present (server never checks scopeRefId, so a
      // server-scope account would always match). This is the only way to reach
      // "accounts exist but none match" across both passes.
      const orphanScopeAccount = makeAccount({ scope: 'user', scopeRefId: 'someone-else', label: 'Orphan' })
      const resolver = new ProviderResolver(makeService([orphanScopeAccount]))
      const { events, stop } = captureTraceEvents()

      await expect(resolver.resolve(BASE_OPTIONS)).rejects.toThrow('NO_PROVIDER_AVAILABLE')
      stop()

      const flowEvents = events.filter((e) => e.flow === 'aiProvider:resolve')
      const failEvent = flowEvents.find((e) => e.level === 'fail')
      expect(failEvent?.fields.reason).toBe('no-scope-match')
      expect(flowEvents.filter((e) => e.level === 'fail')).toHaveLength(1)
    })

    it('match found in pass 1 (modelHint) → step("scope-match", {usedModelHint:true}), ok({accountId, scope})', async () => {
      const withModel = makeAccount({ scope: 'server', label: 'WithModel', model: 'claude-3-5' })
      const resolver = new ProviderResolver(makeService([withModel]))
      const { events, stop } = captureTraceEvents()

      const result = await resolver.resolve({ ...BASE_OPTIONS, modelHint: 'claude-3-5' })
      stop()

      expect(result.label).toBe('WithModel')
      const flowEvents = events.filter((e) => e.flow === 'aiProvider:resolve')
      const matchStep = flowEvents.find((e) => e.level === 'step' && e.label === 'scope-match')
      expect(matchStep?.fields).toMatchObject({ matchedScope: 'server', usedModelHint: true })
      const okEvent = flowEvents.find((e) => e.level === 'ok')
      expect(okEvent?.fields).toMatchObject({ accountId: withModel.id, scope: 'server' })
    })

    it('match found only in pass 2 (fallback, no modelHint) → step("scope-match", {usedModelHint:false})', async () => {
      const noModel = makeAccount({ scope: 'server', label: 'NoModel' })
      const resolver = new ProviderResolver(makeService([noModel]))
      const { events, stop } = captureTraceEvents()

      const result = await resolver.resolve({ ...BASE_OPTIONS, modelHint: 'nonexistent-model' })
      stop()

      expect(result.label).toBe('NoModel')
      const flowEvents = events.filter((e) => e.flow === 'aiProvider:resolve')
      const matchStep = flowEvents.find((e) => e.level === 'step' && e.label === 'scope-match')
      expect(matchStep?.fields).toMatchObject({ matchedScope: 'server', usedModelHint: false })
    })

    it('quota-filter step reports totalAccounts/activeCount/overQuotaCount', async () => {
      const quotaAcc = makeAccount({ scope: 'server', label: 'Quota', quotaLimitDay: 1000 })
      const unlimited = makeAccount({ scope: 'server', label: 'Unlimited', quotaLimitDay: 0 })
      const pending = makeAccount({ scope: 'server', label: 'Pending', status: 'pending' })
      const service = {
        listAccounts: vi.fn().mockResolvedValue([quotaAcc, unlimited, pending]),
        getUsageToday: vi.fn().mockImplementation((id: string) =>
          Promise.resolve({ tokens: id === quotaAcc.id ? 1500 : 0, requests: 0, costUsd: 0 })
        ),
      } as unknown as AIProviderService
      const resolver = new ProviderResolver(service)
      const { events, stop } = captureTraceEvents()

      await resolver.resolve(BASE_OPTIONS)
      stop()

      const flowEvents = events.filter((e) => e.flow === 'aiProvider:resolve')
      const quotaStep = flowEvents.find((e) => e.level === 'step' && e.label === 'quota-filter')
      expect(quotaStep?.fields).toMatchObject({ totalAccounts: 3, activeCount: 2, overQuotaCount: 1 })
    })

    it('infrastructure error (listAccounts throws) → span.fail(err) exactly once, no NO_PROVIDER_AVAILABLE double-fail', async () => {
      const service = {
        listAccounts: vi.fn().mockRejectedValue(new Error('DB connection lost')),
        getUsageToday: vi.fn(),
      } as unknown as AIProviderService
      const resolver = new ProviderResolver(service)
      const { events, stop } = captureTraceEvents()

      await expect(resolver.resolve(BASE_OPTIONS)).rejects.toThrow('DB connection lost')
      stop()

      const flowEvents = events.filter((e) => e.flow === 'aiProvider:resolve')
      const failEvents = flowEvents.filter((e) => e.level === 'fail')
      expect(failEvents).toHaveLength(1)
      expect(failEvents[0]?.fields.err).toContain('DB connection lost')
    })
  })
})
