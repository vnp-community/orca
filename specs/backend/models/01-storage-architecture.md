# Kiến trúc lưu trữ đa backend (`backend/src/main/db/**`, `repositories/**`)

## 1. Lớp trừu tượng Database (`db/types.ts`)

Orca hỗ trợ 5 dialect: `sqlite | mysql | tidb | mariadb | postgresql` (`IDatabaseCapabilities.dialect`,
`db/types.ts:42`). Mọi adapter implement chung interface `IDatabase`:

```ts
type IDatabase = {
  exec(sql): void | Promise<void>
  prepare(sql): IStatement | Promise<IStatement>
  close(): void | Promise<void>
  readonly capabilities: IDatabaseCapabilities   // { walMode, returning, nativeJson, placeholderStyle, dialect }
  transaction<T>(fn): Promise<T>
  query(sql, params?): Promise<Record<string, unknown>[]>
}
```

- `ISyncDatabase` — SQLite (`node:sqlite` `DatabaseSync`), exec/prepare/close đồng bộ.
- `IAsyncDatabase` — MySQL/PostgreSQL/TiDB, mọi method trả `Promise`.

Adapter cụ thể: `db/sqlite/sqlite-adapter.ts`, `db/mysql/mysql-adapter.ts`, `db/postgresql/pg-adapter.ts`.

## 2. Provider Registry (`db/provider.ts`)

Pattern side-effect-register: mỗi adapter tự gọi `registerDatabaseProvider({dialect, connect})` khi được import.
`createDatabase(config)` tra registry theo `config.dialect` rồi gọi `connect()`. Import adapter trước khi gọi
`createDatabase()` — nếu chưa import, `getDatabaseProvider()` throw kèm danh sách dialect đã đăng ký.

## 3. Cấu hình DB (`db/config.ts`, `db/config-loader.ts`, `db/dsn-parser.ts`)

`DatabaseConfigSchema` (Zod discriminated union theo `dialect`):
- `sqlite`: `{ dialect:'sqlite', path, readonly? }`
- `mysql|tidb|mariadb`: `{ dialect, host, port(default 3306), database, username, password, ssl?, pool? }`
- `postgresql`: `{ dialect, host, port(default 5432), database, username, password, ssl?, schema(default 'public'), pool? }`

`loadDatabaseConfig()` ưu tiên theo thứ tự:
1. `ORCA_DB_URL` (DSN, vd. `postgresql://user:pass@host:5432/db?ssl=true`) — parse qua `parseDsn()`.
2. `ORCA_DB_DIALECT` + biến rời (`ORCA_DB_HOST`, `ORCA_DB_NAME`, `ORCA_DB_USER`, `ORCA_DB_PASSWORD`, `ORCA_DB_SSL`,
   `ORCA_DB_POOL_MAX/MIN`) — thiếu `host`/`database`/`username` → cảnh báo, trả `null`.
3. Không có gì → `null` → caller (server-bootstrap) rơi về JSON file fallback.

## 4. Connection Pool (`db/pool.ts`, `db/generic-pool.ts`, `db/pooled-database-adapter.ts`)

`IConnectionPool`: `acquire/release/withConnection/withTransaction/stats/drain/destroy`. `PoolConfig` mặc định:
`min:2, max:10, acquireTimeoutMs:5000, idleTimeoutMs:30000, connectionRetries:3, retryDelayMs:500`.

`PooledDatabaseAdapter` — bridge `IDatabase` ⇄ `IConnectionPool`, dùng riêng cho `AuthManager`/`AuthUserStore`
(vốn được viết cho 1 connection cố định, chưa refactor sang pool trực tiếp):
- `prepare()` chỉ giữ SQL text (deferred) — mỗi `.run()/.get()/.all()` tự acquire/release 1 connection riêng,
  vì statement handle của driver thật gắn với đúng connection tạo ra nó.
- **`transaction()` KHÔNG được hỗ trợ** — throw lỗi rõ ràng thay vì chạy các statement trên các connection khác
  nhau (mất tính atomic). `AuthUserStore` hiện không gọi `.transaction()` nên chưa phát sinh vấn đề — cẩn thận
  nếu thêm code mới cần transaction thật sự qua adapter này.
- `close()` là no-op — vòng đời pool do `server-bootstrap.ts` `shutdown()` quản lý tập trung, không phải từng
  consumer.

## 5. Migration Runner (`db/migrations/runner.ts`, `types.ts`)

- Bảng theo dõi: `schema_migrations (version INTEGER PK, name TEXT, applied_at TEXT)`.
- Mỗi `Migration = { version, name, up(db), down(db) }`. `MigrationRunner.migrate()` chạy tuần tự các migration
  chưa áp dụng, **mỗi migration trong 1 transaction riêng** (atomic per-migration, không phải toàn bộ 1 transaction).
- `rollbackTo(targetVersion)` chạy `down()` theo thứ tự version giảm dần.
- Danh sách migration chính thức: `ALL_MIGRATIONS` (`db/migrations/index.ts`) — 18 migration, version 1→18,
  liệt kê chi tiết ở [02-sql-schema-catalog.md](./02-sql-schema-catalog.md).

### Cross-dialect helpers (`db/migrations/sql-dialect.ts`)

Lý do tồn tại: migration 0001–0010 ban đầu viết SQL chỉ chạy được trên SQLite (`datetime('now')`,
`strftime()`, `AUTOINCREMENT`) — khi bật Postgres thật (b15.openledger.vn) migration crash vì
`function datetime(unknown) does not exist`, để lại `orca_users` chưa được tạo (BUG-BE-RPC-003).
3 helper theo `db.capabilities.dialect`:

| Helper | sqlite | postgresql | mysql/tidb/mariadb |
|---|---|---|---|
| `nowTextDefaultSql()` | `datetime('now')` | `to_char(CURRENT_TIMESTAMP AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS')` | `UTC_TIMESTAMP()` |
| `nowEpochMsDefaultSql()` | `strftime('%s','now')*1000` | `FLOOR(EXTRACT(EPOCH FROM CURRENT_TIMESTAMP)*1000)` | `UNIX_TIMESTAMP()*1000` |
| `autoIncrementPrimaryKeySql()` | `INTEGER PRIMARY KEY AUTOINCREMENT` | `INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY` | `INTEGER PRIMARY KEY AUTO_INCREMENT` |

Với `ALTER TABLE ADD COLUMN` (SQLite không hỗ trợ `DROP COLUMN` trước 3.35): các migration `down()` từ 0013 trở
đi **cố ý no-op** cho các cột thêm bằng ALTER — cột thừa không ảnh hưởng hành vi khi rollback (pattern lặp lại
nhất quán, xem comment trong 0013/0014/0015/0016/0017).

## 6. Repository Pattern (`repositories/**`) — chỉ áp dụng cho 4 entity gốc

`IStateRepository` (`repositories/types.ts`) là interface **chung** cho 2 backend, nhưng **chỉ bọc 4 entity**:
`projects`, `repos`, `sshTargets`, `settings` (không bao gồm bất kỳ entity nào từ migration 0005 trở đi —
Auth/Profile/Project-v5/AIProvider/Workflow/Task/Team/Annotation đều có **service riêng nói thẳng vào SQL pool**,
xem bảng ở dưới).

```ts
type IStateRepository = {
  readonly projects: IProjectRepository       // + findByGroup()
  readonly repos: IRepoRepository             // + findByProject()
  readonly sshTargets: ISshTargetRepository
  readonly settings: IGlobalSettingsRepository // get()/update(patch)
  ping(): Promise<boolean>
  close(): Promise<void>
}
```

`repositories/factory.ts` → `createStateRepository({pool?, dataFile?})`:
- `pool` được truyền → `SqlStateRepository` (đọc/ghi 4 bảng `orca_projects`/`orca_repos`/`orca_ssh_targets`/
  `orca_global_settings` — bảng tạo ở **migration 0004**, tiền tố `orca_` để tách biệt với bảng `projects`/
  `repos`/`ssh_targets`/`settings` không tiền tố của **migration 0001**, xem lưu ý "dormant" ở file 02).
  Toàn bộ entity được lưu dạng `data TEXT` (JSON blob) trong mỗi row — chỉ vài field (name/tab_order/project_id)
  được tách cột riêng để index/query nhanh, còn lại schema tự do (không cần migration khi thêm field mới).
- `dataFile` được truyền (không có `pool`) → `JsonFileStateRepository` (ghi ra 1 file JSON, debounce 200ms,
  `{projects, repos, sshTargets, globalSettings}`).
- Không có cả hai → throw.

`server-bootstrap.ts` (`initializeOrcaServices`) chọn theo cấu hình thực tế lúc bootstrap:
```
dbConfig = loadDatabaseConfig()  // hoặc override qua options.database
nếu dbConfig !== null → tạo pool thật, chạy MigrationRunner, createStateRepository({pool})
nếu dbConfig === null → createStateRepository({dataFile: join(userDataPath, 'store.json')})
```
Đây là **nhánh server mode duy nhất** dùng `JsonFileStateRepository` — Electron/desktop mode **không đi qua
đường này**, nó dùng `persistence.ts`'s `Store` (`orca-data.json`, superset lớn hơn nhiều — xem
[03-electron-desktop-store.md](./03-electron-desktop-store.md)).

## 7. Domain services đi thẳng vào SQL pool (không qua `IStateRepository`)

Từ migration 0005 trở đi, mỗi domain nghiệp vụ mới có **1 service riêng** nhận `IConnectionPool` (hoặc
`IDatabase`) trực tiếp trong constructor, tự viết raw SQL, không qua abstraction `IRepository<T>` chung:

| Service | File | Bảng SQL sở hữu |
|---|---|---|
| `AuthManager` / `AuthUserStore` | `auth/auth-manager.ts` | `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies` |
| `ProfileService` / `ProfileResolver` | `profile/ProfileService.ts`, `profile/ProfileResolver.ts` | `orca_companies`, `orca_departments`, `orca_user_profiles`, `orca_teams.profile_json` |
| `ProjectService` | `project/ProjectService.ts` | `orca_v5_projects`, `orca_v5_project_members` |
| `OrcaProjectSourceProjectService` | `project/OrcaProjectSourceProjectService.ts` | `orca_project_source_projects` |
| `AIProviderService` / `ProviderHealthChecker` | `ai-providers/AIProviderService.ts` | `orca_ai_provider_accounts`, `orca_provider_usage` |
| `WorkflowOrchestrator` / `TemplateResolver` | `workflow/WorkflowOrchestrator.ts` | `orca_workflow_templates`, `orca_workflow_executions`, `orca_workflow_step_executions` |
| `TaskService` / `TaskGrantService` | `task/TaskService.ts`, `task/TaskGrantService.ts` | `orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments`, `orca_team_members`, `orca_teams` |
| `annotation-store.ts` (RPC `annotation.*`) | `code-review/annotation-store.ts` | `orca_annotations` |

Lý do tách: các domain này cần transaction thật, query phức tạp (BFS ancestor resolution cho task grant, DAG
wave execution cho workflow...) mà interface `IRepository<T>` CRUD-đơn-giản không biểu diễn được. Cái giá phải
trả: không có 1 nơi trung tâm để audit "toàn bộ SQL access" — phải đọc từng service.

## 8. Health check (`db/health.ts`, `db/health-monitor.ts`)

`HealthChecker` chạy định kỳ, phân loại `healthy | degraded | unhealthy` dựa trên pool stats + ping latency;
`server-bootstrap.ts` log cảnh báo khi `degraded`/`unhealthy` ngay sau khi tạo pool, và gọi
`dbMonitor.stopPeriodicCheck()` lúc shutdown.

## 9. Tổng kết: 1 request đi tới đâu?

```
Electron Desktop (mặc định)
  → persistence.ts Store → orca-data.json (JSON, 1 file, ~30 field gốc)
  → KHÔNG chạy qua db/**, repositories/**, hay 25 bảng orca_* nào cả

Node.js Server mode, có ORCA_DB_URL / ORCA_DB_DIALECT
  → IConnectionPool (sqlite/mysql/postgresql/tidb) → MigrationRunner áp 18 migration
  → 4 entity gốc (Project/Repo/SshTarget/GlobalSettings): SqlStateRepository → orca_projects/orca_repos/
    orca_ssh_targets/orca_global_settings (migration 0004)
  → Mọi domain v5.0+ (Auth/Profile/Project-v5/AIProvider/Workflow/Task/Team/Annotation): service riêng
    → SQL trực tiếp trên các bảng migration 0005-0018

Node.js Server mode, KHÔNG có DB config
  → JsonFileStateRepository → store.json (chỉ 4 entity gốc)
  → Domain v5.0+ services vẫn cần pool thật → sẽ lỗi lúc khởi tạo nếu không có DB (xem server-bootstrap.ts)

Multi-agent coordinator ("complex path" của Task Graph)
  → OrchestrationDb → orchestration.db (SQLite riêng, tách biệt hoàn toàn — xem file 04)

Credential/secret (SSH key, AI provider key, OAuth token...)
  → 5 cơ chế riêng theo loại secret, KHÔNG đi qua db/** — xem file 05
```
