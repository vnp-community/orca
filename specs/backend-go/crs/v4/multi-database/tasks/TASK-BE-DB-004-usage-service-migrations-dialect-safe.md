# TASK-BE-DB-004: Tách migration `usage-service` theo dialect (`postgres/`, `mysql/`)

**Solution:** BE-DB-SOL-001 §5 | **CR:** CR-DB-002
**Service:** `usage-service`
**Depends on:** TASK-BE-DB-002 (chỉ để nhất quán track order — không phụ thuộc code trực tiếp)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** `impact()` đã chạy thật (dùng chung kết quả với
> TASK-BE-DB-003 vì cùng symbol `Repository`, cùng file bị sửa 1 dòng
> `filepath.Abs`) — risk **LOW**, impactedCount 3, an toàn. Di chuyển
> đúng 4 file migration Postgres vào `migrations/postgres/` bằng `git mv`
> (nội dung xác nhận **giống hệt 100%** bản gốc qua `git show HEAD:... | diff`),
> tạo đúng 4 file MySQL mới trong `migrations/mysql/` theo nội dung mẫu
> trong task doc (không cần điều chỉnh gì — mẫu đã đúng cú pháp MySQL 8).
> Sửa đúng 1 dòng `repository_test.go`'s `filepath.Abs("../../../migrations")`
> → `"../../../migrations/postgres"`. Sửa đúng dòng migrate command trong
> `README.md` (dòng 51-52 gốc) và `Makefile` (dòng 90 gốc, target
> `migrate-all`'s echo) thêm `/postgres`. **Không đổi các dòng khác** của
> README.md dù chúng cũng nhắc tới đường dẫn `migrations/0001_init...`
> (phần "What's implemented") — ngoài phạm vi liệt kê của task này, ghi
> nhận đây là mô tả hơi lỗi thời sau khi tách dialect nhưng không tự ý sửa
> thêm.
>
> **Build/test thật đã chạy**: Postgres — `setupRepository(t)` (dùng bởi
> mọi test trong `repository_test.go`) gọi `migrate -path
> .../migrations/postgres -database ... up` thành công ở MỌI lần chạy
> test integration trong task này và TASK-BE-DB-003 (không có lỗi migration
> nào, chỉ có lỗi ở tầng SQL logic bên trong test body — xem TASK-BE-DB-003
> để biết chi tiết bug tiền tồn tại không liên quan) → xác nhận việc đổi vị
> trí thư mục migration Postgres không phá vỡ gì. MySQL — cài `migrate` CLI
> với `-tags 'postgres,mysql'` (bản cài mặc định trước đó CHỈ có driver
> postgres, thiếu mysql — phải cài lại), rồi chạy thật trên container
> `mysql:8` (`docker run ... -p 3307:3306`): `migrate -path
> .../migrations/mysql -database "mysql://root:orca@tcp(localhost:3307)/usage"
> up` → **thành công** (2 migration áp dụng: `1/u init`, `2/u outbox`),
> `down -all` → **thành công** (rollback sạch cả 2), `up` lại lần 2 →
> **thành công**. Đúng chu trình `up → down → up` yêu cầu ở Verify. Container
> đã dọn (`docker rm -f`) sau khi xác nhận xong.

---

---

## Mục tiêu

Chuyển 4 file migration hiện tại (`migrations/0001_init.{up,down}.sql`,
`0002_outbox.{up,down}.sql`) vào `migrations/postgres/` (nội dung giữ
nguyên 100%, chỉ đổi vị trí), và viết 4 file tương ứng dialect-safe cho
MySQL/TiDB vào `migrations/mysql/`.

**Quyết định tên schema/bảng đã chốt ở BE-DB-SOL-002**: MySQL dùng 1
database tên `usage` (song song schema Postgres `usage`), bảng KHÔNG
prefix (`sessions`, `daily_rollups`, `outbox_events`).

## Files cần sửa

1. `backend-go/services/usage-service/migrations/0001_init.up.sql` → di chuyển thành `backend-go/services/usage-service/migrations/postgres/0001_init.up.sql` (nội dung giữ nguyên)
2. `backend-go/services/usage-service/migrations/0001_init.down.sql` → `.../postgres/0001_init.down.sql` (giữ nguyên)
3. `backend-go/services/usage-service/migrations/0002_outbox.up.sql` → `.../postgres/0002_outbox.up.sql` (giữ nguyên)
4. `backend-go/services/usage-service/migrations/0002_outbox.down.sql` → `.../postgres/0002_outbox.down.sql` (giữ nguyên)
5. `backend-go/services/usage-service/migrations/mysql/0001_init.up.sql` (MỚI)
6. `backend-go/services/usage-service/migrations/mysql/0001_init.down.sql` (MỚI)
7. `backend-go/services/usage-service/migrations/mysql/0002_outbox.up.sql` (MỚI)
8. `backend-go/services/usage-service/migrations/mysql/0002_outbox.down.sql` (MỚI)
9. `backend-go/services/usage-service/internal/adapter/postgres/repository_test.go` (MODIFY — dòng `filepath.Abs("../../../migrations")` phải trỏ `"../../../migrations/postgres"`)
10. `backend-go/services/usage-service/README.md` (MODIFY — dòng 51-52, thêm `/postgres` vào path lệnh `migrate`)
11. `backend-go/Makefile` (MODIFY — dòng 90, ví dụ echo cũng thêm `/postgres`)

## Nội dung `migrations/mysql/0001_init.up.sql`

```sql
-- usage-service owns this database exclusively — no other service reads or
-- writes these tables. MySQL/TiDB variant: no CREATE SCHEMA (a MySQL
-- database IS the schema-equivalent isolation unit — this migration
-- assumes DATABASE_DSN already points at a database named `usage`,
-- mirroring the Postgres variant's `usage` schema name; see
-- specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md).
CREATE TABLE sessions (
    id                  VARCHAR(64) PRIMARY KEY,
    tenant_id           CHAR(36) NOT NULL,
    user_id             CHAR(36) NOT NULL,
    provider            VARCHAR(16) NOT NULL,
    worktree_id         VARCHAR(64) NOT NULL DEFAULT '',
    input_tokens        BIGINT NOT NULL DEFAULT 0,
    output_tokens       BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens   BIGINT NOT NULL DEFAULT 0,
    cache_write_tokens  BIGINT NOT NULL DEFAULT 0,
    cost_usd            DOUBLE NOT NULL DEFAULT 0,
    started_at          TIMESTAMP(6) NOT NULL,
    ended_at            TIMESTAMP(6) NULL,
    request_id          VARCHAR(128) NOT NULL,
    created_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT provider_check CHECK (provider IN ('claude', 'codex', 'opencode')),
    CONSTRAINT input_tokens_check CHECK (input_tokens >= 0),
    CONSTRAINT output_tokens_check CHECK (output_tokens >= 0),
    CONSTRAINT cache_read_tokens_check CHECK (cache_read_tokens >= 0),
    CONSTRAINT cache_write_tokens_check CHECK (cache_write_tokens >= 0),

    UNIQUE KEY uniq_tenant_request (tenant_id, request_id)
);

CREATE INDEX idx_sessions_tenant_user ON sessions (tenant_id, user_id, started_at DESC);
CREATE INDEX idx_sessions_tenant_started ON sessions (tenant_id, started_at DESC);

-- No Row-Level Security equivalent in MySQL/TiDB — application-layer
-- tenant_id scoping in internal/adapter/mysql/repository.go is the ONLY
-- enforcement mechanism here, not a secondary backstop. See
-- TASK-BE-DB-003's finding: this was already true for the Postgres
-- adapter too (no code ever calls SET LOCAL app.tenant_id, so RLS never
-- actually activated there either) — this migration doesn't regress
-- anything, it just stops pretending a backstop exists that never ran.

CREATE TABLE daily_rollups (
    tenant_id            CHAR(36) NOT NULL,
    user_id              CHAR(36) NOT NULL,
    provider             VARCHAR(16) NOT NULL,
    day                  DATE NOT NULL,
    total_input_tokens   BIGINT NOT NULL DEFAULT 0,
    total_output_tokens  BIGINT NOT NULL DEFAULT 0,
    total_cost_usd       DOUBLE NOT NULL DEFAULT 0,
    session_count        BIGINT NOT NULL DEFAULT 0,

    CONSTRAINT daily_rollups_provider_check CHECK (provider IN ('claude', 'codex', 'opencode')),
    PRIMARY KEY (tenant_id, user_id, provider, day)
);
```

`VARCHAR(64)`/`CHAR(36)` thay `TEXT`/`UUID` — MySQL không có kiểu `UUID`
native; `usage-service` đã sinh UUID ở Go (`uuid.NewString()`, dạng
canonical 36 ký tự) nên `CHAR(36)` đủ, không cần đổi logic sinh ID.
`DOUBLE PRECISION` (Postgres) → `DOUBLE` (MySQL, cùng ngữ nghĩa). Không có
`gen_random_uuid()` trong bản gốc nên không có gì phải thay ở khoản này
(đã xác nhận ở BE-DB-SOL-001 §1).

## Nội dung `migrations/mysql/0002_outbox.up.sql`

```sql
-- MySQL/TiDB variant: payload JSON thay JSONB (MySQL 5.7+/TiDB có JSON
-- native nhưng KHÔNG có toán tử ->/@> kiểu JSONB của Postgres — outbox
-- relay (common/outbox.Relay) chỉ đọc payload nguyên khối để publish,
-- không query theo JSON operator, nên không mất chức năng ở use case này).
CREATE TABLE outbox_events (
    id            CHAR(36) PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,
    subject       VARCHAR(255) NOT NULL,
    occurred_at   TIMESTAMP(6) NOT NULL,
    version       INT NOT NULL,
    payload       JSON NOT NULL,
    created_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    published_at  TIMESTAMP(6) NULL
);

CREATE INDEX idx_outbox_events_unpublished ON outbox_events (created_at, published_at);
```

MySQL không hỗ trợ partial index (`WHERE published_at IS NULL` như bản
Postgres) — index đầy đủ `(created_at, published_at)` là thoả hiệp chấp
nhận được ở quy mô `usage-service` (ghi nhận, không phải bug).

## `*.down.sql` (cả 2 dialect)

Mirror đúng `DROP TABLE`/`DROP INDEX` theo thứ tự ngược của `.up.sql`
tương ứng — xem 2 file `.down.sql` hiện có làm mẫu cấu trúc (không đổi
logic, chỉ đổi tên bảng cho khớp bản MySQL không-schema-qualified).

## Verify

```bash
# Xác nhận migrate CLI chạy sạch trên MySQL thật (cần Docker)
docker run --rm -d --name usage-mysql-verify -e MYSQL_ROOT_PASSWORD=orca -e MYSQL_DATABASE=usage -p 3307:3306 mysql:8
# đợi container sẵn sàng rồi:
migrate -path backend-go/services/usage-service/migrations/mysql \
  -database "mysql://root:orca@tcp(localhost:3307)/usage" up
migrate -path backend-go/services/usage-service/migrations/mysql \
  -database "mysql://root:orca@tcp(localhost:3307)/usage" down
migrate -path backend-go/services/usage-service/migrations/mysql \
  -database "mysql://root:orca@tcp(localhost:3307)/usage" up
docker rm -f usage-mysql-verify

# Xác nhận bản Postgres (chỉ đổi vị trí) vẫn chạy y hệt trước
cd backend-go/services/usage-service && go test -tags=integration ./internal/adapter/postgres/... -v
```

`up → down → up` phải chạy sạch trên MySQL thật — theo đúng convention CI
Postgres đã có ở `05-data-architecture.md:66-67` (áp dụng lại cho dialect
mới).

## gitnexus

Không sửa symbol Go nào trực tiếp ở phần migration SQL. File Go duy nhất
bị sửa là `repository_test.go` (1 dòng `filepath.Abs`) — chạy
`impact({target: "Repository", direction: "upstream", file_path: "services/usage-service/internal/adapter/postgres/repository_test.go", kind: "Struct"})`
trước khi sửa: đã xác nhận risk **LOW**, impacted 2 (xem solutions/README.md)
— an toàn.
