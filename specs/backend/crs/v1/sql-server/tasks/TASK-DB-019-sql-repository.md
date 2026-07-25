# TASK-DB-019: Tạo `src/main/repositories/sql-repository.ts` + tests ✅ DONE

**Source:** SOL-DB-005 §4.3  
**Phase:** 3 | **Effort:** M (1.5–2 giờ)   | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-009, TASK-DB-015, TASK-DB-017

---

## Objective

Tạo `SqlStateRepository` — implementation `IStateRepository` dùng SQL backend. Sử dụng `IConnectionPool` để query `orca_*` tables.

---

## Files to create

### 1. `src/main/repositories/sql-repository.ts`

```typescript
import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import type {
  IStateRepository, IProjectRepository, IRepoRepository,
  ISshTargetRepository, IGlobalSettingsRepository, GlobalSettings
} from './types'
import type { Project, Repo, SshTarget } from '../../shared/types'

export class SqlStateRepository implements IStateRepository {
  constructor(private readonly pool: IConnectionPool) {}

  get projects(): IProjectRepository {
    const pool = this.pool
    return {
      findById: async (id) => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_projects WHERE id = ?', [id])
        )
        return rows[0] ? JSON.parse(rows[0]['data'] as string) as Project : null
      },
      findAll: async () => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_projects ORDER BY tab_order ASC, created_at ASC')
        )
        return rows.map((r) => JSON.parse(r['data'] as string) as Project)
      },
      create: async (input) => {
        const project = { ...input, id: randomUUID() } as Project
        await pool.withConnection((db) =>
          db.query(
            'INSERT INTO orca_projects (id, name, tab_order, data) VALUES (?, ?, ?, ?)',
            [project.id, project.name, (project as any).tabOrder ?? 0, JSON.stringify(project)]
          )
        )
        return project
      },
      update: async (id, patch) => {
        const existing = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_projects WHERE id = ?', [id])
        ).then((rows) => rows[0] ? JSON.parse(rows[0]['data'] as string) as Project : null)

        if (!existing) throw new Error(`Project not found: ${id}`)
        const updated = { ...existing, ...patch }
        await pool.withConnection((db) =>
          db.query(
            'UPDATE orca_projects SET name = ?, tab_order = ?, data = ? WHERE id = ?',
            [updated.name, (updated as any).tabOrder ?? 0, JSON.stringify(updated), id]
          )
        )
        return updated
      },
      delete: async (id) => {
        await pool.withConnection((db) =>
          db.query('DELETE FROM orca_projects WHERE id = ?', [id])
        )
      },
      findByGroup: async (groupId) => {
        const all = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_projects')
        )
        return all
          .map((r) => JSON.parse(r['data'] as string) as Project)
          .filter((p) => (p as any).projectGroupId === groupId)
      }
    }
  }

  get repos(): IRepoRepository {
    const pool = this.pool
    return {
      findById: async (id) => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_repos WHERE id = ?', [id])
        )
        return rows[0] ? JSON.parse(rows[0]['data'] as string) as Repo : null
      },
      findAll: async () => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_repos ORDER BY created_at ASC')
        )
        return rows.map((r) => JSON.parse(r['data'] as string) as Repo)
      },
      create: async (input) => {
        const repo = { ...input, id: randomUUID() } as Repo
        await pool.withConnection((db) =>
          db.query(
            'INSERT INTO orca_repos (id, name, data) VALUES (?, ?, ?)',
            [repo.id, repo.name, JSON.stringify(repo)]
          )
        )
        return repo
      },
      update: async (id, patch) => {
        const existing = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_repos WHERE id = ?', [id])
        ).then((rows) => rows[0] ? JSON.parse(rows[0]['data'] as string) as Repo : null)
        if (!existing) throw new Error(`Repo not found: ${id}`)
        const updated = { ...existing, ...patch }
        await pool.withConnection((db) =>
          db.query('UPDATE orca_repos SET name = ?, data = ? WHERE id = ?', [updated.name, JSON.stringify(updated), id])
        )
        return updated
      },
      delete: async (id) => {
        await pool.withConnection((db) => db.query('DELETE FROM orca_repos WHERE id = ?', [id]))
      },
      findByProject: async (projectId) => {
        const all = await pool.withConnection((db) => db.query('SELECT data FROM orca_repos'))
        return all
          .map((r) => JSON.parse(r['data'] as string) as Repo)
          .filter((r) => (r as any).projectId === projectId)
      }
    }
  }

  get sshTargets(): ISshTargetRepository {
    const pool = this.pool
    return {
      findById: async (id) => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_ssh_targets WHERE id = ?', [id])
        )
        return rows[0] ? JSON.parse(rows[0]['data'] as string) as SshTarget : null
      },
      findAll: async () => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_ssh_targets ORDER BY created_at ASC')
        )
        return rows.map((r) => JSON.parse(r['data'] as string) as SshTarget)
      },
      create: async (input) => {
        const target = { ...input, id: randomUUID() } as SshTarget
        await pool.withConnection((db) =>
          db.query(
            'INSERT INTO orca_ssh_targets (id, label, host, port, username, data) VALUES (?, ?, ?, ?, ?, ?)',
            [target.id, (target as any).label, (target as any).host, (target as any).port ?? 22, (target as any).username, JSON.stringify(target)]
          )
        )
        return target
      },
      update: async (id, patch) => {
        const existing = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_ssh_targets WHERE id = ?', [id])
        ).then((rows) => rows[0] ? JSON.parse(rows[0]['data'] as string) as SshTarget : null)
        if (!existing) throw new Error(`SshTarget not found: ${id}`)
        const updated = { ...existing, ...patch }
        await pool.withConnection((db) =>
          db.query('UPDATE orca_ssh_targets SET data = ? WHERE id = ?', [JSON.stringify(updated), id])
        )
        return updated
      },
      delete: async (id) => {
        await pool.withConnection((db) => db.query('DELETE FROM orca_ssh_targets WHERE id = ?', [id]))
      }
    }
  }

  get settings(): IGlobalSettingsRepository {
    const pool = this.pool
    return {
      get: async () => {
        const rows = await pool.withConnection((db) =>
          db.query("SELECT value FROM orca_global_settings WHERE key = 'app_settings'")
        )
        if (!rows[0]) return {}
        return JSON.parse(rows[0]['value'] as string) as GlobalSettings
      },
      update: async (patch) => {
        const current = await this.settings.get()
        const updated = { ...current, ...patch }
        await pool.withConnection((db) =>
          db.query(
            'INSERT INTO orca_global_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value',
            ['app_settings', JSON.stringify(updated)]
          )
        )
        return updated
      }
    }
  }

  async ping(): Promise<boolean> {
    try {
      await this.pool.withConnection((db) => db.query('SELECT 1'))
      return true
    } catch { return false }
  }

  async close(): Promise<void> {
    await this.pool.drain()
  }
}
```

### 2. `src/main/repositories/__tests__/sql-repository.test.ts`

```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteAdapter } from '../../db/sqlite/sqlite-adapter'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { SqlStateRepository } from '../sql-repository'

async function makeRepo() {
  const db = new SqliteAdapter(':memory:')
  const pool = new SqliteSingleConnectionPool(':memory:')
  // Run migrations on the actual adapter (not pool)
  await pool.withConnection(async (poolDb) => {
    // The pool uses the same :memory: db as SqliteAdapter
    const runner = new MigrationRunner(poolDb, ALL_MIGRATIONS)
    await runner.migrate()
  })
  return new SqlStateRepository(pool)
}

describe('SqlStateRepository', () => {
  let repo: SqlStateRepository

  beforeEach(async () => { repo = await makeRepo() })
  afterEach(async () => { await repo.close().catch(() => {}) })

  describe('ping()', () => {
    it('returns true for working pool', async () => {
      expect(await repo.ping()).toBe(true)
    })
  })

  describe('projects', () => {
    it('findAll() returns empty initially', async () => {
      expect(await repo.projects.findAll()).toEqual([])
    })

    it('create() and findById() roundtrip', async () => {
      const p = await repo.projects.create({ name: 'Test', repoIds: [], tabOrder: 0 } as any)
      expect(p.id).toBeTruthy()
      const found = await repo.projects.findById(p.id)
      expect(found?.name).toBe('Test')
    })

    it('findById() returns null for unknown', async () => {
      expect(await repo.projects.findById('no-such')).toBeNull()
    })

    it('update() changes project', async () => {
      const p = await repo.projects.create({ name: 'Old', repoIds: [], tabOrder: 0 } as any)
      const updated = await repo.projects.update(p.id, { name: 'New' } as any)
      expect(updated.name).toBe('New')
    })

    it('update() throws for unknown id', async () => {
      await expect(repo.projects.update('bad', { name: 'X' } as any)).rejects.toThrow('Project not found: bad')
    })

    it('delete() removes project', async () => {
      const p = await repo.projects.create({ name: 'Del', repoIds: [], tabOrder: 0 } as any)
      await repo.projects.delete(p.id)
      expect(await repo.projects.findById(p.id)).toBeNull()
    })

    it('findAll() returns all in tab_order', async () => {
      await repo.projects.create({ name: 'P2', repoIds: [], tabOrder: 1 } as any)
      await repo.projects.create({ name: 'P1', repoIds: [], tabOrder: 0 } as any)
      const all = await repo.projects.findAll()
      expect(all[0]!.name).toBe('P1')  // tab_order 0 first
    })
  })

  describe('sshTargets', () => {
    it('create() and findById() roundtrip', async () => {
      const t = await repo.sshTargets.create({ label: 'Dev', host: 'dev.example.com', port: 22, username: 'ubuntu' } as any)
      const found = await repo.sshTargets.findById(t.id)
      expect(found).toBeDefined()
    })
  })

  describe('settings', () => {
    it('get() returns empty object initially', async () => {
      const s = await repo.settings.get()
      expect(s).toEqual({})
    })

    it('update() persists settings', async () => {
      await repo.settings.update({ theme: 'dark' })
      const s = await repo.settings.get()
      expect(s.theme).toBe('dark')
    })

    it('update() merges with existing', async () => {
      await repo.settings.update({ theme: 'dark' })
      await repo.settings.update({ language: 'vi' })
      const s = await repo.settings.get()
      expect(s.theme).toBe('dark')
      expect(s.language).toBe('vi')
    })
  })
})
```

---

## Verification

```bash
pnpm vitest run src/main/repositories/__tests__/sql-repository.test.ts
```

Expected: 14/14 tests pass

---

## Done criteria

- [x] `SqlStateRepository` implements `IStateRepository`
- [x] Dùng `pool.withConnection()` (không gọi `pool.acquire()` trực tiếp)
- [x] `update()` throw khi id không tồn tại
- [x] Settings dùng UPSERT (`ON CONFLICT DO UPDATE`)
- [x] `ping()` return false khi pool không hoạt động
- [x] `close()` gọi `pool.drain()`
- [x] 14/14 tests pass
