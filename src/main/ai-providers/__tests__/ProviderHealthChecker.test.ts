/**
 * Tests for ProviderHealthChecker (TDD-16) — TASK-026
 *
 * Uses mocks for AIProviderService and RelayConnectionPool.
 * ≥ 7 tests.
 *
 * @module main/ai-providers/__tests__/ProviderHealthChecker.test
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ProviderHealthChecker } from '../ProviderHealthChecker'
import type { AIProviderService } from '../AIProviderService'
import type { RelayConnectionPool } from '../../dev-server/relay-connection-pool'
import type { AIProviderAccount } from '../../../shared/ai-provider-types'

// ── helpers ─────────────────────────────────────────────────────────────────

function makeAccount(id: string, overrides: Partial<AIProviderAccount> = {}): AIProviderAccount {
  return {
    id,
    devServerId: 'srv-1',
    provider: 'anthropic',
    scope: 'server',
    label: `Account ${id}`,
    status: 'active',
    quotaLimitDay: 0,
    createdBy: 'u-admin',
    createdAt: new Date(),
    updatedAt: new Date(),
    ...overrides,
  }
}

function makeRelayPool(): RelayConnectionPool {
  return {} as unknown as RelayConnectionPool
}

/** Flush all pending microtasks (Promise chain) */
async function flushPromises(): Promise<void> {
  // Multiple rounds to settle nested promise chains
  for (let i = 0; i < 10; i++) {
    await Promise.resolve()
  }
}

function makeService(
  accounts: AIProviderAccount[],
  testConnectionResults: Record<string, { ok: boolean; latencyMs: number; error?: string }> = {}
): { service: AIProviderService; updateMock: ReturnType<typeof vi.fn> } {
  const updateMock = vi.fn().mockResolvedValue(undefined)
  const service = {
    getAllAccounts: vi.fn().mockResolvedValue(accounts),
    testConnection: vi.fn().mockImplementation((id: string) =>
      Promise.resolve(
        testConnectionResults[id] ?? { ok: true, latencyMs: 10 }
      )
    ),
    updateAccount: updateMock,
  } as unknown as AIProviderService
  return { service, updateMock }
}

// ── tests ─────────────────────────────────────────────────────────────────────

describe('ProviderHealthChecker', () => {
  let checker: ProviderHealthChecker

  beforeEach(() => {
    vi.useFakeTimers()
    checker = new ProviderHealthChecker()
  })

  afterEach(() => {
    checker.stop()
    vi.useRealTimers()
  })

  // ── 1. start → runCheck called immediately ────────────────────────────────

  it('start: runs check immediately on start', async () => {
    const { service } = makeService([makeAccount('acc-1')])
    checker.start(service, makeRelayPool())
    await flushPromises()
    expect(service.getAllAccounts).toHaveBeenCalledTimes(1)
  })

  // ── 2. Interval set correctly (15 min) ────────────────────────────────────

  it('start: sets 15-minute interval for repeated checks', async () => {
    const { service } = makeService([makeAccount('acc-1')])
    checker.start(service, makeRelayPool())
    await flushPromises()

    // Advance 15 minutes
    vi.advanceTimersByTime(15 * 60 * 1000)
    await flushPromises()

    expect(service.getAllAccounts).toHaveBeenCalledTimes(2)
  })

  // ── 3. stop → interval cleared ───────────────────────────────────────────

  it('stop: clears interval and prevents further checks', async () => {
    const { service } = makeService([makeAccount('acc-1')])
    checker.start(service, makeRelayPool())
    await flushPromises()
    checker.stop()

    // Advance far beyond 15 min
    vi.advanceTimersByTime(60 * 60 * 1000)
    await flushPromises()

    // Should only have been called once (the immediate check)
    expect(service.getAllAccounts).toHaveBeenCalledTimes(1)
  })

  // ── 4. Active accounts → status updated to 'active' ──────────────────────

  it('updates account status to "active" when testConnection succeeds', async () => {
    const acc = makeAccount('acc-ok')
    const { service, updateMock } = makeService([acc], { 'acc-ok': { ok: true, latencyMs: 5 } })
    checker.start(service, makeRelayPool())
    await flushPromises()

    expect(updateMock).toHaveBeenCalledWith('acc-ok', expect.objectContaining({ status: 'active' }))
  })

  // ── 5. Failed accounts → status updated to 'invalid' ─────────────────────

  it('updates account status to "invalid" when testConnection fails without quota error', async () => {
    const acc = makeAccount('acc-fail')
    const { service, updateMock } = makeService([acc], {
      'acc-fail': { ok: false, latencyMs: 0, error: 'Connection refused' }
    })
    checker.start(service, makeRelayPool())
    await flushPromises()

    expect(updateMock).toHaveBeenCalledWith('acc-fail', expect.objectContaining({ status: 'invalid' }))
  })

  // ── 6. Quota error → status 'quota_exceeded' ─────────────────────────────

  it('updates account status to "quota_exceeded" when error contains "quota"', async () => {
    const acc = makeAccount('acc-quota')
    const { service, updateMock } = makeService([acc], {
      'acc-quota': { ok: false, latencyMs: 0, error: 'quota limit reached' }
    })
    checker.start(service, makeRelayPool())
    await flushPromises()

    expect(updateMock).toHaveBeenCalledWith('acc-quota', expect.objectContaining({ status: 'quota_exceeded' }))
  })

  // ── 7. One account fails → others still checked ───────────────────────────

  it('continues checking remaining accounts when one account testConnection throws', async () => {
    const acc1 = makeAccount('acc-1')
    const acc2 = makeAccount('acc-2')
    const acc3 = makeAccount('acc-3')

    const updateMock = vi.fn().mockResolvedValue(undefined)
    const service = {
      getAllAccounts: vi.fn().mockResolvedValue([acc1, acc2, acc3]),
      testConnection: vi.fn().mockImplementation((id: string) => {
        if (id === 'acc-2') return Promise.reject(new Error('unexpected error'))
        return Promise.resolve({ ok: true, latencyMs: 5 })
      }),
      updateAccount: updateMock,
    } as unknown as AIProviderService

    checker.start(service, makeRelayPool())
    await flushPromises()

    // acc-1 and acc-3 should be updated even though acc-2 threw
    const updatedIds = updateMock.mock.calls.map(c => c[0] as string)
    expect(updatedIds).toContain('acc-1')
    expect(updatedIds).toContain('acc-3')
    // acc-2 should NOT have been updated (threw before updateAccount)
    expect(updatedIds).not.toContain('acc-2')
  })

  // ── 8. Non-fatal: getAllAccounts failure doesn't throw ────────────────────

  it('is non-fatal: getAllAccounts failure logs warning but does not throw', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const service = {
      getAllAccounts: vi.fn().mockRejectedValue(new Error('DB error')),
      testConnection: vi.fn(),
      updateAccount: vi.fn(),
    } as unknown as AIProviderService

    checker.start(service, makeRelayPool())
    await flushPromises()

    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining('[ProviderHealthChecker]'),
      expect.any(Error)
    )
    warnSpy.mockRestore()
  })

  // ── 9. stop can be called before start (no-op) ────────────────────────────

  it('stop is safe to call before start', () => {
    expect(() => checker.stop()).not.toThrow()
  })

  // ── 10. lastHealthCheck is updated on each check ──────────────────────────

  it('updates lastHealthCheck date on each successful check', async () => {
    const acc = makeAccount('acc-ts')
    const { service, updateMock } = makeService([acc], { 'acc-ts': { ok: true, latencyMs: 10 } })
    checker.start(service, makeRelayPool())
    await flushPromises()

    const call = updateMock.mock.calls[0] as [string, { lastHealthCheck?: Date }]
    expect(call[1].lastHealthCheck).toBeInstanceOf(Date)
  })
})
