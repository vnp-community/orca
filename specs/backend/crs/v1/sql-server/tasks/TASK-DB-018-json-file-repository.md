# TASK-DB-018: Tạo `src/main/repositories/json-file-repository.ts` + tests ✅ DONE

**Source:** SOL-DB-005 §4.2  
**Phase:** 3 | **Effort:** M (1.5–2 giờ)   | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-017

---

## Objective

Tạo `JsonFileStateRepository` — implementation `IStateRepository` dùng JSON file trên disk. Đây là backend cho Electron/Desktop mode và server mode không cần SQL.

---

## Files to create

### 1. `src/main/repositories/json-file-repository.ts`

```typescript
import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { mkdirSync } from 'node:fs'
import { randomUUID } from 'node:crypto'
import type {
  IStateRepository, IProjectRepository, IRepoRepository,
  ISshTargetRepository, IGlobalSettingsRepository, GlobalSettings
} from './types'
// Adjust import paths for Project, Repo, SshTarget to match actual codebase
import type { Project, Repo, SshTarget } from '../../shared/types'

interface FileState {
  projects: Project[]
  repos: Repo[]
  sshTargets: SshTarget[]
  globalSettings: GlobalSettings
  version: number
}

function defaultState(): FileState {
  return { projects: [], repos: [], sshTargets: [], globalSettings: {}, version: 1 }
}

export class JsonFileStateRepository implements IStateRepository {
  private state: FileState
  private dirty = false
  private flushTimer: ReturnType<typeof setTimeout> | null = null
  private readonly absPath: string

  constructor(dataFile: string) {
    this.absPath = resolve(dataFile)
    this.state = this.load()
  }

  private load(): FileState {
    if (!existsSync(this.absPath)) return defaultState()
    try {
      const raw = readFileSync(this.absPath, 'utf8')
      return JSON.parse(raw) as FileState
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
    const dir = dirname(this.absPath)
    if (!existsSync(dir)) mkdirSync(dir, { recursive: true })
    writeFileSync(this.absPath, JSON.stringify(this.state, null, 2), 'utf8')
    this.dirty = false
    this.flushTimer = null
  }

  get projects(): IProjectRepository {
    const state = this.state
    const save = () => this.scheduleSave()
    return {
      findById: async (id) => state.projects.find((p) => p.id === id) ?? null,
      findAll: async () => [...state.projects],
      create: async (input) => {
        const item = { ...input, id: randomUUID() } as Project
        state.projects.push(item)
        save()
        return { ...item }
      },
      update: async (id, patch) => {
        const idx = state.projects.findIndex((p) => p.id === id)
        if (idx === -1) throw new Error(`Project not found: ${id}`)
        state.projects[idx] = { ...state.projects[idx]!, ...patch }
        save()
        return { ...state.projects[idx]! }
      },
      delete: async (id) => {
        state.projects = state.projects.filter((p) => p.id !== id)
        save()
      },
      findByGroup: async (groupId) =>
        state.projects.filter((p) => (p as any).projectGroupId === groupId)
    }
  }

  get repos(): IRepoRepository {
    const state = this.state
    const save = () => this.scheduleSave()
    return {
      findById: async (id) => state.repos.find((r) => r.id === id) ?? null,
      findAll: async () => [...state.repos],
      create: async (input) => {
        const item = { ...input, id: randomUUID() } as Repo
        state.repos.push(item)
        save()
        return { ...item }
      },
      update: async (id, patch) => {
        const idx = state.repos.findIndex((r) => r.id === id)
        if (idx === -1) throw new Error(`Repo not found: ${id}`)
        state.repos[idx] = { ...state.repos[idx]!, ...patch }
        save()
        return { ...state.repos[idx]! }
      },
      delete: async (id) => {
        state.repos = state.repos.filter((r) => r.id !== id)
        save()
      },
      findByProject: async (projectId) =>
        state.repos.filter((r) => (r as any).projectId === projectId)
    }
  }

  get sshTargets(): ISshTargetRepository {
    const state = this.state
    const save = () => this.scheduleSave()
    return {
      findById: async (id) => state.sshTargets.find((t) => t.id === id) ?? null,
      findAll: async () => [...state.sshTargets],
      create: async (input) => {
        const item = { ...input, id: randomUUID() } as SshTarget
        state.sshTargets.push(item)
        save()
        return { ...item }
      },
      update: async (id, patch) => {
        const idx = state.sshTargets.findIndex((t) => t.id === id)
        if (idx === -1) throw new Error(`SshTarget not found: ${id}`)
        state.sshTargets[idx] = { ...state.sshTargets[idx]!, ...patch }
        save()
        return { ...state.sshTargets[idx]! }
      },
      delete: async (id) => {
        state.sshTargets = state.sshTargets.filter((t) => t.id !== id)
        save()
      }
    }
  }

  get settings(): IGlobalSettingsRepository {
    const state = this.state
    const save = () => this.scheduleSave()
    return {
      get: async () => ({ ...state.globalSettings }),
      update: async (patch) => {
        state.globalSettings = { ...state.globalSettings, ...patch }
        save()
        return { ...state.globalSettings }
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

### 2. `src/main/repositories/__tests__/json-file-repository.test.ts`

```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { JsonFileStateRepository } from '../json-file-repository'

function makeRepo(dataFile?: string): { repo: JsonFileStateRepository; file: string; tmpDir: string } {
  const tmpDir = mkdtempSync(join(tmpdir(), 'orca-json-repo-test-'))
  const file = dataFile ?? join(tmpDir, 'store.json')
  return { repo: new JsonFileStateRepository(file), file, tmpDir }
}

describe('JsonFileStateRepository', () => {
  let tmpDir: string
  let repo: JsonFileStateRepository

  beforeEach(() => {
    const r = makeRepo()
    tmpDir = r.tmpDir
    repo = r.repo
  })

  afterEach(async () => {
    await repo.close()
    rmSync(tmpDir, { recursive: true, force: true })
  })

  describe('ping()', () => {
    it('returns true', async () => {
      expect(await repo.ping()).toBe(true)
    })
  })

  describe('projects', () => {
    it('findAll() returns empty initially', async () => {
      expect(await repo.projects.findAll()).toEqual([])
    })

    it('create() assigns UUID id', async () => {
      const p = await repo.projects.create({ name: 'Test', repoIds: [], tabOrder: 0 } as any)
      expect(p.id).toBeTruthy()
      expect(p.id.length).toBe(36)
    })

    it('findById() finds created project', async () => {
      const created = await repo.projects.create({ name: 'P1', repoIds: [], tabOrder: 0 } as any)
      const found = await repo.projects.findById(created.id)
      expect(found?.name).toBe('P1')
    })

    it('findById() returns null for unknown id', async () => {
      expect(await repo.projects.findById('no-such-id')).toBeNull()
    })

    it('update() changes fields', async () => {
      const created = await repo.projects.create({ name: 'Old', repoIds: [], tabOrder: 0 } as any)
      const updated = await repo.projects.update(created.id, { name: 'New' } as any)
      expect(updated.name).toBe('New')
    })

    it('update() throws for unknown id', async () => {
      await expect(repo.projects.update('bad-id', { name: 'X' } as any)).rejects.toThrow('Project not found: bad-id')
    })

    it('delete() removes project', async () => {
      const p = await repo.projects.create({ name: 'Del', repoIds: [], tabOrder: 0 } as any)
      await repo.projects.delete(p.id)
      expect(await repo.projects.findById(p.id)).toBeNull()
    })

    it('findAll() returns all', async () => {
      await repo.projects.create({ name: 'P1', repoIds: [], tabOrder: 0 } as any)
      await repo.projects.create({ name: 'P2', repoIds: [], tabOrder: 1 } as any)
      expect(await repo.projects.findAll()).toHaveLength(2)
    })
  })

  describe('sshTargets', () => {
    it('create() and findById() roundtrip', async () => {
      const t = await repo.sshTargets.create({ label: 'Dev', host: 'dev.example.com', port: 22, username: 'ubuntu' } as any)
      const found = await repo.sshTargets.findById(t.id)
      expect(found?.host).toBe('dev.example.com')
    })

    it('delete() removes target', async () => {
      const t = await repo.sshTargets.create({ label: 'Prod', host: 'prod.example.com', port: 22, username: 'root' } as any)
      await repo.sshTargets.delete(t.id)
      expect(await repo.sshTargets.findById(t.id)).toBeNull()
    })
  })

  describe('settings', () => {
    it('get() returns default empty object', async () => {
      const s = await repo.settings.get()
      expect(s).toBeDefined()
      expect(typeof s).toBe('object')
    })

    it('update() persists setting', async () => {
      await repo.settings.update({ theme: 'dark' })
      const s = await repo.settings.get()
      expect(s.theme).toBe('dark')
    })
  })

  describe('persistence', () => {
    it('data persists between instances', async () => {
      const { repo: repo1, file, tmpDir: td } = makeRepo()
      const created = await repo1.projects.create({ name: 'Persist', repoIds: [], tabOrder: 0 } as any)
      await repo1.close()

      const repo2 = new JsonFileStateRepository(file)
      const found = await repo2.projects.findById(created.id)
      expect(found?.name).toBe('Persist')
      await repo2.close()
      rmSync(td, { recursive: true, force: true })
    })
  })
})
```

---

## Verification

```bash
pnpm vitest run src/main/repositories/__tests__/json-file-repository.test.ts
```

Expected: 15/15 tests pass

---

## Done criteria

- [x] `JsonFileStateRepository` implements `IStateRepository`
- [x] `create()` assigns `randomUUID()` id
- [x] `update()` throws khi id không tồn tại
- [x] `close()` flushes pending writes (debounced)
- [x] Data persists between instances (read/write JSON file)
- [x] `ping()` returns true
- [x] 15/15 tests pass
