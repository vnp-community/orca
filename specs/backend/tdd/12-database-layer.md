# TDD-12: Database Abstraction Layer

**Document:** TDD-12 (NEW — sql-server CRs)  
**Version:** 1.0  
**Date:** 2026-07-23  
**Domain:** Multi-dialect Database Support — SQLite, MySQL, PostgreSQL, TiDB  
**Source files:**
- `src/main/db/` (provider abstraction, connection pool, migrations, health)
- `src/main/repositories/` (state repository pattern)
- `src/server/health-endpoint.ts`

> **Status: ✅ IMPLEMENTED** — 6/6 solutions | 16 test files | 205 tests | All pass

---

## 1. Mục tiêu

Orca Server Mode hỗ trợ **nhiều database backends**:

```
Trước (v1.x):
  src/main/persistence.ts → JSON file store (PersistedState)
  src/sqlite/ → direct better-sqlite3 access

Sau (v2.0 + sql-server CRs):
  src/main/db/           → Provider abstraction + pool + migrations + health
  src/main/repositories/ → IStateRepository (uniform API, JSON or SQL backend)
  src/server/            → /health, /health/ready, /health/metrics endpoints

  Electron mode:   JSON file store (backward compat, unchanged)
  Server mode:     SQLite | MySQL | PostgreSQL | TiDB (via ORCA_DB_URL)
```

---

## 2. Database Provider Abstraction (`src/main/db/`)

### 2.1 Core Interfaces (`db/types.ts`)

```typescript
type Dialect = 'sqlite' | 'mysql' | 'postgresql' | 'tidb'

interface DatabaseCapabilities {
  dialect: Dialect
  walMode: boolean                    // SQLite WAL mode
  placeholderStyle: 'positional' | 'named'  // ? vs $1/$name
  supportsReturning: boolean
  supportsCTE: boolean
  maxConnections: number
}

interface IDatabase {
  capabilities: DatabaseCapabilities
  query<T = Record<string, unknown>>(sql: string, params?: unknown[]): Promise<T[]>
  exec(sql: string): Promise<void>
  prepare(sql: string): IPreparedStatement
  transaction<T>(fn: (db: IDatabase) => Promise<T>): Promise<T>
  close(): void
}

interface IDatabaseProvider {
  readonly dialect: Dialect
  connect(config: DatabaseConfig): Promise<IDatabase>
}
```

**Provider Registry Pattern:**
```typescript
// src/main/db/registry.ts
const providers = new Map<Dialect, IDatabaseProvider>()

export function registerProvider(provider: IDatabaseProvider): void
export function getProvider(dialect: Dialect): IDatabaseProvider
// Usage: import './mysql/mysql-adapter'  → self-registering
```

### 2.2 File Structure

```
src/main/db/
├── types.ts                        ← Core interfaces (IDatabase, IDatabaseProvider)
├── registry.ts                     ← Provider registry (Map<Dialect, Provider>)
├── errors.ts                       ← DatabaseError, ConnectionError, etc.
├── config.ts                       ← DatabaseConfig Zod schemas
├── config-loader.ts                ← Env var + YAML config loader
├── dsn-parser.ts                   ← DSN URL parser + formatter (masks passwords)
├── pool.ts                         ← IConnectionPool interface + PoolConfig
├── generic-pool.ts                 ← Network DB pool (MySQL/PG/TiDB)
├── health.ts                       ← HealthStatus, HealthChecker interface
├── health-monitor.ts               ← DatabaseHealthMonitor class
├── auto-reconnect.ts               ← Auto-reconnect helper
├── migrations/
│   ├── types.ts                    ← Migration, MigrationRecord interfaces
│   ├── runner.ts                   ← MigrationRunner class
│   ├── index.ts                    ← ALL_MIGRATIONS registry
│   ├── 0001_initial_schema.ts      ← Projects, Repos, SSH Targets, Settings
│   ├── 0002_add_automations.ts     ← Automations + AutomationRuns
│   └── 0003_add_workspace_sessions.ts
├── sqlite/
│   ├── sqlite-adapter.ts           ← SqliteAdapter implements IDatabase
│   └── sqlite-pool.ts              ← SqliteSingleConnectionPool
├── mysql/
│   └── mysql-adapter.ts            ← MySQLAdapter (lazy: import 'mysql2')
└── postgresql/
    └── pg-adapter.ts               ← PgAdapter (lazy: import 'pg')
```

---

## 3. Connection Pool (`db/pool.ts`, `db/generic-pool.ts`)

```typescript
interface IConnectionPool {
  acquire(): Promise<IDatabase>
  release(conn: IDatabase): void
  withConnection<T>(fn: (db: IDatabase) => Promise<T>): Promise<T>
  drain(): Promise<void>     // graceful shutdown — close all connections
  destroy(): Promise<void>   // force close
  stats(): PoolStats
}

interface PoolStats {
  total: number
  idle: number
  waiting: number
  acquired: number
}
```

**SQLite: single-connection shim (no pool overhead):**
```typescript
export class SqliteSingleConnectionPool implements IConnectionPool {
  // Single connection wrapped in pool interface
  // acquire() always returns same connection
  // drain() = close()
}
```

**Network DB: generic pool với reconnect:**
```typescript
export class GenericConnectionPool implements IConnectionPool {
  // min/max connections
  // idleTimeout, acquireTimeout
  // auto-reconnect on connection drop
  async initialize(): Promise<void>  // must call after new()
}
```

---

## 4. Schema Migration (`db/migrations/`)

```typescript
interface Migration {
  id: string           // '20260101_000001_create_projects' (timestamp-prefixed)
  description: string
  dialect: Dialect | 'all'   // 'all' = cross-dialect SQL
  up(db: IDatabase): Promise<void>
  down?(db: IDatabase): Promise<void>
}

class MigrationRunner {
  constructor(db: IDatabase, migrations: Migration[])
  
  async migrate(): Promise<Migration[]>     // Run pending, return applied
  async rollback(steps?: number): Promise<Migration[]>
  async status(): Promise<MigrationStatus[]>
  async currentVersion(): Promise<string | null>
}

// Tracking table (auto-created):
// _orca_migrations (id TEXT PK, applied_at INTEGER, checksum TEXT)
```

**Auto-migration in server-bootstrap.ts:**
```typescript
await pool.withConnection(async (db) => {
  const runner = new MigrationRunner(db, ALL_MIGRATIONS)
  const applied = await runner.migrate()
  // Logs applied migrations, silent if up-to-date
})
```

---

## 5. Database Configuration (`db/config.ts`, `db/dsn-parser.ts`, `db/config-loader.ts`)

### 5.1 DSN Format

| Dialect | DSN Example |
|---------|------------|
| SQLite | `sqlite:///data/orca/db.sqlite` or `sqlite://:memory:` |
| MySQL | `mysql://user:pass@host:3306/dbname` |
| TiDB | `tidb://user:pass@host:4000/dbname` |
| PostgreSQL | `postgresql://user:pass@host:5432/dbname` |

**`formatDsn()` masks password:** `mysql://user:***@host:3306/db`

### 5.2 Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ORCA_DB_URL` | _(none)_ | Full DSN — auto-detect dialect |
| `ORCA_DB_DIALECT` | `sqlite` | Dialect override |
| `ORCA_DB_HOST` | `localhost` | Host (non-SQLite) |
| `ORCA_DB_PORT` | dialect default | Port |
| `ORCA_DB_NAME` | `orca` | Database name |
| `ORCA_DB_USER` | _(required)_ | Username |
| `ORCA_DB_PASSWORD` | _(required)_ | Password |

**Config loader priority:**
```
1. options.database (explicit, passed to initializeOrcaServices)
2. ORCA_DB_URL env var (DSN)
3. ORCA_DB_DIALECT + individual vars
4. Default: SQLite at {userDataPath}/orca-server.db
```

### 5.3 YAML Config (`config/orca-server.yaml`)

```yaml
database:
  dialect: mysql
  host: db.internal
  port: 3306
  name: orca_prod
  user: orca_user
  # password via ORCA_DB_PASSWORD env var (never in file)
  pool:
    min: 2
    max: 10
    idleTimeout: 30000
```

---

## 6. State Repository Pattern (`src/main/repositories/`)

**Strangler Fig strategy** — không break existing code:

```typescript
interface IStateRepository {
  projects: IProjectRepository
  repos: IRepoRepository
  worktrees: IWorktreeRepository
  sshTargets: ISshTargetRepository
  settings: ISettingsRepository
  automations: IAutomationRepository
}

// Phase A: JSON file backend (wraps existing Store)
class JsonFileRepository implements IStateRepository {
  constructor(store: Store)  // delegates to Store methods
}

// Phase C: SQL backend
class SqlStateRepository implements IStateRepository {
  constructor(pool: IConnectionPool)  // uses SQL queries
}

// Factory function (server-bootstrap.ts):
function createStateRepository(options: {
  pool?: IConnectionPool     // → SqlStateRepository
  dataFile?: string          // → JsonFileRepository (via Store)
}): IStateRepository
```

---

## 7. Health Monitoring (`db/health.ts`, `db/health-monitor.ts`)

```typescript
type HealthStatus = 'healthy' | 'degraded' | 'unhealthy'

interface DatabaseHealthCheck {
  status: HealthStatus
  dialect: Dialect
  latencyMs: number
  checkedAt: number
  lastError: string | null
  poolStats?: PoolStats
}

interface HealthChecker {
  check(): Promise<DatabaseHealthCheck>
  getLastCheck(): DatabaseHealthCheck | null
  startPeriodicCheck(intervalMs: number): void
  stopPeriodicCheck(): void
  onStatusChange(cb: (check: DatabaseHealthCheck) => void): () => void
}

class DatabaseHealthMonitor implements HealthChecker {
  constructor(pool: IConnectionPool, dialect: Dialect)
  // Sends SELECT 1 to verify connectivity
  // Thresholds: latency > 500ms → degraded, error → unhealthy
}
```

---

## 8. HTTP Health Endpoints (`src/server/health-endpoint.ts`)

Exposed trên port `:6769` (HTTP server) khi `dbMonitor` provided:

| Endpoint | Response | Use |
|----------|----------|-----|
| `GET /health` | Full JSON: status, dialect, latency, pool stats | Monitoring |
| `GET /health/ready` | `200 OK` or `503 Service Unavailable` | Kubernetes readiness |
| `GET /health/metrics` | Prometheus format | Grafana/Prometheus |

**No authentication required** (designed for cluster-internal probes).

```typescript
export function createHealthEndpoint(
  monitor: HealthChecker,
  options?: { includePoolStats?: boolean }
): (req: IncomingMessage, res: ServerResponse) => void
```

**Docker healthcheck (updated):**
```yaml
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://localhost:6769/health/ready"]
  interval: 30s
  timeout: 5s
  start_period: 15s
```

---

## 9. Integration with Server Bootstrap

```typescript
// src/main/server-bootstrap.ts additions:
export interface ServerBootstrapOptions {
  platform: IPlatformServices
  port?: number
  database?: DatabaseConfig | null  // override env vars; null = force JSON fallback
}

export interface ServerBootstrapResult {
  shutdown(): Promise<void>
  devServerManager: DevServerManager
  dbMonitor: HealthChecker           // → passed to startHttpServer()
  pushManager: WebPushManager        // → passed to registerPushApiRoutes()
}

// Shutdown sequence includes pool drain:
async shutdown() {
  await rpcServer.stop()
  dbMonitor.stopPeriodicCheck()
  await pool.drain()      // ← NEW: graceful DB shutdown
  await daemonShutdown()
}
```

---

## 10. Test Coverage

| Module | Test File | Tests |
|--------|-----------|-------|
| `db/sqlite/sqlite-adapter.ts` | `sqlite-adapter.test.ts` | 22 |
| `db/pool.ts` + `generic-pool.ts` | `generic-pool.test.ts` + conformance | 40 |
| `db/migrations/runner.ts` | `runner.test.ts` | 40 |
| `db/dsn-parser.ts` + `config-loader.ts` | `dsn-parser.test.ts` + `config-loader.test.ts` | 45 |
| `repositories/` | `json-file-repository.test.ts` + `sql-repository.test.ts` | 38 |
| `db/health-monitor.ts` + `health-endpoint.ts` | `health-monitor.test.ts` + `health-endpoint.test.ts` | 46 |
| **Total** | **16 test files** | **205** |

```bash
# Run DB + repository tests:
pnpm vitest run src/main/db/ src/main/repositories/
# → 16 test files, 205 tests, all pass

# Integration tests (needs real DB):
ORCA_TEST_DB_URL=mysql://root@localhost:3306/orca_test \
  pnpm vitest run src/main/db/mysql/
```
