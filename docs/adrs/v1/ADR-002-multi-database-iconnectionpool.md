# ADR-002 — Multi-Database via IConnectionPool Interface

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-002 |
| **Trạng thái** | ✅ Accepted |
| **Ngày** | 2026-07-28 |
| **HLD Ref** | C3.7, C4.3 |
| **Code Ref** | `src/main/db/types.ts`, `src/main/db/pool.ts`, `src/main/db/migrations/` |

---

## Bối cảnh

Orca ban đầu chỉ dùng SQLite (desktop). Khi triển khai Web Server mode cho enterprise, cần hỗ trợ MySQL, PostgreSQL, TiDB — mỗi dialect có API khác nhau, sync/async khác nhau, placeholder style khác nhau (`?` vs `$1` vs `:name`).

**Vấn đề phát hiện từ code:**
- `IDatabase.capabilities.dialect` cho phép runtime dialect detection
- `IDatabase.capabilities.placeholderStyle: 'positional' | 'named' | 'both'`
- SQLite sync API (`ISyncDatabase`) vs async API (`IDatabase`)
- `MigrationRunner` cần chạy trên tất cả dialects

---

## Quyết định

### 1. Interface phân tầng

```typescript
// src/main/db/types.ts
interface IDatabase {
  exec(sql: string): void | Promise<void>
  prepare(sql: string): IStatement | Promise<IStatement>
  query(sql: string, params?: BindValue[]): Promise<Record<string, unknown>[]>
  transaction<T>(fn: () => T | Promise<T>): Promise<T>
  readonly capabilities: IDatabaseCapabilities
}

interface IDatabaseCapabilities {
  dialect: 'sqlite' | 'mysql' | 'tidb' | 'mariadb' | 'postgresql'
  placeholderStyle: 'positional' | 'named' | 'both'
  returning: boolean
  nativeJson: boolean
  walMode: boolean
}
```

### 2. IConnectionPool wraps IDatabase

```typescript
// src/main/db/pool.ts
interface IConnectionPool {
  acquire(): Promise<IDatabase>
  release(db: IDatabase): void
  destroy(): Promise<void>
  readonly stats: PoolStats
}
```

### 3. Dialect adapters

```
src/main/db/
├── sqlite/       → node:sqlite (sync, ISyncDatabase)
├── mysql/        → mysql2/promise (async pool)
├── postgresql/   → pg (async pool)
└── generic-pool.ts  → GenericConnectionPool<T>
```

### 4. DSN-based config

```
ORCA_DB_DSN=sqlite:///path/to/orca.db
ORCA_DB_DSN=mysql://user:pass@host:3306/orca
ORCA_DB_DSN=postgresql://user:pass@host:5432/orca
```

`dsn-parser.ts` → `DatabaseConfig` → `provider.ts` → `IConnectionPool`

### 5. MigrationRunner

```typescript
// src/main/db/migrations/runner.ts
// Dialect-aware: generates CREATE TABLE IF NOT EXISTS + placeholders per dialect
class MigrationRunner {
  async run(pool: IConnectionPool): Promise<void>
  async status(pool: IConnectionPool): Promise<MigrationStatus[]>
  async rollback(pool: IConnectionPool, targetVersion: number): Promise<void>
}
```

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------|
| **IConnectionPool + dialect adapters** ✅ | Plugin-style, không cần ORM, gần với SQL thật |
| ORM (TypeORM, Drizzle) | Abstraction leak, query planning bị ẩn, migration phức tạp |
| Hardcode SQLite | Không scale cho enterprise deployment |
| Separate DB per dialect | Code duplication, maintenance burden |

---

## Hậu quả

**Tích cực:**
- Thêm dialect mới = 1 adapter mới + register vào `provider.ts`
- `MigrationRunner` idempotent, cross-dialect
- Health endpoint `/health/ready` dùng `IConnectionPool.stats`

**Tiêu cực:**
- SQL phải viết "dialect-safe" (tránh SQLite-only syntax trong shared migrations)
- `ISyncDatabase` (SQLite) và async `IDatabase` cần wrapper shim khi dùng chung
- Hiện tại có 5 migrations (0001–0005); v5.0 cần thêm 0006–0010

---

## Migration Roadmap (v5.0)

| Migration | Nội dung |
|-----------|---------|
| 0006 | `orca_company`, `orca_departments` (profile hierarchy) |
| 0007 | `orca_projects`, `orca_project_members` |
| 0008 | `orca_ai_provider_accounts`, `orca_provider_usage` |
| 0009 | `orca_workflow_templates`, `orca_workflow_executions`, `orca_step_executions` |
| 0010 | `orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments` |

---

## Trạng thái Implementation

✅ IDatabase, IConnectionPool, ISyncDatabase — implement xong  
✅ SQLite adapter (node:sqlite sync)  
✅ MySQL adapter (mysql2)  
✅ PostgreSQL adapter (pg)  
✅ MigrationRunner (0001–0005)  
🚧 Migrations 0006–0010 (v5.0/v6.0 schema) — xem [ADR-016](../v2/ADR-016-db-migrations-0006-0010-schema.md) cho schema chi tiết

---

## Liên hệ ADR v2

[ADR-016](../v2/ADR-016-db-migrations-0006-0010-schema.md) mô tả chi tiết schema và rationale cho migrations 0006–0010:
- **0006**: `orca_company`, `orca_departments` (F33 Profile Hierarchy)
- **0007**: `orca_projects`, `orca_project_members` (F34 Project Binding)
- **0008**: `orca_ai_provider_accounts`, `orca_provider_usage` (F35 AI Providers)
- **0009**: `orca_workflow_templates`, `orca_workflow_executions`, `orca_step_executions` (F36 Workflow)
- **0010**: `orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments` (F37 Task Graph)

