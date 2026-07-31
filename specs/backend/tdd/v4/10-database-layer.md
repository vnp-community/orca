# TDD-BE-10: Database Layer

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/main/db/`, `src/main/repositories/`

---

## 1. Module Map

| File | Role |
|------|------|
| `db/types.ts` | `ISyncDatabase`, `IAsyncDatabase`, `IRow` |
| `db/registry.ts` | Adapter registry (dialect → factory) |
| `db/pool.ts` | `IConnectionPool` interface + factory |
| `db/config.ts` | `DatabaseConfig`, env loading |
| `db/dsn-parser.ts` | Parse/format DSN strings |
| `db/config-loader.ts` | `loadDatabaseConfig()` from env |
| `db/health.ts` | `HealthChecker` class |
| `db/health-monitor.ts` | Periodic health checks |
| `db/migrations/` | Migration files 0001→0005 |
| `db/adapters/sqlite.ts` | SQLite adapter (better-sqlite3) |
| `db/adapters/mysql.ts` | MySQL adapter |
| `db/adapters/postgres.ts` | PostgreSQL adapter |
| `repositories/` | `IStateRepository`, adapters, factory |

---

## 2. `IConnectionPool`

```typescript
export interface IConnectionPool {
  dialect: 'sqlite' | 'mysql' | 'postgres'
  // Sync execute (SQLite only)
  executeSync(sql: string, params?: unknown[]): IRow[]
  // Async execute (all dialects)
  execute(sql: string, params?: unknown[]): Promise<IRow[]>
  // Transaction
  transaction<T>(fn: (pool: IConnectionPool) => Promise<T>): Promise<T>
  // Health check
  ping(): Promise<void>
  // Close
  close(): Promise<void>
}
```

---

## 3. Database Config

```typescript
export type DatabaseConfig = {
  dialect:  'sqlite' | 'mysql' | 'postgres'
  filename?: string   // SQLite only (default: userData/orca.db)
  host?:     string
  port?:     number
  database?: string
  user?:     string
  password?: string
  ssl?:      boolean
}
```

**loadDatabaseConfig() từ env:**
- `ORCA_DB_URL` → parseDsn() → DatabaseConfig
- `ORCA_DB_DIALECT=mysql|postgres` → partial config
- Default: `{ dialect: 'sqlite', filename: userData/orca.db }`

---

## 4. Migrations (0001→0005)

| Migration | Tables |
|-----------|--------|
| 0001_initial | `workspace_state` (legacy JSON blob) |
| 0002_dev_servers | `dev_servers` (persisted DevServer records) |
| 0003_notifications | `push_subscriptions` |
| 0004_fleet | `fleet_servers`, `fleet_groups`, `access_policies` |
| 0005_add_auth_schema | `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies` |

**Auto-migration:** chạy tự động khi `initConnectionPool()` được gọi trong server-bootstrap.

---

## 5. `IStateRepository`

```typescript
export interface IStateRepository {
  // Thay thế JSON Store cho SQL backends
  get<T>(key: string): Promise<T | undefined>
  set<T>(key: string, value: T): Promise<void>
  delete(key: string): Promise<void>
  list(): Promise<string[]>
}
```

**Implementations:**
- `JsonFileRepository` — wrap Store (Electron compat, backward compat)
- `SqlStateRepository` — SQL backend (`state_kv` table với TEXT key, TEXT value JSON)

---

## 6. HealthChecker

```typescript
class HealthChecker {
  // Periodic check (default: 10s interval)
  start(): void
  stop(): void

  // Livenss probe (any DB connected?)
  isHealthy(): boolean

  // Readiness probe (all dependencies up?)
  isReady(): boolean

  // Metrics (query count, error rate, latency)
  getMetrics(): DbMetrics
}
```

---

## 7. DSN Parser

```typescript
// Input: postgresql://user:pass@host:5432/dbname
parseDsn(dsn: string): DatabaseConfig

// Output: postgresql://user:***@host:5432/dbname (password masked)
formatDsn(config: DatabaseConfig): string
```

**Supported formats:**
- `sqlite:///path/to/db.sqlite3` hoặc bare file path
- `mysql://user:pass@host:3306/db`
- `postgresql://user:pass@host:5432/db` (alias: `postgres://`)
- `tidb://...` (alias cho MySQL)

---

## 8. Tests (205 tests)

| Module | Tests |
|--------|-------|
| `db/adapters/sqlite.test.ts` | 40 |
| `db/adapters/mysql.test.ts` | 35 |
| `db/adapters/postgres.test.ts` | 35 |
| `db/migrations.test.ts` | 30 |
| `db/health.test.ts` | 25 |
| `db/dsn-parser.test.ts` | 20 |
| `repositories/sql-state-repository.test.ts` | 20 |
