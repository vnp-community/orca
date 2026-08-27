/**
 * MySQL Database Adapter
 *
 * Implements IAsyncDatabase for MySQL, TiDB, MariaDB.
 * Lazy-loads mysql2 to avoid bundling in Electron desktop build.
 * Auto-registers providers for 'mysql', 'tidb', 'mariadb' on import.
 *
 * @module db/mysql/mysql-adapter
 */

import type {
  IAsyncDatabase,
  IStatement,
  IDatabaseCapabilities,
  BindValue,
  StatementResult
} from '../types'
import { registerDatabaseProvider } from '../provider'
import type { MysqlConfig } from '../config'

class MySQLStatement implements IStatement {
  constructor(
    private readonly sql: string,
    private readonly conn: unknown
  ) {}

  async run(...params: BindValue[]): Promise<StatementResult> {
    const [result] = await (this.conn as any).execute(this.sql, params)
    return {
      changes: (result as any).affectedRows ?? 0,
      lastInsertRowid: (result as any).insertId ?? 0
    }
  }

  async get(...params: BindValue[]): Promise<Record<string, unknown> | undefined> {
    const [rows] = await (this.conn as any).execute(this.sql, params)
    return (rows as Record<string, unknown>[])[0]
  }

  async all(...params: BindValue[]): Promise<Record<string, unknown>[]> {
    const [rows] = await (this.conn as any).execute(this.sql, params)
    return rows as Record<string, unknown>[]
  }
}

export class MySQLAdapter implements IAsyncDatabase {
  readonly capabilities: IDatabaseCapabilities = {
    walMode: false,
    returning: false,
    nativeJson: true,
    placeholderStyle: 'positional',
    dialect: 'mysql'
  }

  private constructor(private readonly connection: unknown) {}

  static async connect(config: {
    host: string
    port: number
    database: string
    username: string
    password: string
    ssl?: boolean
    dialect?: 'mysql' | 'tidb' | 'mariadb'
  }): Promise<MySQLAdapter> {
    // Lazy import — avoid bundling mysql2 in Electron desktop build
    let mysql2: any
    try {
      mysql2 = await import('mysql2/promise')
    } catch {
      throw new Error(
        'mysql2 package is not installed. ' +
          'Run: pnpm add mysql2\n' +
          'mysql2 is required for MySQL/TiDB/MariaDB support.'
      )
    }

    const conn = await mysql2.createConnection({
      host: config.host,
      port: config.port,
      database: config.database,
      user: config.username,
      password: config.password,
      ssl: config.ssl ? { rejectUnauthorized: true } : undefined,
      namedPlaceholders: false,
      supportBigNumbers: true,
      bigNumberStrings: false
    })

    return new MySQLAdapter(conn)
  }

  async exec(sql: string): Promise<void> {
    await (this.connection as any).execute(sql)
  }

  async prepare(sql: string): Promise<IStatement> {
    // mysql2 doesn't have an explicit prepare — simulate via execute()
    return new MySQLStatement(sql, this.connection)
  }

  async close(): Promise<void> {
    await (this.connection as any).end()
  }

  async transaction<T>(fn: () => T | Promise<T>): Promise<T> {
    await (this.connection as any).beginTransaction()
    try {
      const result = await fn()
      await (this.connection as any).commit()
      return result
    } catch (err) {
      await (this.connection as any).rollback()
      throw err
    }
  }

  async query<T = Record<string, unknown>>(sql: string, params?: BindValue[]): Promise<T[]> {
    const [rows] = await (this.connection as any).execute(sql, params ?? [])
    return rows as T[]
  }
}

// Auto-register MySQL/TiDB/MariaDB providers when this module is imported
for (const dialect of ['mysql', 'tidb', 'mariadb'] as const) {
  registerDatabaseProvider({
    dialect,
    async connect(config) {
      const cfg = config as MysqlConfig
      return MySQLAdapter.connect({
        host: cfg.host,
        port: cfg.port ?? 3306,
        database: cfg.database,
        username: cfg.username,
        password: cfg.password ?? '',
        ssl: cfg.ssl,
        dialect: cfg.dialect
      })
    }
  })
}
