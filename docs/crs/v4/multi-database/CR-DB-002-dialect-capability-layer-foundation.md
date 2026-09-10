# CR-DB-002 — [BACKLOG] Gỡ Postgres-only feature lock-in + nền tảng dialect capability layer cho `backend-go`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DB-002 |
| **Tên** | Foundation: capability layer + gỡ JSONB/RLS/`gen_random_uuid()` lock-in trước khi thêm dialect thứ 2 |
| **Loại** | Feature (Foundation) |
| **Priority** | 🟠 P1 — **kích hoạt**: CR-DB-001 đã chốt Option B (multi-dialect thật cho `backend-go`), 2026-09-09 |
| **Effort** | XL (nhiều service, nhiều migration — xem §Changes Required) |
| **Phiên bản** | v1.1 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai (Proposed) — sẵn sàng bắt đầu, không còn ở Backlog |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu rà soát F26 ở `backend-go` |
| **Tác động HLD** | `specs/backend-go/tdd/architecture/05-data-architecture.md`, ADR-021 |
| **Tác động Features** | F26 (Multi-Database) |
| **Phụ thuộc** | [CR-DB-001](./CR-DB-001-postgres-lockin-gap-analysis-and-decision.md) — quyết định Option B đã có, CR này có thể bắt đầu |

---

## Bối cảnh & Vấn đề

CR-DB-001 đã chốt quyết định (Option B, 2026-09-09): `backend-go` sẽ hỗ trợ multi-dialect thật. CR-DB-001 xác nhận `backend-go` hiện khoá chặt vào Postgres ở 3 điểm không thể chỉ "thêm 1 adapter" mà giải quyết được — phải xử lý các điểm này **trước** khi CR-DB-003 (adapter MySQL/TiDB) có thể chạy:

1. **Driver native, không qua `database/sql`**: `pgx/v5` (`backend-go/services/*/go.mod`, 17 service) nói chuyện trực tiếp Postgres wire protocol. Không có lớp `database/sql`-compatible ở giữa để inject driver `mysql`/TiDB khác mà không đổi code gọi.
2. **RLS là lớp phòng vệ thứ 2 đang chạy thật**: `specs/backend-go/tdd/architecture/05-data-architecture.md:46-53` — `SET LOCAL app.tenant_id` + policy RLS trên mọi bảng tenant-scoped. MySQL/TiDB không có RLS tương đương — nếu bỏ, tenant isolation chỉ còn dựa 100% vào application-layer scoping (rủi ro tăng, cần đánh giá lại, không thể "cứ bỏ RLS đi" mà không compensating control).
3. **SQL đặc trưng Postgres rải trong 142 migration**: `JSONB` (13 file), `gen_random_uuid()` (25 file), `BIGSERIAL` (`orchestration-service/migrations/0001_init.up.sql:102`), placeholder `$N` (45 file repository) — không có helper dialect-safe nào tương tự `nowTextDefaultSql()`/`autoIncrementPrimaryKeySql()` mà `backend/` legacy đã có (ADR-002/ADR-021 tham chiếu).

## Giải pháp đề xuất

Theo đúng pattern đã dùng cho `backend/` legacy ở CR-001/CR-004 (`docs/crs/v1/sql-server/`), nhưng áp lại cho kiến trúc Go/microservices:

### 1. Capability interface ở tầng Go (song song `ports.go` đã có)

```go
// backend-go/common/dbcapability/capability.go (package mới, dùng chung 17 service)
type Dialect string

const (
    DialectPostgres Dialect = "postgres"
    DialectMySQL    Dialect = "mysql" // hoặc TiDB (MySQL wire protocol)
)

type Capabilities struct {
    Dialect          Dialect
    PlaceholderStyle string // "$N" | "?"
    SupportsRLS      bool
    SupportsJSONB    bool // false cho MySQL — dùng JSON column thường thay thế
    SupportsReturning bool
}
```

Mỗi service's `internal/adapter/<dialect>/` implement cùng interface `ports.go` đã có sẵn (xem CR-DB-001 §4 — impact analysis xác nhận blast radius thấp nhờ interface này) — **không sửa usecase layer**, chỉ thêm implementation mới + factory chọn theo `ORCA_DB_URL`/config, giống pattern `provider.ts` của `backend/` legacy.

### 2. Thay RLS bằng compensating control khi dialect ≠ postgres

- Khi `Dialect != postgres`: bắt buộc thêm 1 lớp application-layer check tương đương (vd. base-repository wrapper luôn inject `WHERE tenant_id = $current`, không phụ thuộc optional) — không được coi "RLS chỉ là defense-in-depth nên bỏ cũng không sao"; phải nâng application-layer scoping từ "chính + backstop" thành "chỉ có chính" một cách tường minh, có test riêng xác nhận không rò tenant khi RLS vắng mặt.

### 3. Viết lại 142 migration dialect-safe hoặc song song

- `gen_random_uuid()` → sinh UUID ở application layer (Go, trước khi insert) khi dialect không hỗ trợ, tránh phụ thuộc hàm SQL riêng của Postgres.
- `JSONB` → cột `JSON`/`TEXT` cho MySQL/TiDB, mất index/operator JSONB — cần rà từng usecase có query theo JSONB operator (`->`, `@>`) xem có bị ảnh hưởng logic không.
- `BIGSERIAL` → `AUTO_INCREMENT` (MySQL) hoặc UUID (khuyến nghị dùng UUID toàn bộ để tránh vấn đề này lặp lại — quyết định riêng, ngoài phạm vi CR này nếu muốn đổi toàn bộ PK strategy).
- Placeholder `$N` → cần lớp SQL builder hoặc tách file migration/query theo dialect (giống `backend/` legacy's cách xử lý `?`/`$1`/`:name` qua `IDatabaseCapabilities.placeholderStyle`).

## Changes Required

| File / Khu vực | Thay đổi | Quy mô |
|---|---|---|
| `backend-go/common/dbcapability/` (mới) | Capability interface + type Dialect | 1 package mới, dùng chung |
| 17 × `services/*/internal/adapter/<dialect>/` (mới, dialect thứ 2) | Implement lại từng `ports.go` interface cho dialect mới | 44 file `adapter/postgres/*.go` hiện có → cần tương ứng ~44 file mới tối thiểu (1:1), nhiều hơn nếu tách theo entity |
| 17 × `services/*/cmd/server/main.go` | Factory chọn adapter theo `ORCA_DB_URL`, thay lời gọi `pgxpool.New()` trực tiếp | 15 file `main.go` gọi `pgxpool.New()` trực tiếp — theo CR-DB-001's impact analysis, mỗi thay đổi risk LOW/1 process nhưng phải sửa toàn bộ 15 |
| 142 file migration `.sql` (`services/*/migrations/`) | Viết lại dialect-safe hoặc thêm biến thể theo dialect (`golang-migrate` hỗ trợ theo tên file) | Rà từng file có `JSONB`/`gen_random_uuid()`/`BIGSERIAL`/`$N` — tối thiểu 25+13+1 = 39 file chắc chắn cần sửa, cộng thêm rà thủ công toàn bộ 142 |
| Tenant isolation (mọi service có bảng tenant-scoped) | Base-repository wrapper application-layer scoping không phụ thuộc RLS | Xem §Giải pháp đề xuất mục 2 — cần thiết kế riêng, không chỉ "thêm code" |
| CI (`testcontainers-go`) | Thêm test matrix chạy song song Postgres + MySQL/TiDB container cho mọi migration thay đổi | Theo đúng convention CI hiện tại của `05-data-architecture.md:66-67` (chạy `up`→`down`→`up` trên Postgres) — nhân đôi cho dialect mới |

## Không thuộc phạm vi CR này

- Chọn adapter MySQL hay TiDB cụ thể, driver Go dùng (`go-sql-driver/mysql` hay khác) — xem CR-DB-003.
- Đổi PK strategy toàn bộ từ UUID-via-`gen_random_uuid()` sang cơ chế khác cho mọi service — chỉ xử lý per-migration khi cần dialect-safe, không refactor toàn bộ schema.
- SQLite adapter cho `backend-go` — theo CR-DB-001 §5, `backend-go` là stack SaaS multi-service (gRPC, Vault, NATS); chạy SQLite cho 1 microservice riêng lẻ trong stack đó không khớp mô hình vận hành (không có multi-writer, không có RLS tương đương) — nếu có nhu cầu single-user/on-prem nhẹ, đó là use case của `backend/` legacy (đã có SQLite từ CR-000~006), không phải `backend-go`.

## Tiêu chí chấp nhận

- [ ] `Capabilities` interface tồn tại, dùng bởi ≥ 1 service pilot (đề xuất: `usage-service` hoặc `annotation-service` — theo `05-data-architecture.md:17` là 2 service traffic thấp nhất, rủi ro thử nghiệm thấp nhất).
- [ ] Service pilot chạy được cả Postgres và dialect thứ 2 qua đổi `ORCA_DB_URL`, không đổi code usecase layer.
- [ ] Test xác nhận tenant isolation vẫn đúng khi RLS không khả dụng (dialect ≠ postgres).
- [ ] CI chạy migration matrix cho cả 2 dialect trên service pilot.
- [ ] `detect_changes()` xác nhận scope thay đổi đúng như dự kiến trước khi merge (bắt buộc theo CLAUDE.md).

## Impact analysis (gitnexus)

**Chưa chạy đầy đủ** — CR này ở Backlog, chưa có service pilot cụ thể để chạy `impact()` có ý nghĩa. Khi Option B được xác nhận và service pilot được chọn, **bắt buộc** chạy `impact({target: "<TênConstructor>", direction: "upstream"})` cho từng constructor `New<X>Repository` của service đó trước khi sửa (theo mẫu đã chạy ở CR-DB-001: `NewCompanyRepository` → risk LOW, impacted 2). Ước tính sơ bộ dựa trên mẫu đó: mỗi constructor có blast radius thấp (1 process, 1 module) — rủi ro kỹ thuật tập trung ở **số lượng** thay đổi (44 repository × 17 service), không ở độ phức tạp từng thay đổi.

## Liên quan

- [CR-DB-001](./CR-DB-001-postgres-lockin-gap-analysis-and-decision.md) — quyết định gốc, **phải duyệt Option B trước**
- [CR-DB-003](./CR-DB-003-mysql-tidb-adapter-backend-go.md) — phụ thuộc CR này
- `docs/crs/v1/sql-server/CR-001-database-provider-abstraction.md`, `CR-004-db-config-dsn-management.md` — pattern tương tự đã làm cho `backend/` legacy (TypeScript)
- `specs/backend-go/tdd/architecture/05-data-architecture.md` — RLS, database-per-service
