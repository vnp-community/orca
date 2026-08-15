/**
 * PgUsageStore — Postgres-backed repository for the `usage` schema
 * (ADR-021 Phase 1)
 *
 * Reads/writes `usage.{claude,codex}_usage_sessions` /
 * `usage.{claude,codex}_usage_daily` / `usage.{claude,codex}_usage_processed_files`
 * (migration 0022_notification_usage_schema.ts). Server mode only.
 *
 * ⚠️ NOT wired into `ClaudeUsageStore`/`CodexUsageStore` — unlike
 * `PgAutomationStore`/`PgWebPushStore`, those two classes' own persistence
 * (`ClaudeUsagePersistedState` — `schemaVersion`, `worktreeFingerprint`,
 * `processedFiles`, `sessions`, `dailyAggregates`, `scanState`) is baked
 * directly into ~870-line classes via `readFileSync`/`writeFileSync` on a
 * hardcoded `orca-*-usage.json` path — there is no swappable interface seam
 * there yet (unlike `Store`, which `AutomationService`/`WebPushManager`
 * already depended on through a narrow slice). Building that seam (extracting
 * an `IUsagePersistence` the way `AutomationStoreDependency` extracts
 * `AutomationService`'s slice of `Store`) is real, separate work — this class
 * is the repository half of that future seam, implemented against the actual
 * schema now so the interface-extraction pass has a verified target to code
 * against, not a second thing to design from scratch.
 *
 * @module main/usage/pg-usage-store
 */

import type { IConnectionPool } from '../db/pool'
import { serviceQualifiedTable } from '../db/migrations/sql-dialect'

export type UsageProvider = 'claude' | 'codex'

export type UsageSessionRecord = {
  sessionId: string
  userId?: string
  firstTimestamp: string
  lastTimestamp: string
  model: string | null
  lastCwd: string | null
  lastGitBranch: string | null
  primaryWorktreeId: string | null
  primaryRepoId: string | null
  turnCount: number
  totalInputTokens: number
  totalOutputTokens: number
  totalCacheReadTokens: number
  totalCacheWriteTokens: number
  locationBreakdown: unknown[]
}

export type UsageDailyAggregateRecord = {
  userId?: string
  day: string
  model: string | null
  projectKey: string
  projectLabel: string
  repoId: string | null
  worktreeId: string | null
  turnCount: number
  zeroCacheReadTurnCount: number
  inputTokens: number
  outputTokens: number
  cacheReadTokens: number
  cacheWriteTokens: number
}

type SessionRow = {
  sessionId: string
  userId: string | null
  firstTimestamp: string
  lastTimestamp: string
  model: string | null
  lastCwd: string | null
  lastGitBranch: string | null
  primaryWorktreeId: string | null
  primaryRepoId: string | null
  turnCount: number
  totalInputTokens: number
  totalOutputTokens: number
  totalCacheReadTokens: number
  totalCacheWriteTokens: number
  locationBreakdownJson: string
}

function rowToSession(row: SessionRow): UsageSessionRecord {
  return {
    sessionId: row.sessionId,
    userId: row.userId ?? undefined,
    firstTimestamp: row.firstTimestamp,
    lastTimestamp: row.lastTimestamp,
    model: row.model,
    lastCwd: row.lastCwd,
    lastGitBranch: row.lastGitBranch,
    primaryWorktreeId: row.primaryWorktreeId,
    primaryRepoId: row.primaryRepoId,
    turnCount: row.turnCount,
    totalInputTokens: row.totalInputTokens,
    totalOutputTokens: row.totalOutputTokens,
    totalCacheReadTokens: row.totalCacheReadTokens,
    totalCacheWriteTokens: row.totalCacheWriteTokens,
    locationBreakdown: JSON.parse(row.locationBreakdownJson)
  }
}

export class PgUsageStore {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly tenantId: string | undefined
  ) {}

  private sessionsTable(provider: UsageProvider, dialect: Parameters<typeof serviceQualifiedTable>[0]): string {
    return serviceQualifiedTable(dialect, 'usage', `${provider}_usage_sessions`)
  }

  private dailyTable(provider: UsageProvider, dialect: Parameters<typeof serviceQualifiedTable>[0]): string {
    return serviceQualifiedTable(dialect, 'usage', `${provider}_usage_daily`)
  }

  async listSessions(provider: UsageProvider, userId?: string): Promise<UsageSessionRecord[]> {
    const conditions: string[] = []
    const params: string[] = []
    if (this.tenantId) {
      conditions.push('tenant_id = ?')
      params.push(this.tenantId)
    }
    if (userId) {
      conditions.push('user_id = ?')
      params.push(userId)
    }
    const where = conditions.length > 0 ? ` WHERE ${conditions.join(' AND ')}` : ''
    const rows = await this.pool.withConnection((db) =>
      db.query<SessionRow>(
        `SELECT session_id as sessionId, user_id as userId, first_timestamp as firstTimestamp,
                last_timestamp as lastTimestamp, model, last_cwd as lastCwd,
                last_git_branch as lastGitBranch, primary_worktree_id as primaryWorktreeId,
                primary_repo_id as primaryRepoId, turn_count as turnCount,
                total_input_tokens as totalInputTokens, total_output_tokens as totalOutputTokens,
                total_cache_read_tokens as totalCacheReadTokens,
                total_cache_write_tokens as totalCacheWriteTokens,
                location_breakdown_json as locationBreakdownJson
         FROM ${this.sessionsTable(provider, db.capabilities.dialect)}${where}`,
        params
      )
    )
    return rows.map(rowToSession)
  }

  /** Upsert (by session_id, which is PRIMARY KEY — INSERT/UPDATE by dialect capability). */
  async upsertSession(provider: UsageProvider, session: UsageSessionRecord): Promise<void> {
    await this.pool.withConnection(async (db) => {
      const table = this.sessionsTable(provider, db.capabilities.dialect)
      const existing = await db.query(`SELECT session_id FROM ${table} WHERE session_id = ?`, [session.sessionId])
      const params = [
        this.tenantId ?? null, session.userId ?? null, session.firstTimestamp, session.lastTimestamp,
        session.model, session.lastCwd, session.lastGitBranch, session.primaryWorktreeId,
        session.primaryRepoId, session.turnCount, session.totalInputTokens, session.totalOutputTokens,
        session.totalCacheReadTokens, session.totalCacheWriteTokens,
        JSON.stringify(session.locationBreakdown)
      ]
      if (existing.length > 0) {
        await db.query(
          `UPDATE ${table} SET tenant_id = ?, user_id = ?, first_timestamp = ?, last_timestamp = ?,
             model = ?, last_cwd = ?, last_git_branch = ?, primary_worktree_id = ?, primary_repo_id = ?,
             turn_count = ?, total_input_tokens = ?, total_output_tokens = ?,
             total_cache_read_tokens = ?, total_cache_write_tokens = ?, location_breakdown_json = ?
           WHERE session_id = ?`,
          [...params, session.sessionId]
        )
      } else {
        await db.query(
          `INSERT INTO ${table} (tenant_id, user_id, first_timestamp, last_timestamp, model, last_cwd,
             last_git_branch, primary_worktree_id, primary_repo_id, turn_count, total_input_tokens,
             total_output_tokens, total_cache_read_tokens, total_cache_write_tokens,
             location_breakdown_json, session_id)
           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
          [...params, session.sessionId]
        )
      }
    })
  }

  /**
   * Upsert a daily aggregate row. Relies on migration 0022's UNIQUE index
   * (tenant_id, user_id, day, model, project_key, repo_id, worktree_id) —
   * see that migration's ⚠️ Phase 1 caveat comment about NULL columns not
   * colliding in that index: callers MUST pass '' instead of null/undefined
   * for repoId/worktreeId to get real upsert semantics.
   */
  async upsertDailyAggregate(provider: UsageProvider, aggregate: UsageDailyAggregateRecord): Promise<void> {
    const repoId = aggregate.repoId ?? ''
    const worktreeId = aggregate.worktreeId ?? ''
    await this.pool.withConnection(async (db) => {
      const table = this.dailyTable(provider, db.capabilities.dialect)
      const existing = await db.query(
        `SELECT id FROM ${table} WHERE tenant_id ${this.tenantId ? '= ?' : 'IS NULL'} AND
           user_id ${aggregate.userId ? '= ?' : 'IS NULL'} AND day = ? AND model ${aggregate.model ? '= ?' : 'IS NULL'}
           AND project_key = ? AND repo_id = ? AND worktree_id = ?`,
        [
          ...(this.tenantId ? [this.tenantId] : []),
          ...(aggregate.userId ? [aggregate.userId] : []),
          aggregate.day,
          ...(aggregate.model ? [aggregate.model] : []),
          aggregate.projectKey, repoId, worktreeId
        ]
      )
      const counters = [
        aggregate.turnCount, aggregate.zeroCacheReadTurnCount, aggregate.inputTokens,
        aggregate.outputTokens, aggregate.cacheReadTokens, aggregate.cacheWriteTokens
      ]
      if (existing.length > 0) {
        const id = (existing[0] as { id: string }).id
        await db.query(
          `UPDATE ${table} SET turn_count = ?, zero_cache_read_turn_count = ?, input_tokens = ?,
             output_tokens = ?, cache_read_tokens = ?, cache_write_tokens = ? WHERE id = ?`,
          [...counters, id]
        )
      } else {
        await db.query(
          `INSERT INTO ${table} (id, tenant_id, user_id, day, model, project_key, project_label,
             repo_id, worktree_id, turn_count, zero_cache_read_turn_count, input_tokens,
             output_tokens, cache_read_tokens, cache_write_tokens)
           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
          [
            `${provider}-${aggregate.day}-${aggregate.projectKey}-${repoId}-${worktreeId}-${aggregate.model ?? ''}`,
            this.tenantId ?? null, aggregate.userId ?? null, aggregate.day, aggregate.model,
            aggregate.projectKey, aggregate.projectLabel, repoId, worktreeId, ...counters
          ]
        )
      }
    })
  }

  async listDailyAggregates(provider: UsageProvider, userId?: string): Promise<UsageDailyAggregateRecord[]> {
    const conditions: string[] = []
    const params: string[] = []
    if (this.tenantId) {
      conditions.push('tenant_id = ?')
      params.push(this.tenantId)
    }
    if (userId) {
      conditions.push('user_id = ?')
      params.push(userId)
    }
    const where = conditions.length > 0 ? ` WHERE ${conditions.join(' AND ')}` : ''
    const rows = await this.pool.withConnection((db) =>
      db.query<{
        userId: string | null
        day: string
        model: string | null
        projectKey: string
        projectLabel: string
        repoId: string | null
        worktreeId: string | null
        turnCount: number
        zeroCacheReadTurnCount: number
        inputTokens: number
        outputTokens: number
        cacheReadTokens: number
        cacheWriteTokens: number
      }>(
        `SELECT user_id as userId, day, model, project_key as projectKey, project_label as projectLabel,
                repo_id as repoId, worktree_id as worktreeId, turn_count as turnCount,
                zero_cache_read_turn_count as zeroCacheReadTurnCount, input_tokens as inputTokens,
                output_tokens as outputTokens, cache_read_tokens as cacheReadTokens,
                cache_write_tokens as cacheWriteTokens
         FROM ${this.dailyTable(provider, db.capabilities.dialect)}${where}
         ORDER BY day DESC`,
        params
      )
    )
    return rows.map((row) => ({
      userId: row.userId ?? undefined,
      day: row.day,
      model: row.model,
      projectKey: row.projectKey,
      projectLabel: row.projectLabel,
      repoId: row.repoId || null,
      worktreeId: row.worktreeId || null,
      turnCount: row.turnCount,
      zeroCacheReadTurnCount: row.zeroCacheReadTurnCount,
      inputTokens: row.inputTokens,
      outputTokens: row.outputTokens,
      cacheReadTokens: row.cacheReadTokens,
      cacheWriteTokens: row.cacheWriteTokens
    }))
  }
}
