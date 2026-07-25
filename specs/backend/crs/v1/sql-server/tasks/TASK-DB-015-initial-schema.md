# TASK-DB-015: Tạo `0001_initial_schema.ts` + `index.ts` + tests

**Source:** SOL-DB-003 §4.3  
**Phase:** 2 | **Effort:** S (45–60 min) | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-013, TASK-DB-014

---

## Objective

Tạo migration `0001_initial_schema` tạo 4 tables cốt lõi (projects, repos, ssh_targets, global_settings), và `index.ts` export `ALL_MIGRATIONS` array.

---

## Files to create

### 1. `src/main/db/migrations/0001_initial_schema.ts`

```typescript
import type { Migration } from './types'

export const migration_0001_initial_schema: Migration = {
  id: '20260723_000001_initial_schema',
  description: 'Initial Orca server schema — projects, repos, SSH targets, global settings',
  dialect: 'all',

  async up(db) {
    const d = db.capabilities.dialect
    const isMySQL = d === 'mysql' || d === 'tidb' || d === 'mariadb'
    const isPg = d === 'postgresql'

    const jsonType = isMySQL ? 'JSON' : isPg ? 'JSONB' : 'TEXT'
    const nowFn = (isMySQL || isPg) ? 'NOW()' : "datetime('now')"

    await db.exec(`CREATE TABLE IF NOT EXISTS orca_projects (
      id TEXT PRIMARY KEY,
      name TEXT NOT NULL,
      tab_order INTEGER NOT NULL DEFAULT 0,
      data ${jsonType} NOT NULL,
      created_at TEXT NOT NULL DEFAULT (${nowFn}),
      updated_at TEXT NOT NULL DEFAULT (${nowFn})
    )${isMySQL ? ' ENGINE=InnoDB DEFAULT CHARSET=utf8mb4' : ''}`)

    await db.exec(`CREATE TABLE IF NOT EXISTS orca_repos (
      id TEXT PRIMARY KEY,
      project_id TEXT,
      name TEXT NOT NULL,
      path TEXT,
      data ${jsonType} NOT NULL,
      created_at TEXT NOT NULL DEFAULT (${nowFn}),
      updated_at TEXT NOT NULL DEFAULT (${nowFn})
    )${isMySQL ? ' ENGINE=InnoDB DEFAULT CHARSET=utf8mb4' : ''}`)

    await db.exec(`CREATE TABLE IF NOT EXISTS orca_ssh_targets (
      id TEXT PRIMARY KEY,
      label TEXT NOT NULL,
      host TEXT NOT NULL,
      port INTEGER NOT NULL DEFAULT 22,
      username TEXT NOT NULL,
      data ${jsonType} NOT NULL,
      created_at TEXT NOT NULL DEFAULT (${nowFn}),
      updated_at TEXT NOT NULL DEFAULT (${nowFn})
    )${isMySQL ? ' ENGINE=InnoDB DEFAULT CHARSET=utf8mb4' : ''}`)

    await db.exec(`CREATE TABLE IF NOT EXISTS orca_global_settings (
      key TEXT PRIMARY KEY,
      value ${jsonType} NOT NULL,
      updated_at TEXT NOT NULL DEFAULT (${nowFn})
    )${isMySQL ? ' ENGINE=InnoDB DEFAULT CHARSET=utf8mb4' : ''}`)
  },

  async down(db) {
    await db.exec('DROP TABLE IF EXISTS orca_global_settings')
    await db.exec('DROP TABLE IF EXISTS orca_ssh_targets')
    await db.exec('DROP TABLE IF EXISTS orca_repos')
    await db.exec('DROP TABLE IF EXISTS orca_projects')
  }
}
```

### 2. `src/main/db/migrations/index.ts`

```typescript
/**
 * All registered migrations — applied in ID order (chronological).
 * Add new migrations by importing and appending to ALL_MIGRATIONS.
 *
 * IMPORTANT: Never reorder or remove migrations from this list.
 * Always append new migrations at the end.
 */

import { migration_0001_initial_schema } from './0001_initial_schema'
import { migration_0002_add_automations } from './0002_add_automations'
import { migration_0003_add_workspace_sessions } from './0003_add_workspace_sessions'
import type { Migration } from './types'

export const ALL_MIGRATIONS: Migration[] = [
  migration_0001_initial_schema,
  migration_0002_add_automations,
  migration_0003_add_workspace_sessions
]

export { migration_0001_initial_schema, migration_0002_add_automations, migration_0003_add_workspace_sessions }
export type { Migration, MigrationRecord, MigrationStatus, MigrateOptions } from './types'
export { MigrationRunner } from './runner'
```

### 3. `src/main/db/migrations/__tests__/0001_initial_schema.test.ts`

```typescript
import { describe, it, expect } from 'vitest'
import { SqliteAdapter } from '../../sqlite/sqlite-adapter'
import { MigrationRunner } from '../runner'
import { migration_0001_initial_schema } from '../0001_initial_schema'

describe('Migration 0001 — Initial Schema (SQLite)', () => {
  it('creates all 4 required tables', async () => {
    const db = new SqliteAdapter(':memory:')
    const runner = new MigrationRunner(db, [migration_0001_initial_schema])
    await runner.migrate()

    const tables = await db.query(
      "SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'orca_%' ORDER BY name"
    )
    const names = tables.map((r) => r['name'] as string)
    expect(names).toContain('orca_projects')
    expect(names).toContain('orca_repos')
    expect(names).toContain('orca_ssh_targets')
    expect(names).toContain('orca_global_settings')
    db.close()
  })

  it('can insert and retrieve a project', async () => {
    const db = new SqliteAdapter(':memory:')
    await migration_0001_initial_schema.up(db)
    await db.query(
      'INSERT INTO orca_projects (id, name, tab_order, data) VALUES (?, ?, ?, ?)',
      ['proj-1', 'My Project', 0, JSON.stringify({ id: 'proj-1', name: 'My Project' })]
    )
    const rows = await db.query('SELECT id, name FROM orca_projects WHERE id = ?', ['proj-1'])
    expect(rows[0]).toMatchObject({ id: 'proj-1', name: 'My Project' })
    db.close()
  })

  it('can insert and retrieve an SSH target', async () => {
    const db = new SqliteAdapter(':memory:')
    await migration_0001_initial_schema.up(db)
    await db.query(
      'INSERT INTO orca_ssh_targets (id, label, host, port, username, data) VALUES (?, ?, ?, ?, ?, ?)',
      ['ssh-1', 'Dev', 'dev.example.com', 22, 'ubuntu', '{}']
    )
    const rows = await db.query('SELECT host FROM orca_ssh_targets WHERE id = ?', ['ssh-1'])
    expect(rows[0]).toMatchObject({ host: 'dev.example.com' })
    db.close()
  })

  it('can insert global settings', async () => {
    const db = new SqliteAdapter(':memory:')
    await migration_0001_initial_schema.up(db)
    await db.query(
      'INSERT INTO orca_global_settings (key, value) VALUES (?, ?)',
      ['theme', '"dark"']
    )
    const rows = await db.query("SELECT value FROM orca_global_settings WHERE key = 'theme'")
    expect(rows[0]!['value']).toBe('"dark"')
    db.close()
  })

  it('down() drops all 4 tables', async () => {
    const db = new SqliteAdapter(':memory:')
    await migration_0001_initial_schema.up(db)
    await migration_0001_initial_schema.down!(db)
    const tables = await db.query(
      "SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'orca_%'"
    )
    expect(tables).toHaveLength(0)
    db.close()
  })

  it('migration is idempotent (CREATE TABLE IF NOT EXISTS)', async () => {
    const db = new SqliteAdapter(':memory:')
    await migration_0001_initial_schema.up(db)
    await expect(migration_0001_initial_schema.up(db)).resolves.toBeUndefined()
    db.close()
  })
})
```

---

## Verification

```bash
pnpm vitest run src/main/db/migrations/__tests__/0001_initial_schema.test.ts
```

Expected: 6/6 tests pass

---

## Done criteria

- [x] `src/main/db/migrations/0001_initial_schema.ts` tạo 4 tables
- [x] Tables dùng `CREATE TABLE IF NOT EXISTS` — idempotent
- [x] Dialect-specific: `JSON` cho MySQL, `JSONB` cho PG, `TEXT` cho SQLite
- [x] `down()` drop tất cả 4 tables theo thứ tự ngược
- [x] `src/main/db/migrations/index.ts` export `ALL_MIGRATIONS`
- [x] 6/6 tests pass
