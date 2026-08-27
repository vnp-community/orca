/**
 * PostgreSQL Database Adapter
 *
 * Implements IAsyncDatabase for PostgreSQL.
 * Lazy-loads pg to avoid bundling in Electron desktop build.
 * Auto-registers the 'postgresql' provider on import.
 *
 * @module db/postgresql/pg-adapter
 */

import type {
  IAsyncDatabase,
  IStatement,
  IDatabaseCapabilities,
  BindValue,
  StatementResult
} from '../types'
import { registerDatabaseProvider } from '../provider'
import type { PostgresConfig } from '../config'

/**
 * BUG-BE-RPC-003 follow-up: every query in this codebase is written once,
 * dialect-agnostic, using SQLite's `?` positional placeholder (never `$N`)
 * — confirmed live: migrations crashed with a raw Postgres "syntax error at
 * or near ','" the moment a `?`-placeholder query ran, because `pg` only
 * understands `$1, $2, ...`. Translate here, once, rather than rewriting
 * every call site across every service.
 *
 * Skips `?` inside single-quoted string literals so a literal `?` in SQL
 * text (not a bind placeholder) is never mistranslated. SQL's doubled `''`
 * escape for a quote-within-a-string still tracks correctly: toggling
 * in/out of "inside a string" on every `'` nets back to the same state
 * across an adjacent `''` pair, and no `?` can occur in that zero-width gap.
 */
export function translatePlaceholders(sql: string): string {
  let result = ''
  let paramIndex = 0
  let inString = false
  for (let i = 0; i < sql.length; i++) {
    const ch = sql[i]
    if (ch === "'") {
      inString = !inString
      result += ch
      continue
    }
    if (ch === '?' && !inString) {
      paramIndex++
      result += `$${paramIndex}`
      continue
    }
    result += ch
  }
  return result
}

class PgStatement implements IStatement {
  private readonly pgSql: string

  constructor(
    sql: string,
    private readonly client: unknown
  ) {
    this.pgSql = translatePlaceholders(sql)
  }

  async run(...params: BindValue[]): Promise<StatementResult> {
    const result = await (this.client as any).query(this.pgSql, params)
    return { changes: result.rowCount ?? 0, lastInsertRowid: 0 }
  }

  async get(...params: BindValue[]): Promise<Record<string, unknown> | undefined> {
    const result = await (this.client as any).query(this.pgSql, params)
    return result.rows[0]
  }

  async all(...params: BindValue[]): Promise<Record<string, unknown>[]> {
    const result = await (this.client as any).query(this.pgSql, params)
    return result.rows
  }
}

export class PostgreSQLAdapter implements IAsyncDatabase {
  readonly capabilities: IDatabaseCapabilities = {
    walMode: false,
    returning: true,
    nativeJson: true,
    placeholderStyle: 'positional',
    dialect: 'postgresql'
  }

  private constructor(private readonly client: unknown) {}

  static async connect(config: {
    host: string
    port: number
    database: string
    username: string
    password: string
    ssl?: boolean
    schema?: string
  }): Promise<PostgreSQLAdapter> {
    // Lazy import — avoid bundling pg in Electron desktop build
    let pg: any
    try {
      pg = await import('pg')
    } catch {
      throw new Error(
        'pg package is not installed. Run: pnpm add pg\n' +
          'pg is required for PostgreSQL support.'
      )
    }

    const client = new pg.Client({
      host: config.host,
      port: config.port,
      database: config.database,
      user: config.username,
      password: config.password,
      ssl: config.ssl ? { rejectUnauthorized: true } : undefined
    })
    await client.connect()

    if (config.schema && config.schema !== 'public') {
      await client.query(`SET search_path TO ${config.schema}`)
    }

    return new PostgreSQLAdapter(client)
  }

  async exec(sql: string): Promise<void> {
    await (this.client as any).query(sql)
  }

  async prepare(sql: string): Promise<IStatement> {
    return new PgStatement(sql, this.client)
  }

  async close(): Promise<void> {
    await (this.client as any).end()
  }

  async transaction<T>(fn: () => T | Promise<T>): Promise<T> {
    await (this.client as any).query('BEGIN')
    try {
      const result = await fn()
      await (this.client as any).query('COMMIT')
      return result
    } catch (err) {
      await (this.client as any).query('ROLLBACK')
      throw err
    }
  }

  async query<T = Record<string, unknown>>(sql: string, params?: BindValue[]): Promise<T[]> {
    const result = await (this.client as any).query(translatePlaceholders(sql), params ?? [])
    return result.rows as T[]
  }
}

// Auto-register PostgreSQL provider when this module is imported
registerDatabaseProvider({
  dialect: 'postgresql',
  async connect(config) {
    const cfg = config as PostgresConfig
    return PostgreSQLAdapter.connect({
      host: cfg.host,
      port: cfg.port ?? 5432,
      database: cfg.database,
      username: cfg.username,
      password: cfg.password ?? '',
      ssl: cfg.ssl,
      schema: cfg.schema
    })
  }
})
