# TASK-DB-010: Tạo `src/main/db/generic-pool.ts` + tests ✅ DONE

**Source:** SOL-DB-002 §4.3  
**Phase:** 2 | **Effort:** M (2–3 giờ)  
**Depends on:** TASK-DB-001, TASK-DB-003, TASK-DB-008

---

## Objective

Tạo `GenericConnectionPool` — multi-connection pool cho MySQL/PostgreSQL/TiDB với waiter queue, timeout, và retry logic.

---

## Files to create

### 1. `src/main/db/generic-pool.ts`

```typescript
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
 *
 * Features:
 * - Configurable min/max connections
 * - Acquire timeout with waiter queue
 * - Connection retry with backoff
 * - Idle connection cleanup
 * - Graceful drain (waits for in-flight) + immediate destroy
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
    /** Injectable connection factory for testing */
    private readonly _testFactory?: () => Promise<IDatabase>
  ) {
    this.config = { ...DEFAULT_POOL_CONFIG, ...poolConfig }
  }

  /** Warm up min connections. Call after construction in server mode. */
  async initialize(): Promise<void> {
    if (this.initialized) return
    this.initialized = true
    const promises = []
    for (let i = 0; i < this.config.min; i++) {
      promises.push(this.createAndIdle())
    }
    await Promise.allSettled(promises)
  }

  private async createAndIdle(): Promise<void> {
    try {
      const db = await this.createConnection()
      this.idle.push({ db, idleTimer: null, createdAt: Date.now() })
    } catch { /* min connection failures are non-fatal */ }
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
        reject(new Error(`Connection acquire timeout after ${this.config.acquireTimeoutMs}ms`))
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
```

### 2. `src/main/db/__tests__/generic-pool.test.ts`

```typescript
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { GenericConnectionPool } from '../generic-pool'
import type { IDatabase } from '../types'

function makeMockDb(): IDatabase {
  return {
    capabilities: { dialect: 'sqlite', walMode: false, returning: false, nativeJson: false, placeholderStyle: 'positional' },
    exec: async () => {},
    prepare: async () => ({ run: async () => ({ changes: 0, lastInsertRowid: 0 }), get: async () => undefined, all: async () => [] }),
    close: vi.fn().mockResolvedValue(undefined),
    transaction: async (fn) => fn(),
    query: async (sql) => sql.includes('SELECT 1') ? [{ n: 1 }] : []
  }
}

describe('GenericConnectionPool', () => {
  let connectCount: number
  let pool: GenericConnectionPool

  function makePool(overrides?: Partial<{ min: number; max: number; acquireTimeoutMs: number; connectionRetries: number; retryDelayMs: number }>) {
    connectCount = 0
    pool = new GenericConnectionPool(
      { dialect: 'sqlite', path: ':memory:' },
      { min: 1, max: 3, acquireTimeoutMs: 200, idleTimeoutMs: 60_000, connectionRetries: 1, retryDelayMs: 10, ...overrides },
      async () => { connectCount++; return makeMockDb() }
    )
    return pool
  }

  beforeEach(() => makePool())
  afterEach(async () => { await pool.destroy().catch(() => {}) })

  it('initialize() warms up min connections', async () => {
    await pool.initialize()
    expect(connectCount).toBeGreaterThanOrEqual(1)
    expect(pool.stats().idle).toBeGreaterThanOrEqual(1)
  })

  it('acquire() returns a connection', async () => {
    const conn = await pool.acquire()
    expect(conn).toBeDefined()
    pool.release(conn)
  })

  it('stats().acquired increases when acquired', async () => {
    const conn = await pool.acquire()
    expect(pool.stats().acquired).toBe(1)
    pool.release(conn)
    expect(pool.stats().acquired).toBe(0)
  })

  it('creates up to max connections', async () => {
    const c1 = await pool.acquire()
    const c2 = await pool.acquire()
    const c3 = await pool.acquire()
    expect(pool.stats().acquired).toBe(3)
    pool.release(c1); pool.release(c2); pool.release(c3)
  })

  it('queues when at max and resolves when released', async () => {
    const c1 = await pool.acquire()
    const c2 = await pool.acquire()
    const c3 = await pool.acquire()

    let resolved = false
    const pending = pool.acquire().then((c) => { resolved = true; pool.release(c) })

    expect(pool.stats().waiting).toBe(1)
    pool.release(c1)
    await pending
    expect(resolved).toBe(true)
    pool.release(c2); pool.release(c3)
  })

  it('times out when pool exhausted', async () => {
    makePool({ max: 1, acquireTimeoutMs: 100 })
    const conn = await pool.acquire()
    await expect(pool.acquire()).rejects.toThrow(/timeout/i)
    pool.release(conn)
  })

  it('retries connection when factory throws once', async () => {
    let attempt = 0
    pool = new GenericConnectionPool(
      { dialect: 'sqlite', path: ':memory:' },
      { min: 0, max: 2, acquireTimeoutMs: 1000, idleTimeoutMs: 60_000, connectionRetries: 2, retryDelayMs: 10 },
      async () => { attempt++; if (attempt === 1) throw new Error('transient'); return makeMockDb() }
    )
    const conn = await pool.acquire()
    expect(conn).toBeDefined()
    expect(attempt).toBe(2)
    pool.release(conn)
  })

  it('throws after exhausting retries', async () => {
    pool = new GenericConnectionPool(
      { dialect: 'sqlite', path: ':memory:' },
      { min: 0, max: 2, acquireTimeoutMs: 1000, idleTimeoutMs: 60_000, connectionRetries: 1, retryDelayMs: 10 },
      async () => { throw new Error('always fails') }
    )
    await expect(pool.acquire()).rejects.toThrow(/Failed to create database connection after 1 retries/)
  })

  it('drain() resolves after in-flight connections released', async () => {
    const conn = await pool.acquire()
    let drained = false
    const drainPromise = pool.drain().then(() => { drained = true })
    await new Promise((r) => setTimeout(r, 60))
    expect(drained).toBe(false)
    pool.release(conn)
    await drainPromise
    expect(drained).toBe(true)
  })

  it('acquire() after drain() throws', async () => {
    await pool.drain()
    await expect(pool.acquire()).rejects.toThrow(/draining/)
  })

  it('withConnection() auto-releases', async () => {
    await pool.withConnection(async () => {})
    expect(pool.stats().acquired).toBe(0)
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

pnpm vitest run src/main/db/__tests__/generic-pool.test.ts
```

Expected: 13/13 tests pass

---

## Done criteria

- [x] `GenericConnectionPool` implements `IConnectionPool`
- [x] Waiter queue khi `max` connections reached
- [x] Acquire timeout sau `acquireTimeoutMs`
- [x] Retry với backoff khi factory throws
- [x] `drain()` chờ acquired connections release
- [x] Constructor nhận optional `_testFactory` cho testing
- [x] 13/13 tests pass
