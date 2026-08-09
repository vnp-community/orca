/**
 * SQLite Single-Connection Pool
 *
 * SQLite doesn't need a real connection pool — it uses a single connection
 * with WAL mode for concurrent reads. This implements IConnectionPool
 * with a single underlying connection for interface compatibility.
 *
 * @module db/sqlite/sqlite-pool
 */

import type { IConnectionPool, PoolStats } from '../pool'
import type { IDatabase } from '../types'
import { SqliteAdapter } from './sqlite-adapter'

export class SqliteSingleConnectionPool implements IConnectionPool {
  private readonly db: SqliteAdapter
  private inUse = false
  private draining = false
  private waiters: Array<() => void> = []

  constructor(path: string, options?: { readonly?: boolean }) {
    this.db = new SqliteAdapter(path, options)
  }

  async acquire(): Promise<IDatabase> {
    if (this.draining) {
      throw new Error('Connection pool is draining — cannot acquire new connections')
    }
    this.inUse = true
    return this.db
  }

  release(_conn: IDatabase): void {
    this.inUse = false
    const waiter = this.waiters.shift()
    if (waiter) waiter()
  }

  async withConnection<T>(fn: (db: IDatabase) => Promise<T>): Promise<T> {
    const conn = await this.acquire()
    try {
      return await fn(conn)
    } finally {
      this.release(conn)
    }
  }

  async withTransaction<T>(fn: (db: IDatabase) => Promise<T>): Promise<T> {
    return this.withConnection((db) => db.transaction(() => fn(db)))
  }

  stats(): PoolStats {
    return {
      total: 1,
      idle: this.inUse ? 0 : 1,
      acquired: this.inUse ? 1 : 0,
      waiting: this.waiters.length
    }
  }

  async drain(): Promise<void> {
    this.draining = true
    try {
      this.db.close()
    } catch {
      // ignore
    }
  }

  async destroy(): Promise<void> {
    this.draining = true
    try {
      this.db.close()
    } catch {
      // ignore
    }
  }
}
