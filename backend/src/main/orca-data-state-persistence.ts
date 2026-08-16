/**
 * PgOrcaDataStatePersistence — Postgres-backed load/save boundary for
 * `Store` (persistence.ts) (ADR-021, "chỉ dùng 1 database")
 *
 * `Store`'s constructor calls its own synchronous `this.load()`
 * (readFileSync/JSON.parse) and every mutator ends in `this.scheduleSave()`
 * → an already-async, already-debounced `writeToDiskAsync()`. Postgres I/O
 * cannot happen inside a synchronous constructor, so this class is used in
 * two different ways, not one uniform interface:
 *
 * 1. `loadRawState()` — called by server-bootstrap.ts BEFORE constructing
 *    `Store`, so the fetched (possibly-null) parsed JSON can be threaded in
 *    via `StoreOptions.preloadedRawState`. `Store`'s `load()` then skips its
 *    own `readFileSync` entirely when that option is present but keeps
 *    every migration/decrypt/defaults-merge line after that read
 *    byte-for-byte unchanged — this class only replaces "where the raw bytes
 *    came from," never the 700+ lines of normalize-on-load logic.
 * 2. `save(rawStateJson)` — called from `Store.writeToDiskAsync()`'s tail
 *    instead of `writeFile`+`rename` when `Store` is constructed with a
 *    `persistOverride`. Takes the exact string `buildStateToSave()` already
 *    produces (secrets pre-encrypted) — no reparsing, no reserialization.
 *
 * Electron desktop mode passes neither option — `Store` behaves exactly as
 * before (file-based) when `StoreOptions.preloadedRawState`/`persistOverride`
 * are omitted.
 *
 * @module main/orca-data-state-persistence
 */

import type { IConnectionPool } from './db/pool'
import { serviceQualifiedTable } from './db/migrations/sql-dialect'

export class PgOrcaDataStatePersistence {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly tenantId: string | undefined,
    private readonly userId: string = ''
  ) {}

  private table(dialect: Parameters<typeof serviceQualifiedTable>[0]): string {
    return serviceQualifiedTable(dialect, 'core', 'orca_data_state_blob')
  }

  /** Returns the raw parsed state object, or `null` if no row exists yet (fresh install). */
  async loadRawState<TState>(): Promise<TState | null> {
    // Why quoted alias: Postgres folds unquoted identifiers to lowercase
    // (`as stateJson` comes back as the column `statejson`, not `stateJson`)
    // — silently produced `row.stateJson === undefined` and
    // `JSON.parse(undefined)` crashed every user-process boot in production
    // (2026-08-16 incident). Double-quoting preserves the exact case on both
    // Postgres and SQLite.
    const rows = await this.pool.withConnection((db) =>
      db.query<{ stateJson: string }>(
        `SELECT state_json as "stateJson" FROM ${this.table(db.capabilities.dialect)}
         WHERE tenant_id = ? AND user_id = ?`,
        [this.tenantId ?? '', this.userId]
      )
    )
    if (!rows[0]) {return null}
    return JSON.parse(rows[0].stateJson) as TState
  }

  /** `payload` is an already-serialized JSON string (Store.buildStateToSave()'s output) — stored as-is. */
  async save(payload: string): Promise<void> {
    await this.pool.withConnection(async (db) => {
      const table = this.table(db.capabilities.dialect)
      const existing = await db.query(
        `SELECT tenant_id FROM ${table} WHERE tenant_id = ? AND user_id = ?`,
        [this.tenantId ?? '', this.userId]
      )
      existing.length > 0
        ? await db.query(
            `UPDATE ${table} SET state_json = ?, updated_at = ? WHERE tenant_id = ? AND user_id = ?`,
            [payload, Date.now(), this.tenantId ?? '', this.userId]
          )
        : await db.query(
            `INSERT INTO ${table} (tenant_id, user_id, state_json, updated_at) VALUES (?, ?, ?, ?)`,
            [this.tenantId ?? '', this.userId, payload, Date.now()]
          )
    })
  }
}
