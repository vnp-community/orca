# ADR-021 — Hợp nhất Server-mode Data Plane vào Postgres, kiến trúc Microservices theo domain

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-021 |
| **Trạng thái** | 🚧 Proposed — Phase 0 (schema scaffolding) đã implement, Phase 1-4 chưa triển khai |
| **Ngày** | 2026-08-15 |
| **HLD Ref** | backend-server-architecture.md, docs/guides/postgres-shared-database-design.md |
| **Code Ref** | `backend/src/main/db/migrations/0019-0022*.ts`, `backend/src/main/tenancy/tenant-context.ts` |
| **Supersedes** | — |
| **Amends** | [ADR-002](../v1/ADR-002-multi-database-iconnectionpool.md) (multi-dialect IConnectionPool), [ADR-016](./ADR-016-db-migrations-0006-0010-schema.md) (schema thật, không phải đề xuất) |
| **Không áp dụng cho** | Electron Desktop mode (giữ nguyên `orca-data.json` cục bộ — xem [specs/backend/models/03](../../../specs/backend/models/03-electron-desktop-store.md)) |

---

## Bối cảnh

Khảo sát toàn bộ data layer (`specs/backend/models/`) cho thấy **server mode** (deployment
`docker-compose.orca.artifact.yml`, `ORCA_MULTI_USER=1`, b15.openledger.vn) hiện dùng **4 cơ chế lưu trữ song
song**, dù đã có Postgres chạy sẵn:

1. Postgres (`orca.db` logic qua `IConnectionPool`) — 25 bảng `orca_*`, nhưng 9/25 bảng dormant (không consumer).
2. File JSON `orca-data.json` (`persistence.ts` `Store`) — vẫn được `AutomationService`/`WebPushManager` dùng
   trong server mode (nhận `Store`, không nhận SQL `pool`) cho Automation/AutomationRun/Push subscription/
   VAPID key.
3. SQLite riêng biệt `orchestration.db` (`OrchestrationDb`) — pipeline multi-agent coordinator, tách biệt hoàn
   toàn khỏi Postgres, không transaction chung, không backup chung.
4. File JSON riêng `orca-claude-usage.json`/`orca-codex-usage.json` — usage tracking, không multi-tenant-aware.

Hệ quả: không có transaction nhất quán giữa các domain liên quan (vd. tạo Task xong dispatch sang
Orchestration là 2 write riêng biệt trên 2 DB khác nhau), không backup/restore tập trung, và **hoàn toàn
không có khái niệm tenant** — mọi bảng không có cột phân biệt tổ chức nào đang sở hữu row nào, dữ liệu multi-
company hiện chỉ tách được ở tầng `orca_companies`/`orca_departments` (Profile hierarchy) chứ chưa lan xuống
Project/Task/Workflow/AI Provider.

## Quyết định

### 1. Một database ENGINE duy nhất: Postgres, thiết kế cross-dialect để chuyển TiDB không cần viết lại

**PostgreSQL và TiDB dùng wire protocol khác nhau** — TiDB tương thích MySQL protocol, không phải Postgres.
`db/provider.ts`/`db/migrations/sql-dialect.ts` (ADR-002) đã hỗ trợ sẵn cả 2 dialect (`postgresql` và
`mysql|tidb|mariadb` dùng chung 1 adapter family MySQL-wire). Quyết định: **triển khai thật trên Postgres bây
giờ, nhưng MỌI migration mới bắt buộc dùng lại `nowTextDefaultSql()`/`nowEpochMsDefaultSql()`/
`autoIncrementPrimaryKeySql()` và tránh tính năng riêng của Postgres** (JSONB operators, arrays, RLS — xem
mục 3) trừ khi bọc sau capability check — để chuyển sang TiDB sau này chỉ là đổi `ORCA_DB_URL`, không phải
viết lại migration.

### 2. Database-per-service: mỗi domain 1 Postgres **schema** riêng, không cross-schema FK

"1 database duy nhất" + "cô lập từng chức năng" được thoả mãn bằng pattern **database-per-service** kinh điển
của microservices — 1 *technology* (Postgres) duy nhất, nhưng mỗi service sở hữu **schema riêng**, không
query chéo schema, không FK SQL chéo schema (chỉ "logical FK" — lưu ID dạng giá trị thường, validate qua gọi
API nội bộ service khác, đúng pattern đã dùng sẵn cho `orca_project_source_projects.project_id` và
`orca_tasks.active_execution_task_id`). Bảng ánh xạ đầy đủ, rationale, và migration path sang container/DB
vật lý riêng từng service nằm ở
[specs/backend/models/08-postgres-microservices-target-architecture.md](../../../specs/backend/models/08-postgres-microservices-target-architecture.md).

13 service schema: `auth`, `tenant`, `project`, `infra`, `ai_provider`, `workflow`, `task`, `orchestration`,
`automation`, `annotation`, `notification`, `usage`, `credential` (chỉ metadata — xem mục 4).

### 3. Multi-tenancy: tenant = `tenant.companies.id`, cô lập ở APPLICATION layer là cơ chế chính

Mọi bảng multi-tenant-eligible có cột `tenant_id UUID NOT NULL` (không phải SQL FK sang `tenant.companies` —
cùng lý do "cô lập" ở mục 2). Cơ chế enforce theo thứ tự ưu tiên:

1. **Chính: `TenantContext` ở application layer** (`tenant-context.ts`, AsyncLocalStorage) — mọi query bắt
   buộc đi qua base repository luôn tự thêm `WHERE tenant_id = $current` — **không phụ thuộc tính năng
   Postgres-only** nên portable sang TiDB.
2. **Phòng vệ thêm (chỉ khi chạy Postgres): Row-Level Security (RLS)** — bật `ALTER TABLE ... ENABLE ROW LEVEL
   SECURITY` + policy theo `current_setting('app.tenant_id')`, set qua `SET LOCAL` đầu mỗi transaction. **Đây
   là lớp phòng vệ thứ 2, KHÔNG được coi là cơ chế duy nhất** — MySQL/TiDB không có RLS, nên logic tenant
   scoping ở app layer luôn phải đúng độc lập với RLS.

Lý do không dùng RLS làm cơ chế chính: tính portable sang TiDB (yêu cầu tương lai của chính ADR này) và tránh
"ảo tưởng an toàn" khi 1 service quên `SET LOCAL app.tenant_id` (RLS mặc định deny an toàn hơn không dùng gì,
nhưng không thay được việc mọi query phải tự giác lọc tenant).

### 4. Credential/secret: KHÔNG đưa giá trị bí mật vào Postgres

Chỉ metadata (trạng thái, thời điểm xoay key, scope, key-handle/reference id) nằm trong schema `credential`.
Giá trị secret thật tiếp tục qua cơ chế hiện có — `WebCredentialStore` (AES-256-GCM per-user), Electron
`safeStorage`, OS Keychain, encrypted-blob-qua-relay cho AI provider key (xem
[specs/backend/models/05](../../../specs/backend/models/05-credential-secret-stores.md)) — tránh gộp toàn bộ
rủi ro bảo mật vào 1 điểm lỗi duy nhất.

### 5. Giao tiếp liên-service: HTTP nội bộ, gateway tái dùng RPC namespace hiện có

`runtime/rpc/methods/index.ts` đã namespace theo domain (`ssh.ts`, `worktree.ts`, `annotation.ts`,
`orchestration.ts`...) — khi tách service thật (Phase 3), mỗi namespace handler trở thành 1 thin proxy gọi
HTTP nội bộ tới service tương ứng thay vì thực thi logic tại chỗ. `orca` server hiện tại (cổng 6768/6769) đóng
vai trò **API Gateway** — không cần thêm gateway mới. Xác thực liên-service: secret dùng chung qua biến môi
trường (cùng pattern `ORCA_AGENT_API_SECRET` đã có), service discovery giai đoạn đầu dùng Docker DNS trên
mạng `orca-net` (đã có sẵn trong `docker-compose.orca.artifact.yml`) — chưa cần Consul/etcd ở quy mô hiện tại.

## Cross-Dialect Considerations

Giữ nguyên bảng compatibility của ADR-016 (`UNIX_TIMESTAMP()` vs `EXTRACT(epoch...)` vs `strftime`,
`AUTO_INCREMENT` vs `GENERATED ALWAYS AS IDENTITY`...); bổ sung: **RLS (mục 3) và `CREATE SCHEMA` không có
tương đương 1-1 ở MySQL/TiDB** (TiDB không có RLS; MySQL "schema" = "database", không phải namespace phụ
trong 1 database) — khi chuyển TiDB, chiến lược schema-per-service sẽ đổi thành **database-per-service theo
đúng nghĩa TiDB** (mỗi service 1 `CREATE DATABASE` MySQL-style), migration code (dùng dialect helper) không
đổi, chỉ đổi cách `ORCA_DB_URL` trỏ tới.

## Trạng thái Implementation — Phased rollout

| Phase | Nội dung | Trạng thái |
|---|---|---|
| **0** | Migration scaffolding: tạo 13 schema + retrofit `tenant_id` cho bảng active + bảng mới cho Orchestration/Automation/Notification/Usage. Additive, không phá vỡ runtime hiện tại. | ✅ Implemented (migration 0019-0022) |
| **1** | Application-layer tenant isolation (`TenantContext` + base repository) + wire service code hiện có (AutomationService, WebPushManager, Claude/CodexUsageStore) sang đọc/ghi Postgres thay vì JSON, đứng sau interface không đổi | ⏳ Chưa triển khai |
| **2** | Migrate dữ liệu `orchestration.db` (SQLite) → schema `orchestration` (Postgres); viết migration script data-level, không chỉ DDL | ⏳ Chưa triển khai |
| **3** | Tách từng service thành container/process riêng sau `orca` gateway, bắt đầu từ service rủi ro thấp nhất (`usage`/`notification`) làm pilot trước khi tách `auth`/`tenant`/`project` | ⏳ Chưa triển khai |
| **4** | Dọn dẹp: DROP bảng dormant (migration 0001/0002/0003, `orca_terminal_sessions`, `orca_port_forwards`, `orca_push_subscriptions` cũ) sau khi xác nhận production (b15.openledger.vn) không còn dữ liệu; xoá `orchestration.db`/JSON automation-push-usage khỏi code path server mode | ⏳ Chưa triển khai |

## Cross-References

| Resource | Mô tả |
|---|---|
| [specs/backend/models/](../../../specs/backend/models/) | Khảo sát as-is toàn bộ data layer — nguồn cho ADR này |
| [specs/backend/models/08-postgres-microservices-target-architecture.md](../../../specs/backend/models/08-postgres-microservices-target-architecture.md) | Kế hoạch chi tiết: bảng ánh xạ service↔schema, DDL, phased rollout đầy đủ |
| [ADR-002](../v1/ADR-002-multi-database-iconnectionpool.md) | Nền tảng `IConnectionPool`/`MigrationRunner` đa dialect |
| [docs/guides/postgres-shared-database-design.md](../../guides/postgres-shared-database-design.md) | Thiết kế Postgres dùng chung hiện tại cho multi-user process fork |
