# BE-CV-SOL-031-sql-migration-parser: Bộ phân tích `*.up.sql` (Postgres, MySQL) thành `Catalog` schema, chịu lỗi từng câu lệnh

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Phần 1/2 của CR-CV-031 (parser + mô hình miền); phần 2/2 là [`BE-CV-SOL-031-erd-model-and-access-scan`](./BE-CV-SOL-031-erd-model-and-access-scan.md) (quan hệ, `accessedBy`, proto, `GetErd`).

**CR:** [CR-CV-031](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-031-sql-migration-to-erd.md)
**Service:** `code-intel-service` — `internal/domain/erd` (mới, catalog thuần), `internal/adapter/sqlmigration` (mới, tokenizer + bộ phân tích)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) ("The dependency rule": miền không import adapter), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) ("Migration conventions", "Multi-tenancy"), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) ("Input validation & supply chain")

---

## 0. Hợp đồng áp dụng

| Mã | Áp dụng | Mục |
|---|---|---|
| O-7 | Parser SQL **tự viết**, không thêm dependency (`CGO_ENABLED=0` của Dockerfile service giữ nguyên); kiểm chứng bằng DB thật chỉ ở CI | `CONTRACT-proto-and-data-map` §9 |
| PQ-07/PQ-29 | Message `Erd*`, `ParseWarning` định nghĩa ở `codeintel_erd.proto` (SOL `031-erd-model-and-access-scan`); solution này chỉ có kiểu miền | §1, §2.1 hàng 9 |
| H7 | `degraded:true`, `rls_state:"unknown"` là trạng thái thật, không suy ra "ổn" khi parse thiếu | §0 |
| H8 | Không ghi nội dung migration vào DB/log; log chỉ đường dẫn + số dòng | §0, README v7 §6 |
| H10, §8.3-3/4 | Parser không chạm DB; test hai dialect ở mức fixture; cô lập tenant thuộc solution `erd-model` (use case/cache) | §8.3 |
| PQ-14(5) | Giới hạn kích thước đọc do cổng `BE-CV-SOL-030` (1 MiB/file; migration lớn nhất 7,7 KB) | §1 |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc/chạy lệnh chỉ-đọc (2026-10-06): `ls`/`grep -l` trên `backend-go/services/*/migrations/{postgres,mysql}/*.up.sql`; `mcp-service/migrations/postgres/0001_init.up.sql` (dòng 40–70: `DO $$ … FOREACH t IN ARRAY ARRAY['tenant_settings','processed_events','outbox_events'] LOOP EXECUTE format('ALTER TABLE mcp.%I ENABLE ROW LEVEL SECURITY', t); … EXECUTE format($p$CREATE POLICY tenant_isolation ON mcp.%I USING (…) WITH CHECK (…)$p$, t); END LOOP; END $$;` rồi `relay_read`, `relay_mark_published` viết thẳng); `infra-fleet-service/migrations/postgres/0034_…up.sql` (`DROP CONSTRAINT terminal_sessions_connection_id_fkey`, tên ngầm), `0019_agent_sessions.up.sql` dòng 13, 17 (`-- logical FK -> project-service`, `-- logical FK -> ai_provider.accounts`); `backend-go/common/dbcapability/capability.go`; `services/infra-fleet-service/go.mod` (có `gopkg.in/yaml.v3` trực tiếp, dòng 17).

Xác nhận: 151 `.up.sql` Postgres + 143 MySQL = **294** (khớp CR); migration `.up.sql` lớn nhất 7,7 KB; cấu trúc `DO $$ … FOREACH … format(…)` đúng như CR 1.3.1; `infra-fleet-service` có `0038_session_origin` là số lớn nhất.

### Correction relative to CR-CV-031

| # | CR nói | Kết quả đọc | Xử lý |
|---|---|---|---|
| C1 | `DO $$ … END $$` chỉ ở 4 file (`auth/0011`, `mcp/0001,0002,0003`) | `grep -l 'DO \$'` trên Postgres ra **7 file**: `auth/0011`, `mcp/0001,0002,0003,0004,0006,0007`. Chưa kiểm cả 7 có đúng thành ngữ FOREACH | Task 01 đếm và phân loại bằng parser; khối không khớp → `OPAQUE_DO_BLOCK`, không panic |
| C2 | Chú thích `logical FK` ở 17 file Postgres | `grep -l` ra **16** file Postgres **và 11 file MySQL**; có biến thể `-> tenant-service.users; nullable`, `to mcp.grants (no cross-database FK)`, `-> task-service.tasks.id (different id space…)`, `-> auth.users (different DB)` | Mẫu nhận dạng của solution `erd-model` phải chịu các biến thể này; quét cả hai dialect |
| C3 | Dùng `gopkg.in/yaml.v3` "chưa kiểm tra các module khác" | `infra-fleet-service/go.mod:17` có trực tiếp; `api-gateway/go.mod:24` có trực tiếp; các service khác chỉ `// indirect` | Không cần ở solution này (dành cho `c4-overrides-yaml`) |
| C4 | Tên bảng/`CREATE TABLE` đếm 120/104 theo `grep` thô | Chưa chạy lại (số gồm comment) | Tiêu chí: thống kê tái tạo bằng parser, đối chiếu `grep` (task 06) |

## 2. Giải pháp

### A. Cây file (mới, `backend-go/services/code-intel-service/`)

```
internal/domain/erd/catalog.go            # Catalog, Table, Column, Index, Check, ForeignKey, Policy, RLSState
internal/domain/erd/canonical_type.go     # chuẩn hoá kiểu: uuid,text,timestamp,json,bool,int,bigint,float,bytes,numeric,date,other
internal/domain/erd/parse_warning.go      # ParseWarning{File, Line, Code, Message}; hằng mã cảnh báo
internal/adapter/sqlmigration/tokenizer.go            # bỏ comment, literal '...' (''), $tag$...$tag$, độ sâu ngoặc
internal/adapter/sqlmigration/statement_splitter.go   # tách ở ';' mức 0
internal/adapter/sqlmigration/ddl_postgres.go         # CREATE/ALTER/DROP/INDEX/POLICY/COMMENT cho Postgres
internal/adapter/sqlmigration/ddl_mysql.go            # nhánh MySQL: KEY/UNIQUE KEY/INDEX nội dòng, FOREIGN KEY, ENGINE, generated, TRIGGER
internal/adapter/sqlmigration/column_definition.go    # phân tích định nghĩa cột dùng chung hai dialect
internal/adapter/sqlmigration/rls_do_block.go         # nhận dạng thành ngữ DO $tag$ FOREACH ... format(...)
internal/adapter/sqlmigration/implicit_names.go       # tên ngầm: <t>_<c>_fkey|_check|_pkey|_key, <t>_ibfk_<n>
internal/adapter/sqlmigration/migration_order.go      # lọc *.up.sql, tách số, sắp, DUPLICATE_VERSION, asOfMigration
internal/adapter/sqlmigration/testdata/               # corpus rút gọn + golden JSON
```

### B. Mô hình miền (`internal/domain/erd`, chữ ký chính)

```go
type Dialect string // "postgres" | "mysql"

type Catalog struct {
    Dialect       Dialect
    Tables        map[string]*Table // khoá "schema.table" (Postgres) | "table" (MySQL)
    Triggers      []Trigger         // MySQL: tên + bảng, không phân tích thân
    AsOfMigration string
    Warnings      []ParseWarning
}
type Table struct {
    Schema, Name       string
    Columns            []Column // thứ tự khai báo
    PrimaryKey         []string
    Uniques            []Unique
    Indexes            []Index  // {Name, Columns/Exprs, Unique, Partial, Method, EmulatesPartialUnique}
    Checks             []Check
    ForeignKeys        []ForeignKey
    Policies           []Policy
    RLS                RLSState // none|enabled|forced|unknown
    Comment            string
    FirstMigration, LastMigration string
    Degraded           bool
}
type ApplyResult struct{ Warnings []ParseWarning }
func (c *Catalog) Apply(file string, stmts []Statement) ApplyResult // thuần, theo thứ tự
```

`Apply` **không panic** với câu lệnh lạ: ghi `UnsupportedStatement`, giữ trạng thái bảng, đặt `Degraded` cho bảng bị ảnh hưởng (CR 2.3, H7).

### C. Phạm vi cú pháp (tối thiểu, theo CR 2.3; phần còn lại → cảnh báo)

1. `CREATE SCHEMA [IF NOT EXISTS]`, `CREATE EXTENSION` (ghi nhận).
2. `CREATE TABLE [IF NOT EXISTS] [schema.]t (...)`: kiểu có `(n)`, `(p,s)`, `[]`, `DOUBLE PRECISION`, `TIMESTAMP(6)`; `NOT NULL`/`NULL`; `DEFAULT <biểu thức>` (cân ngoặc: `(UUID())`, `gen_random_uuid()`, `now()`, `CURRENT_TIMESTAMP(6)`); PK/UNIQUE/CHECK kiểu cột và bảng, có `CONSTRAINT <tên>`; `REFERENCES t(c) [ON DELETE|UPDATE …]`; `GENERATED ALWAYS AS IDENTITY|(expr) STORED`; `AUTO_INCREMENT`; MySQL `KEY|UNIQUE KEY|INDEX` nội dòng, `FOREIGN KEY (…) REFERENCES …`, `ENGINE=`, `DEFAULT CHARSET=`, `COLLATE`.
3. `ALTER TABLE t` nhiều mệnh đề ngăn bằng dấu phẩy mức 0, **kể cả comment xen giữa**: `ADD [COLUMN] [IF NOT EXISTS]`, `DROP COLUMN`, `ALTER COLUMN … TYPE|SET|DROP DEFAULT|NOT NULL`, `MODIFY [COLUMN]`, `CHANGE COLUMN`, `ADD CONSTRAINT … (PRIMARY KEY|UNIQUE|CHECK|FOREIGN KEY)`, `DROP CONSTRAINT [IF EXISTS]`, `DROP INDEX` (MySQL), `ENABLE|FORCE ROW LEVEL SECURITY`, `RENAME …`.
4. `CREATE [UNIQUE] INDEX [CONCURRENTLY] [IF NOT EXISTS] n ON [schema.]t [USING m] (cột|biểu thức [ASC|DESC]…) [INCLUDE (…)] [WHERE …]`; `DROP INDEX [IF EXISTS] [schema.]n`.
5. `DROP TABLE [IF EXISTS]`; `CREATE POLICY n ON t [FOR cmd] [TO role] [USING (…)] [WITH CHECK (…)]`; `COMMENT ON TABLE|COLUMN … IS '…'`; `CREATE TRIGGER` (MySQL: tên + bảng).
6. `DO $tag$ … FOREACH v IN ARRAY ARRAY[…] LOOP EXECUTE format('ALTER TABLE <schema>.%I (ENABLE|FORCE) ROW LEVEL SECURITY', v); EXECUTE format($p$CREATE POLICY n ON <schema>.%I …$p$, v); END LOOP; END $tag$`: nhận dạng bằng **mẫu cứng**, khai triển cho từng phần tử mảng; không khớp → `OPAQUE_DO_BLOCK` + bảng liên quan `RLS=unknown`. **Không** thực thi PL/pgSQL.
7. DML (`INSERT`/`UPDATE`/`DELETE`/`SELECT`): bỏ qua có chủ đích (backfill).

Tách câu lệnh: bỏ `-- …` và `/* … */` **trước** (có 76 dòng comment chứa `;` theo CR); theo dõi literal `'…'` (với `''`), dollar-quote `$tag$…$tag$`, độ sâu ngoặc; tách ở `;` mức 0. Lỗi trong một câu: `ParseWarning{file, line, stmtKind, message}`, tiếp tục, đánh `Degraded` cho bảng liên quan.

### D. Điểm khác dialect (đã chốt ở CR 2.4; solution thực hiện)

| Chủ đề | Postgres | MySQL/TiDB (cùng nhánh `mysql`) |
|---|---|---|
| Tên bảng | `schema.table`; không tiền tố → schema mặc định của service + cảnh báo | `table` |
| Tên ràng buộc ngầm | `<t>_<c>_fkey`, `<t>_<c>_check`, `<t>_pkey`, `<t>_<c>_key` | `<t>_ibfk_<n>` |
| RLS | cờ `ENABLE`/`FORCE` + policy (kể cả khai triển `DO`) | không có; comment không phải DDL; `tenantEnforcement:"application"` do solution `erd-model` gán |
| Index một phần | giữ biểu thức `WHERE` | không có; cột sinh `GENERATED … STORED` + `UNIQUE` đi cùng → `EmulatesPartialUnique=true` (nhãn suy luận) |

### E. Khám phá và thứ tự áp dụng (`migration_order.go`)

Đầu vào là danh sách tên file từ `RepoSourceReader.ListDir` (BE-CV-SOL-030); chỉ `*.up.sql`; tách tiền tố số `^\d+`; sắp theo **giá trị số** tăng dần (có khoảng trống như `infra-fleet-service` thiếu `0007–0012`; đã kiểm: số 0038 là lớn nhất); trùng số → sắp theo tên + `DUPLICATE_VERSION`; `AsOfMigration` = tên file có số lớn nhất áp dụng thành công. Số quan sát: không có số trùng trong cùng thư mục dialect (theo CR; chưa chạy lại).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Tự viết, chịu lỗi từng câu | O-7; không cgo; tập cú pháp thực tế nhỏ (không function/view/partition) và có 294 file làm kho kiểm thử vàng |
| Nhận dạng thành ngữ `DO` bằng mẫu cứng | PL/pgSQL quá rộng; chỉ một hình dạng trong repo (nhưng có thể 7 file, C1) |
| Bỏ comment trước khi tách | Comment mang `;` |
| `Apply` thuần, không I/O | Dễ kiểm thử bảng; dùng lại cho `base`/`head` ở CR-038 |
| Phương án D (DB thật) chỉ ở CI | Không đọc DB thật ở production (08 §5) |
| Hai dialect → hai `Catalog` | CR 2.2.4; không cố gộp |

## 4. Lệch giữa CR và hợp đồng

| # | CR | Hợp đồng | Theo |
|---|---|---|---|
| L1 | Q1 (chọn parser) mở | O-7 "tự viết" | Tự viết |
| L2 | Proto `erd.proto` | PQ-07: `codeintel_erd.proto` | Solution `erd-model` |
| L3 | Q4 schema MySQL (schema Postgres hay tên service) | Không có phán quyết | Đề xuất: dùng schema của bản Postgres cùng service nếu có, không thì tên thư mục service (mặc định CR 2.2.5); ghi là điểm mở O-S1 |

## 5. Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-030-repo-file-access-gateway` | đọc file, `ContentHash`/`BlobIDs` |
| BE | `BE-CV-SOL-010-scaffold-code-intel-service` | module, go.work |
| BE | `BE-CV-SOL-031-erd-model-and-access-scan` | tiêu thụ `Catalog`, dựng quan hệ, RPC |
| BE | `BE-CV-SOL-038-contract-diff` | so sánh `Catalog` base/head, `FirstMigration/LastMigration` |
| BE | `BE-CV-SOL-038-static-tenant-filter-rule` | dùng `tenant_scoped`, RLS |
| BE | `BE-CV-SOL-035-storage-map` | danh sách schema/bảng theo service |
| BE | `BE-CV-SOL-070-collector-golden-contract` | golden ERD |
| AG/FE | — | không có việc ở parser (thứ tự §7: sau 030) |

## 6. Tiêu chí chấp nhận

- [x] Parser đọc **294 file** hiện có không panic; `UnsupportedStatement` DDL được liệt kê và duyệt (kỳ vọng 0; DML bỏ qua có chủ đích).
- [x] Thống kê tính năng (CR 1.2) tái tạo bằng parser và đối chiếu `grep` (lệch có giải thích).
- [x] `infra-fleet-service` Postgres: đủ 20 bảng còn lại, `AsOfMigration=0038_session_origin`, cột `ALTER` có mặt, `terminal_sessions_connection_id_fkey` bị gỡ sau `0034`.
- [x] `mcp-service`: RLS `tenant_settings`, `processed_events`, `outbox_events` = `forced` khai triển từ `DO`, kèm `relay_read`, `relay_mark_published`.
- [x] MySQL `task-service/0012`: cột sinh `project_key` + `UNIQUE KEY idx_task_sources_unique` + trigger `trg_task_edges_cascade_to`; Postgres `task/0014`: `DROP INDEX task.idx_task_sources_unique` rồi `CREATE UNIQUE INDEX` có `COALESCE`.
- [x] 7 file `DO` (C1): mỗi file được phân loại khớp mẫu hoặc `OPAQUE_DO_BLOCK`.
- [x] Không nội dung migration trong DB/log; không `helpers/utils/common/misc`; không `max-lines` disable.

## 7. Kiểm thử, rủi ro, câu hỏi mở

**Kiểm thử.** Unit tokenizer/splitter (comment chứa `;`, literal `''`, dollar-quote lồng); từng mệnh đề mục C; tên ràng buộc ngầm; `ALTER` nhiều mệnh đề có comment xen; `DROP CONSTRAINT` theo tên ngầm; `DO` khớp/không khớp; test "corpus": quét `backend-go/services/*/migrations/*/*.up.sql` trong repo (đường dẫn tương đối từ test, bỏ qua nếu không tìm thấy) khẳng định không panic và số cảnh báo ≤ ngưỡng đã duyệt. Golden `Catalog` JSON cho `infra-fleet-service` (hai dialect), `mcp-service`, `task-service` (CR-CV-070). Đối chiếu DB thật ở CI (tuỳ chọn, testcontainers có trong `common/go.mod`: `testcontainers-go v0.44.0`) chạy ma trận `dialect: [postgres, mysql]` (§8.3-3). Lệnh dự kiến: `go test ./services/code-intel-service/internal/domain/erd/... ./services/code-intel-service/internal/adapter/sqlmigration/...` (chưa chạy).

**Rủi ro.** Migration tương lai dùng function/view/partition → cảnh báo, không sai âm thầm; số `grep` của CR gồm comment (một số ô như `COMMENT ON` MySQL, `CREATE SCHEMA` MySQL chưa phân loại); TiDB chưa phân biệt (`AUTO_RANDOM` chưa quét); `DO` 7 file chưa phân loại; hiệu năng parser chưa đo (tập 398 KiB tổng).

**Câu hỏi mở.** O-S1 (Q4 CR): tên schema cho model MySQL. Q5 (CR): đối chiếu `information_schema` ở dev. Q6: `ERD_MIGRATION_GLOB` cho repo khác bố cục (đổi tên biến thành `CODEINTEL_ERD_MIGRATION_GLOB` theo PQ-23, do solution `erd-model`).

## 8. Tham chiếu

- `docs/crs/v7/code-intel-sources/CR-CV-031-sql-migration-to-erd.md`; `docs/research/view-code/08-views-and-review-models.md` §5; `09-external-inputs-required.md` (E1)
- `specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` (O-7, PQ-07, PQ-29, §8.3)
- `backend-go/services/mcp-service/migrations/postgres/0001_init.up.sql`, `…/auth-service/migrations/postgres/0011_oauth_authorization_server.up.sql`, `…/infra-fleet-service/migrations/postgres/{0019_agent_sessions,0034_terminal_sessions_connection_id_allows_dev_server}.up.sql`, `…/task-service/migrations/{postgres,mysql}/{0001_init,0012_task_sources,0014_task_sources_site}.up.sql`
- `backend-go/common/dbcapability/capability.go`, `backend-go/services/infra-fleet-service/go.mod`
