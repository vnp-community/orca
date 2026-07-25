# Task Index — SQL Server (Multi-Database Support)
## AI-Executable Task Breakdown

**Date:** 2026-07-23  
**Source:** `specs/backend/crs/v1/sql-server/solutions/`  
**CRs:** `docs/crs/v1/sql-server/`  
**Target:** `src/main/db/`, `src/main/repositories/`, `src/server/` — Orca codebase (TypeScript)

---

## Task List

| Task | Title | SOL | Phase | Effort | Status |
|------|-------|-----|-------|--------|--------|
| [TASK-DB-001](./TASK-DB-001-db-types.md) | Tạo `src/main/db/types.ts` — IDatabase interfaces | SOL-DB-001 | 1 | XS | ✅ Done |
| [TASK-DB-002](./TASK-DB-002-db-provider.md) | Tạo `src/main/db/provider.ts` — DatabaseProvider registry | SOL-DB-001 | 1 | XS | ✅ Done |
| [TASK-DB-003](./TASK-DB-003-db-config-schema.md) | Tạo `src/main/db/config.ts` — Zod schemas | SOL-DB-004 | 1 | S | ✅ Done |
| [TASK-DB-004](./TASK-DB-004-dsn-parser.md) | Tạo `src/main/db/dsn-parser.ts` + tests | SOL-DB-004 | 1 | S | ✅ Done |
| [TASK-DB-005](./TASK-DB-005-config-loader.md) | Tạo `src/main/db/config-loader.ts` + tests | SOL-DB-004 | 1 | S | ✅ Done |
| [TASK-DB-006](./TASK-DB-006-sqlite-adapter.md) | Tạo `src/main/db/sqlite/sqlite-adapter.ts` + tests | SOL-DB-001 | 1 | M | ✅ Done |
| [TASK-DB-007](./TASK-DB-007-sqlite-compat-shim.md) | Cập nhật `src/main/sqlite/sync-database.ts` — backward compat shim | SOL-DB-001 | 1 | XS | ✅ Done |
| [TASK-DB-008](./TASK-DB-008-pool-interface.md) | Tạo `src/main/db/pool.ts` — IConnectionPool interface | SOL-DB-002 | 2 | XS | ✅ Done |
| [TASK-DB-009](./TASK-DB-009-sqlite-pool.md) | Tạo `src/main/db/sqlite/sqlite-pool.ts` + tests | SOL-DB-002 | 2 | S | ✅ Done |
| [TASK-DB-010](./TASK-DB-010-generic-pool.md) | Tạo `src/main/db/generic-pool.ts` + tests | SOL-DB-002 | 2 | M | ✅ Done |
| [TASK-DB-011](./TASK-DB-011-mysql-adapter.md) | Tạo `src/main/db/mysql/mysql-adapter.ts` + tests | SOL-DB-002 | 2 | M | ✅ Done |
| [TASK-DB-012](./TASK-DB-012-pg-adapter.md) | Tạo `src/main/db/postgresql/pg-adapter.ts` + tests | SOL-DB-002 | 2 | M | ✅ Done |
| [TASK-DB-013](./TASK-DB-013-migration-types.md) | Tạo `src/main/db/migrations/types.ts` | SOL-DB-003 | 2 | XS | ✅ Done |
| [TASK-DB-014](./TASK-DB-014-migration-runner.md) | Tạo `src/main/db/migrations/runner.ts` + tests | SOL-DB-003 | 2 | M | ✅ Done |
| [TASK-DB-015](./TASK-DB-015-initial-schema.md) | Tạo `0001_initial_schema.ts` + tests | SOL-DB-003 | 2 | S | ✅ Done |
| [TASK-DB-016](./TASK-DB-016-more-migrations.md) | Tạo `0002_add_automations.ts` + `0003_add_workspace_sessions.ts` | SOL-DB-003 | 2 | S | ✅ Done |
| [TASK-DB-017](./TASK-DB-017-repository-types.md) | Tạo `src/main/repositories/types.ts` — IStateRepository | SOL-DB-005 | 3 | XS | ✅ Done |
| [TASK-DB-018](./TASK-DB-018-json-file-repository.md) | Tạo `src/main/repositories/json-file-repository.ts` + tests | SOL-DB-005 | 3 | M | ✅ Done |
| [TASK-DB-019](./TASK-DB-019-sql-repository.md) | Tạo `src/main/repositories/sql-repository.ts` + tests | SOL-DB-005 | 3 | M | ✅ Done |
| [TASK-DB-020](./TASK-DB-020-repository-factory.md) | Tạo `src/main/repositories/factory.ts` | SOL-DB-005 | 3 | XS | ✅ Done |
| [TASK-DB-021](./TASK-DB-021-health-interfaces.md) | Tạo `src/main/db/health.ts` — HealthChecker interface | SOL-DB-006 | 3 | XS | ✅ Done |
| [TASK-DB-022](./TASK-DB-022-health-monitor.md) | Tạo `src/main/db/health-monitor.ts` + tests | SOL-DB-006 | 3 | S | ✅ Done |
| [TASK-DB-023](./TASK-DB-023-health-endpoint.md) | Tạo `src/server/health-endpoint.ts` + tests | SOL-DB-006 | 3 | S | ✅ Done |
| [TASK-DB-024](./TASK-DB-024-server-bootstrap-update.md) | Cập nhật `src/main/server-bootstrap.ts` — DB integration | SOL-DB-002,005,006 | 4 | M | ✅ Done |
| [TASK-DB-025](./TASK-DB-025-http-server-health.md) | Cập nhật `src/server/http-server.ts` — expose health routes | SOL-DB-006 | 4 | S | ✅ Done |
| [TASK-DB-026](./TASK-DB-026-server-entry-db.md) | Cập nhật `src/server/index.ts` — truyền DB config | SOL-DB-004 | 4 | XS | ✅ Done |
| [TASK-DB-027](./TASK-DB-027-example-config.md) | Tạo `config/orca-server.example.yaml` + Docker Compose update | SOL-DB-004,006 | 4 | XS | ✅ Done |

---

## Thực thi theo phase

```
PHASE 1 — DB Types & Config Foundation (không depend gì):
  TASK-DB-001 → TASK-DB-002           (SOL-DB-001: IDatabase types + provider)
  TASK-DB-003 → TASK-DB-004 → TASK-DB-005  (SOL-DB-004: Config, DSN, Loader)

PHASE 1 (tiếp) — SQLite Adapter (depends on TASK-DB-001, DB-002):
  TASK-DB-006 → TASK-DB-007           (SOL-DB-001: SqliteAdapter + compat shim)

PHASE 2 — Connection Pool (depends on Phase 1):
  TASK-DB-008                         (pool interface)
  TASK-DB-009 → TASK-DB-010          (SQLite pool + Generic pool)
  TASK-DB-011 → TASK-DB-012          (MySQL + PostgreSQL adapters) [parallel]

PHASE 2 (tiếp) — Migrations (depends on TASK-DB-006, DB-008):
  TASK-DB-013 → TASK-DB-014          (Migration types + runner)
  TASK-DB-015 → TASK-DB-016          (Schema migrations) [parallel]

PHASE 3 — Repository Pattern (depends on Phase 2):
  TASK-DB-017                         (IStateRepository types)
  TASK-DB-018 → TASK-DB-019          (JSON + SQL repositories) [parallel]
  TASK-DB-020                         (factory)
  TASK-DB-021 → TASK-DB-022          (Health interfaces + monitor)
  TASK-DB-023                         (Health endpoint)

PHASE 4 — Integration (depends on Phase 3):
  TASK-DB-024                         (server-bootstrap.ts update)
  TASK-DB-025 → TASK-DB-026          (http-server + server entry)
  TASK-DB-027                         (example config + docker)
```

---

## Dependency Graph

```
TASK-DB-001 (IDatabase types)
  ├─► TASK-DB-002 (provider registry)
  │     └─► TASK-DB-006 (SqliteAdapter)
  │           └─► TASK-DB-007 (compat shim)
  └─► TASK-DB-008 (IConnectionPool)
        ├─► TASK-DB-009 (SqlitePool)
        │     └─► [feeds into TASK-DB-014, TASK-DB-017]
        ├─► TASK-DB-010 (GenericPool)
        ├─► TASK-DB-011 (MySQLAdapter)
        └─► TASK-DB-012 (PgAdapter)

TASK-DB-003 (DatabaseConfig schema)
  └─► TASK-DB-004 (DSN parser)
        └─► TASK-DB-005 (config loader)
              └─► TASK-DB-024 (server-bootstrap)

TASK-DB-006 + TASK-DB-008
  └─► TASK-DB-013 (Migration types)
        └─► TASK-DB-014 (MigrationRunner)
              └─► TASK-DB-015 (0001_initial_schema)
                    └─► TASK-DB-016 (more migrations)

TASK-DB-014 + TASK-DB-009 + TASK-DB-017
  └─► TASK-DB-018 (JsonFileRepository)
  └─► TASK-DB-019 (SqlRepository)
        └─► TASK-DB-020 (factory)
              └─► TASK-DB-024 (server-bootstrap)

TASK-DB-009 → TASK-DB-021 → TASK-DB-022 (HealthMonitor)
  └─► TASK-DB-023 (health-endpoint)
        └─► TASK-DB-025 (http-server update)
              └─► TASK-DB-024 (server-bootstrap)

TASK-DB-024 → TASK-DB-026 → TASK-DB-027
```

---

## Effort Legend

| Symbol | Thời gian ước tính |
|--------|-------------------|
| XS | < 30 phút |
| S | 30–90 phút |
| M | 1.5–3 giờ |
| L | 3–6 giờ |

---

## Done Metrics (Target)

| Metric | Target |
|--------|--------|
| Tasks | **27/27** |
| Unit tests (new) | **≥ 120** |
| TS compile errors | **0** |
| Existing tests regression | **0** |
| `pnpm vitest run src/main/db/` | ✅ pass |
| `pnpm vitest run src/main/repositories/` | ✅ pass |
| Server mode với `ORCA_DB_URL=mysql://...` | ✅ boot |
| Desktop Electron mode | ✅ no regression |
