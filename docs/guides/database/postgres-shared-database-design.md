# Postgres dùng chung — thiết kế + thực thi (BUG-BE-RPC-001/002/003)

**Ngày:** 2026-08-14. Follow-up trực tiếp của [BUG-BE-RPC-001](../bugs/bug-be-rpc-001-userid-not-forwarded.md)/
[BUG-BE-RPC-002](../bugs/bug-be-rpc-002-devservermanager-sync-get-in-user-process.md) — sau khi cả 2 fix
đó lên production, `project.create` lộ lỗi kế tiếp: `FOREIGN KEY constraint failed`.

## Root cause thật — BUG-BE-RPC-003: mỗi user-process có 1 DB SQLite cô lập riêng

Với `ORCA_MULTI_USER=1` (chế độ web hiện tại), mỗi user đăng nhập được `SessionManager`
`fork()` ra **1 process con riêng** (`spawnUserProcess()`), chạy `initializeOrcaServices()` lần
thứ 2 (lần đầu là process chính `orca-server`). Khi KHÔNG cấu hình `ORCA_DB_URL` (mặc định trước
hôm nay), `pool` (nơi `ProjectService`/`TeamService`/... sống) fallback về SQLite tại
`join(userDataPath, 'orca-server.db')` — và `userDataPath` **khác nhau theo từng process**:
process chính dùng `/data/orca`, mỗi user-process dùng `/data/orca/users/<userId>` riêng
(`SessionManager.spawnUserProcess()` set `ORCA_USER_DATA_PATH`).

**Hậu quả xác nhận bằng cách đọc trực tiếp 2 file SQLite trên server:**
```
/data/orca/orca-server.db                                    → orca_users: id=5540a296...  (DB trung tâm, dùng để login)
/data/orca/users/5540a296.../orca-server.db → orca_users: id=a7d8a58f...  (DB riêng của process đó — user_id KHÁC)
```
Cùng 1 email (`admin@b15.openledger.vn`) nhưng **2 `id` khác nhau** — vì `ensureFirstAdminUser()`
tự sinh `randomUUID()` ĐỘC LẬP ở mỗi file. `project.create` ghi `INSERT INTO
orca_v5_project_members (..., user_id, ...)` với `user_id = ctx.userId` (= `5540a296...`, đúng
đắn nhờ fix BUG-BE-RPC-001) — nhưng bảng `orca_users` TRONG CHÍNH user-process đó không có dòng
nào khớp id này → `FOREIGN KEY constraint failed`.

**Ảnh hưởng lớn hơn Create Project**: mọi tính năng thật sự CẦN chia sẻ dữ liệu giữa nhiều user
(Team, OrcaProject sharing) không thể hoạt động đúng với kiến trúc SQLite-cô-lập-theo-process này
dù BUG-BE-RPC-001/002 đã sửa xong — user A tạo Team, user B (process khác) sẽ không bao giờ thấy
được, vì 2 file SQLite hoàn toàn tách biệt.

## Quyết định: chuyển sang Postgres dùng chung (đã chốt, đã thực thi)

## Phát hiện thêm khi thiết kế: `authDb` KHÔNG theo `dbConfig` — nếu chỉ trỏ `pool` sang Postgres sẽ hỏng đăng nhập

`AuthManager`/`AuthUserStore` (login, session, `orca_users`/`orca_sessions`) dùng 1 connection
`IDatabase` RIÊNG (`authDb = new SqliteAuthAdapter(authDbPath)`), **hoàn toàn tách khỏi `pool`**
(nơi migrations thật sự chạy). `authDbPath`'s công thức chỉ theo `dbConfig.path` khi
`dbConfig.dialect === 'sqlite'` — với dialect khác (Postgres), nó fallback về
`join(userDataPath, 'orca-server.db')` **y hệt như trước**, tức vẫn SQLite-cô-lập-theo-process.

Nếu chỉ đổi `pool` sang Postgres mà không sửa `authDb`: `orca_users`/`orca_sessions`'s schema
(migration 0005) chỉ còn được tạo trong Postgres (vì migrations chạy qua `pool`) — user-process
mới spawn sẽ có `authDb` trỏ vào 1 file SQLite **không có bảng `orca_users` nào cả** →
**hỏng đăng nhập hoàn toàn** cho mọi user-process mới. Đây là lý do quyết định #1 (ban đầu) bị
tạm dừng để thiết kế kỹ hơn trước khi code.

## Fix: `PooledDatabaseAdapter` — cầu nối `IDatabase` ↔ `IConnectionPool`

File mới: `backend/src/main/db/pooled-database-adapter.ts`. `AuthUserStore` được viết cho 1
connection `IDatabase` luôn mở (gọi `this.db.prepare(sql)` rồi `.get()/.run()/.all()` ngay trong
cùng 1 hàm async, không giữ statement qua nhiều thao tác khác nhau) — không phải cho
`IConnectionPool`'s pattern `withConnection(fn)`. `PooledDatabaseAdapter` bọc 1 `IConnectionPool`
thành `IDatabase` mà KHÔNG cần sửa `AuthUserStore`:

- `prepare(sql)` **không** đụng tới pool — chỉ giữ lại chuỗi SQL (deferred).
- `.get()/.run()/.all()` (gọi trên statement trả về) mới thật sự `pool.withConnection(...)` —
  acquire, prepare thật, execute, release — mỗi lời gọi độc lập, đúng ngữ nghĩa pooling.
- `.transaction()` **cố ý throw lỗi rõ ràng** thay vì chạy sai âm thầm — vì callback của
  `IDatabase.transaction(fn)` không nhận `db` tham số, nếu bên trong gọi lại `prepare()` sẽ acquire
  1 connection KHÁC (không transactional). `AuthUserStore` hiện không dùng `.transaction()` nên
  không bị ảnh hưởng — chỉ chặn trước cho tương lai.
- `.close()` là no-op — pool là tài nguyên DÙNG CHUNG (Project/Team/Task cũng dùng), không được
  drain/destroy chỉ vì `AuthManager` gọi `close()`.

`server-bootstrap.ts`: `authDb` giờ là `await PooledDatabaseAdapter.create(pool)` khi
`dbConfig && dbConfig.dialect !== 'sqlite'` (đúng điều kiện đã chọn `GenericConnectionPool` cho
`pool` phía trên) — **fallback y hệt hành vi cũ** (`SqliteAuthAdapter` riêng) khi không cấu hình
DB dùng chung, không có gì đổi cho deployment không dùng Postgres.

## Hạ tầng: Postgres container trong `deploy/dev`

Thêm service `postgres` (`postgres:16-alpine`, volume riêng `orca-postgres-data`) vào cả 3 file
compose (`docker-compose.orca.artifact.yml` — file THẬT đang deploy qua
`sync-to-server-artifact.sh`; `docker-compose.orca.yml` — bản build-trên-server; `docker-compose.yml`
— bản 1-server gộp Nginx, để đồng bộ, dù không phải bản đang dùng cho `b15.openledger.vn`).

`orca` service thêm `ORCA_DB_URL: postgresql://${ORCA_DB_USER}:${ORCA_DB_PASSWORD}@postgres:5432/${ORCA_DB_NAME}`
+ `depends_on: postgres: condition: service_healthy`. `.env` thêm `ORCA_DB_USER`/`ORCA_DB_PASSWORD`
(sinh bằng `openssl rand -hex 24`)/`ORCA_DB_NAME`.

**Vì sao propagate đúng tới mọi user-process mà không cần sửa gì thêm**: `ORCA_DB_URL` chỉ cần
set ở container-level env — `loadDatabaseConfig()` đọc `process.env` trực tiếp, và
`SessionManager.spawnUserProcess()` fork mỗi user-process với `env: { ...process.env, ... }` —
spread nguyên vẹn toàn bộ env của process chính, bao gồm `ORCA_DB_URL`.

## ⚠️ Mất dữ liệu SQLite hiện có — chấp nhận được (dev/test server)

Đây là server dev (`deploy/dev/`), toàn bộ dữ liệu hiện có là dữ liệu test trong phiên làm việc
này (do các bug BUG-BE-RPC-001/002/003 nên hầu như chưa tạo được Project/Team nào thành công qua
UI thật). Chuyển sang Postgres = bắt đầu từ database rỗng — `ensureFirstAdminUser()` sẽ tự tạo
lại tài khoản admin với ĐÚNG `ORCA_ADMIN_EMAIL`/`ORCA_ADMIN_PASSWORD` đã có sẵn trong `.env`
(`admin@b15.openledger.vn`) — đăng nhập tiếp tục hoạt động với cùng thông tin, không cần đổi gì
phía người dùng.

## Việc CỐ Ý không đụng: `WebCredentialStore`

`WebCredentialStore` (token Bitbucket/Azure DevOps/Gitea mã hoá riêng từng user) dùng path theo
`userDataPath` TƯƠNG TỰ, nhưng đây là THIẾT KẾ ĐÚNG — mỗi user nên có vault credential RIÊNG,
không phải lỗi cần sửa giống `authDb`. Không migrate cái này sang Postgres.

## ⚠️ Cập nhật 2026-08-14 (sau khi thử deploy thật): TẠM DỪNG — migrations không tương thích Postgres

Deploy thật lên `b15.openledger.vn` phát hiện thêm 2 lớp lỗi liên tiếp, mỗi lớp phải sửa xong mới
lộ ra lớp sau:

1. **`pg` package chưa từng được cài** trong toàn bộ monorepo (`pnpm-lock.yaml` xác nhận 0 kết
   quả) — dù `pg-adapter.ts` đã có sẵn code. Đã sửa: `pnpm add pg` (backend/package.json). Lộ
   thêm 2 pre-existing gap không liên quan (root `config/patches/`, `config/scripts/
   rebuild-native-deps.mjs` bị thiếu, chỉ có bản sao ở `desktop/config/` — Dockerfile.artifact tự
   COPY nên production build không bị ảnh hưởng, nhưng `pnpm install` chạy trực tiếp trên máy dev
   thì luôn fail) — đã copy nguyên trạng sang root để sửa luôn.

2. **Migrations 0001-0017 là SQL SQLite thuần, không chạy được trên Postgres**: sau khi có `pg`,
   server crash ngay ở bước migration: `function datetime(unknown) does not exist` (SQLite có
   hàm `datetime()`, Postgres không có) — migration thất bại (log "non-fatal" nhưng hậu quả là
   **`orca_users` không bao giờ được tạo**), server crash-loop ngay sau đó khi
   `ensureFirstAdminUser()` cố query 1 bảng không tồn tại.

**Đã tạm phục hồi dịch vụ**: comment `ORCA_DB_URL`/`depends_on: postgres` trong cả 3 file
compose — server quay lại SQLite fallback (như trước khi có Postgres), xác nhận `dialect: sqlite`,
healthy, 0 lỗi qua `/health`. **Giữ nguyên toàn bộ phần code đã viết** (`PooledDatabaseAdapter`,
wiring trong `server-bootstrap.ts`, `postgres` service trong compose, `pg` dependency) — chỉ tắt
đường dẫn kích hoạt (`ORCA_DB_URL`), sẵn sàng bật lại ngay khi migrations đã tương thích.

**Việc còn lại trước khi bật lại** (chưa làm, ngoài phạm vi phiên hôm nay — cần rà soát kỹ, không
vội): audit + sửa cả 17 file `backend/src/main/db/migrations/000*.ts` cho tương thích cả 2
dialect — ít nhất cần xử lý: `datetime()`/`strftime()` (SQLite) → tương đương Postgres
(`NOW()`, hoặc dùng `INTEGER`/epoch ms nhất quán như phần lớn migration đã làm, tránh gọi hàm
ngày-giờ SQL-cụ-thể); `INTEGER PRIMARY KEY AUTOINCREMENT` (SQLite) → `SERIAL`/`GENERATED ALWAYS
AS IDENTITY` (Postgres); kiểu `INTEGER` dùng làm boolean (SQLite không có `BOOLEAN` thật) — cần
xác nhận Postgres adapter có tự chuyển đổi hay migration phải viết khác nhau theo dialect (có
thể cần `IDatabase.capabilities.dialect` để rẽ nhánh SQL ngay trong từng migration, hoặc viết 1
lớp trừu tượng SQL builder — cần thiết kế, không phải sửa nhỏ).

## ✅ Cập nhật 2026-08-14 (tiếp): đã sửa xong, verify thật, đã bật lại

Sau khi tắt tạm để phục hồi dịch vụ, đã audit + sửa dứt điểm cả 2 lớp lỗi tìm thấy:

### 1. 7 file migration dùng SQL SQLite thuần

`backend/src/main/db/migrations/sql-dialect.ts` (mới) — 3 helper thuần hàm (`nowTextDefaultSql`,
`nowEpochMsDefaultSql`, `autoIncrementPrimaryKeySql`), rẽ nhánh theo `db.capabilities.dialect`.
Áp dụng vào: `0001_initial_schema.ts`, `0002_add_automations.ts`, `0003_add_workspace_sessions.ts`,
`0004_orca_app_tables.ts` (đều dùng `datetime('now')`), `0005_add_auth_schema.ts`,
`0008_ai_providers.ts`, `0010_tasks.ts` (đều có `AUTOINCREMENT`, `0010` thêm `strftime(...)`).

### 2. `pg-adapter.ts` chưa dịch placeholder — ảnh hưởng TOÀN BỘ query, không chỉ migration

Phát hiện khi test thật: sau khi sửa xong (1), migration tự nó chạy được, nhưng bước
`INSERT INTO schema_migrations (...) VALUES (?, ?, ?)` (chính `MigrationRunner`, không phải file
migration nào) crash `syntax error at or near ","` — vì `pg` (driver Postgres) cần placeholder
`$1, $2, $3`, không hiểu `?` (quy ước SQLite mà TOÀN BỘ codebase dùng, `AuthUserStore`,
`ProjectService`, `TeamService`, ... không trừ chỗ nào). Sửa 1 chỗ duy nhất:
`translatePlaceholders()` trong `pg-adapter.ts`, áp dụng cho mọi `query()`/`prepare()` — bỏ qua
`?` nằm trong chuỗi literal (`'...'`) để không dịch nhầm.

### 3. Cột `INTEGER` lưu epoch-milliseconds tràn số ở Postgres

Phát hiện tiếp khi test insert thật: Postgres `INTEGER` là 32-bit (~2.1 tỷ), nhưng
`Date.now()` (epoch ms, 13 chữ số) vượt xa giới hạn đó — SQLite's `INTEGER` không có giới hạn
kiểu này (type affinity linh hoạt) nên không lộ ra khi chỉ test SQLite. Đổi toàn bộ cột
timestamp (`created_at`, `updated_at`, `expires_at`, `added_at`, `due_date`, `paused_at`,
`rotation_grace_until`, ...) từ `INTEGER` → `BIGINT` — an toàn cho cả 2 dialect (`BIGINT` ở
SQLite vẫn chỉ là INTEGER affinity, hành vi giống hệt trước). Cột nhỏ thật sự (`port`, `enabled`,
`priority`, boolean 0/1, ...) giữ nguyên `INTEGER`.

### Verify thật trước khi bật lại (không chỉ tin tưởng, đã chạy thật)

Dựng 1 Postgres cục bộ (`docker run postgres:16-alpine`), chạy `MigrationRunner` thật với
`ALL_MIGRATIONS` (17 file) — áp dụng sạch. Test tiếp: `AuthUserStore.countAdmins()`-style query
thật, insert vào bảng có `AUTOINCREMENT`-cũ (`orca_audit_log`), và **đúng luồng đã crash trong
production** (`project.create`: insert `orca_users` → insert `orca_v5_projects` → insert
`orca_v5_project_members` với FK) — tất cả chạy đúng, không lỗi.

Đã bật lại `ORCA_DB_URL`/`depends_on: postgres` trong cả 3 file compose. Test thêm:
`sql-dialect.test.ts` (9 case), `pg-adapter.test.ts` (6 case, gồm case chuỗi literal chứa `?`) —
backend suite tổng 139/139 pass.

## Kế hoạch verify (bắt buộc trước khi coi là xong)

1. Unit test `PooledDatabaseAdapter` (fake `IConnectionPool`) — xác nhận acquire/release đúng số
   lần, `prepare()` không chạm pool, `.transaction()` throw rõ ràng, `.close()` không drain pool.
2. Backend test suite đầy đủ không đổi hành vi (SQLite-fallback path không chạm tới).
3. Deploy thật, đọc boot log xác nhận: `[DB Config] Using database from ORCA_DB_URL:
   postgresql://...`, container `postgres` healthy trước khi `orca` start.
4. Đăng nhập lại bằng `admin@b15.openledger.vn` — xác nhận vẫn vào được (admin được re-seed vào
   Postgres rỗng).
5. Thử Create Project thật qua UI — xác nhận KHÔNG còn `FOREIGN KEY constraint failed`.
6. **Quan trọng nhất — verify đúng vấn đề gốc đã hết**: buộc spawn 1 user-process MỚI cho cùng
   user đó (kill process cũ hoặc đợi idle timeout, hoặc reconnect sau khi container restart) rồi
   xác nhận Project vừa tạo VẪN hiển thị đúng — chứng minh dữ liệu giờ thật sự dùng chung qua
   Postgres, không còn cô lập theo process.
