/**
 * Tests for ProviderResolver (TDD-16) — TASK-026
 *
 * Uses mocks for AIProviderService.
 * ≥ 15 tests.
 *
 * @module main/ai-providers/__tests__/ProviderResolver.test
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ProviderResolver } from '../ProviderResolver'
import type { AIProviderService } from '../AIProviderService'
import type { AIProviderAccount } from '../../../shared/ai-provider-types'

// ── helpers ─────────────────────────────────────────────────────────────────

function makeAccount(overrides: Partial<AIProviderAccount> = {}): AIProviderAccount {
  return {
    id: 'acc-' + Math.random().toString(36).slice(2, 8),
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
})
