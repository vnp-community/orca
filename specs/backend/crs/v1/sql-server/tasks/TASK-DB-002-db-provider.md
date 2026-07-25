# TASK-DB-002: Tạo `src/main/db/provider.ts` — DatabaseProvider registry ✅ DONE

**Source:** SOL-DB-001 §4.2  
**Phase:** 1 | **Effort:** XS (< 30 min) | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-001

---

## Objective

Tạo `src/main/db/provider.ts` — registry pattern để các database adapters tự register, và factory function `createDatabase()`.

---

## Context cần đọc

- `src/main/db/types.ts` (TASK-DB-001)
- SOL-DB-001 §4.2

---

## Files to create

### 1. `src/main/db/provider.ts`

```typescript
/**
 * Database Provider Registry
 *
 * Adapters tự register khi được import (side-effect):
 *   import '../db/sqlite/sqlite-adapter'  // registers 'sqlite' provider
 *   import '../db/mysql/mysql-adapter'    // registers 'mysql' provider
 *
 * @module db/provider
 */

import type { IDatabase, DatabaseProvider, IDatabaseCapabilities, DatabaseConfig } from './types'

type Dialect = IDatabaseCapabilities['dialect']

const _registry = new Map<Dialect, DatabaseProvider>()

/**
 * Register a database provider for a dialect.
 * If a provider already exists for that dialect, it will be overwritten (last-write-wins).
 */
export function registerDatabaseProvider(provider: DatabaseProvider): void {
  _registry.set(provider.dialect, provider)
}

/**
 * Retrieve a registered provider by dialect.
 * @throws Error if no provider registered for the requested dialect.
 */
export function getDatabaseProvider(dialect: Dialect): DatabaseProvider {
  const provider = _registry.get(dialect)
  if (!provider) {
    const available = [..._registry.keys()].join(', ') || '(none registered)'
    throw new Error(
      `No database provider registered for dialect: "${dialect}". ` +
      `Available dialects: ${available}. ` +
      `Make sure to import the adapter before calling createDatabase().`
    )
  }
  return provider
}

/**
 * Create a database connection using the registered provider for config.dialect.
 * @throws Error if no provider registered for config.dialect.
 */
export async function createDatabase(config: DatabaseConfig): Promise<IDatabase> {
  const provider = getDatabaseProvider(config.dialect as Dialect)
  return provider.connect(config)
}

/**
 * FOR TESTING ONLY — reset all registered providers.
 * Do not call in production code.
 * @internal
 */
export function clearProviderRegistry(): void {
  _registry.clear()
}

/**
 * FOR TESTING ONLY — get a snapshot of registered dialects.
 * @internal
 */
export function getRegisteredDialects(): Dialect[] {
  return [..._registry.keys()]
}
```

### 2. `src/main/db/__tests__/provider.test.ts`

```typescript
import { describe, it, expect, beforeEach } from 'vitest'
import {
  registerDatabaseProvider,
  getDatabaseProvider,
  createDatabase,
  clearProviderRegistry,
  getRegisteredDialects
} from '../provider'
import type { DatabaseProvider, IDatabase, IDatabaseCapabilities } from '../types'

function makeMockDb(): IDatabase {
  return {
    capabilities: { dialect: 'sqlite', walMode: true, returning: false, nativeJson: false, placeholderStyle: 'positional' },
    exec: () => {},
    prepare: () => ({ run: () => ({ changes: 0, lastInsertRowid: 0 }), get: () => undefined, all: () => [] }),
    close: () => {},
    transaction: async (fn) => fn(),
    query: async () => []
  }
}

describe('DatabaseProvider Registry', () => {
  beforeEach(() => {
    clearProviderRegistry()
  })

  it('getRegisteredDialects() returns empty array initially', () => {
    expect(getRegisteredDialects()).toEqual([])
  })

  it('registerDatabaseProvider() registers a provider', () => {
    const provider: DatabaseProvider = { dialect: 'sqlite', connect: async () => makeMockDb() }
    registerDatabaseProvider(provider)
    expect(getDatabaseProvider('sqlite')).toBe(provider)
  })

  it('getDatabaseProvider() throws for unregistered dialect', () => {
    expect(() => getDatabaseProvider('mysql')).toThrow('No database provider registered for dialect: "mysql"')
  })

  it('getDatabaseProvider() error message lists available dialects', () => {
    registerDatabaseProvider({ dialect: 'postgresql', connect: async () => makeMockDb() })
    try {
      getDatabaseProvider('mysql')
    } catch (err) {
      expect((err as Error).message).toContain('postgresql')
    }
  })

  it('createDatabase() delegates to provider.connect()', async () => {
    const mockDb = makeMockDb()
    const connectSpy = vi.fn().mockResolvedValue(mockDb)
    registerDatabaseProvider({ dialect: 'sqlite', connect: connectSpy })

    const db = await createDatabase({ dialect: 'sqlite', path: ':memory:' })
    expect(db).toBe(mockDb)
    expect(connectSpy).toHaveBeenCalledOnce()
  })

  it('registerDatabaseProvider() overwrites existing provider for same dialect', () => {
    const p1: DatabaseProvider = { dialect: 'sqlite', connect: async () => makeMockDb() }
    const p2: DatabaseProvider = { dialect: 'sqlite', connect: async () => makeMockDb() }
    registerDatabaseProvider(p1)
    registerDatabaseProvider(p2)
    expect(getDatabaseProvider('sqlite')).toBe(p2)
  })

  it('getRegisteredDialects() lists all registered dialects', () => {
    registerDatabaseProvider({ dialect: 'sqlite', connect: async () => makeMockDb() })
    registerDatabaseProvider({ dialect: 'mysql', connect: async () => makeMockDb() })
    const dialects = getRegisteredDialects()
    expect(dialects).toContain('sqlite')
    expect(dialects).toContain('mysql')
  })

  it('clearProviderRegistry() removes all providers', () => {
    registerDatabaseProvider({ dialect: 'sqlite', connect: async () => makeMockDb() })
    clearProviderRegistry()
    expect(getRegisteredDialects()).toHaveLength(0)
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# TypeScript compile
npx tsc --noEmit 2>&1 | grep "db/provider" | head -10

# Run tests
pnpm vitest run src/main/db/__tests__/provider.test.ts
```

Expected:
- 8/8 tests pass
- Zero TS errors

---

## Done criteria

- [x] `src/main/db/provider.ts` tồn tại với 4 exports
- [x] `src/main/db/__tests__/provider.test.ts` pass 8 tests
- [x] `clearProviderRegistry()` và `getRegisteredDialects()` chỉ dùng cho test
- [x] `getDatabaseProvider()` error message bao gồm danh sách dialects có sẵn
- [x] Không có `import 'electron'`
