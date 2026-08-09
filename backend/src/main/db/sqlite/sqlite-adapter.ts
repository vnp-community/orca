/**
 * SQLite Database Adapter
 *
 * Implements ISyncDatabase using Node.js built-in node:sqlite module.
 * Auto-registers the 'sqlite' DatabaseProvider on import.
 *
 * @module db/sqlite/sqlite-adapter
 */

import { existsSync } from 'node:fs'
import { DatabaseSync } from 'node:sqlite'
import type {
  ISyncDatabase,
  IStatement,
  IDatabaseCapabilities,
  BindValue,
  StatementResult
} from '../types'
import { registerDatabaseProvider } from '../provider'
import type { SqliteConfig } from '../config'

type NodeSQLInputValue = Parameters<ReturnType<DatabaseSync['prepare']>['run']>[0]

/** Wrapper that adapts node:sqlite StatementSync to IStatement */
class SqliteStatement implements IStatement {
  constructor(private readonly stmt: ReturnType<DatabaseSync['prepare']>) {}

  run(...params: BindValue[]): StatementResult {
    const result = this.stmt.run(...(params as NodeSQLInputValue[]))
    return {
      changes: (result as { changes: number }).changes ?? 0,
      lastInsertRowid: (result as { lastInsertRowid: number }).lastInsertRowid ?? 0
    }
  }

  get(...params: BindValue[]): Record<string, unknown> | undefined {
    return this.stmt.get(...(params as NodeSQLInputValue[])) as
      | Record<string, unknown>
      | undefined
  }

  all(...params: BindValue[]): Record<string, unknown>[] {
    return this.stmt.all(...(params as NodeSQLInputValue[])) as Record<string, unknown>[]
  }
}

/**
 * SqliteAdapter — ISyncDatabase implementation for Node.js server mode.
 * Uses node:sqlite (built-in since Node.js 22.5.0).
 */
export class SqliteAdapter implements ISyncDatabase {
  private readonly db: DatabaseSync

  readonly capabilities: IDatabaseCapabilities = {
    walMode: true,
    returning: false,
    nativeJson: false,
    placeholderStyle: 'positional',
    dialect: 'sqlite'
  }

  constructor(
    path: string,
    options: {
      readonly?: boolean
      fileMustExist?: boolean
      timeout?: number
    } = {}
  ) {
    if (options.fileMustExist && path !== ':memory:' && !existsSync(path)) {
      throw new Error(`SQLite database does not exist: "${path}"`)
    }

    this.db = new DatabaseSync(path, {
      readOnly: options.readonly ?? false,
      timeout: options.timeout
    } as Parameters<typeof DatabaseSync>[1])
  }

  exec(sql: string): void {
    this.db.exec(sql)
  }

  prepare(sql: string): IStatement {
    return new SqliteStatement(this.db.prepare(sql))
  }

  pragma(sql: string, options?: { simple?: boolean }): unknown {
    const stmt = this.db.prepare(`PRAGMA ${sql}`)
    if (options?.simple) {
      const row = stmt.get() as Record<string, unknown> | undefined
      return row ? Object.values(row)[0] : undefined
    }
    return stmt.all()
  }

  close(): void {
    this.db.close()
  }

  async transaction<T>(fn: () => T | Promise<T>): Promise<T> {
    this.db.exec('BEGIN')
    try {
      const result = await fn()
      this.db.exec('COMMIT')
      return result
    } catch (err) {
      try {
        this.db.exec('ROLLBACK')
      } catch {
        // ignore rollback errors
      }
      throw err
    }
  }

  async query(sql: string, params?: BindValue[]): Promise<Record<string, unknown>[]> {
    const stmt = this.prepare(sql) as SqliteStatement
    return stmt.all(...(params ?? []))
  }
}

// Auto-register SQLite provider when this module is imported
registerDatabaseProvider({
  dialect: 'sqlite',
  async connect(config) {
    const cfg = config as SqliteConfig
    return new SqliteAdapter(cfg.path, { readonly: cfg.readonly })
  }
})
