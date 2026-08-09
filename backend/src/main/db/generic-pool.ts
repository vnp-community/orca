/**
 * Generic Multi-Connection Pool
 *
 * Connection pool for network databases (MySQL, PostgreSQL, TiDB).
 * Features: configurable min/max, waiter queue with timeout, retry with backoff,
 * idle cleanup, graceful drain + immediate destroy.
 *
 * @module db/generic-pool
 */

import type { IConnectionPool, PoolStats, PoolConfig } from './pool'
import type { IDatabase, DatabaseConfig } from './types'
import { DEFAULT_POOL_CONFIG } from './pool'
import { createDatabase } from './provider'

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

interface PooledConnection {
  db: IDatabase
  idleTimer: ReturnType<typeof setTimeout> | null
  createdAt: number
}

interface Waiter {
  resolve: (conn: IDatabase) => void
  reject: (err: Error) => void
  timer: ReturnType<typeof setTimeout>
}

/**
 * Generic multi-connection pool for network databases (MySQL, PostgreSQL, TiDB).
 */
export class GenericConnectionPool implements IConnectionPool {
  private readonly config: PoolConfig
  private idle: PooledConnection[] = []
  private acquired = new Set<IDatabase>()
  private waiters: Waiter[] = []
  private draining = false
  private initialized = false

  constructor(
    private readonly dbConfig: DatabaseConfig,
    poolConfig: Partial<PoolConfig> = {},
    /** Injectable connection factory — for testing without real DB */
    private readonly _testFactory?: () => Promise<IDatabase>
  ) {
    this.config = { ...DEFAULT_POOL_CONFIG, ...poolConfig }
  }

  /** Warm up min connections. Call after construction in server mode. */
  async initialize(): Promise<void> {
    if (this.initialized) return
    this.initialized = true
    const promises: Promise<void>[] = []
    for (let i = 0; i < this.config.min; i++) {
      promises.push(this.createAndIdle())
    }
    await Promise.allSettled(promises)
  }

  private async createAndIdle(): Promise<void> {
    try {
      const db = await this.createConnection()
      this.idle.push({ db, idleTimer: null, createdAt: Date.now() })
    } catch {
      // min connection failures are non-fatal
    }
  }

  private async createConnection(): Promise<IDatabase> {
    const factory = this._testFactory ?? (() => createDatabase(this.dbConfig))
    let lastErr: Error | null = null

    for (let attempt = 0; attempt <= this.config.connectionRetries; attempt++) {
      try {
        return await factory()
      } catch (err) {
        lastErr = err as Error
        if (attempt < this.config.connectionRetries) {
          await delay(this.config.retryDelayMs * (attempt + 1))
        }
      }
    }

    throw new Error(
      `Failed to create database connection after ${this.config.connectionRetries} retries: ${lastErr?.message}`
    )
  }

  async acquire(): Promise<IDatabase> {
    if (this.draining) throw new Error('Connection pool is draining')

    // Return idle connection if available
    const pooled = this.idle.pop()
    if (pooled) {
      if (pooled.idleTimer) clearTimeout(pooled.idleTimer)
      this.acquired.add(pooled.db)
      return pooled.db
    }

    // Create new connection if under max
    const totalConns = this.idle.length + this.acquired.size
    if (totalConns < this.config.max) {
      const db = await this.createConnection()
      this.acquired.add(db)
      return db
    }

    // Queue as waiter with timeout
    return new Promise<IDatabase>((resolve, reject) => {
      const timer = setTimeout(() => {
        const idx = this.waiters.findIndex((w) => w.timer === timer)
        if (idx !== -1) this.waiters.splice(idx, 1)
        reject(
          new Error(
            `Connection acquire timeout after ${this.config.acquireTimeoutMs}ms`
          )
        )
      }, this.config.acquireTimeoutMs)

      this.waiters.push({ resolve, reject, timer })
    })
  }

  release(conn: IDatabase): void {
    if (!this.acquired.has(conn)) return
    this.acquired.delete(conn)

    // Give to next waiter if any
    const waiter = this.waiters.shift()
    if (waiter) {
      clearTimeout(waiter.timer)
      this.acquired.add(conn)
      waiter.resolve(conn)
      return
    }

    // Put back to idle with idle timer
    if (!this.draining) {
      const idleTimer = setTimeout(() => {
        this.idle = this.idle.filter((p) => p.db !== conn)
        conn.close().catch(() => {})
      }, this.config.idleTimeoutMs)

      this.idle.push({ db: conn, idleTimer, createdAt: Date.now() })
    } else {
      conn.close().catch(() => {})
    }
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
      total: this.idle.length + this.acquired.size,
      idle: this.idle.length,
      acquired: this.acquired.size,
      waiting: this.waiters.length
    }
  }

  async drain(): Promise<void> {
    this.draining = true

    // Reject all waiters
    for (const waiter of this.waiters) {
      clearTimeout(waiter.timer)
      waiter.reject(new Error('Connection pool is draining'))
    }
    this.waiters = []

    // Wait for in-flight connections to be released
    while (this.acquired.size > 0) {
      await delay(50)
    }

    // Close all idle connections
    for (const pooled of this.idle) {
      if (pooled.idleTimer) clearTimeout(pooled.idleTimer)
      await pooled.db.close().catch(() => {})
    }
    this.idle = []
  }

  async destroy(): Promise<void> {
    this.draining = true

    for (const waiter of this.waiters) {
      clearTimeout(waiter.timer)
      waiter.reject(new Error('Connection pool destroyed'))
    }
    this.waiters = []

    for (const pooled of this.idle) {
      if (pooled.idleTimer) clearTimeout(pooled.idleTimer)
      await pooled.db.close().catch(() => {})
    }
    this.idle = []

    for (const conn of this.acquired) {
      await conn.close().catch(() => {})
    }
    this.acquired.clear()
  }
}
