import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../sqlite-pool'

describe('SqliteSingleConnectionPool', () => {
  let pool: SqliteSingleConnectionPool

  beforeEach(() => {
    pool = new SqliteSingleConnectionPool(':memory:')
  })

  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

  it('acquire() returns IDatabase', async () => {
    const conn = await pool.acquire()
    expect(conn).toBeDefined()
    expect(typeof conn.query).toBe('function')
    pool.release(conn)
  })

  it('release() does not throw', async () => {
    const conn = await pool.acquire()
    expect(() => pool.release(conn)).not.toThrow()
  })

  it('stats().total is always 1', () => {
    expect(pool.stats().total).toBe(1)
  })

  it('stats().idle is 1 when not acquired', () => {
    expect(pool.stats().idle).toBe(1)
    expect(pool.stats().acquired).toBe(0)
  })

  it('stats().acquired is 1 when in use', async () => {
    const conn = await pool.acquire()
    expect(pool.stats().acquired).toBe(1)
    expect(pool.stats().idle).toBe(0)
    pool.release(conn)
  })

  it('withConnection() executes query and releases', async () => {
    const rows = await pool.withConnection((db) => db.query('SELECT 1 AS n'))
    expect(rows).toHaveLength(1)
    expect(pool.stats().acquired).toBe(0)
  })

  it('withConnection() releases on error', async () => {
    await expect(
      pool.withConnection(async () => {
        throw new Error('test error')
      })
    ).rejects.toThrow('test error')
    expect(pool.stats().acquired).toBe(0)
  })

  it('withTransaction() commits on success', async () => {
    await pool.withConnection((db) => db.exec('CREATE TABLE tx_test (id INTEGER)') as Promise<void>)
    await pool.withTransaction(async (db) => {
      await db.query('INSERT INTO tx_test VALUES (1)')
    })
    const rows = await pool.withConnection((db) => db.query('SELECT * FROM tx_test'))
    expect(rows).toHaveLength(1)
  })

  it('withTransaction() rolls back on error', async () => {
    await pool.withConnection((db) => db.exec('CREATE TABLE tx_rb (id INTEGER)') as Promise<void>)
    await expect(
      pool.withTransaction(async (db) => {
        await db.query('INSERT INTO tx_rb VALUES (42)')
        throw new Error('forced rollback')
      })
    ).rejects.toThrow('forced rollback')
    const rows = await pool.withConnection((db) => db.query('SELECT * FROM tx_rb'))
    expect(rows).toHaveLength(0)
  })

  it('drain() resolves without error', async () => {
    await expect(pool.drain()).resolves.toBeUndefined()
  })

  it('acquire() after drain() throws', async () => {
    await pool.drain()
    await expect(pool.acquire()).rejects.toThrow(/draining/)
  })

  it('always returns the same connection object', async () => {
    const conn1 = await pool.acquire()
    pool.release(conn1)
    const conn2 = await pool.acquire()
    expect(conn1).toBe(conn2)
    pool.release(conn2)
  })
})
