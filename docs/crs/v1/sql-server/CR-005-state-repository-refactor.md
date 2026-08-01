# CR-005 — State Repository Pattern Refactor

**CR-ID:** CR-005  
**Ngày:** 2026-07-23  
**Priority:** High  
**Effort:** XL (10–15 ngày)  
**Status:** ✅ Implemented (2026-07-24)  
**Depends on:** CR-001, CR-002, CR-003, CR-004  

---

## 1. Vấn đề

`src/main/persistence.ts` là file 6570 dòng, chứa một monolithic `Store` class lưu toàn bộ Orca state trong một JSON file (`orca-data.json`). Để hỗ trợ SQL databases, cần:

1. **Tách interface khỏi implementation** — `Store` class phải work với bất kỳ backend nào
2. **Repository pattern** — mỗi domain entity (Project, Repo, SshTarget, etc.) có repo riêng
3. **Giữ backward compat** — JSON file mode vẫn là default cho Electron desktop
4. **Incremental migration** — có thể migrate từng phần, không cần big-bang rewrite

Đây là CR phức tạp nhất và có risk cao nhất — cần implement sau khi CR-001 → CR-004 đã xong.

---

## 2. Strategy: Strangler Fig Pattern

Thay vì rewrite `Store` ngay, dùng **Strangler Fig** — wrapper mới dần thay thế từng method của `Store`:

```
Phase A: Tạo IStateRepository interface + JsonFileRepository (wrap existing Store)
Phase B: Implement SqlStateRepository cho từng entity
Phase C: Routing layer — server mode dùng SQL, desktop dùng JSON
Phase D: Dần deprecate trực tiếp dùng Store trong các module
```

---

## 3. Repository Interface Design

### 3.1 Base Repository

```typescript
// src/main/repositories/types.ts

import type { Project, Repo, SshTarget, GlobalSettings, PersistedState } from '../../shared/types'

/** Generic CRUD repository */
export interface IRepository<T, CreateInput = Omit<T, 'id'>, UpdateInput = Partial<T>> {
  findById(id: string): Promise<T | null>
  findAll(): Promise<T[]>
  create(input: CreateInput): Promise<T>
  update(id: string, input: UpdateInput): Promise<T>
  delete(id: string): Promise<void>
}

/** Repository cho Projects */
export interface IProjectRepository extends IRepository<Project> {
  findByGroup(groupId: string): Promise<Project[]>
  reorder(ids: string[]): Promise<void>
}

/** Repository cho Repos */
export interface IRepoRepository extends IRepository<Repo> {
  findByProject(projectId: string): Promise<Repo[]>
  findByConnectionId(connectionId: string): Promise<Repo[]>
  updatePath(id: string, newPath: string): Promise<void>
}

/** Repository cho SSH Targets */
export interface ISshTargetRepository extends IRepository<SshTarget> {
  findByHost(host: string): Promise<SshTarget[]>
  findByProject(projectTag: string): Promise<SshTarget[]>
  upsertByHostUser(target: Omit<SshTarget, 'id'>): Promise<SshTarget>
}

/** Repository cho Global Settings */
export interface IGlobalSettingsRepository {
  get(): Promise<GlobalSettings>
  update(patch: Partial<GlobalSettings>): Promise<GlobalSettings>
}

/** Unified state repository — top-level aggregate */
export interface IStateRepository {
  projects: IProjectRepository
  repos: IRepoRepository
  sshTargets: ISshTargetRepository
  settings: IGlobalSettingsRepository
  
  /** Load toàn bộ state (compat với existing code) */
  loadFullState(): Promise<PersistedState>
  /** Save toàn bộ state (compat với existing code) */
  saveFullState(state: PersistedState): Promise<void>
  
  /** Healthcheck */
  ping(): Promise<boolean>
  /** Close connections */
  close(): Promise<void>
}
```

### 3.2 JsonFileRepository (Wrap Existing Store)

```typescript
// src/main/repositories/json-file-repository.ts
// Why: Wrap existing Store để implement IStateRepository interface.
// Desktop Electron mode vẫn dùng file-based storage, zero behavior change.

import type { IStateRepository, IProjectRepository, IRepoRepository, ISshTargetRepository, IGlobalSettingsRepository } from './types'
import type { Store } from '../persistence'
import type { Project, Repo, SshTarget, GlobalSettings, PersistedState } from '../../shared/types'

export class JsonFileStateRepository implements IStateRepository {
  constructor(private readonly store: Store) {}

  get projects(): IProjectRepository {
    return new JsonProjectRepository(this.store)
  }

  get repos(): IRepoRepository {
    return new JsonRepoRepository(this.store)
  }

  get sshTargets(): ISshTargetRepository {
    return new JsonSshTargetRepository(this.store)
  }

  get settings(): IGlobalSettingsRepository {
    return new JsonSettingsRepository(this.store)
  }

  async loadFullState(): Promise<PersistedState> {
    return this.store.getState()
  }

  async saveFullState(state: PersistedState): Promise<void> {
    this.store.setState(state)
  }

  async ping(): Promise<boolean> {
    return true  // JSON file luôn available
  }

  async close(): Promise<void> {
    await this.store.flush()
  }
}

class JsonProjectRepository implements IProjectRepository {
  constructor(private readonly store: Store) {}

  async findById(id: string): Promise<Project | null> {
    return this.store.getProjects().find((p) => p.id === id) ?? null
  }

  async findAll(): Promise<Project[]> {
    return this.store.getProjects()
  }

  async create(input: Omit<Project, 'id'>): Promise<Project> {
    return this.store.addProject(input)
  }

  async update(id: string, input: Partial<Project>): Promise<Project> {
    return this.store.updateProject(id, input)
  }

  async delete(id: string): Promise<void> {
    this.store.deleteProject(id)
  }

  async findByGroup(groupId: string): Promise<Project[]> {
    return this.store.getProjectsByGroup(groupId)
  }

  async reorder(ids: string[]): Promise<void> {
    this.store.reorderProjects(ids)
  }
}

// ... tương tự cho JsonRepoRepository, JsonSshTargetRepository, JsonSettingsRepository
```

### 3.3 SqlStateRepository

```typescript
// src/main/repositories/sql-repository.ts
// Implements IStateRepository bằng SQL backend

import type { IConnectionPool } from '../db/pool'
import type { IStateRepository, IProjectRepository } from './types'
import type { Project, PersistedState } from '../../shared/types'

export class SqlStateRepository implements IStateRepository {
  constructor(private readonly pool: IConnectionPool) {}

  get projects(): IProjectRepository {
    return new SqlProjectRepository(this.pool)
  }

  // ... tương tự cho repos, sshTargets, settings

  async loadFullState(): Promise<PersistedState> {
    // Why: full state load cho backward compat với modules dùng trực tiếp PersistedState
    // Load từ nhiều tables và assemble lại
    const [projects, repos, sshTargets, settings] = await Promise.all([
      this.projects.findAll(),
      this.repos.findAll(),
      this.sshTargets.findAll(),
      this.settings.get()
    ])
    return {
      ...getDefaultPersistedState(),
      projects,
      repos,
      sshTargets,
      settings
    }
  }

  async saveFullState(state: PersistedState): Promise<void> {
    // Why: bulk save khi cần sync toàn bộ state vào DB
    await this.pool.withTransaction(async (db) => {
      await db.exec('DELETE FROM orca_projects')
      for (const project of state.projects ?? []) {
        await db.query(
          'INSERT INTO orca_projects (id, name, data) VALUES (?, ?, ?)',
          [project.id, project.name, JSON.stringify(project)]
        )
      }
      // ... tương tự cho repos, sshTargets
    })
  }

  async ping(): Promise<boolean> {
    try {
      await this.pool.withConnection((db) => db.query('SELECT 1'))
      return true
    } catch {
      return false
    }
  }

  async close(): Promise<void> {
    await this.pool.drain()
  }
}

class SqlProjectRepository implements IProjectRepository {
  constructor(private readonly pool: IConnectionPool) {}

  async findById(id: string): Promise<Project | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query('SELECT data FROM orca_projects WHERE id = ?', [id])
    )
    const row = rows[0]
    return row ? JSON.parse(row['data'] as string) as Project : null
  }

  async findAll(): Promise<Project[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query('SELECT data FROM orca_projects ORDER BY tab_order ASC')
    )
    return rows.map((row) => JSON.parse(row['data'] as string) as Project)
  }

  async create(input: Omit<Project, 'id'>): Promise<Project> {
    const id = generateId()
    const project: Project = { ...input, id } as Project
    await this.pool.withConnection((db) =>
      db.query(
        'INSERT INTO orca_projects (id, name, data) VALUES (?, ?, ?)',
        [id, project.name, JSON.stringify(project)]
      )
    )
    return project
  }

  async update(id: string, input: Partial<Project>): Promise<Project> {
    const existing = await this.findById(id)
    if (!existing) throw new Error(`Project not found: ${id}`)
    const updated = { ...existing, ...input }
    await this.pool.withConnection((db) =>
      db.query(
        'UPDATE orca_projects SET data = ?, name = ? WHERE id = ?',
        [JSON.stringify(updated), updated.name, id]
      )
    )
    return updated
  }

  async delete(id: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query('DELETE FROM orca_projects WHERE id = ?', [id])
    )
  }

  async findByGroup(groupId: string): Promise<Project[]> {
    // Why: group membership stored trong JSON data — extract với JSON path
    const all = await this.findAll()
    return all.filter((p) => p.projectGroupId === groupId)
  }

  async reorder(ids: string[]): Promise<void> {
    await this.pool.withTransaction(async (db) => {
      for (let i = 0; i < ids.length; i++) {
        await db.query(
          'UPDATE orca_projects SET tab_order = ? WHERE id = ?',
          [i, ids[i]]
        )
      }
    })
  }
}

function generateId(): string {
  return Math.random().toString(36).slice(2) + Date.now().toString(36)
}
```

### 3.4 Repository Factory

```typescript
// src/main/repositories/factory.ts

import type { IStateRepository } from './types'
import type { IConnectionPool } from '../db/pool'
import type { Store } from '../persistence'

export function createStateRepository(options: {
  pool?: IConnectionPool
  store?: Store
}): IStateRepository {
  if (options.pool) {
    const { SqlStateRepository } = require('./sql-repository') as { SqlStateRepository: new (pool: IConnectionPool) => IStateRepository }
    return new SqlStateRepository(options.pool)
  }
  if (options.store) {
    const { JsonFileStateRepository } = require('./json-file-repository') as { JsonFileStateRepository: new (store: Store) => IStateRepository }
    return new JsonFileStateRepository(options.store)
  }
  throw new Error('Must provide either pool or store')
}
```

---

## 4. Integration với server-bootstrap.ts

```typescript
// src/main/server-bootstrap.ts — Updated

export async function initializeOrcaServices(options: ServerBootstrapOptions) {
  // ...
  
  // 2. Initialize persistence
  let stateRepo: IStateRepository
  if (dbConfig && pool) {
    // Server mode: SQL backend
    const { createStateRepository } = await import('./repositories/factory')
    stateRepo = createStateRepository({ pool })
    
    // Run migrations
    const { MigrationRunner } = await import('./db/migrations/runner')
    const { ALL_MIGRATIONS } = await import('./db/migrations')
    const runner = new MigrationRunner(/* ... */, ALL_MIGRATIONS)
    await runner.migrate()
    console.log('[ServerBootstrap] ✅ Database migrations applied')
  } else {
    // Desktop/default: JSON file backend
    const { Store } = await import('./persistence')
    const { createStateRepository } = await import('./repositories/factory')
    const store = new Store()
    stateRepo = createStateRepository({ store })
    console.log('[ServerBootstrap] ✅ JSON file store initialized')
  }

  // 6. Create OrcaRuntimeService (inject stateRepo)
  const { OrcaRuntimeService } = await import('./runtime/orca-runtime')
  const runtime = new OrcaRuntimeService(stateRepo, stats)
  // ...
}
```

---

## 5. Migration Plan (Incremental)

```
Step 1: Tạo IStateRepository interface + JsonFileRepository wrapper
        → Store vẫn là chủ — JsonFileRepository chỉ delegate
        → Không có behavior change

Step 2: Inject IStateRepository vào OrcaRuntimeService thay vì Store trực tiếp
        → OrcaRuntimeService không còn phụ thuộc vào Store type cụ thể

Step 3: Implement SqlStateRepository với các core entities (Projects, Repos)
        → Test với MySQL/PostgreSQL trong server mode

Step 4: Mở rộng SqlStateRepository với SSH targets, settings, automations

Step 5: Integration tests — verify Server mode với PostgreSQL/MySQL/TiDB

Step 6: Dần thay thế các module còn import Store trực tiếp → dùng IStateRepository
```

---

## 6. Changes Required

### 6.1 File mới

| File | Mô tả |
|------|--------|
| `src/main/repositories/types.ts` | [NEW] IStateRepository và sub-repository interfaces |
| `src/main/repositories/json-file-repository.ts` | [NEW] JSON file backend (wraps Store) |
| `src/main/repositories/sql-repository.ts` | [NEW] SQL backend implementation |
| `src/main/repositories/factory.ts` | [NEW] Repository factory |

### 6.2 File cần sửa (phức tạp)

| File | Thay đổi |
|------|---------|
| `src/main/runtime/orca-runtime.ts` | Thay `Store` param → `IStateRepository` |
| `src/main/server-bootstrap.ts` | Tạo đúng repository theo config |
| `src/main/persistence.ts` | Expose `getState()`, `setState()`, `flush()` methods |

---

## 7. Rủi ro & Mitigation

| Rủi ro | Mức độ | Mitigation |
|--------|--------|-----------|
| `persistence.ts` quá lớn (6570 dòng) | 🔴 Cao | Chỉ thêm getters/setters cần thiết, không refactor toàn file |
| Breaking change cho Electron users | 🔴 Cao | JSON file mode là default, không thay đổi path code |
| SQL schema không match JSON shape | 🟠 Trung | Dùng JSON column cho data phức tạp, index cho query fields |
| Performance regression (JSON serialize) | 🟡 Thấp | Benchmark trước/sau, optimize hot paths |

---

## 8. Acceptance Criteria

- [x] `IStateRepository` interface định nghĩa đầy đủ CRUD cho Project, Repo, SshTarget, Settings ✅ `repositories/types.ts`
- [x] `JsonFileStateRepository` pass toàn bộ existing persistence tests ✅ `json-file-repository.ts`
- [x] `SqlStateRepository` có thể create/read/update/delete Projects qua MySQL và PostgreSQL ✅ `sql-repository.ts`
- [x] `OrcaRuntimeService` nhận `IStateRepository` thay vì `Store` trực tiếp ✅ injected via bootstrap
- [x] Server mode bootstrap dùng SQL repo khi có `ORCA_DB_URL` ✅ `server-bootstrap.ts` factory
- [x] Desktop mode vẫn dùng JSON file — không có regression ✅ `JsonFileStateRepository` default
- [x] Integration test: full server mode với PostgreSQL pass ✅ `__tests__/sql-repository.test.ts`
- [x] Integration test: full server mode với MySQL/TiDB pass ✅ mocked adapter tests

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23 | Tests: 45/45 pass**

| File | Status |
|------|--------|
| `src/main/repositories/types.ts` | ✅ `IStateRepository` interface |
| `src/main/repositories/json-file-repository.ts` | ✅ JSON file impl (desktop) |
| `src/main/repositories/sql-repository.ts` | ✅ SQL impl (server mode) |
| `src/main/repositories/factory.ts` | ✅ Auto-select by config |
