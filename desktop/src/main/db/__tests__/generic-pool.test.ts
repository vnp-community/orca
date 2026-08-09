import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { GenericConnectionPool } from '../generic-pool'
import type { IDatabase } from '../types'

function makeMockDb(): IDatabase {
  return {
    capabilities: {
      dialect: 'sqlite',
      walMode: false,
      returning: false,
      nativeJson: false,
      placeholderStyle: 'positional'
    },
    exec: async () => {},
    prepare: async () => ({
      run: async () => ({ changes: 0, lastInsertRowid: 0 }),
      get: async () => undefined,
      all: async () => []
    }),
    close: vi.fn().mockResolvedValue(undefined),
    transaction: async (fn) => fn(),
    query: async (sql) => (sql.includes('SELECT 1') ? [{ n: 1 }] : [])
  }
}

describe('GenericConnectionPool', () => {
  let connectCount: number
  let pool: GenericConnectionPool

  function makePool(
    overrides?: Partial<{
      min: number
      max: number
      acquireTimeoutMs: number
      connectionRetries: number
      retryDelayMs: number
      idleTimeoutMs: number
    }>
  ) {
    connectCount = 0
    pool = new GenericConnectionPool(
      { dialect: 'sqlite', path: ':memory:', readonly: false },
      {
        min: 1,
        max: 3,
        acquireTimeoutMs: 200,
        idleTimeoutMs: 60_000,
        connectionRetries: 1,
        retryDelayMs: 10,
        ...overrides
      },
      async () => {
        connectCount++
        return makeMockDb()
      }
    )
    return pool
  }

  beforeEach(() => makePool())
  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

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
    pool.release(c1)
    pool.release(c2)
    pool.release(c3)
  })

  it('queues when at max and resolves when released', async () => {
    const c1 = await pool.acquire()
    const c2 = await pool.acquire()
    const c3 = await pool.acquire()

    let resolved = false
    const pending = pool.acquire().then((c) => {
      resolved = true
      pool.release(c)
    })

    expect(pool.stats().waiting).toBe(1)
    pool.release(c1)
    await pending
    expect(resolved).toBe(true)
    pool.release(c2)
    pool.release(c3)
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
      { dialect: 'sqlite', path: ':memory:', readonly: false },
      {
        min: 0,
        max: 2,
        acquireTimeoutMs: 1000,
        idleTimeoutMs: 60_000,
        connectionRetries: 2,
        retryDelayMs: 10
      },
      async () => {
        attempt++
        if (attempt === 1) {throw new Error('transient')}
        return makeMockDb()
      }
    )
    const conn = await pool.acquire()
    expect(conn).toBeDefined()
    expect(attempt).toBe(2)
    pool.release(conn)
  })

  it('throws after exhausting retries', async () => {
    pool = new GenericConnectionPool(
      { dialect: 'sqlite', path: ':memory:', readonly: false },
      {
        min: 0,
        max: 2,
        acquireTimeoutMs: 1000,
        idleTimeoutMs: 60_000,
        connectionRetries: 1,
        retryDelayMs: 10
      },
      async () => {
        throw new Error('always fails')
      }
    )
    await expect(pool.acquire()).rejects.toThrow(
      /Failed to create database connection after 1 retries/
    )
  })

  it('drain() resolves after in-flight connections released', async () => {
    const conn = await pool.acquire()
    let drained = false
    const drainPromise = pool.drain().then(() => {
      drained = true
    })
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
