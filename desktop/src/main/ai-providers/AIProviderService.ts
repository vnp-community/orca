/**
 * AIProviderService — CRUD service for AI provider accounts and usage tracking (TDD-16)
 *
 * Key principles:
 * - Credentials NEVER stored on Orca Server — only written to Dev Server via relay
 * - testConnection() is non-throwing (returns { ok: false, error } on failure)
 * - recordUsage() uses UPSERT ON CONFLICT for idempotent accumulation
 *
 * Pattern follows sql-repository.ts: pool.withConnection((db) => db.query(...))
 *
 * @module main/ai-providers/AIProviderService
 */

import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import type { DevServerManager } from '../dev-server/dev-server-manager'
import type { RelayConnectionPool } from '../dev-server/relay-connection-pool'
import { Tracers } from '../../shared/trace/tracers'
import type {
  AIProviderAccount,
  AIProviderType,
  AIProviderScope,
  AIProviderStatus,
  ProviderUsageToday,
} from '../../shared/ai-provider-types'

/** Raw DB row from orca_ai_provider_accounts */
type AccountRow = {
  id: string
  devServerId: string
  provider: string
  scope: string
  scopeRefId: string | null
  label: string
  model: string | null
  baseUrl: string | null
  status: string
  lastHealthCheck: number | null
  quotaLimitDay: number
  createdBy: string
  createdAt: number
  updatedAt: number
}

/** Parameters for creating a new provider account */
export type CreateAccountParams = {
  devServerId: string
  provider: AIProviderType
  scope: AIProviderScope
  scopeRefId?: string
  label: string
  model?: string
  baseUrl?: string
  quotaLimitDay?: number
  createdBy: string
}

/** Partial update payload */
export type UpdateAccountParams = {
  label?: string
  model?: string
  baseUrl?: string
  status?: AIProviderStatus
  lastHealthCheck?: Date
  quotaLimitDay?: number
}

function rowToAccount(r: AccountRow): AIProviderAccount {
  return {
    id: r.id,
    devServerId: r.devServerId,
    provider: r.provider as AIProviderType,
    scope: r.scope as AIProviderScope,
    scopeRefId: r.scopeRefId ?? undefined,
    label: r.label,
    model: r.model ?? undefined,
    baseUrl: r.baseUrl ?? undefined,
    status: r.status as AIProviderStatus,
    lastHealthCheck: r.lastHealthCheck ? new Date(r.lastHealthCheck) : undefined,
    quotaLimitDay: r.quotaLimitDay,
    createdBy: r.createdBy,
    createdAt: new Date(r.createdAt),
    updatedAt: new Date(r.updatedAt),
  }
}

export class AIProviderService {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly devServerManager: DevServerManager,
    private readonly relayPool: RelayConnectionPool
  ) {}

  // ── CRUD ────────────────────────────────────────────────────────────────────

  /** Create a new provider account. Returns the created account. */
  async createAccount(params: CreateAccountParams): Promise<AIProviderAccount> {
    const id = randomUUID()
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_ai_provider_accounts
           (id, dev_server_id, provider, scope, scope_ref_id, label, model, base_url,
            status, last_health_check, quota_limit_day, created_by, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [
          id,
          params.devServerId,
          params.provider,
          params.scope,
          params.scopeRefId ?? null,
          params.label,
          params.model ?? null,
          params.baseUrl ?? null,
          'pending',
          null,
          params.quotaLimitDay ?? 0,
          params.createdBy,
          now,
          now,
        ]
      )
    )
    return {
      id,
      devServerId: params.devServerId,
      provider: params.provider,
      scope: params.scope,
      scopeRefId: params.scopeRefId,
      label: params.label,
      model: params.model,
      baseUrl: params.baseUrl,
      status: 'pending',
      quotaLimitDay: params.quotaLimitDay ?? 0,
      createdBy: params.createdBy,
      createdAt: new Date(now),
      updatedAt: new Date(now),
    }
  }

  /** Get a provider account by ID. Returns null if not found. */
  async getAccount(accountId: string): Promise<AIProviderAccount | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<AccountRow>(
        `SELECT
           id, dev_server_id as devServerId, provider, scope, scope_ref_id as scopeRefId,
           label, model, base_url as baseUrl, status,
           last_health_check as lastHealthCheck, quota_limit_day as quotaLimitDay,
           created_by as createdBy, created_at as createdAt, updated_at as updatedAt
         FROM orca_ai_provider_accounts WHERE id = ?`,
        [accountId]
      )
    )
    if (!rows[0]) {return null}
    return rowToAccount(rows[0])
  }

  /** List accounts for a dev server, optionally filtered by scope. */
  async listAccounts(devServerId: string, scope?: AIProviderScope): Promise<AIProviderAccount[]> {
    let sql = `SELECT
      id, dev_server_id as devServerId, provider, scope, scope_ref_id as scopeRefId,
      label, model, base_url as baseUrl, status,
      last_health_check as lastHealthCheck, quota_limit_day as quotaLimitDay,
      created_by as createdBy, created_at as createdAt, updated_at as updatedAt
    FROM orca_ai_provider_accounts WHERE dev_server_id = ?`
    const params: unknown[] = [devServerId]
    if (scope) {
      sql += ' AND scope = ?'
      params.push(scope)
    }
    const rows = await this.pool.withConnection((db) => db.query<AccountRow>(sql, params))
    return rows.map(rowToAccount)
  }

  /** Get all accounts across all dev servers. */
  async getAllAccounts(): Promise<AIProviderAccount[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<AccountRow>(
        `SELECT
           id, dev_server_id as devServerId, provider, scope, scope_ref_id as scopeRefId,
           label, model, base_url as baseUrl, status,
           last_health_check as lastHealthCheck, quota_limit_day as quotaLimitDay,
           created_by as createdBy, created_at as createdAt, updated_at as updatedAt
         FROM orca_ai_provider_accounts ORDER BY created_at DESC`
      )
    )
    return rows.map(rowToAccount)
  }

  /** Update a provider account (partial patch). */
  async updateAccount(accountId: string, patch: UpdateAccountParams): Promise<void> {
    const now = Date.now()
    const sets: string[] = ['updated_at = ?']
    const values: unknown[] = [now]

    if (patch.label !== undefined) { sets.push('label = ?'); values.push(patch.label) }
    if (patch.model !== undefined) { sets.push('model = ?'); values.push(patch.model) }
    if (patch.baseUrl !== undefined) { sets.push('base_url = ?'); values.push(patch.baseUrl) }
    if (patch.status !== undefined) { sets.push('status = ?'); values.push(patch.status) }
    if (patch.lastHealthCheck !== undefined) {
      sets.push('last_health_check = ?')
      values.push(patch.lastHealthCheck.getTime())
    }
    if (patch.quotaLimitDay !== undefined) {
      sets.push('quota_limit_day = ?')
      values.push(patch.quotaLimitDay)
    }

    values.push(accountId)
    await this.pool.withConnection((db) =>
      db.query(`UPDATE orca_ai_provider_accounts SET ${sets.join(', ')} WHERE id = ?`, values)
    )
  }

  /** Delete a provider account (cascades to usage). */
  async deleteAccount(accountId: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query('DELETE FROM orca_ai_provider_accounts WHERE id = ?', [accountId])
    )
  }

  // ── Relay operations ─────────────────────────────────────────────────────────

  /**
   * Write an encrypted credential to the dev server via relay.
   * NEVER stores the credential on Orca Server.
   */
  async writeCredentialToDevServer(
    accountId: string,
    encryptedBlob: string,
    iv: string,
    traceId?: string // optional — forwarded from ai-provider-rpc-handler.ts when FE sends one
  ): Promise<void> {
    // SECURITY: only trace blobLength (byte count) — never encryptedBlob/iv/apiKey.
    const span = Tracers.aiProviderWriteCredFlow.start(
      { accountId, blobLength: encryptedBlob.length },
      traceId ? { id: traceId } : undefined
    )

    try {
      const account = await this.getAccount(accountId)
      if (!account) {
        span.fail('ACCOUNT_NOT_FOUND', { accountId })
        throw new Error(`ACCOUNT_NOT_FOUND: ${accountId}`)
      }

      const server = this.devServerManager.get(account.devServerId)
      if (!server) {
        span.fail('DEV_SERVER_NOT_FOUND', { accountId, devServerId: account.devServerId })
        throw new Error(`DEV_SERVER_NOT_FOUND: ${account.devServerId}`)
      }
      span.step('lookup-account', { accountId, devServerId: account.devServerId })

      const relay = await this.relayPool.getOrConnect(account.devServerId, server)
      span.step('relay-connect', { devServerId: account.devServerId })

      // SECURITY: params sent to relay carry the real encryptedBlob/iv, but they
      // must never be attached to trace fields.
      span.step('agent-call', { method: 'ai.provider.writeCredential', accountId })
      await relay.call('ai.provider.writeCredential', { accountId, encryptedBlob, iv })

      // FIX TASK-AIP-001: Update status pending → active after successful credential write.
      // Without this, resolveForProject() returns no candidates (it filters for status='active')
      // and all AI features fail silently.
      await this.updateAccount(accountId, { status: 'active' })
      span.ok({ accountId, status: 'active' })
    } catch (err) {
      span.fail(err, { accountId })
      throw err
    }
  }

  /**
   * Test connectivity for a provider account via relay.
   * Non-throwing — returns { ok: false, error } on any failure.
   */
  async testConnection(accountId: string): Promise<{ ok: boolean; latencyMs: number; error?: string }> {
    const start = Date.now()
    try {
      const account = await this.getAccount(accountId)
      if (!account) {return { ok: false, latencyMs: 0, error: `ACCOUNT_NOT_FOUND: ${accountId}` }}

      const server = this.devServerManager.get(account.devServerId)
      if (!server) {return { ok: false, latencyMs: 0, error: `DEV_SERVER_NOT_FOUND: ${account.devServerId}` }}

      const relay = await this.relayPool.getOrConnect(account.devServerId, server)
      await relay.call('ai.provider.testConnection', { accountId })

      return { ok: true, latencyMs: Date.now() - start }
    } catch (err) {
      return {
        ok: false,
        latencyMs: Date.now() - start,
        error: err instanceof Error ? err.message : String(err),
      }
    }
  }

  // ── Usage tracking ───────────────────────────────────────────────────────────

  /**
   * Record daily token/cost usage via UPSERT.
   * Accumulates tokens and requests; cost_usd is summed.
   */
  async recordUsage(
    accountId: string,
    tokens: number,
    requests: number,
    costUsd: number
  ): Promise<void> {
    const date = new Date().toISOString().slice(0, 10) // YYYY-MM-DD
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_provider_usage (account_id, date, tokens_used, requests, cost_usd)
         VALUES (?, ?, ?, ?, ?)
         ON CONFLICT(account_id, date) DO UPDATE SET
           tokens_used = tokens_used + excluded.tokens_used,
           requests    = requests    + excluded.requests,
           cost_usd    = cost_usd    + excluded.cost_usd`,
        [accountId, date, tokens, requests, costUsd]
      )
    )
  }

  /** Get today's aggregated usage for a provider account. */
  async getUsageToday(accountId: string): Promise<ProviderUsageToday> {
    const date = new Date().toISOString().slice(0, 10)
    const rows = await this.pool.withConnection((db) =>
      db.query<{ tokensUsed: number; requests: number; costUsd: number }>(
        `SELECT tokens_used as tokensUsed, requests, cost_usd as costUsd
         FROM orca_provider_usage WHERE account_id = ? AND date = ?`,
        [accountId, date]
      )
    )
    if (!rows[0]) {return { tokens: 0, requests: 0, costUsd: 0 }}
    return { tokens: rows[0].tokensUsed, requests: rows[0].requests, costUsd: rows[0].costUsd }
  }

  // ── Provider resolution ──────────────────────────────────────────────────────

  /**
   * Resolve the best AI provider account for a project/user.
   * Priority: user-scope > project-scope > server-scope, modelHint filter first.
   * Returns null if no account matches.
   */
  async resolveForProject(
    devServerId: string,
    projectId: string,
    userId: string,
    modelHint?: string
  ): Promise<AIProviderAccount | null> {
    const all = await this.listAccounts(devServerId)
    const active = all.filter(a => a.status === 'active')

    const scopePriority: { scope: AIProviderScope; scopeRefId?: string }[] = [
      { scope: 'user', scopeRefId: userId },
      { scope: 'project', scopeRefId: projectId },
      { scope: 'server' },
    ]

    // Try with modelHint first, then without
    for (const useModelHint of [true, false]) {
      for (const { scope, scopeRefId } of scopePriority) {
        const candidates = active.filter(a => {
          if (a.scope !== scope) {return false}
          if (scopeRefId && a.scopeRefId !== scopeRefId) {return false}
          if (useModelHint && modelHint && a.model !== modelHint) {return false}
          return true
        })
        if (candidates[0]) {return candidates[0]}
      }
    }

    return null
  }
}
