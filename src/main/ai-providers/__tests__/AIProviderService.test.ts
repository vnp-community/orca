/**
 * Tests for AIProviderService (TDD-16) — TASK-026
 *
 * Uses in-memory SQLite + ALL_MIGRATIONS, mocks for relay.
 * ≥ 15 tests.
 *
 * @module main/ai-providers/__tests__/AIProviderService.test
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { AIProviderService } from '../AIProviderService'
import type { DevServerManager } from '../../dev-server/dev-server-manager'
import type { RelayConnectionPool } from '../../dev-server/relay-connection-pool'

// ── helpers ────────────────────────────────────────────────────────────────

const DS_ID = 'srv-1'
const RELAY_CALL_MOCK = vi.fn().mockResolvedValue({ ok: true })
const FAKE_RELAY = { call: RELAY_CALL_MOCK }

function makeDSM(hasServer = true): DevServerManager {
  return {
    get: vi.fn().mockReturnValue(
      hasServer ? { id: DS_ID, name: 'Test Server' } : null
    ),
  } as unknown as DevServerManager
}

function makeRelayPool(): RelayConnectionPool {
  return {
    getOrConnect: vi.fn().mockResolvedValue(FAKE_RELAY),
  } as unknown as RelayConnectionPool
}

async function makeService(): Promise<{
  pool: SqliteSingleConnectionPool
  service: AIProviderService
  relayPool: RelayConnectionPool
}> {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const relayPool = makeRelayPool()
  const service = new AIProviderService(pool, makeDSM(), relayPool)
  return { pool, service, relayPool }
}

const BASE_PARAMS = {
  devServerId: DS_ID,
  provider: 'anthropic' as const,
  scope: 'server' as const,
  label: 'Team Claude',
  createdBy: 'u-admin',
}

// ── tests ──────────────────────────────────────────────────────────────────

describe('AIProviderService', () => {
  let pool: SqliteSingleConnectionPool
  let service: AIProviderService

  beforeEach(async () => {
    RELAY_CALL_MOCK.mockClear()
    const setup = await makeService()
    pool = setup.pool
    service = setup.service
  })

  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

  // ── 1. createAccount → status = 'pending' ─────────────────────────────────

  it('createAccount: returns account with status=pending', async () => {
    const account = await service.createAccount(BASE_PARAMS)
    expect(account.id).toMatch(/^[0-9a-f-]{36}$/)
    expect(account.status).toBe('pending')
    expect(account.provider).toBe('anthropic')
    expect(account.label).toBe('Team Claude')
    expect(account.createdBy).toBe('u-admin')
  })

  // ── 2. getAccount → null for non-existent ─────────────────────────────────

  it('getAccount: returns null for non-existent id', async () => {
    const result = await service.getAccount('no-such-id')
    expect(result).toBeNull()
  })

  // ── 3. listAccounts → filters by devServerId ──────────────────────────────

  it('listAccounts: filters by devServerId', async () => {
    await service.createAccount(BASE_PARAMS)
    await service.createAccount({ ...BASE_PARAMS, devServerId: 'other-srv', label: 'Other' })
    const list = await service.listAccounts(DS_ID)
    expect(list.length).toBe(1)
    expect(list[0].label).toBe('Team Claude')
  })

  // ── 4. listAccounts → filters by scope ────────────────────────────────────

  it('listAccounts: filters by scope when provided', async () => {
    await service.createAccount({ ...BASE_PARAMS, scope: 'server', label: 'Server' })
    await service.createAccount({ ...BASE_PARAMS, scope: 'user', label: 'User', scopeRefId: 'u-1' })
    const serverOnly = await service.listAccounts(DS_ID, 'server')
    expect(serverOnly.length).toBe(1)
    expect(serverOnly[0].label).toBe('Server')
  })

  // ── 5. updateAccount → updates status field ───────────────────────────────

  it('updateAccount: updates status field', async () => {
    const account = await service.createAccount(BASE_PARAMS)
    await service.updateAccount(account.id, { status: 'active' })
    const updated = await service.getAccount(account.id)
    expect(updated?.status).toBe('active')
  })

  // ── 6. updateAccount → updates lastHealthCheck ────────────────────────────

  it('updateAccount: updates lastHealthCheck', async () => {
    const account = await service.createAccount(BASE_PARAMS)
    const now = new Date()
    await service.updateAccount(account.id, { lastHealthCheck: now })
    const updated = await service.getAccount(account.id)
    expect(updated?.lastHealthCheck).toBeInstanceOf(Date)
  })

  // ── 7. deleteAccount → removes account ────────────────────────────────────

  it('deleteAccount: removes account', async () => {
    const account = await service.createAccount(BASE_PARAMS)
    await service.deleteAccount(account.id)
    const result = await service.getAccount(account.id)
    expect(result).toBeNull()
  })

  // ── 8. writeCredentialToDevServer → calls relay.call ─────────────────────

  it('writeCredentialToDevServer: calls relay.call with writeCredential', async () => {
    const account = await service.createAccount(BASE_PARAMS)
    await service.writeCredentialToDevServer(account.id, 'encrypted-blob', 'iv-12345')
    expect(RELAY_CALL_MOCK).toHaveBeenCalledWith(
      'ai.provider.writeCredential',
      expect.objectContaining({
        accountId: account.id,
        encryptedBlob: 'encrypted-blob',
        iv: 'iv-12345',
      })
    )
  })

  // ── 9. writeCredentialToDevServer → does NOT store plaintext ─────────────

  it('writeCredentialToDevServer: does NOT store credential in DB', async () => {
    const account = await service.createAccount(BASE_PARAMS)
    await service.writeCredentialToDevServer(account.id, 'encrypted-blob', 'iv-12345')
    // Re-fetch from DB — credential should NOT be in any account field
    const stored = await service.getAccount(account.id)
    const json = JSON.stringify(stored)
    expect(json).not.toContain('encrypted-blob')
    expect(json).not.toContain('iv-12345')
  })

  // ── 10. testConnection → { ok: false } when relay fails (no throw) ────────

  it('testConnection: returns { ok: false } when relay throws (non-throwing)', async () => {
    const failRelayPool = {
      getOrConnect: vi.fn().mockRejectedValue(new Error('Connection refused')),
    } as unknown as RelayConnectionPool
    const pool2 = new SqliteSingleConnectionPool(':memory:')
    await pool2.withConnection(async (db) => {
      const runner = new MigrationRunner(db, ALL_MIGRATIONS)
      await runner.migrate()
    })
    const failService = new AIProviderService(pool2, makeDSM(), failRelayPool)
    const account = await failService.createAccount(BASE_PARAMS)

    // Should NOT throw
    const result = await failService.testConnection(account.id)
    expect(result.ok).toBe(false)
    expect(result.error).toContain('Connection refused')
    await pool2.destroy().catch(() => {})
  })

  // ── 11. recordUsage → creates new record ──────────────────────────────────

  it('recordUsage: creates new usage record', async () => {
    const account = await service.createAccount(BASE_PARAMS)
    await service.recordUsage(account.id, 1000, 5, 0.02)
    const usage = await service.getUsageToday(account.id)
    expect(usage.tokens).toBe(1000)
    expect(usage.requests).toBe(5)
    expect(usage.costUsd).toBeCloseTo(0.02)
  })

  // ── 12. recordUsage → UPSERT adds to existing record ──────────────────────

  it('recordUsage: UPSERT accumulates usage', async () => {
    const account = await service.createAccount(BASE_PARAMS)
    await service.recordUsage(account.id, 1000, 5, 0.02)
    await service.recordUsage(account.id, 500, 2, 0.01)
    const usage = await service.getUsageToday(account.id)
    expect(usage.tokens).toBe(1500)
    expect(usage.requests).toBe(7)
    expect(usage.costUsd).toBeCloseTo(0.03)
  })

  // ── 13. getUsageToday → { tokens: 0 } for no usage ───────────────────────

  it('getUsageToday: returns zeros for no usage', async () => {
    const account = await service.createAccount(BASE_PARAMS)
    const usage = await service.getUsageToday(account.id)
    expect(usage).toEqual({ tokens: 0, requests: 0, costUsd: 0 })
  })

  // ── 14. getUsageToday → returns accumulated usage ─────────────────────────

  it('getUsageToday: returns correct accumulated totals', async () => {
    const account = await service.createAccount(BASE_PARAMS)
    await service.recordUsage(account.id, 2000, 10, 0.05)
    const usage = await service.getUsageToday(account.id)
    expect(usage.tokens).toBe(2000)
  })

  // ── 15. getAllAccounts → returns all ──────────────────────────────────────

  it('getAllAccounts: returns all accounts', async () => {
    await service.createAccount(BASE_PARAMS)
    await service.createAccount({ ...BASE_PARAMS, label: 'Second' })
    const all = await service.getAllAccounts()
    expect(all.length).toBe(2)
  })

  // ── 16. createAccount with model + baseUrl ────────────────────────────────

  it('createAccount: stores model and baseUrl', async () => {
    const account = await service.createAccount({
      ...BASE_PARAMS,
      model: 'claude-3-5-sonnet',
      baseUrl: 'https://api.anthropic.com',
    })
    const fetched = await service.getAccount(account.id)
    expect(fetched?.model).toBe('claude-3-5-sonnet')
    expect(fetched?.baseUrl).toBe('https://api.anthropic.com')
  })
})
