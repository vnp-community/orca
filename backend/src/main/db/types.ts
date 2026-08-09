/**
 * Database Abstraction Layer — Core Types
 *
 * Defines interfaces for multi-database support (SQLite, MySQL, PostgreSQL, TiDB).
 * Implementations: SqliteAdapter, MySQLAdapter, PgAdapter.
 *
 * @module db/types
 */

import type { DatabaseConfig } from './config'

/** Allowed bind parameter values for SQL statements */
export type BindValue = string | number | bigint | Buffer | null | undefined

/** Result of a DML statement (INSERT/UPDATE/DELETE) */
export interface StatementResult {
  changes: number
  lastInsertRowid: number | bigint
}

/** A prepared SQL statement (sync or async depending on backend) */
export interface IStatement {
  run(...params: BindValue[]): StatementResult | Promise<StatementResult>
  get(
    ...params: BindValue[]
  ): Record<string, unknown> | undefined | Promise<Record<string, unknown> | undefined>
  all(...params: BindValue[]): Record<string, unknown>[] | Promise<Record<string, unknown>[]>
  iterate?(...params: BindValue[]): IterableIterator<Record<string, unknown>>
}

/** Capabilities advertised by a database adapter */
export interface IDatabaseCapabilities {
  /** Whether WAL journal mode is active (SQLite only) */
  walMode: boolean
  /** Whether RETURNING clause is supported */
  returning: boolean
  /** Whether JSON functions are natively available */
  nativeJson: boolean
  /** Placeholder style: positional (?), named (:name), or both */
  placeholderStyle: 'positional' | 'named' | 'both'
  /** Database dialect identifier */
  dialect: 'sqlite' | 'mysql' | 'tidb' | 'mariadb' | 'postgresql'
}

/** Base database interface — works for both sync and async adapters */
export interface IDatabase {
  exec(sql: string): void | Promise<void>
  prepare(sql: string): IStatement | Promise<IStatement>
  close(): void | Promise<void>
  readonly capabilities: IDatabaseCapabilities
  transaction<T>(fn: () => T | Promise<T>): Promise<T>
  query(sql: string, params?: BindValue[]): Promise<Record<string, unknown>[]>
}

/**
 * Synchronous database interface — for SQLite (node:sqlite DatabaseSync).
 * exec/prepare/close are synchronous; transaction/query are async wrappers.
 */
export interface ISyncDatabase extends IDatabase {
  exec(sql: string): void
  prepare(sql: string): IStatement
  close(): void
  /** SQLite-specific PRAGMA accessor */
  pragma?(sql: string, options?: { simple?: boolean }): unknown
}

/**
 * Asynchronous database interface — for MySQL, PostgreSQL, TiDB.
 * All methods return Promises.
 */
export interface IAsyncDatabase extends IDatabase {
  exec(sql: string): Promise<void>
  prepare(sql: string): Promise<IStatement>
  close(): Promise<void>
}

/**
 * Factory for creating database connections.
 * Each dialect registers a provider via registerDatabaseProvider().
 */
export interface DatabaseProvider {
  readonly dialect: IDatabaseCapabilities['dialect']
  connect(config: DatabaseConfig): Promise<IDatabase>
}

export type { DatabaseConfig }
