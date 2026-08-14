/**
 * PooledDatabaseAdapter — `IDatabase` facade over a pooled `IConnectionPool`
 *
 * `AuthManager`/`AuthUserStore` (auth-user-store.ts) were built against a single,
 * always-open `IDatabase` connection (originally `SqliteAuthAdapter`, one SQLite
 * file handle for the process's lifetime) — not the `IConnectionPool` abstraction
 * `ProjectService`/`TeamService`/etc. use. Wiring auth onto a shared network
 * database (Postgres) needs the SAME pooled connection lifecycle those services
 * already get, without rewriting every `AuthUserStore` method — this adapter
 * bridges the two: it satisfies `IDatabase` by transparently acquiring/releasing
 * a pooled connection around every operation.
 *
 * Why deferred prepare(): a statement handle from a real driver's `prepare()` is
 * bound to the connection that created it. If this adapter acquired a connection
 * just to prepare a statement and then released it immediately, the returned
 * statement's later `.run()/.get()/.all()` would execute against a connection
 * that may have been handed to a different caller by then. Instead, `prepare()`
 * here only captures the SQL text — each `.run()/.get()/.all()` call acquires
 * its own connection, prepares, executes, and releases. This matches exactly how
 * `AuthUserStore` already calls it (`await this.db.prepare(sql)` immediately
 * followed by `.get(...)`/`.run(...)` in the same async function, never holding
 * a statement across unrelated operations) — no caller changes needed.
 *
 * See docs/guides/postgres-shared-database-design.md for the full design this
 * class is part of (BUG-BE-RPC-001/002 root-cause follow-up).
 *
 * @module db/pooled-database-adapter
 */

import type { IConnectionPool } from './pool'
import type { BindValue, IDatabase, IDatabaseCapabilities, IStatement, StatementResult } from './types'

class DeferredPooledStatement implements IStatement {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly sql: string
  ) {}

  async run(...params: BindValue[]): Promise<StatementResult> {
    return this.pool.withConnection(async (db) => {
      const stmt = await db.prepare(this.sql)
      return stmt.run(...params)
    })
  }

  async get(...params: BindValue[]): Promise<Record<string, unknown> | undefined> {
    return this.pool.withConnection(async (db) => {
      const stmt = await db.prepare(this.sql)
      return stmt.get(...params)
    })
  }

  async all(...params: BindValue[]): Promise<Record<string, unknown>[]> {
    return this.pool.withConnection(async (db) => {
      const stmt = await db.prepare(this.sql)
      return stmt.all(...params)
    })
  }

  // Why no iterate(): a pooled cursor would need to hold its connection open
  // across an unbounded number of caller-driven `.next()` calls — defeats
  // pooling and risks starving other callers if the consumer stops iterating
  // early. Nothing in AuthUserStore (this adapter's only consumer) uses it.
}

export class PooledDatabaseAdapter implements IDatabase {
  private constructor(
    private readonly pool: IConnectionPool,
    readonly capabilities: IDatabaseCapabilities
  ) {}

  /**
   * `capabilities` must be known synchronously (it's a readonly property, not
   * a method) but reading it requires a live connection — acquire one once at
   * construction time rather than guessing dialect-specific capabilities from
   * config. One extra round-trip at bootstrap; every later call reuses the pool.
   */
  static async create(pool: IConnectionPool): Promise<PooledDatabaseAdapter> {
    const capabilities = await pool.withConnection((db) => Promise.resolve(db.capabilities))
    return new PooledDatabaseAdapter(pool, capabilities)
  }

  async exec(sql: string): Promise<void> {
    await this.pool.withConnection(async (db) => {
      await db.exec(sql)
    })
  }

  prepare(sql: string): IStatement {
    return new DeferredPooledStatement(this.pool, sql)
  }

  async close(): Promise<void> {
    // No-op by design: the pool's lifecycle (drain/destroy) is owned by
    // server-bootstrap.ts's shutdown(), not by individual IDatabase consumers
    // like AuthManager — closing here would tear down the pool for every other
    // service sharing it (ProjectService, TeamService, ...).
  }

  async query(sql: string, params?: BindValue[]): Promise<Record<string, unknown>[]> {
    return this.pool.withConnection((db) => db.query(sql, params))
  }

  async transaction<T>(_fn: () => T | Promise<T>): Promise<T> {
    // Why not implemented: IDatabase.transaction()'s callback takes no `db`
    // argument — it's expected to call back into `this`'s own prepare()/query()
    // and have those reuse the SAME connection the transaction opened. This
    // adapter's prepare() deliberately acquires a FRESH pooled connection per
    // statement (see class doc comment), so naively wrapping pool.withTransaction()
    // here would silently run each statement on a different connection —
    // no atomicity, no rollback-together guarantee, and no error to warn the
    // caller. AuthUserStore (this adapter's only consumer) never calls
    // .transaction() today. Fail loud instead of shipping a transaction() that
    // looks correct but isn't, in case a future caller adds one.
    throw new Error(
      'PooledDatabaseAdapter.transaction() is not supported — see class doc comment. ' +
        'If a caller needs real cross-statement atomicity, acquire a connection via ' +
        'the IConnectionPool directly (pool.withTransaction) instead of through this adapter.'
    )
  }
}
