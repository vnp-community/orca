/**
 * Connection Pool Interface
 *
 * Abstracts connection lifecycle for both single-connection (SQLite)
 * and multi-connection (MySQL, PostgreSQL) pools.
 *
 * @module db/pool
 */

import type { IDatabase } from './types'

/** Real-time statistics for a connection pool */
export type PoolStats = {
  /** Total connections managed (idle + acquired) */
  total: number
  /** Connections currently idle (available) */
  idle: number
  /** Connections currently in use (acquired) */
  acquired: number
  /** Requests waiting for an available connection */
  waiting: number
}

/** Configuration for connection pools (used by GenericConnectionPool) */
export type PoolConfig = {
  /** Minimum connections to keep alive */
  min: number
  /** Maximum connections allowed */
  max: number
  /** Milliseconds to wait for a connection before throwing timeout */
  acquireTimeoutMs: number
  /** Milliseconds before an idle connection is destroyed */
  idleTimeoutMs: number
  /** Number of retry attempts when creating a connection fails */
  connectionRetries: number
  /** Milliseconds to wait between retry attempts */
  retryDelayMs: number
}

/** Default pool configuration */
export const DEFAULT_POOL_CONFIG: PoolConfig = {
  min: 2,
  max: 10,
  acquireTimeoutMs: 5_000,
  idleTimeoutMs: 30_000,
  connectionRetries: 3,
  retryDelayMs: 500
}

/**
 * Connection pool contract.
 * Both single-connection (SQLite) and multi-connection pools implement this.
 */
export type IConnectionPool = {
  /**
   * Acquire a connection from the pool.
   * @throws Error if pool is draining or acquire times out.
   */
  acquire(): Promise<IDatabase>

  /**
   * Release a connection back to the pool.
   * Silently ignored if connection is not from this pool.
   */
  release(conn: IDatabase): void

  /**
   * Acquire a connection, run fn, then release. Auto-releases on error too.
   */
  withConnection<T>(fn: (db: IDatabase) => Promise<T>): Promise<T>

  /**
   * Acquire a connection, run fn inside a transaction, then release.
   * Rolls back automatically on error.
   */
  withTransaction<T>(fn: (db: IDatabase) => Promise<T>): Promise<T>

  /** Get current pool statistics */
  stats(): PoolStats

  /**
   * Graceful shutdown — waits for in-flight connections, then closes all.
   * After drain(), acquire() will throw.
   */
  drain(): Promise<void>

  /**
   * Immediate shutdown — closes all connections without waiting.
   * Use for error recovery or test teardown.
   */
  destroy(): Promise<void>
}
