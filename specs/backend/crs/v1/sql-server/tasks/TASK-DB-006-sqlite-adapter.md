# TASK-DB-006: Tạo `src/main/db/sqlite/sqlite-adapter.ts` + tests ✅ DONE

**Source:** SOL-DB-001 §4.3  
**Phase:** 1 | **Effort:** M (1.5–2 giờ) | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-001, TASK-DB-002, TASK-DB-003

---

## Objective

Tạo `SqliteAdapter` class implements `ISyncDatabase`, tự register provider, và toàn bộ test suite. Đây là implementation thay thế `SyncDatabase` hiện tại.

---

## Context cần đọc

- `src/main/sqlite/sync-database.ts` — xem API hiện tại để đảm bảo compat
- `src/main/db/types.ts` (TASK-DB-001)
- `src/main/db/provider.ts` (TASK-DB-002)
- SOL-DB-001 §4.3

---

## Files to create

### 1. `src/main/db/sqlite/sqlite-adapter.ts`

```typescript
/**
 * SQLite Database Adapter
 *
 * Implements ISyncDatabase using Node.js built-in node:sqlite module.
 * Auto-registers the 'sqlite' DatabaseProvider on import.
 *
 * @module db/sqlite/sqlite-adapter
 */

import { existsSync } from 'node:fs'
import { DatabaseSync } from 'node:sqlite'
import type { ISyncDatabase, IStatement, IDatabaseCapabilities, BindValue, StatementResult } from '../types'
import { registerDatabaseProvider } from '../provider'
import type { SqliteConfig } from '../config'

/** Wrapper that adapts node:sqlite StatementSync to IStatement */
class SqliteStatement implements IStatement {
  constructor(private readonly stmt: ReturnType<DatabaseSync['prepare']>) {}

  run(...params: BindValue[]): StatementResult {
    const result = this.stmt.run(...(params as any[]))
    return {
      changes: (result as any).changes ?? 0,
      lastInsertRowid: (result as any).lastInsertRowid ?? 0
    }
  }

  get(...params: BindValue[]): Record<string, unknown> | undefined {
    return this.stmt.get(...(params as any[])) as Record<string, unknown> | undefined
  }

  all(...params: BindValue[]): Record<string, unknown>[] {
    return this.stmt.all(...(params as any[])) as Record<string, unknown>[]
  }
}

/**
 * SqliteAdapter — ISyncDatabase implementation for Node.js server mode.
 * Uses node:sqlite (built-in since Node.js 22.5.0).
 */
export class SqliteAdapter implements ISyncDatabase {
  private readonly db: DatabaseSync

  readonly capabilities: IDatabaseCapabilities = {
    walMode: true,
    returning: false,
    nativeJson: false,
    placeholderStyle: 'positional',
    dialect: 'sqlite'
  }

  constructor(
    path: string,
    options: {
      readonly?: boolean
      fileMustExist?: boolean
      timeout?: number
    } = {}
  ) {
    if (options.fileMustExist && path !== ':memory:' && !existsSync(path)) {
      throw new Error(`SQLite database does not exist: "${path}"`)
    }

    this.db = new DatabaseSync(path, {
      readOnly: options.readonly ?? false,
      timeout: options.timeout
    } as any)
  }

  exec(sql: string): void {
    this.db.exec(sql)
  }

  prepare(sql: string): IStatement {
    return new SqliteStatement(this.db.prepare(sql))
  }

  pragma(sql: string, options?: { simple?: boolean }): unknown {
    const stmt = this.db.prepare(`PRAGMA ${sql}`)
    if (options?.simple) {
      const row = stmt.get() as Record<string, unknown> | undefined
      return row ? Object.values(row)[0] : undefined
    }
    return stmt.all()
  }

  close(): void {
    this.db.close()
  }

  async transaction<T>(fn: () => T | Promise<T>): Promise<T> {
    this.db.exec('BEGIN')
    try {
      const result = await fn()
      this.db.exec('COMMIT')
      return result
    } catch (err) {
      try { this.db.exec('ROLLBACK') } catch { /* ignore rollback errors */ }
      throw err
    }
  }

  async query(sql: string, params?: BindValue[]): Promise<Record<string, unknown>[]> {
    const stmt = this.prepare(sql) as SqliteStatement
    return stmt.all(...(params ?? []))
  }
}

// Auto-register SQLite provider when this module is imported
registerDatabaseProvider({
  dialect: 'sqlite',
  async connect(config) {
    const cfg = config as SqliteConfig
    return new SqliteAdapter(cfg.path, { readonly: cfg.readonly })
  }
})
```

### 2. `src/main/db/sqlite/__tests__/sqlite-adapter.test.ts`

```typescript
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
      expect(() => new SqliteAdapter('/nonexistent/path.db', { fileMustExist: true }))
        .toThrow('SQLite database does not exist')
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
      expect(() => db.exec('CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)')).not.toThrow()
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
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

pnpm vitest run src/main/db/sqlite/__tests__/sqlite-adapter.test.ts

# Verify provider auto-registration
node -e "
import('./src/main/db/sqlite/sqlite-adapter.js').then(() => {
  const { getRegisteredDialects } = require('./src/main/db/provider.js')
  console.log(getRegisteredDialects())  // should include 'sqlite'
})
"
```

Expected: 22/22 tests pass

---

## Done criteria

- [x] `src/main/db/sqlite/sqlite-adapter.ts` tồn tại
- [x] `SqliteAdapter` implements `ISyncDatabase` (TypeScript compile OK)
- [x] Auto-registers `sqlite` provider on import (side effect)
- [x] `transaction()` rollback khi fn throws
- [x] `pragma()` với `simple: true` trả scalar
- [x] `src/main/db/sqlite/__tests__/sqlite-adapter.test.ts` pass 22 tests
- [x] Không có `import 'electron'`
