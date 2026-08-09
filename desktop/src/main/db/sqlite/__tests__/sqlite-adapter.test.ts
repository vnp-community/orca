import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { SqliteAdapter } from '../sqlite-adapter'

describe('SqliteAdapter', () => {
  let tmpDir: string
  let dbPath: string

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'orca-sqlite-test-'))
    dbPath = join(tmpDir, 'test.db')
  })

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true })
  })

  describe('constructor', () => {
    it('creates new database file when path does not exist', () => {
      const db = new SqliteAdapter(dbPath)
      expect(db).toBeDefined()
      db.close()
    })

    it('opens in-memory database with :memory:', () => {
      const db = new SqliteAdapter(':memory:')
      db.exec('CREATE TABLE t (id INTEGER)')
      const rows = db.prepare('SELECT * FROM t').all()
      expect(rows).toEqual([])
      db.close()
    })

    it('throws when fileMustExist=true and file missing', () => {
      expect(() => new SqliteAdapter('/nonexistent/path.db', { fileMustExist: true })).toThrow(
        'SQLite database does not exist'
      )
    })

    it('opens existing file with fileMustExist=true', () => {
      const creator = new SqliteAdapter(dbPath)
      creator.exec('CREATE TABLE t (id INTEGER)')
      creator.close()

      const db = new SqliteAdapter(dbPath, { fileMustExist: true })
      expect(db).toBeDefined()
      db.close()
    })
  })

  describe('capabilities', () => {
    it('reports dialect as sqlite', () => {
      const db = new SqliteAdapter(':memory:')
      expect(db.capabilities.dialect).toBe('sqlite')
      db.close()
    })

    it('reports placeholderStyle as positional', () => {
      const db = new SqliteAdapter(':memory:')
      expect(db.capabilities.placeholderStyle).toBe('positional')
      db.close()
    })
  })

  describe('exec', () => {
    it('creates table without error', () => {
      const db = new SqliteAdapter(':memory:')
      expect(() =>
        db.exec('CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)')
      ).not.toThrow()
      db.close()
    })

    it('throws on invalid SQL', () => {
      const db = new SqliteAdapter(':memory:')
      expect(() => db.exec('INVALID SQL')).toThrow()
      db.close()
    })
  })

  describe('prepare + IStatement', () => {
    let db: SqliteAdapter

    beforeEach(() => {
      db = new SqliteAdapter(':memory:')
      db.exec('CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, val INTEGER)')
      db.exec("INSERT INTO items VALUES (1, 'alpha', 10)")
      db.exec("INSERT INTO items VALUES (2, 'beta', 20)")
    })

    afterEach(() => db.close())

    it('all() returns all rows', () => {
      const rows = db.prepare('SELECT * FROM items ORDER BY id').all()
      expect(rows).toHaveLength(2)
      expect(rows[0]).toMatchObject({ id: 1, name: 'alpha', val: 10 })
    })

    it('get() returns first matching row', () => {
      const row = db.prepare('SELECT * FROM items WHERE id = ?').get(1)
      expect(row).toMatchObject({ id: 1, name: 'alpha' })
    })

    it('get() returns undefined for no match', () => {
      expect(db.prepare('SELECT * FROM items WHERE id = ?').get(999)).toBeUndefined()
    })

    it('run() returns changes count', () => {
      const result = db.prepare('UPDATE items SET val = ? WHERE id = ?').run(99, 1)
      expect(result.changes).toBe(1)
    })

    it('all() with params filters results', () => {
      const rows = db.prepare('SELECT name FROM items WHERE val > ?').all(15)
      expect(rows).toHaveLength(1)
      expect(rows[0]).toMatchObject({ name: 'beta' })
    })
  })

  describe('pragma', () => {
    it('pragma() returns array by default', () => {
      const db = new SqliteAdapter(':memory:')
      const result = db.pragma('journal_mode')
      expect(Array.isArray(result)).toBe(true)
      db.close()
    })

    it('pragma(simple=true) returns scalar', () => {
      const db = new SqliteAdapter(':memory:')
      const mode = db.pragma('journal_mode', { simple: true })
      expect(typeof mode).toBe('string')
      db.close()
    })

    it('user_version pragma returns 0 by default', () => {
      const db = new SqliteAdapter(':memory:')
      const ver = db.pragma('user_version', { simple: true })
      expect(ver).toBe(0)
      db.close()
    })
  })

  describe('transaction', () => {
    it('commits on success', async () => {
      const db = new SqliteAdapter(':memory:')
      db.exec('CREATE TABLE t (id INTEGER)')
      await db.transaction(async () => {
        db.exec('INSERT INTO t VALUES (1)')
        db.exec('INSERT INTO t VALUES (2)')
      })
      expect(db.prepare('SELECT * FROM t').all()).toHaveLength(2)
      db.close()
    })

    it('rolls back when fn throws', async () => {
      const db = new SqliteAdapter(':memory:')
      db.exec('CREATE TABLE t (id INTEGER)')
      await expect(
        db.transaction(async () => {
          db.exec('INSERT INTO t VALUES (1)')
          throw new Error('forced rollback')
        })
      ).rejects.toThrow('forced rollback')
      expect(db.prepare('SELECT * FROM t').all()).toHaveLength(0)
      db.close()
    })
  })

  describe('query', () => {
    it('returns array of row objects', async () => {
      const db = new SqliteAdapter(':memory:')
      db.exec('CREATE TABLE t (id INTEGER, name TEXT)')
      db.exec("INSERT INTO t VALUES (1, 'foo')")
      const rows = await db.query('SELECT * FROM t')
      expect(rows).toEqual([{ id: 1, name: 'foo' }])
      db.close()
    })

    it('accepts params', async () => {
      const db = new SqliteAdapter(':memory:')
      db.exec('CREATE TABLE t (id INTEGER)')
      db.exec('INSERT INTO t VALUES (42)')
      const rows = await db.query('SELECT id FROM t WHERE id = ?', [42])
      expect(rows).toHaveLength(1)
      db.close()
    })

    it('returns empty array for no results', async () => {
      const db = new SqliteAdapter(':memory:')
      db.exec('CREATE TABLE t (id INTEGER)')
      expect(await db.query('SELECT * FROM t')).toEqual([])
      db.close()
    })
  })

  describe('close', () => {
    it('does not throw', () => {
      const db = new SqliteAdapter(':memory:')
      expect(() => db.close()).not.toThrow()
    })
  })
})
