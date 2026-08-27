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
export type StatementResult = {
  changes: number
  lastInsertRowid: number | bigint
}

/** A prepared SQL statement (sync or async depending on backend) */
export type IStatement = {
  run(...params: BindValue[]): StatementResult | Promise<StatementResult>
  get(
    ...params: BindValue[]
  ): Record<string, unknown> | undefined | Promise<Record<string, unknown> | undefined>
  all(...params: BindValue[]): Record<string, unknown>[] | Promise<Record<string, unknown>[]>
  iterate?(...params: BindValue[]): IterableIterator<Record<string, unknown>>
}

/** Capabilities advertised by a database adapter */
export type IDatabaseCapabilities = {
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
export type IDatabase = {
  exec(sql: string): void | Promise<void>
  prepare(sql: string): IStatement | Promise<IStatement>
  close(): void | Promise<void>
  readonly capabilities: IDatabaseCapabilities
  transaction<T>(fn: () => T | Promise<T>): Promise<T>
  /**
   * Generic defaults to the pre-existing untyped-row shape, so every call
   * site that never wrote `query<T>(...)` keeps compiling and behaving
   * identically — this widens the signature, it does not change it. Callers
   * that DO write `query<SomeRowShape>(...)` (11 files as of ADR-021 Phase 1
   * — ProfileService, TaskService, WorkflowOrchestrator, AIProviderService,
   * TeamService, annotation-store.ts, and the new Pg*Store repositories under
   * ADR-021) were relying on a generic parameter this signature never
   * actually declared; TS silently rejected all of them (TS2558 "Expected 0
   * type arguments, but got 1"). No adapter's *implementation* changes here —
   * `T` is compile-time only, erased at runtime, so this is a type-only fix.
   */
  query<T = Record<string, unknown>>(sql: string, params?: BindValue[]): Promise<T[]>
}

/**
 * Synchronous database interface — for SQLite (node:sqlite DatabaseSync).
 * exec/prepare/close are synchronous; transaction/query are async wrappers.
 */
export type ISyncDatabase = {
  exec(sql: string): void
  prepare(sql: string): IStatement
  close(): void
  /** SQLite-specific PRAGMA accessor */
  pragma?(sql: string, options?: { simple?: boolean }): unknown
} & IDatabase

/**
 * Asynchronous database interface — for MySQL, PostgreSQL, TiDB.
 * All methods return Promises.
 */
export type IAsyncDatabase = {
  exec(sql: string): Promise<void>
  prepare(sql: string): Promise<IStatement>
  close(): Promise<void>
} & IDatabase

/**
 * Factory for creating database connections.
 * Each dialect registers a provider via registerDatabaseProvider().
 */
export type DatabaseProvider = {
  readonly dialect: IDatabaseCapabilities['dialect']
  connect(config: DatabaseConfig): Promise<IDatabase>
}

export type { DatabaseConfig }
