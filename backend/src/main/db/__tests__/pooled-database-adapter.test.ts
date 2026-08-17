import { describe, it, expect, vi, beforeEach } from 'vitest'
import { PooledDatabaseAdapter } from '../pooled-database-adapter'
import type { IConnectionPool } from '../pool'
import type { IDatabase, IDatabaseCapabilities } from '../types'

const FAKE_CAPS: IDatabaseCapabilities = {
  walMode: false,
  returning: true,
  nativeJson: true,
  placeholderStyle: 'positional',
  dialect: 'postgresql'
}

function makeFakePool(): { pool: IConnectionPool; acquireCount: number; releaseCount: number } {
  let acquireCount = 0
  let releaseCount = 0

  const fakeConn: IDatabase = {
    capabilities: FAKE_CAPS,
    exec: vi.fn().mockResolvedValue(undefined),
    close: vi.fn().mockResolvedValue(undefined),
    transaction: vi.fn(),
    query: vi.fn().mockResolvedValue([{ id: 'row-from-query' }]),
    prepare: vi.fn().mockResolvedValue({
      run: vi.fn().mockResolvedValue({ changes: 1, lastInsertRowid: 1 }),
      get: vi.fn().mockResolvedValue({ id: 'row-from-get' }),
      all: vi.fn().mockResolvedValue([{ id: 'row-from-all' }])
    })
  }

  const pool: IConnectionPool = {
    acquire: vi.fn(),
    release: vi.fn(),
    async withConnection<T>(fn: (db: IDatabase) => Promise<T>): Promise<T> {
      acquireCount++
      try {
        return await fn(fakeConn)
      } finally {
        releaseCount++
      }
    },
    async withTransaction<T>(fn: (db: IDatabase) => Promise<T>): Promise<T> {
      return fn(fakeConn)
    },
    stats: vi.fn().mockReturnValue({ total: 1, idle: 1, acquired: 0, waiting: 0 }),
    drain: vi.fn().mockResolvedValue(undefined),
    destroy: vi.fn().mockResolvedValue(undefined)
  }

  return {
    pool,
    get acquireCount() { return acquireCount },
    get releaseCount() { return releaseCount }
  } as unknown as { pool: IConnectionPool; acquireCount: number; releaseCount: number }
}

describe('PooledDatabaseAdapter', () => {
  let fake: ReturnType<typeof makeFakePool>

  beforeEach(() => {
    fake = makeFakePool()
  })

  it('create() reads capabilities from one acquired connection', async () => {
    const adapter = await PooledDatabaseAdapter.create(fake.pool)
    expect(adapter.capabilities).toEqual(FAKE_CAPS)
  })

  it('prepare().get() acquires a connection lazily — not at prepare() time', async () => {
    const adapter = await PooledDatabaseAdapter.create(fake.pool)
    const acquiresAfterPrepare = fake.acquireCount
    const stmt = adapter.prepare('SELECT * FROM orca_users WHERE id = ?')
    expect(fake.acquireCount).toBe(acquiresAfterPrepare) // prepare() itself touches nothing

    const row = await stmt.get('u1')
    expect(row).toEqual({ id: 'row-from-get' })
    expect(fake.acquireCount).toBe(acquiresAfterPrepare + 1)
    expect(fake.releaseCount).toBe(fake.acquireCount) // every acquire was released
  })

  it('prepare().run() and prepare().all() each acquire+release their own connection', async () => {
    const adapter = await PooledDatabaseAdapter.create(fake.pool)
    const stmt = adapter.prepare('UPDATE orca_users SET name = ? WHERE id = ?')
    await stmt.run('New Name', 'u1')
    const stmt2 = adapter.prepare('SELECT * FROM orca_users')
    await stmt2.all()

    expect(fake.acquireCount).toBe(3) // create()'s capabilities read + run() + all()
    expect(fake.releaseCount).toBe(3)
  })

  it('query() delegates through the pool', async () => {
    const adapter = await PooledDatabaseAdapter.create(fake.pool)
    const rows = await adapter.query('SELECT 1')
    expect(rows).toEqual([{ id: 'row-from-query' }])
  })

  it('exec() delegates through the pool', async () => {
    const adapter = await PooledDatabaseAdapter.create(fake.pool)
    await adapter.exec('CREATE TABLE foo (id TEXT)')
    expect(fake.acquireCount).toBeGreaterThan(0)
  })

  it('close() is a no-op — does not drain/destroy the shared pool', async () => {
    const adapter = await PooledDatabaseAdapter.create(fake.pool)
    await adapter.close()
    expect(fake.pool.drain).not.toHaveBeenCalled()
    expect(fake.pool.destroy).not.toHaveBeenCalled()
  })

  it('transaction() throws instead of silently running statements on different connections', async () => {
    const adapter = await PooledDatabaseAdapter.create(fake.pool)
    await expect(adapter.transaction(() => 1)).rejects.toThrow(/not supported/)
  })
})
