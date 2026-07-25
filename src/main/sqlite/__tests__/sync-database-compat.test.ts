/**
 * Backward compatibility test for sync-database.ts.
 * Verifies that existing import patterns still work.
 * NOTE: sync-database.ts is kept as-is (no modification needed).
 * SqliteAdapter is the new implementation for server mode.
 */
import { describe, it, expect } from 'vitest'
import SyncDatabase from '../sync-database'

describe('SyncDatabase backward compat', () => {
  it('SyncDatabase is a constructor (class)', () => {
    expect(typeof SyncDatabase).toBe('function')
  })

  it('new SyncDatabase(":memory:") creates database', () => {
    const db = new SyncDatabase(':memory:')
    expect(db).toBeDefined()
    db.close()
  })

  it('db.exec() works', () => {
    const db = new SyncDatabase(':memory:')
    expect(() => db.exec('CREATE TABLE t (id INTEGER)')).not.toThrow()
    db.close()
  })

  it('db.prepare() returns StatementSync', () => {
    const db = new SyncDatabase(':memory:')
    db.exec('CREATE TABLE t (id INTEGER)')
    const stmt = db.prepare('SELECT * FROM t')
    expect(typeof stmt.all).toBe('function')
    expect(stmt.all()).toEqual([])
    db.close()
  })

  it('db.pragma() is callable', () => {
    const db = new SyncDatabase(':memory:')
    expect(() => db.pragma('journal_mode')).not.toThrow()
    db.close()
  })

  it('db.close() works', () => {
    const db = new SyncDatabase(':memory:')
    expect(() => db.close()).not.toThrow()
  })

  it('INSERT + SELECT roundtrip', () => {
    const db = new SyncDatabase(':memory:')
    db.exec('CREATE TABLE t (id INTEGER, val TEXT)')
    db.prepare('INSERT INTO t VALUES (?, ?)').run(1, 'hello')
    const row = db.prepare('SELECT * FROM t WHERE id = ?').get(1) as
      | Record<string, unknown>
      | undefined
    expect(row).toMatchObject({ id: 1, val: 'hello' })
    db.close()
  })
})
