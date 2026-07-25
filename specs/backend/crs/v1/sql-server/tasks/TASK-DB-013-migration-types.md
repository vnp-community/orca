# TASK-DB-013: Tạo `src/main/db/migrations/types.ts`

**Source:** SOL-DB-003 §4.1  
**Phase:** 2 | **Effort:** XS (< 20 min)  
**Depends on:** TASK-DB-001

---

## Objective

Tạo `src/main/db/migrations/types.ts` — interfaces cho migration framework.

---

## Files to create

### `src/main/db/migrations/types.ts`

```typescript
/**
 * Schema Migration Types
 *
 * Defines the contract for database schema migrations across multiple dialects.
 *
 * @module db/migrations/types
 */

import type { IDatabase, IDatabaseCapabilities } from '../types'

/** Supported migration dialects (subset of IDatabaseCapabilities.dialect) */
export type MigrationDialect = 'all' | IDatabaseCapabilities['dialect']

/**
 * A single schema migration.
 * ID format: YYYYMMDD_HHMMSS_description (sortable chronologically)
 *
 * Example: '20260723_000001_initial_schema'
 */
export interface Migration {
  /** Unique, sortable identifier for this migration */
  readonly id: string
  /** Human-readable description */
  readonly description: string
  /**
   * Which dialect this migration targets.
   * Use 'all' for cross-dialect migrations (ANSI SQL).
   */
  readonly dialect: MigrationDialect
  /** Apply the migration */
  up(db: IDatabase): Promise<void>
  /** Revert the migration (optional) */
  down?(db: IDatabase): Promise<void>
}

/** A record of an applied migration stored in the migrations table */
export interface MigrationRecord {
  /** Migration ID */
  id: string
  /** ISO timestamp when migration was applied */
  appliedAt: string
  /** SHA256 checksum (truncated) for tampering detection */
  checksum: string
  /** Execution time in milliseconds */
  executionMs: number
}

/** Current migration status report */
export interface MigrationStatus {
  /** Migrations not yet applied */
  pending: Migration[]
  /** Records of applied migrations, ordered by applied_at ASC */
  applied: MigrationRecord[]
  /** ID of the most recently applied migration, or null if none */
  current: string | null
}

/** Options for MigrationRunner.migrate() */
export interface MigrateOptions {
  /** If true, log what would be done but don't execute */
  dryRun?: boolean
  /** Stop applying migrations at this ID (inclusive) */
  target?: string
}
```

---

## Verification

```bash
npx tsc --noEmit 2>&1 | grep "migrations/types" | head -5
```

Expected: Zero errors.

---

## Done criteria

- [x] `src/main/db/migrations/types.ts` tồn tại
- [x] Export `Migration`, `MigrationRecord`, `MigrationStatus`, `MigrateOptions`, `MigrationDialect`
- [x] `Migration.id` format documented: YYYYMMDD_HHMMSS_description
- [x] `Migration.down` là optional
- [x] Không có `any`
