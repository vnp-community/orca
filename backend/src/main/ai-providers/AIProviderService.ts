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
import type { BindValue } from '../db/types'
import type { DevServerManager } from '../dev-server/dev-server-manager'
import type { RelayConnectionPool } from '../dev-server/relay-connection-pool'
import { Tracers } from '../../shared/trace/tracers'
import type { AuditLogger } from '../auth/audit-logger'
import type {
  AIProviderAccount,
  AIProviderType,
  AIProviderScope,
  AIProviderStatus,
  ProviderUsageToday,
} from '../../shared/ai-provider-types'

/** BUG-BE-HLD-014: default old-credential grace window when rotating a key. */
export const DEFAULT_ROTATION_GRACE_PERIOD_MS = 30_000

/** Result of a rotateKey() call — reported back over RPC. */
export type RotateKeyResult = {
  accountId: string
  status: AIProviderStatus
  rotationGraceUntil: Date
}

/** Shadow account id used to stage the new credential during a rotation. */
function rotationShadowId(accountId: string): string {
  return `${accountId}::rotating`
}

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
  /** BUG-BE-HLD-014: NULL unless status='rotating' (migration 0015) */
  rotationGraceUntil: number | null
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
  /** BUG-BE-HLD-014: pass `null` to clear once rotation completes/fails */
  rotationGraceUntil?: Date | null
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
    rotationGraceUntil: r.rotationGraceUntil ? new Date(r.rotationGraceUntil) : undefined,
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
    private readonly relayPool: RelayConnectionPool,
    // BUG-BE-HLD-014: optional so existing call sites keep compiling until
    // they're updated to inject a real AuditLogger.
    private readonly auditLogger?: AuditLogger
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

    // BUG-BE-HLD-014: audit trail for account creation. Never blocks the caller.
    void this.auditLogger?.log({
      action: 'aiProvider.create',
      userId: params.createdBy,
      userEmail: params.createdBy, // RpcContext has no separate email field at this layer
      ip: '',
      details: { accountId: id, provider: params.provider, scope: params.scope, devServerId: params.devServerId },
    })

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
           id, dev_server_id as "devServerId", provider, scope, scope_ref_id as "scopeRefId",
           label, model, base_url as "baseUrl", status,
           last_health_check as "lastHealthCheck", quota_limit_day as "quotaLimitDay",
           created_by as "createdBy", created_at as "createdAt", updated_at as "updatedAt"
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
      id, dev_server_id as "devServerId", provider, scope, scope_ref_id as "scopeRefId",
      label, model, base_url as "baseUrl", status,
      last_health_check as "lastHealthCheck", quota_limit_day as "quotaLimitDay",
      created_by as "createdBy", created_at as "createdAt", updated_at as "updatedAt"
    FROM orca_ai_provider_accounts WHERE dev_server_id = ?`
    const params: BindValue[] = [devServerId]
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
           id, dev_server_id as "devServerId", provider, scope, scope_ref_id as "scopeRefId",
           label, model, base_url as "baseUrl", status,
           last_health_check as "lastHealthCheck", quota_limit_day as "quotaLimitDay",
           created_by as "createdBy", created_at as "createdAt", updated_at as "updatedAt"
         FROM orca_ai_provider_accounts ORDER BY created_at DESC`
      )
    )
    return rows.map(rowToAccount)
  }

  /** Update a provider account (partial patch). */
  async updateAccount(accountId: string, patch: UpdateAccountParams, actorUserId?: string): Promise<void> {
    const now = Date.now()
    const sets: string[] = ['updated_at = ?']
    const values: BindValue[] = [now]

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
    // BUG-BE-HLD-014: rotationGraceUntil is set by rotateKey() and cleared (null)
    // by completeRotation() — `undefined` here means "leave column untouched".
    if (patch.rotationGraceUntil !== undefined) {
      sets.push('rotation_grace_until = ?')
      values.push(patch.rotationGraceUntil ? patch.rotationGraceUntil.getTime() : null)
    }

    values.push(accountId)
    await this.pool.withConnection((db) =>
      db.query(`UPDATE orca_ai_provider_accounts SET ${sets.join(', ')} WHERE id = ?`, values)
    )

    // BUG-BE-HLD-014: audit only real CRUD edits from callers that pass actorUserId
    // (RPC handler does); internal calls from rotateKey/completeRotation/health
    // checker log their own dedicated action codes instead (see below).
    if (actorUserId && patch.status === undefined) {
      void this.auditLogger?.log({
        action: 'aiProvider.update',
        userId: actorUserId,
        userEmail: actorUserId,
        ip: '',
        details: { accountId, patchedFields: Object.keys(patch) },
      })
    }
  }

  /** Delete a provider account (cascades to usage). */
  async deleteAccount(accountId: string, actorUserId?: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query('DELETE FROM orca_ai_provider_accounts WHERE id = ?', [accountId])
    )
    void this.auditLogger?.log({
      action: 'aiProvider.delete',
      userId: actorUserId ?? 'unknown',
      userEmail: actorUserId ?? 'unknown',
      ip: '',
      details: { accountId },
    })
  }

  // ── Key rotation (BUG-BE-HLD-014) ─────────────────────────────────────────────

  /**
   * Rotate the credential for an active account with a grace period.
   *
   * The REAL credential file (`${accountId}.enc` on the dev server) is left
   * untouched until completeRotation() commits — so requests already using
   * the old key, or new requests that resolve to this account while status
   * is 'rotating', keep working with the old key for the whole grace window.
   * The new key is staged + connection-tested at a shadow account id first,
   * so a bad key never touches the real credential or flips the account out
   * of 'active'.
   */
  async rotateKey(
    accountId: string,
    newCredential: { encryptedBlob: string; iv: string },
    options?: { gracePeriodMs?: number; actorUserId?: string }
  ): Promise<RotateKeyResult> {
    const gracePeriodMs = options?.gracePeriodMs ?? DEFAULT_ROTATION_GRACE_PERIOD_MS

    const account = await this.getAccount(accountId)
    if (!account) {throw new Error(`ACCOUNT_NOT_FOUND: ${accountId}`)}
    if (account.status === 'rotating') {throw new Error(`ROTATION_IN_PROGRESS: ${accountId}`)}
    if (account.status !== 'active') {
      throw new Error(`INVALID_STATUS_FOR_ROTATION: ${accountId} is '${account.status}', expected 'active'`)
    }

    const server = this.devServerManager.get(account.devServerId)
    if (!server) {throw new Error(`DEV_SERVER_NOT_FOUND: ${account.devServerId}`)}
    const relay = await this.relayPool.getOrConnect(account.devServerId, server)

    // Stage the new credential at a shadow id — never touches the real file.
    const shadowAccountId = rotationShadowId(accountId)
    await relay.call('ai.provider.writeCredential', {
      accountId: shadowAccountId,
      encryptedBlob: newCredential.encryptedBlob,
      iv: newCredential.iv,
    })

    const test = await relay.call<{ ok: boolean; error?: string }>(
      'ai.provider.testConnection',
      { accountId: shadowAccountId }
    )
    if (!test.ok) {
      throw new Error(`ROTATION_TEST_FAILED: ${test.error ?? 'unknown error'}`)
    }

    const rotationGraceUntil = new Date(Date.now() + gracePeriodMs)
    await this.updateAccount(accountId, { status: 'rotating', rotationGraceUntil })

    void this.auditLogger?.log({
      action: 'aiProvider.rotateKey.started',
      userId: options?.actorUserId ?? 'unknown',
      userEmail: options?.actorUserId ?? 'unknown',
      ip: '',
      details: { accountId, gracePeriodMs, blobLength: newCredential.encryptedBlob.length },
    })

    // Primary completion path. ProviderHealthChecker's 15-minute sweep is the
    // crash-recovery fallback if the process restarts before this fires —
    // completeRotation() re-reads the (still-encrypted) blob from the shadow
    // slot, so no credential needs to survive in process memory.
    const timer = setTimeout(() => {
      this.completeRotation(accountId).catch((err) =>
        console.error(`[AIProviderService] completeRotation failed for ${accountId}:`, err)
      )
    }, gracePeriodMs)
    timer.unref?.()

    return { accountId, status: 'rotating', rotationGraceUntil }
  }

  /**
   * Commit a rotation: copy the staged shadow credential onto the real
   * accountId and flip status back to 'active'. Idempotent no-op if the
   * account is no longer 'rotating' (already completed, deleted, or the
   * rotation was superseded).
   */
  async completeRotation(accountId: string): Promise<void> {
    const account = await this.getAccount(accountId)
    if (!account || account.status !== 'rotating') {return}

    const server = this.devServerManager.get(account.devServerId)
    if (!server) {
      await this.updateAccount(accountId, { status: 'unreachable', rotationGraceUntil: null })
      return
    }

    try {
      const relay = await this.relayPool.getOrConnect(account.devServerId, server)
      // Read back the ENCRYPTED blob staged in rotateKey() — never decrypted
      // on Orca Server (ADR-008: credentials only ever live on the dev server).
      const shadow = await relay.call<{ encryptedBlob: string; iv: string }>(
        'ai.provider.readCredential',
        { accountId: rotationShadowId(accountId) }
      )
      await relay.call('ai.provider.writeCredential', {
        accountId,
        encryptedBlob: shadow.encryptedBlob,
        iv: shadow.iv,
      })

      await this.updateAccount(accountId, { status: 'active', rotationGraceUntil: null })
      void this.auditLogger?.log({
        action: 'aiProvider.rotateKey.completed',
        userId: 'system',
        userEmail: 'system',
        ip: '',
        details: { accountId },
      })
    } catch (err) {
      // Real credential at ${accountId}.enc was never touched before this
      // catch — commit failed while copying, so we surface 'invalid' rather
      // than silently leaving 'rotating' (and the account unusable) forever.
      await this.updateAccount(accountId, { status: 'invalid', rotationGraceUntil: null })
      void this.auditLogger?.log({
        action: 'aiProvider.rotateKey.failed',
        userId: 'system',
        userEmail: 'system',
        ip: '',
        details: { accountId, error: err instanceof Error ? err.message : String(err) },
      })
      throw err
    }
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

      // BUG-BE-HLD-014: audit credential writes — length only, never the blob/iv.
      void this.auditLogger?.log({
        action: 'aiProvider.writeCredential',
        userId: 'system', // caller identity is enforced upstream by assertAccountAccess()
        userEmail: 'system',
        ip: '',
        details: { accountId, blobLength: encryptedBlob.length },
      })

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
        `SELECT tokens_used as "tokensUsed", requests, cost_usd as "costUsd"
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
    // BUG-BE-HLD-014: 'rotating' accounts still serve requests with the OLD
    // credential until completeRotation() commits — treat them as usable.
    const active = all.filter(a => a.status === 'active' || a.status === 'rotating')

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
