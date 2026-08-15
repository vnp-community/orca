/**
 * PgUsageStatePersistence — Postgres-backed `UsageStatePersistence<TState>`
 * (ADR-021, "chỉ dùng 1 database")
 *
 * Whole-state JSON blob in `usage.{claude,codex}_usage_state_blob`
 * (migration 0023_usage_state_blob.ts), one row per (tenant, user). See
 * usage-state-persistence.ts's module doc comment for why a blob table
 * instead of the granular tables from migration 0022.
 *
 * @module main/usage/pg-usage-state-persistence
 */

import type { IConnectionPool } from '../db/pool'
import { serviceQualifiedTable } from '../db/migrations/sql-dialect'
import type { UsageStatePersistence } from './usage-state-persistence'

export class PgUsageStatePersistence<TState> implements UsageStatePersistence<TState> {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly provider: 'claude' | 'codex',
    private readonly tenantId: string | undefined,
    private readonly userId: string = ''
  ) {}

  private table(dialect: Parameters<typeof serviceQualifiedTable>[0]): string {
    return serviceQualifiedTable(dialect, 'usage', `${this.provider}_usage_state_blob`)
  }

  async load(): Promise<TState | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<{ stateJson: string }>(
        `SELECT state_json as stateJson FROM ${this.table(db.capabilities.dialect)}
         WHERE tenant_id = ? AND user_id = ?`,
        [this.tenantId ?? '', this.userId]
      )
    )
    if (!rows[0]) {return null}
    return JSON.parse(rows[0].stateJson) as TState
  }

  async save(state: TState): Promise<void> {
    await this.pool.withConnection(async (db) => {
      const table = this.table(db.capabilities.dialect)
      const existing = await db.query(
        `SELECT tenant_id FROM ${table} WHERE tenant_id = ? AND user_id = ?`,
        [this.tenantId ?? '', this.userId]
      )
      const stateJson = JSON.stringify(state)
      if (existing.length > 0) {
        await db.query(
          `UPDATE ${table} SET state_json = ?, updated_at = ? WHERE tenant_id = ? AND user_id = ?`,
          [stateJson, Date.now(), this.tenantId ?? '', this.userId]
        )
      } else {
        await db.query(
          `INSERT INTO ${table} (tenant_id, user_id, state_json, updated_at) VALUES (?, ?, ?, ?)`,
          [this.tenantId ?? '', this.userId, stateJson, Date.now()]
        )
      }
    })
  }
}
