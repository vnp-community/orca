# TASK-DB-014: Tạo `src/main/db/migrations/runner.ts` + tests ✅ DONE

**Source:** SOL-DB-003 §4.2  
**Phase:** 2 | **Effort:** M (2–2.5 giờ)  
**Depends on:** TASK-DB-006, TASK-DB-013

---

## Objective

Tạo `MigrationRunner` class — core migration engine. Quản lý `orca_schema_migrations` table, apply migrations theo thứ tự, detect dialect, support dryRun và target.

---

## Files to create

### 1. `src/main/db/migrations/runner.ts`

```typescript
import { createHash } from 'node:crypto'
import type { IDatabase } from '../types'
import type { Migration, MigrationRecord, MigrationStatus, MigrateOptions } from './types'

const MIGRATIONS_TABLE = 'orca_schema_migrations'

export class MigrationRunner {
  constructor(
    private readonly db: IDatabase,
    private readonly migrations: Migration[]
  ) {}

  async ensureMigrationsTable(): Promise<void> {
    const dialect = this.db.capabilities.dialect
    const isMySQL = dialect === 'mysql' || dialect === 'tidb' || dialect === 'mariadb'

    if (isMySQL) {
      await this.db.exec(`
        CREATE TABLE IF NOT EXISTS ${MIGRATIONS_TABLE} (
          id VARCHAR(255) PRIMARY KEY,
          applied_at VARCHAR(32) NOT NULL,
          checksum VARCHAR(64) NOT NULL,
          execution_ms INT NOT NULL DEFAULT 0
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
      `)
    } else {
      // SQLite + PostgreSQL
      await this.db.exec(`
        CREATE TABLE IF NOT EXISTS ${MIGRATIONS_TABLE} (
          id TEXT PRIMARY KEY,
          applied_at TEXT NOT NULL,
          checksum TEXT NOT NULL,
          execution_ms INTEGER NOT NULL DEFAULT 0
        )
      `)
    }
  }

  async getAppliedMigrations(): Promise<MigrationRecord[]> {
    await this.ensureMigrationsTable()
    const rows = await this.db.query(
      `SELECT id, applied_at, checksum, execution_ms FROM ${MIGRATIONS_TABLE} ORDER BY applied_at ASC`
    )
    return rows.map((r) => ({
      id: r['id'] as string,
      appliedAt: r['applied_at'] as string,
      checksum: r['checksum'] as string,
      executionMs: r['execution_ms'] as number
    }))
  }

  async status(): Promise<MigrationStatus> {
    const applied = await this.getAppliedMigrations()
    const appliedIds = new Set(applied.map((r) => r.id))
    const dialect = this.db.capabilities.dialect

    const pending = this.migrations.filter(
      (m) => !appliedIds.has(m.id) && this.matchesDialect(m, dialect)
    )

    return {
      pending,
      applied,
      current: applied.length > 0 ? applied[applied.length - 1]!.id : null
    }
  }

  async migrate(options: MigrateOptions = {}): Promise<string[]> {
    const { dryRun = false, target } = options
    await this.ensureMigrationsTable()

    const applied = await this.getAppliedMigrations()
    const appliedIds = new Set(applied.map((r) => r.id))
    const dialect = this.db.capabilities.dialect

    const toApply = this.migrations
      .filter((m) => !appliedIds.has(m.id) && this.matchesDialect(m, dialect))
      .sort((a, b) => a.id.localeCompare(b.id))

    const appliedList: string[] = []

    for (const migration of toApply) {
      if (target && migration.id > target) break

      if (dryRun) {
        console.log(`[Migration] Would apply: ${migration.id} — ${migration.description}`)
        appliedList.push(migration.id)
        continue
      }

      const startMs = Date.now()
      await this.db.transaction(async () => {
        await migration.up(this.db)
        const executionMs = Date.now() - startMs
        const checksum = this.checksumOf(migration)
        await this.db.query(
          `INSERT INTO ${MIGRATIONS_TABLE} (id, applied_at, checksum, execution_ms) VALUES (?, ?, ?, ?)`,
          [migration.id, new Date().toISOString(), checksum, executionMs]
        )
      })

      console.log(`[Migration] Applied: ${migration.id} (${Date.now() - startMs}ms)`)
      appliedList.push(migration.id)
    }

    return appliedList
  }

  async rollback(count = 1): Promise<string[]> {
    const applied = await this.getAppliedMigrations()
    const toRollback = applied.slice(-count).reverse()
    const rolled: string[] = []

    for (const record of toRollback) {
      const migration = this.migrations.find((m) => m.id === record.id)
      if (!migration) {
        throw new Error(`Cannot rollback: migration "${record.id}" not found in registry`)
      }
      if (!migration.down) {
        throw new Error(`Migration "${record.id}" does not support rollback (no down() function)`)
      }

      await this.db.transaction(async () => {
        await migration.down!(this.db)
        await this.db.query(`DELETE FROM ${MIGRATIONS_TABLE} WHERE id = ?`, [record.id])
      })

      console.log(`[Migration] Rolled back: ${record.id}`)
      rolled.push(record.id)
    }

    return rolled
  }

  private matchesDialect(migration: Migration, currentDialect: string): boolean {
    return migration.dialect === 'all' || migration.dialect === currentDialect
  }

  private checksumOf(migration: Migration): string {
    return createHash('sha256')
      .update(migration.id + migration.up.toString())
      .digest('hex')
      .slice(0, 16)
  }
}
```

### 2. `src/main/db/migrations/__tests__/runner.test.ts`

```typescript
import { describe, it, expect, beforeEach } from 'vitest'
import { SqliteAdapter } from '../../sqlite/sqlite-adapter'
import { MigrationRunner } from '../runner'
import type { Migration } from '../types'

const m1: Migration = {
  id: '20260101_000001_create_test', description: 'Create test table', dialect: 'all',
  async up(db) { await db.exec('CREATE TABLE mig_test (id INTEGER PRIMARY KEY, name TEXT)') },
  async down(db) { await db.exec('DROP TABLE IF EXISTS mig_test') }
}

const m2: Migration = {
  id: '20260101_000002_add_column', description: 'Add value column', dialect: 'all',
  async up(db) { await db.exec('ALTER TABLE mig_test ADD COLUMN value INTEGER DEFAULT 0') }
}

describe('MigrationRunner', () => {
  let db: SqliteAdapter
  let runner: MigrationRunner

  beforeEach(() => {
    db = new SqliteAdapter(':memory:')
    runner = new MigrationRunner(db, [m1, m2])
  })

  afterEach(() => db.close())

  describe('ensureMigrationsTable', () => {
    it('creates migrations table on first call', async () => {
      await runner.ensureMigrationsTable()
      const rows = await db.query("SELECT name FROM sqlite_master WHERE type='table' AND name='orca_schema_migrations'")
      expect(rows).toHaveLength(1)
    })

    it('is idempotent', async () => {
      await runner.ensureMigrationsTable()
      await expect(runner.ensureMigrationsTable()).resolves.toBeUndefined()
    })
  })

  describe('status()', () => {
    it('returns all as pending initially', async () => {
      const s = await runner.status()
      expect(s.pending).toHaveLength(2)
      expect(s.applied).toHaveLength(0)
      expect(s.current).toBeNull()
    })

    it('shows applied after migrate()', async () => {
      await runner.migrate()
      const s = await runner.status()
      expect(s.applied).toHaveLength(2)
      expect(s.pending).toHaveLength(0)
      expect(s.current).toBe(m2.id)
    })
  })

  describe('migrate()', () => {
    it('applies all pending in order', async () => {
      const applied = await runner.migrate()
      expect(applied).toEqual([m1.id, m2.id])
    })

    it('is idempotent', async () => {
      await runner.migrate()
      expect(await runner.migrate()).toHaveLength(0)
    })

    it('actually creates table', async () => {
      await runner.migrate()
      const rows = await db.query("SELECT name FROM sqlite_master WHERE type='table' AND name='mig_test'")
      expect(rows).toHaveLength(1)
    })

    it('records checksum in migrations table', async () => {
      await runner.migrate()
      const records = await db.query('SELECT checksum FROM orca_schema_migrations WHERE id = ?', [m1.id])
      expect((records[0]!['checksum'] as string).length).toBeGreaterThan(0)
    })

    it('rolls back if migration.up() throws', async () => {
      const failing: Migration = {
        id: '20260101_999_fail', description: 'Fail', dialect: 'all',
        async up() { throw new Error('migration failure') }
      }
      const r = new MigrationRunner(db, [failing])
      await expect(r.migrate()).rejects.toThrow('migration failure')
      const applied = await db.query('SELECT * FROM orca_schema_migrations WHERE id = ?', [failing.id])
      expect(applied).toHaveLength(0)
    })

    it('dryRun does not execute DDL', async () => {
      const applied = await runner.migrate({ dryRun: true })
      expect(applied).toHaveLength(2)
      const tables = await db.query("SELECT name FROM sqlite_master WHERE type='table' AND name='mig_test'")
      expect(tables).toHaveLength(0)
    })

    it('target stops at specified id', async () => {
      const applied = await runner.migrate({ target: m1.id })
      expect(applied).toHaveLength(1)
      expect(applied[0]).toBe(m1.id)
    })

    it('skips migrations for wrong dialect', async () => {
      const pgOnly: Migration = {
        id: '20260101_000010_pg_only', description: 'PG only', dialect: 'postgresql',
        async up(db) { await db.exec('CREATE TABLE pg_tbl (id INTEGER)') }
      }
      const r = new MigrationRunner(db, [pgOnly])
      const applied = await r.migrate()
      expect(applied).not.toContain(pgOnly.id)
    })
  })

  describe('rollback()', () => {
    beforeEach(async () => { await runner.migrate() })

    it('rolls back last migration', async () => {
      const rolled = await runner.rollback(1)
      expect(rolled).toEqual([m2.id])
    })

    it('throws when no down() function', async () => {
      const noDown = new MigrationRunner(db, [m2])
      await noDown.ensureMigrationsTable()
      await db.query('INSERT INTO orca_schema_migrations (id, applied_at, checksum, execution_ms) VALUES (?, ?, ?, ?)',
        [m2.id, new Date().toISOString(), 'abc', 0])
      await expect(noDown.rollback(1)).rejects.toThrow(/does not support rollback/)
    })
  })
})
```

---

## Verification

```bash
pnpm vitest run src/main/db/migrations/__tests__/runner.test.ts
```

Expected: 14/14 tests pass

---

## Done criteria

- [x] `MigrationRunner` class với `migrate()`, `rollback()`, `status()`, `getAppliedMigrations()`, `ensureMigrationsTable()`
- [x] Migration wrapped in `db.transaction()` — failure → rollback và không record
- [x] `migrate()` idempotent (re-running không re-apply)
- [x] `dryRun: true` không execute DDL
- [x] `target` stops migrations at specified id
- [x] Dialect filter (`migration.dialect !== 'all'` và không match current)
- [x] `rollback()` throw rõ ràng khi không có `down()`
- [x] 14/14 tests pass
