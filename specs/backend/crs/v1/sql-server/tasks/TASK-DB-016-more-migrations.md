# TASK-DB-016: Tạo `0002_add_automations.ts` + `0003_add_workspace_sessions.ts`

**Source:** SOL-DB-003  
**Phase:** 2 | **Effort:** S (30–45 min)  
**Depends on:** TASK-DB-013, TASK-DB-015

---

## Objective

Tạo 2 migrations bổ sung: automations/automation_runs và workspace_sessions. Cập nhật `ALL_MIGRATIONS` trong `index.ts`.

---

## Files to create

### 1. `src/main/db/migrations/0002_add_automations.ts`

```typescript
import type { Migration } from './types'

export const migration_0002_add_automations: Migration = {
  id: '20260723_000002_add_automations',
  description: 'Add automations and automation_runs tables',
  dialect: 'all',

  async up(db) {
    const d = db.capabilities.dialect
    const isMySQL = d === 'mysql' || d === 'tidb' || d === 'mariadb'
    const isPg = d === 'postgresql'
    const jsonType = isMySQL ? 'JSON' : isPg ? 'JSONB' : 'TEXT'
    const nowFn = (isMySQL || isPg) ? 'NOW()' : "datetime('now')"
    const mySuffix = isMySQL ? ' ENGINE=InnoDB DEFAULT CHARSET=utf8mb4' : ''

    await db.exec(`CREATE TABLE IF NOT EXISTS orca_automations (
      id TEXT PRIMARY KEY,
      name TEXT NOT NULL,
      project_id TEXT,
      enabled INTEGER NOT NULL DEFAULT 1,
      data ${jsonType} NOT NULL,
      created_at TEXT NOT NULL DEFAULT (${nowFn}),
      updated_at TEXT NOT NULL DEFAULT (${nowFn})
    )${mySuffix}`)

    await db.exec(`CREATE TABLE IF NOT EXISTS orca_automation_runs (
      id TEXT PRIMARY KEY,
      automation_id TEXT NOT NULL,
      status TEXT NOT NULL,
      started_at TEXT NOT NULL,
      finished_at TEXT,
      result ${jsonType},
      error TEXT
    )${mySuffix}`)
  },

  async down(db) {
    await db.exec('DROP TABLE IF EXISTS orca_automation_runs')
    await db.exec('DROP TABLE IF EXISTS orca_automations')
  }
}
```

### 2. `src/main/db/migrations/0003_add_workspace_sessions.ts`

```typescript
import type { Migration } from './types'

export const migration_0003_add_workspace_sessions: Migration = {
  id: '20260723_000003_add_workspace_sessions',
  description: 'Add workspace_sessions table for server-mode client tracking',
  dialect: 'all',

  async up(db) {
    const d = db.capabilities.dialect
    const isMySQL = d === 'mysql' || d === 'tidb' || d === 'mariadb'
    const isPg = d === 'postgresql'
    const nowFn = (isMySQL || isPg) ? 'NOW()' : "datetime('now')"
    const mySuffix = isMySQL ? ' ENGINE=InnoDB DEFAULT CHARSET=utf8mb4' : ''

    await db.exec(`CREATE TABLE IF NOT EXISTS orca_workspace_sessions (
      id TEXT PRIMARY KEY,
      client_id TEXT NOT NULL,
      repo_id TEXT,
      connected_at TEXT NOT NULL DEFAULT (${nowFn}),
      last_active_at TEXT NOT NULL DEFAULT (${nowFn}),
      metadata TEXT
    )${mySuffix}`)
  },

  async down(db) {
    await db.exec('DROP TABLE IF EXISTS orca_workspace_sessions')
  }
}
```

### 3. `src/main/db/migrations/__tests__/all-migrations.test.ts`

```typescript
import { describe, it, expect } from 'vitest'
import { SqliteAdapter } from '../../sqlite/sqlite-adapter'
import { MigrationRunner } from '../runner'
import { ALL_MIGRATIONS } from '../index'

describe('ALL_MIGRATIONS — full run (SQLite)', () => {
  it('applies all migrations without error', async () => {
    const db = new SqliteAdapter(':memory:')
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    const applied = await runner.migrate()
    expect(applied).toHaveLength(ALL_MIGRATIONS.length)
    db.close()
  })

  it('ALL_MIGRATIONS is sorted by id', () => {
    const ids = ALL_MIGRATIONS.map((m) => m.id)
    const sorted = [...ids].sort()
    expect(ids).toEqual(sorted)
  })

  it('no duplicate ids', () => {
    const ids = ALL_MIGRATIONS.map((m) => m.id)
    const unique = new Set(ids)
    expect(unique.size).toBe(ids.length)
  })

  it('creates all expected tables', async () => {
    const db = new SqliteAdapter(':memory:')
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()

    const tables = await db.query(
      "SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'orca_%' ORDER BY name"
    )
    const names = tables.map((r) => r['name'] as string)
    expect(names).toContain('orca_projects')
    expect(names).toContain('orca_repos')
    expect(names).toContain('orca_ssh_targets')
    expect(names).toContain('orca_global_settings')
    expect(names).toContain('orca_automations')
    expect(names).toContain('orca_automation_runs')
    expect(names).toContain('orca_workspace_sessions')
    db.close()
  })
})
```

---

## Verification

```bash
pnpm vitest run src/main/db/migrations/__tests__/all-migrations.test.ts
```

Expected: 4/4 tests pass, 3 migrations applied

---

## Done criteria

- [x] `0002_add_automations.ts` tạo `orca_automations` và `orca_automation_runs`
- [x] `0003_add_workspace_sessions.ts` tạo `orca_workspace_sessions`
- [x] `index.ts` export `ALL_MIGRATIONS` array với cả 3 migrations
- [x] Migrations sorted by id (chronological)
- [x] No duplicate ids
- [x] `all-migrations.test.ts` pass 4 tests
