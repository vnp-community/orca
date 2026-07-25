# TASK-DB-001: Tạo `src/main/db/types.ts` — IDatabase interfaces ✅ DONE

**Source:** SOL-DB-001 §4.1  
**Phase:** 1 | **Effort:** XS (< 30 min) | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** —

---

## Objective

Tạo file `src/main/db/types.ts` định nghĩa toàn bộ TypeScript interfaces cho database abstraction layer: `IDatabase`, `ISyncDatabase`, `IAsyncDatabase`, `IStatement`, `IDatabaseCapabilities`, `DatabaseProvider`.

---

## Context cần đọc

- `src/main/sqlite/sync-database.ts` — hiểu API hiện tại của SyncDatabase
- `specs/backend/crs/v1/sql-server/solutions/SOL-DB-001-database-provider-abstraction.md` §4.1

---

## Files to create

### `src/main/db/types.ts`

```typescript
/**
 * Database Abstraction Layer — Core Types
 *
 * Defines interfaces for multi-database support (SQLite, MySQL, PostgreSQL, TiDB).
 * Implementations: SqliteAdapter, MySQLAdapter, PgAdapter.
 *
 * @module db/types
 */

/** Allowed bind parameter values for SQL statements */
export type BindValue = string | number | bigint | Buffer | null | undefined

/** Result of a DML statement (INSERT/UPDATE/DELETE) */
export interface StatementResult {
  changes: number
  lastInsertRowid: number | bigint
}

/** A prepared SQL statement (sync or async depending on backend) */
export interface IStatement {
  run(...params: BindValue[]): StatementResult | Promise<StatementResult>
  get(...params: BindValue[]): Record<string, unknown> | undefined | Promise<Record<string, unknown> | undefined>
  all(...params: BindValue[]): Record<string, unknown>[] | Promise<Record<string, unknown>[]>
  iterate?(...params: BindValue[]): IterableIterator<Record<string, unknown>>
}

/** Capabilities advertised by a database adapter */
export interface IDatabaseCapabilities {
  /** Whether WAL journal mode is active (SQLite only) */
  walMode: boolean
  /** Whether RETURNING clause is supported */
  returning: boolean
  /** Whether JSON functions are natively available */
  nativeJson: boolean
  /** Placeholder style: positional (?), named (:name), or both */
  placeholderStyle: 'positional' | 'named' | 'both'
  /** Database dialect identifier */
  dialect: 'sqlite' | 'mysql' | 'tidb' | 'mariadb' | 'postgresql'
}

/** Base database interface — works for both sync and async adapters */
export interface IDatabase {
  exec(sql: string): void | Promise<void>
  prepare(sql: string): IStatement | Promise<IStatement>
  close(): void | Promise<void>
  readonly capabilities: IDatabaseCapabilities
  transaction<T>(fn: () => T | Promise<T>): Promise<T>
  query(sql: string, params?: BindValue[]): Promise<Record<string, unknown>[]>
}

/**
 * Synchronous database interface — for SQLite (node:sqlite DatabaseSync).
 * exec/prepare/close are synchronous; transaction/query are async wrappers.
 */
export interface ISyncDatabase extends IDatabase {
  exec(sql: string): void
  prepare(sql: string): IStatement
  close(): void
  /** SQLite-specific PRAGMA accessor */
  pragma?(sql: string, options?: { simple?: boolean }): unknown
}

/**
 * Asynchronous database interface — for MySQL, PostgreSQL, TiDB.
 * All methods return Promises.
 */
export interface IAsyncDatabase extends IDatabase {
  exec(sql: string): Promise<void>
  prepare(sql: string): Promise<IStatement>
  close(): Promise<void>
}

/**
 * Factory for creating database connections.
 * Each dialect registers a provider via registerDatabaseProvider().
 */
export interface DatabaseProvider {
  readonly dialect: IDatabaseCapabilities['dialect']
  connect(config: DatabaseConfig): Promise<IDatabase>
}

// Forward declaration — DatabaseConfig is defined in config.ts
// Imported here to avoid circular dependency at runtime
import type { DatabaseConfig } from './config'
export type { DatabaseConfig }
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# TypeScript compile — file phải không có lỗi
npx tsc --noEmit 2>&1 | grep "db/types" | head -20

# Verify exports
node -e "const t = require('./src/main/db/types'); console.log('OK')" 2>&1 || echo "TS only — OK if no import error"
```

Expected:
- Zero TS errors trong `src/main/db/types.ts`
- Không có `import 'electron'`

---

## Done criteria

- [x] `src/main/db/types.ts` tồn tại
- [x] Export `BindValue`, `StatementResult`, `IStatement`, `IDatabaseCapabilities`
- [x] Export `IDatabase`, `ISyncDatabase`, `IAsyncDatabase`, `DatabaseProvider`
- [x] Không có `any` trong type definitions
- [x] Không có `import` từ `'electron'`
- [x] `pnpm tsc --noEmit` pass (no errors in db/types.ts)
