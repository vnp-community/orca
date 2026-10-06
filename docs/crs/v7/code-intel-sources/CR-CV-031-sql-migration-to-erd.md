# CR-CV-031 — Parse SQL migration thành `ErdModel` (Postgres, MySQL) và liên kết bảng với code

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-031 |
| **Tên** | Dựng ERD theo service từ `backend-go/services/*/migrations/{postgres,mysql}/*.up.sql`, kèm liên kết logic giữa service và cạnh bảng → hàm repository |
| **Loại** | Feature (parser + use case + RPC `GetErd`) |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-030 (đọc file), CR-CV-011/012 (binding, bảng), CR-CV-020 (`SymbolRef`, envelope `CodeIntelResult`) |
| **Mở khoá** | CR-CV-034 (`StoreAccess`), CR-CV-035 (bản đồ lưu trữ), CR-CV-038 (so sánh schema), CR-CV-057 (lens ERD) |
| **Tác động** | `backend-go/proto/orca/codeintel/v1` (message ERD, RPC `GetErd`), `code-intel-service/internal/domain/erd` (mới), `internal/usecase/build_erd.go` (mới), `internal/adapter/sqlmigration` (mới, parser), `internal/adapter/gosqlscan` (mới, quét tên bảng trong Go) |
| **Phụ thuộc dữ liệu ngoài** | **E1** (migration, có trong repo); **E7** (quan hệ logic giữa service: một phần có sẵn dưới dạng chú thích, phần còn lại cần bạn cung cấp); E14 (`information_schema`, tuỳ chọn, mặc định không dùng); E15 (ADR, chưa đọc) |

---

## 1. Bối cảnh và vấn đề

[08 §5](../../../research/view-code/08-views-and-review-models.md) yêu cầu ERD theo service lấy từ migration, không đọc DB thật, hỗ trợ cả Postgres và MySQL, kèm liên kết ngược về code. Khảo sát (2026-10-05, đọc file và `grep` trên toàn bộ `backend-go/services/*/migrations`):

### 1.1 Quy mô và bố cục

- 588 file `.sql` = 294 `.up.sql` + 294 `.down.sql`; 302 file ở `postgres/`, 286 ở `mysql/`. 17 service có thư mục; `mcp-service` chỉ có `postgres/` (16 file, 8 `.up`), 16 service còn lại có cả hai. Tổng dung lượng `.up.sql` ~ 398 KiB; file lớn nhất 7,7 KiB (`orchestration-service/migrations/mysql/0001_init.up.sql`). Mọi file nhỏ hơn rất nhiều trần 1 MiB của CR-CV-030.
- Tên file `NNNN_<mô tả>.up.sql`. Số có **khoảng trống** (ví dụ `infra-fleet-service`: `0001..0006` rồi `0013..0038`, không có `0007..0012`) và **không có số trùng** trong cùng một thư mục dialect (đã kiểm tra). Runtime dùng golang-migrate (`deploy/dev/scripts/migrate.sh`, một-lần `migrate-<svc>` trong compose); thứ tự áp dụng = số tăng dần.
- Postgres: mỗi service một schema (`CREATE SCHEMA IF NOT EXISTS <tên>`; 17 schema): `ai_provider, annotation, auth, automation, credential, infra, issuestatussync, issuetracking, mcp, notification, orchestration, project, scm, task, tenant, usage, workflow`. **Tên schema không luôn bằng tên service** (`credential` cho `credential-broker-service`, `scm` cho `scm-integration-service`, `usage`, `ai_provider`). Bảng luôn viết có tiền tố schema.
- MySQL: mỗi service một database, bảng **không** có tiền tố; `CHAR(36)` thay `UUID` (≥ 92 file), `TIMESTAMP(6)`, `JSON` thay `JSONB`, không RLS, một số bảng có `ENGINE=InnoDB`.
- Không có khoá ngoại xuyên service: mọi `REFERENCES` tìm thấy ở Postgres trỏ về cùng schema (đã liệt kê toàn bộ ở mức tên schema; khớp quy ước README v7 mục 6).

### 1.2 Tính năng SQL thực tế (đếm bằng `grep -E` trên `*.up.sql`, tính cả dòng comment nên là cận trên)

| Mệnh đề | Postgres (số file / dòng) | MySQL (số file / dòng) |
|---|---|---|
| `CREATE TABLE` (đầu dòng) | 76 / 120 | 70 / 104 |
| `ALTER TABLE` | 119 / 168 | 77 / 105 |
| `ADD COLUMN` (nhiều mệnh đề, kèm comment xen giữa) | 71 / 176 | 72 / 179 |
| `ALTER COLUMN` | 3 / 3 | 0 |
| `MODIFY COLUMN` | 0 | 2 / 2 |
| `DROP COLUMN`, `DROP TABLE`, `RENAME ...` | 0 | 0 |
| `DROP INDEX` | 1 | 1 (`ALTER TABLE … DROP INDEX`) |
| `DROP CONSTRAINT` | 4 file (có 1 mệnh đề lệnh thật ở mỗi file `0003 task`, `0005 workflow`, `0004 credential`, `0034 infra`) | chỉ ở comment |
| `CREATE [UNIQUE] INDEX` | 106 / 189 | 94 / 158 |
| index một phần (`WHERE …`) | có (vd `idx_infra_agent_sessions_active_per_worktree_user`) | **không có**; thay bằng cột sinh `GENERATED ALWAYS AS (…) STORED` + `UNIQUE` (14 file, 28 dòng) |
| index biểu thức (`COALESCE(..)::uuid` trong cột index) | có (`task/0014`) | không thấy |
| `CHECK (…)` (cột và bảng, đa dòng) | 53 / 133 | 46 / 89 |
| `REFERENCES` / `FOREIGN KEY` | 54 / 93 (viết kiểu cột) | 50 / 86 (`REFERENCES`), 36 file có `FOREIGN KEY` |
| `ADD CONSTRAINT` | 5 | 16 file |
| `UNIQUE` (cột/bảng/`CONSTRAINT … UNIQUE`/`UNIQUE KEY`) | 46 | 42 |
| `PRIMARY KEY (a, b)` bảng-cấp | 28 (cả hai dialect) | |
| `ENABLE ROW LEVEL SECURITY` / `FORCE` | 56 file / 88 dòng (**một phần trong `DO $$`**) | 13 file nhưng đều là comment |
| `CREATE POLICY` | 57 file / 94 dòng; tên `tenant_isolation` 81 lần | 9 file, comment |
| `COMMENT ON` | 2 | 5 (xem lại: có thể là comment thường) |
| `CREATE SCHEMA` | 17 | 14 (kiểm tra ngữ cảnh) |
| `CREATE EXTENSION`/`TYPE` | 1 (`pgcrypto`) | 0 |
| `DO $$ … END $$` | 4 file (`auth/0011`, `mcp/0001,0002,0003`) | 0 |
| `CREATE TRIGGER` | 0 | 1 (`task/0001`) |
| `CREATE FUNCTION/VIEW/MATERIALIZED/PARTITION` | 0 | 0 |
| DML (`INSERT`/`UPDATE … SET`/`DELETE FROM`) | 4 file (backfill) | 5 file |
| `GENERATED ALWAYS AS IDENTITY` / `AUTO_INCREMENT` | 6 / 4 | |
| Kiểu thường gặp | `UUID`, `TEXT`, `TIMESTAMPTZ`, `JSONB`, `INT`, `BOOLEAN`, `BIGINT`, `BYTEA`, `DOUBLE PRECISION`, `DATE`, `NUMERIC`, `BIGSERIAL` | `CHAR(36)`, `TIMESTAMP(6)`, `TEXT`, `JSON`, `VARCHAR(n)`, `INT`, `BIGINT`, `BOOLEAN`, `DOUBLE`, `BLOB`, `DECIMAL` |

Ghi chú về số liệu: đây là `grep` thô, chưa phải thống kê từ parser; một số dòng nằm trong comment (nhiều migration MySQL có comment dài chứa ví dụ SQL Postgres, vd `-- ALTER TABLE ... ENABLE ROW LEVEL SECURITY`). Việc đếm lại bằng chính parser nằm ở tiêu chí chấp nhận.

### 1.3 Các điểm khó đã gặp khi đọc file mẫu

1. **RLS sinh động trong `DO $$`**: `mcp-service/migrations/postgres/0001_init.up.sql:47-57` và `auth-service/.../0011_oauth_authorization_server.up.sql:80-90` dùng `FOREACH t IN ARRAY ARRAY['a','b',…] LOOP EXECUTE format('ALTER TABLE mcp.%I ENABLE ROW LEVEL SECURITY', t); … format($p$CREATE POLICY tenant_isolation ON mcp.%I USING (…)$p$, t)`. Một parser DDL tĩnh không thấy các policy này. Phải nhận dạng đúng thành ngữ này, không thực thi PL/pgSQL.
2. **Ràng buộc đặt tên ngầm**: `infra-fleet-service/migrations/postgres/0034_…up.sql:16` `DROP CONSTRAINT terminal_sessions_connection_id_fkey` tham chiếu tên do Postgres tự sinh (`<bảng>_<cột>_fkey`); `task/0003` `tasks_status_check` (`<bảng>_<cột>_check`). Parser phải tự sinh tên ngầm như Postgres. MySQL dùng `<bảng>_ibfk_N` (chưa thấy `DROP FOREIGN KEY` nào, nhưng thấy `DROP INDEX`).
3. **Comment mang thông tin**: nhiều comment có dấu chấm phẩy (`--` kèm `;`: 76 dòng ở Postgres), nên phải bỏ comment **trước** khi tách câu lệnh. Không thấy chuỗi literal chứa `;`; vẫn phải xử lý literal `'…'` và dollar-quote (`$$`, `$p$`).
4. **Chú thích liên kết logic có sẵn**: `-- logical FK -> project-service` (và biến thể `logical FK -> tenant-service.users`, `auth.users`, `infra-fleet-service.dev_servers`, `task-service.tasks.id`, `ai_provider.accounts`) ở 17 file Postgres; ví dụ `infra/0019_agent_sessions.up.sql`: `worktree_id UUID NOT NULL, -- logical FK -> project-service`. Đây là nguồn E7 rẻ nhất.
5. **Mệnh đề `ALTER TABLE` nhiều khoá**, có comment xen kẽ (`ai-provider-service/migrations/mysql/0003_…`), cột `GENERATED … STORED` thêm bằng `ADD COLUMN`.
6. **MySQL không có RLS/partial index** và nhắc lại bằng comment: không được coi comment là DDL. Với TiDB, dùng chung dialect MySQL (README v6/series cho phép DSN `tidb://`; ở đây không phân biệt).
7. **Giá trị mặc định kiểu biểu thức**: `DEFAULT (UUID())`, `DEFAULT (JSON_ARRAY())`, `DEFAULT gen_random_uuid()`, `DEFAULT now()`, `CURRENT_TIMESTAMP(6)`.

### 1.4 Hiện trạng thư viện

`go.work` liệt kê 22 module (`common`, `proto`, `cmd/orca-cli`, 19 service); `go.mod` của `common`, `proto`, `infra-fleet-service` (đã đọc) không có thư viện parse SQL, chỉ có `pgx/v5`, `go-sql-driver/mysql` (một số service) và `gopkg.in/yaml.v3`. `golang-migrate` chạy dưới dạng image, không là dependency Go. Dockerfile các service đã đọc (`annotation`, `api-gateway`, `scm-integration`, `task`, `credential-broker`) build với `CGO_ENABLED=0`. Module cache cục bộ của máy khảo sát không chứa thư viện parse SQL.

## 2. Giải pháp đề xuất

### 2.1 Chọn cách parse (CHƯA CHỐT, cần duyệt)

| Phương án | Ưu | Nhược | Ghi chú |
|---|---|---|---|
| **A. Parser tự viết (tokenizer + bộ phân tích DDL có chịu lỗi), không dependency mới** (đề xuất) | Không thêm dependency, `CGO_ENABLED=0` giữ nguyên; kiểm soát chính xác tập con thực tế (mục 1.2); câu lệnh lạ chỉ sinh cảnh báo, không hỏng cả service; xử lý được thành ngữ `DO $$` bằng nhận dạng mẫu | Phải tự bảo trì; mỗi dialect cần nhánh riêng; rủi ro sót cú pháp hiếm | Tập tính năng nhỏ (không function/view/partition), có sẵn 294 file làm kho kiểm thử vàng |
| B. `pganalyze/pg_query_go` (libpg_query) cho Postgres | Parse đúng chuẩn Postgres | Cần cgo; xung đột `CGO_ENABLED=0` của Dockerfile hiện tại; `DO` body là chuỗi mờ, vẫn phải nhận dạng mẫu | Biến thể wasm (`wasilibs/go-pgquery`) tránh cgo nhưng thêm runtime wasm; chưa đánh giá |
| C. `pingcap/tidb/pkg/parser` hoặc `vitess.io/.../sqlparser` cho MySQL | Parse đúng MySQL/TiDB | Dependency lớn; không giải quyết Postgres | Chưa đánh giá kích thước, giấy phép, tương thích Go 1.25/1.26 |
| D. Dựng DB thật (testcontainers) rồi đọc `information_schema` | Chính xác nhất | Nặng, cần Docker trên `code-intel-service`, khác nguyên tắc "không đọc DB thật" của 08 §5 | Chỉ dùng làm bộ kiểm chứng ở CI (xem mục 5), không là đường chạy chính |

Đề xuất: **A**, kiểm chứng bằng D ở CI. Chưa chốt vì: (1) chưa chạy thử đo độ phủ của tokenizer trên 294 file; (2) chưa đánh giá B/C (kích thước, giấy phép, cgo). Nếu bị loại A thì CR phải chuyển sang B+C và Dockerfile phải đổi.

### 2.2 Quy ước thư mục và thứ tự áp dụng

1. Khám phá: `ListDir(backend-go/services)` (CR-CV-030) → mỗi service có `migrations/` → dialect = tên thư mục con (`postgres`|`mysql`). Đường dẫn gốc `backend-go/services` là cấu hình (`ERD_MIGRATION_GLOB`, mặc định `backend-go/services/*/migrations/{postgres,mysql}`), vì repo khác có thể đặt khác.
2. Chỉ đọc `*.up.sql`; bỏ `*.down.sql`. Tách tiền tố số (`^\d+`), sắp tăng dần theo giá trị số; nếu trùng số thì sắp theo tên và ghi cảnh báo `DUPLICATE_VERSION` (hiện chưa có).
3. `asOfMigration` = tên file có số lớn nhất đã áp dụng thành công.
4. Hai dialect của cùng service được dựng thành **hai `ErdModel`**; UI mặc định Postgres, có công tắc. Công cụ không cố gộp hai dialect; thay vào đó có kiểm tra lệch (mục 2.8).
5. Schema của model: Postgres lấy từ `CREATE SCHEMA`; MySQL không có tiền tố nên dùng schema của bản Postgres cùng service nếu có, không thì tên thư mục service (xem Q4).

### 2.3 Mô hình nội bộ

`internal/domain/erd` (mới): `Catalog` (map `schema.table` → `Table`), áp lệnh theo thứ tự. Mỗi `Table`: `Columns` (thứ tự khai báo), `PrimaryKey []string`, `Uniques`, `Indexes` (tên, cột/biểu thức, `unique`, `partial: string`), `Checks` (tên, biểu thức thô đã chuẩn hoá khoảng trắng), `ForeignKeys`, `Policies`, `RLS{Enabled,Forced}`, `Comment`, `FirstMigration`, `LastMigration` (số, dùng cho CR-CV-038 và `ChangeOverlay.touchedTables`).

Các câu lệnh hỗ trợ **tối thiểu** (dựa trên mục 1.2; mọi thứ khác → `UnsupportedStatement` cảnh báo, giữ nguyên trạng thái bảng):

1. `CREATE SCHEMA [IF NOT EXISTS]`; `CREATE EXTENSION` (bỏ qua, ghi nhận).
2. `CREATE TABLE [IF NOT EXISTS] [schema.]t ( cột…, ràng buộc… ) [tuỳ chọn bảng]`: cột với kiểu (kể cả `(n)`, `(p,s)`, `[]`, `DOUBLE PRECISION`, `TIMESTAMP(6)`), `NOT NULL`/`NULL`, `DEFAULT <biểu thức cân ngoặc>`, `PRIMARY KEY` kiểu cột và bảng, `UNIQUE` kiểu cột và bảng, `CHECK (…)` có tên hoặc không, `REFERENCES t(c) [ON DELETE|UPDATE …]`, `GENERATED ALWAYS AS IDENTITY|(expr) STORED`, `AUTO_INCREMENT`, `CONSTRAINT <tên>` tiền tố; MySQL: `KEY`/`UNIQUE KEY`/`INDEX` nội dòng, `FOREIGN KEY (…) REFERENCES …`, `ENGINE=…`, `DEFAULT CHARSET=…`, `COLLATE`.
3. `ALTER TABLE t` với danh sách mệnh đề phân tách dấu phẩy ở mức ngoặc 0: `ADD [COLUMN] [IF NOT EXISTS]`, `DROP COLUMN`, `ALTER COLUMN … TYPE|SET|DROP DEFAULT|NOT NULL`, `MODIFY [COLUMN]`, `CHANGE COLUMN`, `ADD CONSTRAINT … (PRIMARY KEY|UNIQUE|CHECK|FOREIGN KEY)`, `DROP CONSTRAINT [IF EXISTS]`, `DROP INDEX` (MySQL), `ENABLE|FORCE ROW LEVEL SECURITY`, `RENAME …` (hiện chưa dùng nhưng hỗ trợ vì rẻ).
4. `CREATE [UNIQUE] INDEX [CONCURRENTLY] [IF NOT EXISTS] n ON [schema.]t [USING m] (cột|biểu thức [ASC|DESC]…) [INCLUDE (…)] [WHERE …]`; `DROP INDEX [IF EXISTS] [schema.]n` (Postgres, tên có tiền tố schema như `DROP INDEX task.idx_task_sources_unique`).
5. `DROP TABLE [IF EXISTS]` (chưa dùng; hỗ trợ cho tương lai).
6. `CREATE POLICY n ON t [FOR cmd] [TO role] [USING (…)] [WITH CHECK (…)]`; `COMMENT ON TABLE|COLUMN … IS '…'`.
7. Thành ngữ `DO $tag$ … FOREACH v IN ARRAY ARRAY[…] LOOP EXECUTE format('ALTER TABLE <schema>.%I (ENABLE|FORCE) ROW LEVEL SECURITY', v); EXECUTE format($p$CREATE POLICY n ON <schema>.%I …$p$, v); END LOOP; END $tag$`: nhận dạng bằng mẫu cứng, khai triển cho từng phần tử mảng. Khối `DO` không khớp mẫu → cảnh báo `OPAQUE_DO_BLOCK`, bảng liên quan đánh dấu `rls: "unknown"`.
8. `CREATE TRIGGER` (MySQL): ghi nhận tên trigger và bảng; không phân tích thân (`task/0001`).
9. DML (`INSERT`/`UPDATE`/`DELETE`) và `SELECT` trong `.up.sql`: **bỏ qua** (backfill); không ảnh hưởng schema.

Bộ chuẩn hoá kiểu (`Column.type`): giữ kiểu gốc (`type`) và thêm `canonicalType` (`uuid`, `text`, `timestamp`, `json`, `bool`, `int`, `bigint`, `float`, `bytes`, `numeric`, `date`, `other`) để UI vẽ và để so sánh hai dialect: `CHAR(36)` ↔ `uuid` chỉ khi cột là khoá/`*_id` (ngưỡng heuristic, nhãn "suy luận").

Tách câu lệnh: bỏ `-- …` và `/* … */`; theo dõi literal `'…'` (với `''`), dollar-quote `$tag$…$tag$`, và độ sâu ngoặc; tách ở `;` mức 0. Khi lỗi cú pháp trong một câu lệnh: ghi `ParseWarning{file, line, stmtKind, message}`, **tiếp tục** với câu lệnh sau, và đánh dấu bảng bị ảnh hưởng `degraded:true`.

### 2.4 Xử lý đặc thù theo dialect

| Chủ đề | Postgres | MySQL/TiDB |
|---|---|---|
| Tên bảng | `schema.table`; bảng không tiền tố → `search_path` mặc định = schema của service, ghi cảnh báo | `table` |
| Tên ràng buộc ngầm | `<t>_<c>_fkey`, `<t>_<c>_check`, `<t>_pkey`, `<t>_<c>_key` | `<t>_ibfk_<n>`, khoá/chỉ mục theo tên khai báo |
| RLS | Bảng cờ `ENABLE`/`FORCE`; policy là chuỗi, gồm khai triển `DO` | Không có; comment bị bỏ qua. Trường `tenantEnforcement: "application"` |
| Index một phần | `partial` giữ biểu thức `WHERE` | Không có; nếu thấy cột sinh `GENERATED … STORED` + `UNIQUE` đi kèm, đánh dấu `emulatesPartialUnique:true` (suy luận) |
| Kiểu | như 2.3 | như 2.3 |
| Quy tắc `IF NOT EXISTS` | bỏ qua nếu đã có | bỏ qua nếu đã có |

### 2.5 Dựng quan hệ và ERD theo service

- `Relation.kind="fk"`: từ `REFERENCES`/`FOREIGN KEY` (cùng service), có `onDelete`, `onUpdate`.
- `Relation.kind="logical"`, `cross_service:true` khi cột tham chiếu bảng của service khác. Nguồn theo thứ tự ưu tiên (mỗi quan hệ mang `source` và `confidence`):
  1. **Khai báo tay** trong tệp `erd-links.yaml` trong repo (E7, vị trí xem Q2): `{from: {service, table, column}, to: {service, table, column}, note}`; `confidence: 1.0`, `source: "declared"`.
  2. **Chú thích trong migration** khớp biểu thức `logical FK\s*(->|to|into|back to)\s*([a-z_-]+)(\.([a-z_]+))?(\.([a-z_]+))?` trên cùng dòng với khai báo cột; đích có thể chỉ là tên service (`project-service`) hoặc `service.bảng(.cột)`; nếu chỉ có service thì cột đích mặc định `id` và bảng đích suy từ tên cột (`worktree_id` → `worktrees`) nếu tìm thấy duy nhất trong ERD service đích, ngược lại để trống bảng. `confidence: 0.8`, `source: "comment"`.
  3. **Quy ước tên** `*_id` khớp tên bảng số ít của service khác một cách duy nhất. `confidence: 0.4`, `source: "naming"`, **tắt mặc định** (chỉ bật bằng tham số `includeInferred`) vì nhiễu.
  4. `tenant_id` xuất hiện ở hầu hết bảng (kiểm tra thô: 89 file `.up.sql` Postgres nhắc `tenant_id`); **không vẽ** thành cạnh; thay bằng thuộc tính `Table.tenantScoped:true`.
- `cardinality` suy từ `UNIQUE`/`PRIMARY KEY` trên cột nguồn (`one-to-one` hay `many-to-one`); mặc định `many-to-one`.
- ERD theo service: `GetErd(service)` trả bảng của service đó và các quan hệ `logical` đi ra/đi vào dưới dạng `externalRefs` (nét đứt ở UI); không trả cả 17 service trong một lần (ngân sách: ≤ 300 bảng, ≤ 2 000 cột một lần; kết quả vượt thì `truncated`).

### 2.6 Liên kết bảng → code (`accessedBy`)

1. Với service đã chọn, liệt kê `internal/adapter/postgres/*.go` và `internal/adapter/mysql/*.go` (số file không-test: 1 đến 18 mỗi dialect mỗi service; `infra-fleet-service` 17/18; `api-gateway`, `git-gateway-service` không có) qua CR-CV-030.
2. Dùng thư viện chuẩn `go/parser` (không dependency mới) với `ParseFile`, duyệt `*ast.BasicLit` kiểu chuỗi (raw và interpreted) và `fmt.Sprintf` với chuỗi định dạng; mỗi literal khớp từ khoá SQL được quét bằng mẫu `\b(FROM|JOIN|INTO|UPDATE|DELETE\s+FROM|TRUNCATE(\s+TABLE)?)\s+([a-z_][a-z0-9_]*\.)?([a-z_][a-z0-9_]*)` (không phân biệt hoa/thường), rồi **đối chiếu với tập bảng đã biết của ERD** (loại trừ từ khoá và bí danh). Thao tác: `FROM|JOIN`→`read`; `INSERT INTO|UPDATE|DELETE FROM|TRUNCATE`→`write`; `INSERT … SELECT` cho cả hai.
3. Hàm bao ngoài: `*ast.FuncDecl` chứa literal; `SymbolRef.key = "method:<relPath>:<Receiver>.<Name>"` hoặc `function:<relPath>:<Name>` theo quy tắc README mục 3.4 (kind chữ thường). `startLine/endLine` lấy từ `fset`.
4. Kết quả mang `source:"static-scan"`, `confidence: 0.9` khi bảng có tiền tố schema khớp (Postgres) và `0.6` khi chỉ khớp tên trần (MySQL, hoặc Postgres không tiền tố). Truy vấn dựng động (nối chuỗi, tên bảng là biến) sẽ bị sót: ghi `dynamicSqlSuspected` khi literal chứa `%s`/`%v` ngay sau `FROM|INTO|UPDATE`.
5. Kiểm chứng sơ bộ (grep, chưa dùng AST): ở `infra-fleet-service`, 20 bảng tạo bởi migration Postgres; 19 bảng xuất hiện dưới dạng `infra.<bảng>` trong `internal/adapter/postgres/*.go`; bảng còn lại (`provider_registry_entries`) không được tham chiếu và không thấy bị `DROP` (nguyên nhân chưa xác minh; tình huống "bảng không có nơi truy cập" là một phát hiện hợp lệ, không phải lỗi).
6. Các ngữ cảnh khác (như `usecase` đọc bảng thẳng) không được quét; chỉ `adapter/{postgres,mysql}` (nơi duy nhất theo kiến trúc hexagonal; `migration_*_test.go` nằm ngoài vì `_test.go` bị loại).

### 2.7 Proto (mới, thuộc CR này)

Trong `backend-go/proto/orca/codeintel/v1/erd.proto` (khai báo theo mẫu v6 CR-REQ-001: chỉ RPC có message):

```proto
service CodeIntelService { rpc GetErd(GetErdRequest) returns (GetErdResponse); }  // thêm vào service chung của CR-CV-010

message GetErdRequest {
  string repo_binding_id = 1;
  string service = 2;            // tên thư mục service, ví dụ "infra-fleet-service"
  string dialect = 3;            // "postgres" | "mysql"; rỗng = postgres
  string ref = 4;                // rỗng = working tree; hoặc commit 40/64 hex
  bool include_access = 5;       // tính accessedBy (đắt hơn)
  bool include_inferred = 6;     // bật quan hệ "naming"
}
message GetErdResponse { ErdModel model = 1; CodeIntelResultMeta meta = 2; repeated ParseWarning warnings = 3; }

message ErdModel {
  string service = 1; string dialect = 2; string schema = 3; string as_of_migration = 4;
  repeated ErdTable tables = 5; repeated ErdRelation relations = 6; repeated ErdExternalRef external_refs = 7;
}
message ErdTable {
  string name = 1; string schema = 2; repeated ErdColumn columns = 3; repeated string pk = 4;
  repeated ErdIndex indexes = 5; repeated ErdCheck checks = 6; repeated ErdPolicy rls = 7;
  string rls_state = 8;          // "enabled" | "forced" | "none" | "unknown"
  string comment = 9; bool tenant_scoped = 10; bool degraded = 11;
  string first_migration = 12; string last_migration = 13;
  repeated TableAccess accessed_by = 14;
}
message ErdColumn { string name = 1; string type = 2; string canonical_type = 3; bool nullable = 4;
                    string default_expr = 5; bool is_pk = 6; bool is_fk = 7; string comment = 8; bool generated = 9; }
message ErdIndex  { string name = 1; repeated string columns = 2; bool unique = 3; string partial = 4; string method = 5; bool emulates_partial_unique = 6; }
message ErdCheck  { string name = 1; string expr = 2; }
message ErdPolicy { string name = 1; string command = 2; string using_expr = 3; string with_check_expr = 4; }
message ErdRelation {
  string kind = 1;               // "fk" | "logical"
  ErdEndpoint from = 2; ErdEndpoint to = 3;
  string cardinality = 4; bool cross_service = 5;
  string source = 6;             // "ddl" | "declared" | "comment" | "naming"
  double confidence = 7; string on_delete = 8; string note = 9;
}
message ErdEndpoint { string service = 1; string table = 2; repeated string columns = 3; }
message ErdExternalRef { string service = 1; string table = 2; }
message TableAccess { SymbolRef symbol = 1; string op = 2; /* read|write */ double confidence = 3; }
message ParseWarning { string file = 1; int32 line = 2; string code = 3; string message = 4; }
```

`SymbolRef` và `CodeIntelResultMeta` (phần `{sources, headCommit, stale, truncated, totalCount}` của README 3.2) do CR-CV-020 định nghĩa; tên message cuối cùng theo CR đó. Frontend sinh TS từ proto (CR-CV-050). Trường tên `Relation.cross_service` theo 08 §5.

### 2.8 Kiểm tra lệch hai dialect (phát hiện, không phải chức năng chính)

Cho service có cả hai dialect, so tập bảng và tập cột: bảng/cột chỉ có ở một bên được trả dưới dạng `ParseWarning{code:"DIALECT_DRIFT"}`. Ví dụ hợp lệ đã thấy: `mcp-service` chỉ Postgres (16 file), `project-service/0019_worktree_lineage` thêm khoá ngoại bằng `ADD CONSTRAINT` ở MySQL. Số bảng `CREATE TABLE`: 120 (Postgres) so với 104 (MySQL); chênh 16 khớp với 17 bảng `mcp` chỉ ở Postgres (kiểm tra thô, chưa bằng parser).

### 2.9 Cache

Kết quả cache theo `(repo_binding, service, dialect, headCommit, params_hash)` ở `graph_snapshots` (CR-CV-022); `params_hash` bao gồm tập `(path, ContentHash)` của file bẩn. Từng file `.up.sql` parse độc lập không thể gộp (ALTER phụ thuộc thứ tự), nên cache ở mức catalog cuối, không ở mức file. Khi `ContentHash` toàn bộ tập file migration của service không đổi thì dùng lại (cơ chế `BlobIDs` của CR-CV-030).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Không đọc DB thật (E14 tuỳ chọn) | 08 §5, và không có quyền/chuỗi kết nối ở `code-intel-service` |
| Áp `.up.sql` theo số, bỏ `.down.sql` | Trạng thái cuối là kết quả của đường đi lên; đúng với golang-migrate |
| Chịu lỗi từng câu lệnh | Tránh một cú pháp lạ làm hỏng ERD cả service; nhưng luôn hiển thị `degraded` |
| Nhận dạng thành ngữ `DO $$ FOREACH … format(…)` thay vì chạy PL/pgSQL | Chỉ 4 file dùng và cùng một hình dạng; mô phỏng PL/pgSQL quá rộng |
| Không vẽ `tenant_id` thành cạnh | Hầu hết bảng có; sẽ làm sơ đồ vô nghĩa |
| `go/parser` chuẩn để quét tên bảng | Không dependency mới; đúng ngữ cảnh hàm bao ngoài |
| Kết quả có `source`/`confidence` cho từng quan hệ | 08 §5: logic liên service là suy luận hoặc khai báo tay |
| ERD theo một service mỗi lần | Giới hạn ngân sách và đúng nguyên tắc "service sở hữu DB riêng" |

## 4. Tiêu chí chấp nhận

- [ ] Parser đọc **294 file `.up.sql`** hiện có mà không panic; số `ParseWarning` loại `UnsupportedStatement` được liệt kê và đã duyệt (kỳ vọng 0 cho DDL, DML được bỏ qua có chủ đích). Thống kê tính năng thực (bảng 1.2) được tái tạo bằng parser và đối chiếu với `grep` (lệch giải thích được).
- [ ] `infra-fleet-service`, Postgres: `ErdModel` có đủ 20 bảng tạo bởi migration trừ các bảng đã bị `DROP` (nếu có), mỗi bảng có `tenant_scoped`, RLS đúng, `asOfMigration = 0038_…`; cột thêm bằng `ALTER` (vd `approval_status`, `group_id` ở `0030`) có mặt; `terminal_sessions_connection_id_fkey` đã bị gỡ sau `0034`.
- [ ] `mcp-service`: RLS của `tenant_settings`, `processed_events`, `outbox_events` được khai triển từ khối `DO $$` (trạng thái `forced`), kèm hai policy `relay_read`, `relay_mark_published`.
- [ ] MySQL `task-service/0012`: cột sinh `project_key` và `UNIQUE KEY idx_task_sources_unique` được nhận; trigger `trg_task_edges_cascade_to` được ghi nhận.
- [ ] Postgres `task/0014`: `DROP INDEX task.idx_task_sources_unique` rồi `CREATE UNIQUE INDEX` có biểu thức `COALESCE(…)` cho trạng thái cuối đúng.
- [ ] Các chú thích `logical FK` (ở 17 file Postgres) được sinh thành quan hệ `kind=logical` với `source=comment`; mỗi quan hệ trỏ đúng service đích hoặc báo đích chưa xác định.
- [ ] `accessedBy` ở `infra-fleet-service` tìm ra ít nhất bảng `infra.dev_servers` ở `repository.go` với `op` đúng và `SymbolRef.key` hợp lệ; bảng `provider_registry_entries` hiện không có truy cập được báo `accessedBy=[]`.
- [ ] Kết quả luôn có `meta.headCommit`, `stale`, `truncated`; ERD > ngân sách thì `truncated=true`.
- [ ] Không có nội dung migration nào bị ghi vào DB hoặc log; log chỉ có đường dẫn, số dòng.
- [ ] Cả hai dialect cho service có hai thư mục; `DIALECT_DRIFT` đúng cho `mcp-service`.
- [ ] Không tên `helpers`/`utils`/`common`/`misc`; không `max-lines` disable.

## 5. Kiểm thử

- **Unit parser (bảng ca):** tách câu lệnh (comment chứa `;`, literal, dollar-quote lồng); mỗi mệnh đề ở 2.3; tên ràng buộc ngầm; ALTER nhiều mệnh đề với comment xen; `DROP CONSTRAINT` theo tên ngầm; `DO $$` khớp mẫu và không khớp mẫu.
- **Golden (CR-CV-070):** snapshot `ErdModel` JSON cho 3 service đại diện (`infra-fleet-service` Postgres+MySQL, `mcp-service`, `task-service`), cập nhật có chủ đích.
- **Đối chiếu DB thật ở CI (phương án D, tuỳ chọn):** chạy migration trên Postgres/MySQL bằng testcontainers (đã có trong `common/go.mod`), đọc `information_schema` và so với `ErdModel` (bảng, cột, nullable, PK). Không là đường chạy production.
- **Quét code:** bảng ca với `Sprintf`, chuỗi raw nhiều dòng, bí danh bảng, `JOIN`, `INSERT … SELECT`, truy vấn động.
- **Hiệu năng:** dựng ERD `infra-fleet-service` (128 file migration, 17+18 file adapter) với agent giả độ trễ 100 ms; mục tiêu < 10 s lần đầu, < 1 s khi trúng cache (con số mục tiêu, chưa đo).
- Chưa chạy bất kỳ test nào ở thời điểm viết.

## 6. Rủi ro và điểm chưa kiểm chứng

- Tập SQL thực tế có thể lớn lên (function, view, partition xuất hiện ở migration mới); parser tự viết phải có đường mở rộng và cảnh báo rõ. Rủi ro bảo trì (mục 2.1).
- Các số `grep` ở 1.2 gồm cả comment; một số ô (vd `COMMENT ON` MySQL, `CREATE SCHEMA` MySQL 14 file) chưa phân loại là lệnh thật hay comment.
- Quét tên bảng theo literal bỏ sót SQL dựng động và có thể báo nhầm khi chuỗi chứa từ khoá giống SQL (lọc bằng tập bảng đã biết). Độ chính xác chưa đo.
- Heuristic E7 mức tên cột chưa được thử; giữ tắt mặc định.
- Dialect TiDB chưa được phân biệt với MySQL; cú pháp riêng TiDB (`AUTO_RANDOM`, `SHARD_ROW_ID_BITS`) chưa thấy trong migration nhưng chưa quét kỹ.
- `fs.readDir` chỉ liệt kê tối đa theo hạn mức; 128 file của `infra-fleet-service` dưới `REPOFS_MAX_DIR_ENTRIES`.
- Số `CREATE SCHEMA` MySQL (14) có thể là comment; schema MySQL của service lấy từ Postgres hay từ thư mục chưa chốt (Q4).

## 7. Câu hỏi mở

- **Q1.** Chốt phương án parser (mục 2.1): A tự viết (đề xuất) hay B+C thư viện?
- **Q2.** `erd-links.yaml` (E7) đặt ở đâu và ai duyệt: trong repo (`docs/code-intel/erd-links.yaml`, đọc qua CR-CV-030) hay bảng DB như `c4_overrides` (CR-CV-011)? Điều này làm README mục 3.5 thiếu một bảng nếu chọn DB.
- **Q3.** Có hiển thị quan hệ `naming` không (độ nhiễu)? Mặc định tắt.
- **Q4.** Tên schema MySQL: dùng schema Postgres tương ứng hay tên service?
- **Q5.** Có cần đối chiếu `information_schema` (E14) cho môi trường dev để phát hiện lệch migration/DB, ngoài CI?
- **Q6.** Phạm vi `ERD_MIGRATION_GLOB` cho repo không theo bố cục `backend-go/services/*`.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 3.4, 3.5, 3.6, 6)
- `/opt/repos/orca/docs/research/view-code/08-views-and-review-models.md` (§5), `09-external-inputs-required.md` (E1, E7, E14, E15)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/migrations/postgres/0001_init.up.sql`, `.../postgres/0019_agent_sessions.up.sql`, `.../postgres/0034_terminal_sessions_connection_id_allows_dev_server.up.sql`, `.../mysql/0001_init.up.sql`, `.../mysql/0019_agent_sessions.up.sql`
- `/opt/repos/orca/backend-go/services/mcp-service/migrations/postgres/0001_init.up.sql` (khối `DO $$`), `/opt/repos/orca/backend-go/services/auth-service/migrations/postgres/0011_oauth_authorization_server.up.sql`
- `/opt/repos/orca/backend-go/services/task-service/migrations/{postgres,mysql}/0001_init.up.sql`, `0012_task_sources.up.sql`, `0014_task_sources_site.up.sql`
- `/opt/repos/orca/backend-go/services/ai-provider-service/migrations/mysql/0003_account_registration_fields.up.sql`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/{postgres,mysql}/repository.go`
- `/opt/repos/orca/backend-go/go.work`, `backend-go/common/go.mod`, `backend-go/services/infra-fleet-service/go.mod`, `backend-go/services/task-service/deploy/Dockerfile` (`CGO_ENABLED=0`)
- `/opt/repos/orca/deploy/dev/scripts/migrate.sh`
- `/opt/repos/orca/docs/crs/v6/request-service-foundation/CR-REQ-001-scaffold-request-service.md` (mẫu proto/CR)
- CR liên quan: CR-CV-011, 012, 020, 022, 030, 034, 035, 038, 057, 070
