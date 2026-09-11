# BE-DB-SOL-005: Dialect capability layer + MySQL/TiDB adapter cho `annotation-service`

> **✅ Implemented (2026-09-11).** Áp dụng nguyên vẹn pattern đã xác lập ở
> BE-DB-SOL-001/002 (`usage-service`, pilot) — không thiết kế lại.

**CR:** CR-DB-002, CR-DB-003
**Service:** `backend-go/services/annotation-service`
**Depends on:** `backend-go/common/dbcapability` (đã có sẵn từ pilot, dùng
lại nguyên vẹn, không sửa), `backend-go/common/testutil.StartMySQL` (đã có
sẵn, dùng lại)
**Task tương ứng:** [TASK-BE-DB-010](../tasks/TASK-BE-DB-010-annotation-service-mysql-rollout.md)

---

## 1. Audit thật trước khi implement (đã Read đầy đủ, không suy đoán)

`annotation-service` được ghi nhận "chưa audit" trong CR-DB-001/002 gốc —
audit trực tiếp cho kết quả:

- **1 repository**: `internal/usecase/ports.go`'s `Repository` interface
  (7 method: `CreateAnnotation`, `ListAnnotations`, `GetAnnotation`,
  `UpdateAnnotation`, `DeleteAnnotation`, `FindByRequestID`, `MarkSent`),
  implement bởi `internal/adapter/postgres/repository.go` (1 struct, 1
  bảng `annotation.annotations`). `ports.go` còn có 1 port thứ 2,
  `OPAClient` — **không phải DB-backed**, implement bởi
  `internal/adapter/opaclient` (gọi `common/policy.Evaluator`, không chạm
  SQL) — không cần adapter MySQL, đúng như phạm vi task đã lưu ý trước.
- **6 file migration** (3 cặp up/down): `0001_init`, `0002_annotation_request_id`,
  `0003_annotation_side_range_sent` — khớp đúng số liệu
  ROLLOUT-TRACKING.md's audit (6 migration).
- **KHÔNG dùng JSONB** — mọi cột là `TEXT`/`UUID`/`INTEGER`/`SMALLINT`/
  `BOOLEAN`/`TIMESTAMPTZ`, không cột JSON nào. Khác với dự đoán chung của
  CR-DB-001/002 rằng nhiều service "chắc có" JSONB.
- **CÓ RLS**: `0001_init.up.sql` bật `ENABLE ROW LEVEL SECURITY` +
  `CREATE POLICY tenant_isolation` trên `annotation.annotations` — quan
  trọng cho §4 dưới đây.
- **CÓ `gen_random_uuid()`**: `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
  (0001) và `request_id TEXT NOT NULL DEFAULT gen_random_uuid()::text`
  (0002, chỉ dùng cho backfill 1 lần trong ALTER TABLE, không phải giá trị
  ổn định). Nhưng: `internal/usecase/create_annotation.go:71` gọi
  `uuid.NewString()` ở Go **trước khi** truyền vào
  `Repository.CreateAnnotation`, và `internal/adapter/postgres/repository.go`'s
  `CreateAnnotation` luôn INSERT `id` tường minh (không dựa vào DEFAULT).
  Nghĩa là — giống hệt phát hiện của BE-DB-SOL-001 §1 cho `usage-service`
  — cột `DEFAULT gen_random_uuid()` **chưa từng thực sự được ứng dụng
  dùng tới**, có thể bỏ hoàn toàn ở bản MySQL mà không mất chức năng.
- **KHÔNG dùng `RETURNING`** ở `CreateAnnotation`/`DeleteAnnotation`
  (plain `INSERT`/`DELETE`), nhưng **CÓ** ở `UpdateAnnotation`/`MarkSent`
  (`UPDATE ... RETURNING ...`) — cần dịch sang "UPDATE rồi query lại" cho
  MySQL, xem §3.

## 2. `common/dbcapability` — dùng lại nguyên vẹn

Không sửa `backend-go/common/dbcapability/capability.go`. Không phát hiện
bug nào trong package này khi dùng cho service thứ 2 (khác với usage-service
chỉ dùng test 1 lần, annotation-service xác nhận lại
`DetectDialectFromDSN` hoạt động đúng cho cả `mysql://` model DSN của
testcontainers).

## 3. Adapter MySQL — `internal/adapter/mysql/repository.go`

Dịch từng method của `internal/adapter/postgres/repository.go` sang
`database/sql`/MySQL, cùng khuôn BE-DB-SOL-002. 2 điểm khác biệt quan
trọng, không có trong `usage-service`'s adapter:

### 3.1. `UpdateAnnotation`/`MarkSent` không có `RETURNING` — và một cạm
bẫy KHÔNG chỉ là "thiếu RETURNING"

Dịch ngây thơ "bỏ RETURNING, kiểm tra `RowsAffected() == 0` để biết not
found, rồi SELECT lại nếu > 0" **SAI** cho `UpdateAnnotation`: driver
`go-sql-driver/mysql` mặc định báo `RowsAffected()` theo ngữ nghĩa "số
dòng THỰC SỰ ĐỔI GIÁ TRỊ", không phải "số dòng khớp WHERE" — khác hẳn
Postgres/pgx (luôn đếm mọi dòng UPDATE chạm vào, kể cả giá trị y hệt cũ).
Một client gọi lại `UpdateAnnotation` với đúng `content`/`resolved` đã có
sẵn (một retry hợp lệ, không phải lỗi) sẽ nhận `RowsAffected() == 0` dù
dòng đó **có tồn tại và khớp tenant/id** — nếu code coi `0` là "not
found", request retry hợp lệ sẽ trả nhầm 404.

**Giải pháp chọn**: `UpdateAnnotation`/`MarkSent` **không đọc
`RowsAffected()` để quyết định not-found nữa** — luôn chạy `UPDATE` rồi
`SELECT` lại đúng tập `(tenant_id, id)`/`(tenant_id, id IN (...))` để biết
kết quả thật (giống hệt điều `RETURNING` cho Postgres, chỉ tách thành 2
câu lệnh). `GetAnnotation`'s not-found path (SELECT rỗng → `sql.ErrNoRows`)
xử lý đúng "not found" độc lập với UPDATE đã đổi gì. Đã viết test riêng
xác nhận (`TestRepository_UpdateAnnotation_NoopRetryStillSucceeds`) — PASS
thật trên MySQL 8 (xem §5).

`DeleteAnnotation` **không bị** cạm bẫy này: `DELETE`'s `RowsAffected()`
luôn đếm theo "khớp WHERE" ở mọi dialect (không có khái niệm "đổi giá
trị" cho DELETE) — giữ nguyên logic kiểm tra `affected == 0`.

### 3.2. `MarkSent`'s `id = ANY($3)` → `IN (?,?,...)` động

Giống `usage-service`'s `MarkPublished`: MySQL không có tương đương
`= ANY($1)`, phải build `IN (...)` động theo `len(ids)`, và guard
`len(ids) == 0` → return `nil, nil` ngay (MySQL `IN ()` là lỗi cú pháp,
Postgres's `= ANY('{}')` thì không).

## 4. Compensating control cho tenant isolation khi RLS vắng mặt

Đọc lại `internal/adapter/postgres/repository.go`: mọi query đã filter
tường minh theo `tenant_id` (`ListAnnotations`, `GetAnnotation`,
`UpdateAnnotation`, `DeleteAnnotation`, `FindByRequestID`, `MarkSent`) —
giống hệt kết luận BE-DB-SOL-001 §4 cho `usage-service`: RLS ở
`annotation.annotations` **chưa từng thực sự active** (không có
`SET LOCAL app.tenant_id` nào trong toàn bộ `backend-go`, không migration
nào dùng `FORCE ROW LEVEL SECURITY`). Bỏ RLS ở bản MySQL không hạ thấp
mức bảo vệ thực tế — application-layer scoping đã là cơ chế duy nhất có
thật, ở cả 2 dialect.

Test mới `TestRepository_ListAnnotations_DoesNotLeakAcrossTenants` +
`GetAnnotation` cross-tenant check trong cùng test — mirror
[TASK-BE-DB-003](../tasks/TASK-BE-DB-003-usage-service-tenant-isolation-test-without-rls.md)'s
pattern, xem TASK-BE-DB-010 §Kết quả thực tế cho kết quả chạy.

## 5. Migration dialect-safe — điểm khác biệt so với `usage-service`

- **`file_path`/`repo_id` là `TEXT` không giới hạn độ dài** — Postgres cho
  phép index trên `TEXT` không giới hạn; MySQL/InnoDB giới hạn key length
  3072 byte, không index trực tiếp `TEXT` được. `idx_annotations_file_lookup`
  bản MySQL dùng prefix index (`repo_id(255), file_path(255)`) — mất khả
  năng phân biệt 2 giá trị chỉ khác nhau sau ký tự 255 (không đáng kể cho
  path/repo-id thực tế), tính năng chức năng (lookup) không đổi, chỉ độ
  chọn lọc index giảm nhẹ về lý thuyết.
- **Không có index `WHERE worktree_id IS NOT NULL`** (partial index) —
  MySQL không có partial index; `idx_annotations_worktree` bản MySQL index
  đầy đủ `(tenant_id, worktree_id)`, đúng chức năng, chỉ lớn hơn
  Postgres's bản partial.
- **`request_id`/`original_code` không cần `DEFAULT` khi ALTER TABLE ADD
  COLUMN NOT NULL** — khác `usage-service` (không có ALTER TABLE thêm cột
  NOT NULL nào cần bàn tới điểm này): migration MySQL của
  `annotation-service` luôn chạy trên bảng RỖNG (deployment MySQL mới,
  không phải Postgres có sẵn dữ liệu chuyển sang) nên không cần
  `DEFAULT '...'` rồi `DROP DEFAULT` như bản Postgres phải làm để backfill
  dòng cũ — xác nhận chạy thật không lỗi (§Kết quả thực tế, TASK-BE-DB-010).

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| `impact()` cho `run`/`Config`/`Load` (`cmd/server/main.go`, `internal/config/config.go`) và `Repository` (`ports.go`) | Đã chạy — LOW cho cả 4 (impactedCount 1/2/3/2) | Xem TASK-BE-DB-010 |
| `UpdateAnnotation`/`MarkSent`'s RowsAffected-vs-RETURNING dịch sai | Đã tránh — chọn "UPDATE rồi SELECT lại" thay vì đọc RowsAffected để quyết định not-found | Xem §3.1, test riêng đã PASS |
| Prefix-length index (MySQL) giảm độ chọn lọc so với Postgres's full-TEXT index | Thấp | Không ảnh hưởng đúng-sai kết quả query, chỉ hiệu năng lý thuyết — ngoài phạm vi đo hiệu năng của CR này |
| Driver TiDB thật chưa test | Trung bình | Kế thừa nguyên trạng thái đã ghi ở BE-DB-SOL-002 — chưa có service nào trong rollout này chạy thật trên `pingcap/tidb` |

## Không thuộc phạm vi solution này

- 13 service còn lại của rollout 15-service (ROLLOUT-TRACKING.md) — nhân
  rộng, ngoài phạm vi.
- `OPAClient` port — không DB-backed, không cần adapter dialect nào (xem §1).
- Data migration tool Postgres→MySQL cho khách hàng thật — theo loại trừ
  chung của CR-DB-003 (khách hàng chọn dialect từ đầu).

## Liên quan

- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (pattern gốc)
- `backend-go/services/annotation-service/internal/usecase/ports.go`
- `backend-go/services/annotation-service/internal/adapter/postgres/repository.go`
- `backend-go/services/annotation-service/internal/adapter/mysql/repository.go` (mới)
- [TASK-BE-DB-010](../tasks/TASK-BE-DB-010-annotation-service-mysql-rollout.md)
