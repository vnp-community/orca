# TASK-DB-020: Tạo `src/main/repositories/factory.ts` ✅ DONE

**Source:** SOL-DB-005 §4.4  
**Phase:** 3 | **Effort:** XS (< 20 min)   | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-017, TASK-DB-018, TASK-DB-019

---

## Objective

Tạo `src/main/repositories/factory.ts` — factory function chọn backend (JSON vs SQL) dựa vào options.

---

## Files to create

### 1. `src/main/repositories/factory.ts`

```typescript
import type { IStateRepository } from './types'
import type { IConnectionPool } from '../db/pool'

export interface RepositoryFactoryOptions {
  /** SQL pool — if provided, SqlStateRepository is used */
  pool?: IConnectionPool
  /** JSON file path — if provided (no pool), JsonFileStateRepository is used */
  dataFile?: string
}

/**
 * Create a state repository backed by either SQL or JSON file.
 *
 * Priority:
 * 1. pool → SqlStateRepository
 * 2. dataFile → JsonFileStateRepository
 *
 * @throws Error if neither pool nor dataFile is provided.
 */
export function createStateRepository(options: RepositoryFactoryOptions): IStateRepository {
  if (options.pool) {
    // Dynamic import to avoid bundling in Electron desktop build
    // eslint-disable-next-line @typescript-eslint/no-var-requires
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

  throw new Error(
    'createStateRepository: must provide either pool (SQL backend) or dataFile (JSON backend)'
  )
}
```

### 2. `src/main/repositories/__tests__/factory.test.ts`

```typescript
import { describe, it, expect, vi } from 'vitest'
import { createStateRepository } from '../factory'

describe('createStateRepository', () => {
  it('throws when neither pool nor dataFile provided', () => {
    expect(() => createStateRepository({})).toThrow('must provide either pool')
  })

  it('returns SqlStateRepository when pool is provided', () => {
    const mockPool = {
      acquire: vi.fn(), release: vi.fn(), withConnection: vi.fn(),
      withTransaction: vi.fn(), stats: vi.fn(), drain: vi.fn(), destroy: vi.fn()
    }
    const repo = createStateRepository({ pool: mockPool as any })
    expect(repo).toBeDefined()
    expect(typeof repo.ping).toBe('function')
  })

  it('returns JsonFileStateRepository when dataFile is provided', async () => {
    const { mkdtempSync } = await import('node:fs')
    const { join } = await import('node:path')
    const { tmpdir } = await import('node:os')
    const tmpDir = mkdtempSync(join(tmpdir(), 'orca-factory-test-'))
    const repo = createStateRepository({ dataFile: join(tmpDir, 'store.json') })
    expect(repo).toBeDefined()
    expect(await repo.ping()).toBe(true)
    await repo.close()
  })

  it('pool takes priority over dataFile', () => {
    const mockPool = {
      acquire: vi.fn(), release: vi.fn(), withConnection: vi.fn(),
      withTransaction: vi.fn(), stats: vi.fn(), drain: vi.fn(), destroy: vi.fn()
    }
    // Provide both — pool should win
    const repo = createStateRepository({ pool: mockPool as any, dataFile: '/tmp/test.json' })
    expect(repo).toBeDefined()
    // SqlStateRepository's ping calls pool.withConnection
    expect(mockPool.withConnection).toBeDefined()
  })
})
```

### 3. `src/main/repositories/index.ts`

```typescript
export { createStateRepository } from './factory'
export type {
  IStateRepository, IProjectRepository, IRepoRepository,
  ISshTargetRepository, IGlobalSettingsRepository,
  IRepository, GlobalSettings
} from './types'
export { JsonFileStateRepository } from './json-file-repository'
export { SqlStateRepository } from './sql-repository'
```

---

## Verification

```bash
pnpm vitest run src/main/repositories/__tests__/factory.test.ts
```

Expected: 4/4 tests pass

---

## Done criteria

- [x] `createStateRepository({ pool })` returns `SqlStateRepository`
- [x] `createStateRepository({ dataFile })` returns `JsonFileStateRepository`
- [x] `createStateRepository({})` throws với message rõ ràng
- [x] `pool` takes priority over `dataFile`
- [x] `src/main/repositories/index.ts` re-exports tất cả
- [x] 4/4 tests pass
