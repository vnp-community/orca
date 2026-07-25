# TASK-DB-003: Tạo `src/main/db/config.ts` — Zod DatabaseConfig schemas ✅ DONE

**Source:** SOL-DB-004 §4.1  
**Phase:** 1 | **Effort:** S (30–45 min) | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** —

---

## Objective

Tạo `src/main/db/config.ts` với Zod schemas cho `DatabaseConfig` (discriminated union: sqlite | mysql | tidb | mariadb | postgresql). File này không depend vào types.ts — chỉ cần `zod`.

---

## Context cần đọc

- SOL-DB-004 §4.1 (DatabaseConfig schema design)

---

## Files to create

### 1. `src/main/db/config.ts`

```typescript
/**
 * Database Configuration Schemas
 *
 * Zod schemas for type-safe database configuration.
 * Used by: DSN parser, config loader, server bootstrap.
 *
 * @module db/config
 */

import { z } from 'zod'

// ── Pool Config ─────────────────────────────────────────────────────────────

const PoolConfigSchema = z.object({
  /** Minimum connections to keep alive in pool */
  min: z.number().int().min(0).default(2),
  /** Maximum connections allowed in pool */
  max: z.number().int().min(1).default(10),
  /** Milliseconds to wait for connection before timeout */
  acquireTimeoutMs: z.number().int().positive().default(5_000),
  /** Milliseconds before an idle connection is destroyed */
  idleTimeoutMs: z.number().int().positive().default(30_000),
  /** Number of retries when creating a connection fails */
  connectionRetries: z.number().int().min(0).default(3),
  /** Milliseconds between connection retry attempts */
  retryDelayMs: z.number().int().positive().default(500)
})

// ── Dialect-specific schemas ─────────────────────────────────────────────────

const SqliteConfigSchema = z.object({
  dialect: z.literal('sqlite'),
  /** File path or ':memory:' for in-memory database */
  path: z.string().min(1),
  /** Open database in read-only mode */
  readonly: z.boolean().default(false)
})

const MysqlConfigSchema = z.object({
  dialect: z.union([
    z.literal('mysql'),
    z.literal('tidb'),
    z.literal('mariadb')
  ]),
  host: z.string().min(1),
  port: z.number().int().min(1).max(65535).default(3306),
  database: z.string().min(1),
  username: z.string().min(1),
  password: z.string().default(''),
  ssl: z.boolean().optional(),
  pool: PoolConfigSchema.optional()
})

const PostgresConfigSchema = z.object({
  dialect: z.literal('postgresql'),
  host: z.string().min(1),
  port: z.number().int().min(1).max(65535).default(5432),
  database: z.string().min(1),
  username: z.string().min(1),
  password: z.string().default(''),
  ssl: z.boolean().optional(),
  /** PostgreSQL schema name (default: 'public') */
  schema: z.string().default('public'),
  pool: PoolConfigSchema.optional()
})

// ── Main discriminated union ─────────────────────────────────────────────────

export const DatabaseConfigSchema = z.discriminatedUnion('dialect', [
  SqliteConfigSchema,
  MysqlConfigSchema,
  PostgresConfigSchema
])

// ── Exported types ───────────────────────────────────────────────────────────

export type DatabaseConfig = z.infer<typeof DatabaseConfigSchema>
export type PoolConfig = z.infer<typeof PoolConfigSchema>
export type SqliteConfig = z.infer<typeof SqliteConfigSchema>
export type MysqlConfig = z.infer<typeof MysqlConfigSchema>
export type PostgresConfig = z.infer<typeof PostgresConfigSchema>

export { PoolConfigSchema }
```

### 2. `src/main/db/__tests__/config.test.ts`

```typescript
import { describe, it, expect } from 'vitest'
import { DatabaseConfigSchema } from '../config'

describe('DatabaseConfigSchema', () => {
  describe('sqlite', () => {
    it('validates minimal SQLite config', () => {
      const result = DatabaseConfigSchema.safeParse({ dialect: 'sqlite', path: '/data/db.sqlite' })
      expect(result.success).toBe(true)
    })

    it('defaults readonly to false', () => {
      const result = DatabaseConfigSchema.safeParse({ dialect: 'sqlite', path: ':memory:' })
      expect(result.success && result.data.readonly).toBe(false)
    })

    it('rejects sqlite without path', () => {
      const result = DatabaseConfigSchema.safeParse({ dialect: 'sqlite' })
      expect(result.success).toBe(false)
    })
  })

  describe('mysql', () => {
    const base = { dialect: 'mysql', host: 'localhost', database: 'orca', username: 'root' }

    it('validates minimal MySQL config', () => {
      const result = DatabaseConfigSchema.safeParse(base)
      expect(result.success).toBe(true)
    })

    it('defaults port to 3306', () => {
      const result = DatabaseConfigSchema.safeParse(base)
      expect(result.success && result.data.dialect !== 'sqlite' && result.data.port).toBe(3306)
    })

    it('defaults password to empty string', () => {
      const result = DatabaseConfigSchema.safeParse(base)
      expect(result.success && result.data.dialect !== 'sqlite' && result.data.password).toBe('')
    })

    it('validates tidb dialect', () => {
      const result = DatabaseConfigSchema.safeParse({ ...base, dialect: 'tidb', port: 4000 })
      expect(result.success).toBe(true)
    })

    it('validates mariadb dialect', () => {
      const result = DatabaseConfigSchema.safeParse({ ...base, dialect: 'mariadb' })
      expect(result.success).toBe(true)
    })

    it('rejects mysql without host', () => {
      const result = DatabaseConfigSchema.safeParse({ dialect: 'mysql', database: 'orca', username: 'root' })
      expect(result.success).toBe(false)
    })

    it('validates pool config with defaults', () => {
      const result = DatabaseConfigSchema.safeParse({ ...base, pool: {} })
      expect(result.success).toBe(true)
      if (result.success && result.data.dialect === 'mysql') {
        expect(result.data.pool?.min).toBe(2)
        expect(result.data.pool?.max).toBe(10)
        expect(result.data.pool?.acquireTimeoutMs).toBe(5000)
      }
    })
  })

  describe('postgresql', () => {
    const base = { dialect: 'postgresql', host: 'pg.host', database: 'orca', username: 'orca_user' }

    it('validates minimal PostgreSQL config', () => {
      const result = DatabaseConfigSchema.safeParse(base)
      expect(result.success).toBe(true)
    })

    it('defaults port to 5432', () => {
      const result = DatabaseConfigSchema.safeParse(base)
      expect(result.success && result.data.dialect === 'postgresql' && result.data.port).toBe(5432)
    })

    it('defaults schema to public', () => {
      const result = DatabaseConfigSchema.safeParse(base)
      expect(result.success && result.data.dialect === 'postgresql' && result.data.schema).toBe('public')
    })
  })

  describe('invalid', () => {
    it('rejects unknown dialect', () => {
      const result = DatabaseConfigSchema.safeParse({ dialect: 'redis', host: 'localhost' })
      expect(result.success).toBe(false)
    })

    it('rejects port out of range', () => {
      const result = DatabaseConfigSchema.safeParse({
        dialect: 'mysql', host: 'h', database: 'd', username: 'u', port: 99999
      })
      expect(result.success).toBe(false)
    })
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# Check zod is available
node -e "require('zod')" && echo "zod OK"

# TypeScript compile
npx tsc --noEmit 2>&1 | grep "db/config" | head -10

# Run tests
pnpm vitest run src/main/db/__tests__/config.test.ts
```

Expected:
- 14/14 tests pass
- Zero TS errors

---

## Done criteria

- [x] `src/main/db/config.ts` tồn tại
- [x] Export `DatabaseConfigSchema`, `DatabaseConfig`, `SqliteConfig`, `MysqlConfig`, `PostgresConfig`, `PoolConfig`
- [x] `discriminatedUnion` trên `dialect` field
- [x] Default values đúng: mysql port=3306, pg port=5432, pool.min=2, pool.max=10
- [x] `src/main/db/__tests__/config.test.ts` pass 15 tests (15 tests run, vượt 14 target)
