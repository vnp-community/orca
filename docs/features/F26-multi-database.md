# F26 — Multi-Database Support

| Trường | Giá trị |
|--------|---------|
| **ID** | F26 |
| **Tên** | Multi-Database Support |
| **Ưu tiên** | P1 |
| **Trạng thái** | ✅ Phát hành |
| **CRs** | [sql-server CR-001~006](../crs/v1/sql-server/) |
| **TDD** | [TDD-12: Database Layer](../specs/backend/tdd/12-database-layer.md) |
| **Phiên bản** | v3.0+ |
| **ADR References** | ADR-002 |
| **HLD References** | C3.7, C4.3 |

---

## Mô tả

Orca Server hỗ trợ nhiều loại database — **SQLite** (mặc định, embedded), **MySQL**, **PostgreSQL**, **TiDB** — thông qua một lớp abstraction thống nhất. Desktop mode giữ nguyên SQLite.

---

## Vấn đề cần giải quyết

Orca Server cần scale vượt giới hạn của SQLite khi:
- Nhiều users đồng thời (concurrent writes)
- Production deployment trên cloud (external DB service)
- Enterprise: dùng PostgreSQL/MySQL cluster sẵn có

---

## Tính năng chi tiết

### Database Provider Abstraction
```typescript
interface ISyncDatabase { /* SQLite — synchronous better-sqlite3 */ }
interface IAsyncDatabase { /* MySQL/PG — async queries */ }
interface IConnectionPool { acquire(): Promise<Conn>; release(c): void; drain(): Promise<void> }
```

### Dialects hỗ trợ

| Dialect | Driver | Use case |
|---------|--------|---------|
| `sqlite` | better-sqlite3 | Default, dev, single-user |
| `mysql` | mysql2 | Production, cloud |
| `postgresql` | pg | Production, cloud |
| `tidb` | mysql2 (compat) | Distributed SQL |

### DSN Configuration

```bash
# Tất cả cấu hình qua một env var:
ORCA_DB_URL=sqlite:///data/orca.db
ORCA_DB_URL=mysql://user:pass@host:3306/orca_prod
ORCA_DB_URL=postgresql://user:pass@host:5432/orca_prod
ORCA_DB_URL=tidb://user:pass@host:4000/orca_prod
```

Hoặc YAML config (`orca-server.yaml`):
```yaml
database:
  dialect: postgresql
  host: db.company.com
  port: 5432
  database: orca_prod
  username: orca_user
```

### Schema Migrations
- `MigrationRunner` — apply/rollback/status
- **5 migrations**: 0001 initial → 0002 automations → 0003 sessions → 0004 app tables → 0005 auth schema
- Cross-dialect: cùng migration code chạy trên SQLite/MySQL/PG
- Server bootstrap tự động migrate khi khởi động

### State Repository Pattern
```typescript
interface IStateRepository {
  getProject(id): Promise<Project | null>
  upsertProject(p): Promise<void>
  // + SshTarget, Settings, Repo...
}
// Desktop: JsonFileStateRepository (JSON file)
// Server:  SqlStateRepository (any dialect)
// Auto-select via factory based on ORCA_DB_URL
```

### Health Monitoring
- `GET /health` — cached DB status (fast, 200/503)
- `GET /health/ready` — live DB query, 503 nếu down
- `GET /health/metrics` — Prometheus text format
- Background health check mỗi 30s
- `onStatusChange` callback khi status thay đổi

---

## Tiêu chí chấp nhận

- [x] `parseDsn()` parse đúng tất cả dialect formats
- [x] `loadDatabaseConfig()` đọc từ `ORCA_DB_URL` env
- [x] SQLite adapter passes toàn bộ existing tests
- [x] MySQL + PostgreSQL adapter có thể connect và execute basic queries
- [x] MigrationRunner apply/rollback cross-dialect
- [x] Idempotent — chạy migrate 2 lần không apply lại
- [x] IStateRepository — JSON file (desktop) + SQL (server)
- [x] `/health`, `/health/ready`, `/health/metrics` hoạt động
- [x] Password không xuất hiện trong logs
- [x] Server bootstrap tự migrate khi khởi động

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| DB types + interfaces | `src/main/db/types.ts` |
| DB provider factory | `src/main/db/provider.ts` |
| DSN parser | `src/main/db/dsn-parser.ts` |
| Config loader | `src/main/db/config-loader.ts`, `config.ts` |
| SQLite adapter | `src/main/db/sqlite/sqlite-adapter.ts` |
| MySQL adapter | `src/main/db/mysql/mysql-adapter.ts` |
| PostgreSQL adapter | `src/main/db/postgresql/pg-adapter.ts` |
| Connection pool | `src/main/db/pool.ts`, `generic-pool.ts` |
| Migration runner | `src/main/db/migrations/runner.ts` |
| IStateRepository | `src/main/repositories/types.ts` |
| JSON repo | `src/main/repositories/json-file-repository.ts` |
| SQL repo | `src/main/repositories/sql-repository.ts` |
| Health checker | `src/main/db/health.ts`, `health-monitor.ts` |
| Health endpoint | `src/server/health-endpoint.ts` |

**Tests:** 205 tests | **Env:** `ORCA_DB_URL`

---

## Metrics

| KPI | Mục tiêu |
|-----|----------|
| `/health/ready` response | < 100ms |
| Migration time (5 migrations) | < 1s |
| SQLite backward compat | 100% (0 regressions) |
