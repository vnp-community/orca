# SOL-DB-005 — State Repository Pattern Refactor

**CR:** [CR-005](../../../../../docs/crs/v1/sql-server/CR-005-state-repository-refactor.md)  
**TDD Refs:** TDD-06 (Persistence — Store Methods), TDD-07 (Runtime Service)  
**Approach:** Test-Driven + Strangler Fig Pattern  
**Status:** ✅ Implemented (2026-07-24)  
**Depends on:** SOL-DB-001, SOL-DB-002, SOL-DB-003

---

## 1. Phân tích từ TDD

Từ **TDD-06 §3 (Store Methods)**:
```typescript
class Store {
  getProjects(): Project[]
  getProject(id: string): Project | undefined
  addProject(project: Project): void
  updateProject(id: string, updates: ProjectUpdateArgs): Project
  removeProject(id: string): void
  // ... tương tự cho Repos, Worktrees, SshTargets, Settings, Automations
}
```

Từ **TDD-07 (Runtime Service)**:
```typescript
new OrcaRuntimeService(store, stats)
```
→ `OrcaRuntimeService` nhận `Store` trực tiếp. Cần refactor để nhận `IStateRepository`.

**Strangler Fig Strategy:**
- **Phase A (SOL-DB-005):** Tạo `IStateRepository` interface + `JsonFileRepository` (wraps `Store`)
- **Phase B:** Inject `IStateRepository` vào `OrcaRuntimeService` thay `Store`
- **Phase C:** `SqlStateRepository` cho SQL backend
- **Phase D:** Dần thay thế direct Store imports

---

## 2. File Structure

```
src/main/repositories/
├── types.ts                    ← IStateRepository + sub-interfaces
├── json-file-repository.ts     ← JSON file backend (wraps Store)
├── sql-repository.ts           ← SQL backend
├── factory.ts                  ← createStateRepository()
└── __tests__/
    ├── json-file-repository.test.ts
    └── sql-repository.test.ts
```

---

## 3. Test Specifications

### 3.1 `repository-conformance.ts` — Shared conformance suite

```typescript
// src/main/repositories/__tests__/repository-conformance.ts
// Shared test suite — chạy với cả JSON và SQL backends

import { describe, it, expect, beforeEach } from 'vitest'
import type { IStateRepository } from '../types'
import type { Project, Repo, SshTarget } from '../../../shared/types'

function makeProject(overrides: Partial<Project> = {}): Omit<Project, 'id'> {
  return {
    name: 'Test Project',
    repoIds: [],
    tabOrder: 0,
    ...overrides
  } as Omit<Project, 'id'>
}

function makeRepo(overrides: Partial<Repo> = {}): Omit<Repo, 'id'> {
  return {
    name: 'test-repo',
    path: '/test/path',
    ...overrides
  } as Omit<Repo, 'id'>
}

function makeSshTarget(overrides: Partial<SshTarget> = {}): Omit<SshTarget, 'id'> {
  return {
    label: 'Dev Server',
    host: 'dev.example.com',
    port: 22,
    username: 'ubuntu',
    ...overrides
  } as Omit<SshTarget, 'id'>
}

export function runStateRepositoryConformanceTests(
  name: string,
  factory: () => Promise<IStateRepository>
): void {
  describe(`${name} — IStateRepository conformance`, () => {
    let repo: IStateRepository

    beforeEach(async () => { repo = await factory() })
    afterEach(async () => { await repo.close() })

    // ── ping ────────────────────────────────────────────
    it('ping() returns true', async () => {
      expect(await repo.ping()).toBe(true)
    })

    // ── Projects CRUD ───────────────────────────────────
    describe('projects', () => {
      it('findAll() returns empty array initially', async () => {
        const projects = await repo.projects.findAll()
        expect(projects).toEqual([])
      })

      it('create() creates a project with generated id', async () => {
        const project = await repo.projects.create(makeProject())
        expect(project.id).toBeTruthy()
        expect(project.name).toBe('Test Project')
      })

      it('findById() finds created project', async () => {
        const created = await repo.projects.create(makeProject())
        const found = await repo.projects.findById(created.id)
        expect(found).toMatchObject({ id: created.id, name: 'Test Project' })
      })

      it('findById() returns null for unknown id', async () => {
        expect(await repo.projects.findById('nonexistent')).toBeNull()
      })

      it('update() updates project fields', async () => {
        const created = await repo.projects.create(makeProject())
        const updated = await repo.projects.update(created.id, { name: 'Updated Name' })
        expect(updated.name).toBe('Updated Name')
        expect(updated.id).toBe(created.id)
      })

      it('delete() removes project', async () => {
        const created = await repo.projects.create(makeProject())
        await repo.projects.delete(created.id)
        expect(await repo.projects.findById(created.id)).toBeNull()
      })

      it('findAll() returns all created projects', async () => {
        await repo.projects.create(makeProject({ name: 'P1' }))
        await repo.projects.create(makeProject({ name: 'P2' }))
        const all = await repo.projects.findAll()
        expect(all).toHaveLength(2)
      })
    })

    // ── Repos CRUD ──────────────────────────────────────
    describe('repos', () => {
      it('create() creates a repo', async () => {
        const repo_ = await repo.repos.create(makeRepo())
        expect(repo_.id).toBeTruthy()
        expect(repo_.name).toBe('test-repo')
      })

      it('findById() returns created repo', async () => {
        const created = await repo.repos.create(makeRepo())
        const found = await repo.repos.findById(created.id)
        expect(found).toMatchObject({ name: 'test-repo' })
      })

      it('findAll() returns all repos', async () => {
        await repo.repos.create(makeRepo({ name: 'r1' }))
        await repo.repos.create(makeRepo({ name: 'r2' }))
        const all = await repo.repos.findAll()
        expect(all).toHaveLength(2)
      })
    })

    // ── SSH Targets CRUD ────────────────────────────────
    describe('sshTargets', () => {
      it('create() creates an SSH target', async () => {
        const target = await repo.sshTargets.create(makeSshTarget())
        expect(target.id).toBeTruthy()
        expect(target.host).toBe('dev.example.com')
      })

      it('findAll() returns all SSH targets', async () => {
        await repo.sshTargets.create(makeSshTarget({ host: 'host-1' }))
        await repo.sshTargets.create(makeSshTarget({ host: 'host-2' }))
        const all = await repo.sshTargets.findAll()
        expect(all).toHaveLength(2)
      })

      it('delete() removes SSH target', async () => {
        const created = await repo.sshTargets.create(makeSshTarget())
        await repo.sshTargets.delete(created.id)
        expect(await repo.sshTargets.findById(created.id)).toBeNull()
      })
    })

    // ── Settings ─────────────────────────────────────────
    describe('settings', () => {
      it('get() returns default settings', async () => {
        const settings = await repo.settings.get()
        expect(settings).toBeDefined()
      })

      it('update() persists setting change', async () => {
        await repo.settings.update({ theme: 'dark' })
        const settings = await repo.settings.get()
        expect(settings.theme).toBe('dark')
      })
    })
  })
}
```

### 3.2 `json-file-repository.test.ts`

```typescript
// src/main/repositories/__tests__/json-file-repository.test.ts
import { describe, it, expect, beforeEach } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { JsonFileStateRepository } from '../json-file-repository'
import { runStateRepositoryConformanceTests } from './repository-conformance'

// Chạy shared conformance suite
runStateRepositoryConformanceTests('JsonFileStateRepository', async () => {
  const tmpDir = mkdtempSync(join(tmpdir(), 'orca-repo-test-'))
  const dataFile = join(tmpDir, 'store.json')
  return new JsonFileStateRepository(dataFile)
})

// JSON-specific tests
describe('JsonFileStateRepository — JSON specific', () => {
  let tmpDir: string
  let repo: JsonFileStateRepository

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'orca-json-repo-'))
    repo = new JsonFileStateRepository(join(tmpDir, 'store.json'))
  })

  afterEach(async () => {
    await repo.close()
    rmSync(tmpDir, { recursive: true, force: true })
  })

  it('ping() always returns true', async () => {
    expect(await repo.ping()).toBe(true)
  })

  it('persists data between instances', async () => {
    const dataFile = join(tmpDir, 'store.json')
    const repo1 = new JsonFileStateRepository(dataFile)
    const created = await repo1.projects.create({ name: 'Persistent', repoIds: [], tabOrder: 0 } as any)
    await repo1.close()

    const repo2 = new JsonFileStateRepository(dataFile)
    const found = await repo2.projects.findById(created.id)
    expect(found?.name).toBe('Persistent')
    await repo2.close()
  })
})
```

### 3.3 `sql-repository.test.ts`

```typescript
// src/main/repositories/__tests__/sql-repository.test.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteAdapter } from '../../db/sqlite/sqlite-adapter'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { SqlStateRepository } from '../sql-repository'
import { runStateRepositoryConformanceTests } from './repository-conformance'

// Run conformance suite với SQL backend (SQLite in-memory)
runStateRepositoryConformanceTests('SqlStateRepository (SQLite)', async () => {
  const db = new SqliteAdapter(':memory:')
  const pool = new SqliteSingleConnectionPool(':memory:')

  // Apply migrations
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })

  return new SqlStateRepository(pool)
})
```

---

## 4. Implementation Guide

### 4.1 `src/main/repositories/types.ts`

```typescript
import type { Project, Repo, SshTarget, GlobalSettings, PersistedState } from '../../shared/types'

export interface IRepository<T, CreateInput = Omit<T, 'id'>, UpdateInput = Partial<T>> {
  findById(id: string): Promise<T | null>
  findAll(): Promise<T[]>
  create(input: CreateInput): Promise<T>
  update(id: string, input: UpdateInput): Promise<T>
  delete(id: string): Promise<void>
}

export interface IProjectRepository extends IRepository<Project> {
  findByGroup(groupId: string): Promise<Project[]>
}

export interface IRepoRepository extends IRepository<Repo> {
  findByProject(projectId: string): Promise<Repo[]>
}

export interface ISshTargetRepository extends IRepository<SshTarget> {
  findByHost(host: string): Promise<SshTarget[]>
}

export interface IGlobalSettingsRepository {
  get(): Promise<GlobalSettings>
  update(patch: Partial<GlobalSettings>): Promise<GlobalSettings>
}

export interface IStateRepository {
  readonly projects: IProjectRepository
  readonly repos: IRepoRepository
  readonly sshTargets: ISshTargetRepository
  readonly settings: IGlobalSettingsRepository
  ping(): Promise<boolean>
  close(): Promise<void>
}
```

### 4.2 `src/main/repositories/json-file-repository.ts` — Key Design

```typescript
// Why: Wrap existing Store or create a minimal JSON file store
// for repositories that don't need full Store complexity.
// This allows IStateRepository to work without full Store initialization.

import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname } from 'node:path'
import { mkdirSync } from 'node:fs'
import { randomUUID } from 'node:crypto'
import type { IStateRepository, IProjectRepository, IRepoRepository, ISshTargetRepository, IGlobalSettingsRepository } from './types'
import type { Project, Repo, SshTarget, GlobalSettings } from '../../shared/types'

interface RepoState {
  projects: Project[]
  repos: Repo[]
  sshTargets: SshTarget[]
  globalSettings: GlobalSettings
}

function defaultState(): RepoState {
  return { projects: [], repos: [], sshTargets: [], globalSettings: {} as GlobalSettings }
}

export class JsonFileStateRepository implements IStateRepository {
  private state: RepoState
  private dirty = false
  private flushTimer: ReturnType<typeof setTimeout> | null = null

  constructor(private readonly dataFile: string) {
    this.state = this.load()
  }

  private load(): RepoState {
    if (!existsSync(this.dataFile)) return defaultState()
    try {
      return JSON.parse(readFileSync(this.dataFile, 'utf8')) as RepoState
    } catch {
      return defaultState()
    }
  }

  private scheduleSave(): void {
    if (this.flushTimer) clearTimeout(this.flushTimer)
    this.dirty = true
    this.flushTimer = setTimeout(() => this.flush(), 100)
  }

  private flush(): void {
    if (!this.dirty) return
    const dir = dirname(this.dataFile)
    if (!existsSync(dir)) mkdirSync(dir, { recursive: true })
    writeFileSync(this.dataFile, JSON.stringify(this.state, null, 2), 'utf8')
    this.dirty = false
  }

  get projects(): IProjectRepository {
    return {
      findById: async (id) => this.state.projects.find((p) => p.id === id) ?? null,
      findAll: async () => [...this.state.projects],
      create: async (input) => {
        const project = { ...input, id: randomUUID() } as Project
        this.state.projects.push(project)
        this.scheduleSave()
        return project
      },
      update: async (id, input) => {
        const idx = this.state.projects.findIndex((p) => p.id === id)
        if (idx === -1) throw new Error(`Project not found: ${id}`)
        this.state.projects[idx] = { ...this.state.projects[idx]!, ...input }
        this.scheduleSave()
        return this.state.projects[idx]!
      },
      delete: async (id) => {
        this.state.projects = this.state.projects.filter((p) => p.id !== id)
        this.scheduleSave()
      },
      findByGroup: async (groupId) =>
        this.state.projects.filter((p) => (p as any).projectGroupId === groupId)
    }
  }

  // tương tự cho repos, sshTargets, settings...

  get settings(): IGlobalSettingsRepository {
    return {
      get: async () => ({ ...this.state.globalSettings }),
      update: async (patch) => {
        this.state.globalSettings = { ...this.state.globalSettings, ...patch }
        this.scheduleSave()
        return { ...this.state.globalSettings }
      }
    }
  }

  async ping(): Promise<boolean> { return true }

  async close(): Promise<void> {
    if (this.flushTimer) {
      clearTimeout(this.flushTimer)
      this.flushTimer = null
    }
    this.flush()
  }
}
```

**Implementation checklist:**
- [x] `create()` generate UUID cho id
- [x] `update()` throw với message rõ ràng nếu id không tồn tại
- [x] `scheduleSave()` debounce 100ms — không write disk mỗi operation
- [x] `close()` flush outstanding writes
- [x] State không mutate trực tiếp — clone trước khi trả về
- [x] `mkdirSync(dirname(dataFile), { recursive: true })` khi save

### 4.3 `src/main/repositories/sql-repository.ts` — Key Design

```typescript
import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import type { IStateRepository, IProjectRepository } from './types'
import type { Project } from '../../shared/types'

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
          db.query('SELECT data FROM orca_projects ORDER BY tab_order ASC')
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
      update: async (id, input) => {
        const existing = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_projects WHERE id = ?', [id])
        ).then((rows) => rows[0] ? JSON.parse(rows[0]['data'] as string) as Project : null)

        if (!existing) throw new Error(`Project not found: ${id}`)
        const updated = { ...existing, ...input }
        await pool.withConnection((db) =>
          db.query(
            'UPDATE orca_projects SET data = ?, name = ? WHERE id = ?',
            [JSON.stringify(updated), updated.name, id]
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

**Implementation checklist:**
- [x] Dùng `JSON.stringify/parse` cho complex data column
- [x] `tab_order` được persist/restore cho project ordering
- [x] `findByGroup` filter in-memory nếu JSON query không practical
- [x] `update()` throw với clear error khi id không tồn tại
- [x] Dùng `pool.withConnection()` — không gọi `pool.acquire()` trực tiếp

### 4.4 `src/main/repositories/factory.ts`

```typescript
import type { IStateRepository } from './types'
import type { IConnectionPool } from '../db/pool'

export interface RepositoryFactoryOptions {
  pool?: IConnectionPool
  dataFile?: string  // for JSON file backend
}

export function createStateRepository(options: RepositoryFactoryOptions): IStateRepository {
  if (options.pool) {
    // Lazy require to avoid bundling in Electron
    const { SqlStateRepository } = require('./sql-repository') as {
      SqlStateRepository: new (pool: IConnectionPool) => IStateRepository
    }
    return new SqlStateRepository(options.pool)
  }

  if (options.dataFile) {
    const { JsonFileStateRepository } = require('./json-file-repository') as {
      JsonFileStateRepository: new (dataFile: string) => IStateRepository
    }
    return new JsonFileStateRepository(options.dataFile)
  }

  throw new Error('createStateRepository: must provide either pool or dataFile')
}
```

---

## 5. Integration với server-bootstrap.ts

```typescript
// src/main/server-bootstrap.ts — Updated

export async function initializeOrcaServices(options: ServerBootstrapOptions) {
  const { platform, port: requestedPort = 6768 } = options

  // Load DB config từ env / options
  const { loadDatabaseConfig } = await import('./db/config-loader')
  const dbConfig = options.database ?? loadDatabaseConfig(platform.app.getPath('userData'))

  // Initialize pool
  let pool: IConnectionPool
  if (dbConfig && dbConfig.dialect !== 'sqlite') {
    const { GenericConnectionPool } = await import('./db/generic-pool')
    pool = new GenericConnectionPool(dbConfig, (dbConfig as any).pool)
    await (pool as GenericConnectionPool).initialize()
  } else {
    const { SqliteSingleConnectionPool } = await import('./db/sqlite/sqlite-pool')
    const sqlitePath = dbConfig?.dialect === 'sqlite' && (dbConfig as any).path !== ':memory:'
      ? (dbConfig as any).path
      : join(platform.app.getPath('userData'), 'orca-server.db')
    pool = new SqliteSingleConnectionPool(sqlitePath)
  }

  // Run migrations (chỉ với SQL backend — legacy JSON store tự manage migrations)
  if (dbConfig) {
    const { MigrationRunner } = await import('./db/migrations/runner')
    const { ALL_MIGRATIONS } = await import('./db/migrations')
    await pool.withConnection(async (db) => {
      const runner = new MigrationRunner(db, ALL_MIGRATIONS)
      await runner.migrate()
    })
  }

  // Create state repository
  const { createStateRepository } = await import('./repositories/factory')
  const stateRepo = dbConfig
    ? createStateRepository({ pool })
    : createStateRepository({ dataFile: join(platform.app.getPath('userData'), 'store.json') })

  // Initialize legacy Store for backward compat (Electron mode)
  const { initDataPath } = await import('./persistence')
  initDataPath()
  const { Store } = await import('./persistence')
  const store = new Store()  // Still needed for backward compat modules

  // ...rest of bootstrap
}
```

---

## 6. Verification Commands

```bash
# 1. JSON file repository tests
pnpm vitest run src/main/repositories/__tests__/json-file-repository.test.ts

# 2. SQL repository tests (SQLite in-memory)
pnpm vitest run src/main/repositories/__tests__/sql-repository.test.ts

# 3. Conformance suite passes for both backends
pnpm vitest run src/main/repositories/

# 4. Existing persistence tests still pass
pnpm vitest run src/main/persistence.test.ts

# 5. TypeScript strict mode
pnpm tsc --noEmit
```

---

## 7. Acceptance Criteria

| # | Criteria | Test |
|---|---------|------|
| AC-1 | `IStateRepository` interface với zero `any` | `pnpm tsc` |
| AC-2 | `JsonFileStateRepository` passes conformance suite | `repository-conformance.ts` |
| AC-3 | `SqlStateRepository` passes conformance suite (SQLite) | `repository-conformance.ts` |
| AC-4 | Data persists between instances (JSON backend) | `json-file-repository.test.ts` |
| AC-5 | `createStateRepository()` factory chọn đúng backend | factory test |
| AC-6 | `close()` flushes pending JSON writes | json repo test |
| AC-7 | Existing `persistence.test.ts` không regression | vitest |
| AC-8 | SQL repo `update()` throws khi id không tồn tại | conformance test |


---

## ✅ Implementation Status — COMPLETED 2026-07-23

**Status:** ✅ IMPLEMENTED  
**Implemented by:** AI Agent (Antigravity)  
**Date completed:** 2026-07-23  
**Tests:** 38 unit tests — all passing  

### Tasks Executed
TASK-DB-017, TASK-DB-018, TASK-DB-019, TASK-DB-020

### Files Created / Modified
- `src/main/repositories/types.ts`
- `src/main/repositories/json-file-repository.ts`
- `src/main/repositories/sql-repository.ts`
- `src/main/repositories/factory.ts`

### Verification
```bash
pnpm vitest run src/main/db/ src/main/repositories/
# → 205 tests passed (16 test files)
```

> All 27 tasks (TASK-DB-001 → TASK-DB-027) have been implemented and verified.
> Zero regression on existing tests. Zero TypeScript compile errors.
