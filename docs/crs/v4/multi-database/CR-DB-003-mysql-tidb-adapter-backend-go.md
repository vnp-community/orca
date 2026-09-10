# CR-DB-003 — [BACKLOG] MySQL/TiDB adapter cho `backend-go` (on-prem enterprise)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DB-003 |
| **Tên** | Triển khai adapter MySQL/TiDB cụ thể cho `backend-go`, theo use case "Enterprise: dùng cluster sẵn có" của F26 gốc |
| **Loại** | Feature |
| **Priority** | 🟠 P1 — **kích hoạt** (CR-DB-001 đã chốt Option B, 2026-09-09) — vẫn phải **chờ CR-DB-002 xong** trước khi bắt đầu code (phụ thuộc kỹ thuật, không phải phụ thuộc quyết định business nữa) |
| **Effort** | L–XL (phụ thuộc số service cần dialect thứ 2 — không nhất thiết phải là toàn bộ 17 service ngay từ đầu) |
| **Phiên bản** | v1.1 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai (Proposed) — chờ CR-DB-002 hoàn thành (phụ thuộc kỹ thuật: capability layer), không còn chờ xác nhận business |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu rà soát F26 ở `backend-go` |
| **Tác động HLD** | `specs/backend-go/tdd/architecture/04-tech-stack.md`, `05-data-architecture.md` |
| **Tác động Features** | F26 (Multi-Database) |
| **Phụ thuộc** | **Cứng vào [CR-DB-002](./CR-DB-002-dialect-capability-layer-foundation.md)** (capability layer phải tồn tại trước) — [CR-DB-001](./CR-DB-001-postgres-lockin-gap-analysis-and-decision.md)'s Option B đã quyết định |

---

## Bối cảnh & Vấn đề

Ngay cả khi CR-DB-001 chốt Option B và CR-DB-002 xây xong capability layer, vẫn còn thiếu **adapter thật** kết nối MySQL/TiDB — đây là phần "khách hàng thật sự chạy được", tương ứng use case F26 gốc (`docs/features/F26-multi-database.md:28`): *"Enterprise: dùng PostgreSQL/MySQL cluster sẵn có"*.

CR-DB-000/001~006 (`docs/crs/v1/sql-server/`) đã làm việc này **cho `backend/` legacy** — MySQL adapter tại `src/main/db/mysql/mysql-adapter.ts`, dùng driver `mysql2`. `backend-go` cần adapter tương đương ở Go, nhưng **không thể copy trực tiếp** vì driver, ORM/query-layer, và mô hình pool khác hoàn toàn (Node `mysql2` vs Go driver; `pgxpool` sizing theo Vault dynamic secrets — xem `04-tech-stack.md:34` — MySQL driver Go cần cơ chế credential rotation tương đương, chưa có tiền lệ trong `backend-go`).

## Giải pháp đề xuất

### 1. Chọn driver Go + xác nhận tương thích TiDB

- `github.com/go-sql-driver/mysql` (qua `database/sql`, chuẩn) hoặc driver TiDB-aware nếu cần tận dụng tính năng riêng (TiFlash, HTAP) — quyết định cụ thể cần benchmark, ngoài phạm vi liệt kê chi tiết ở đây.
- Xác nhận lại: TiDB dùng MySQL wire protocol (ADR-021 dòng 40 đã ghi) — 1 driver MySQL chuẩn dùng được cho cả MySQL và TiDB, khớp cách F26 gốc làm (`mysql2 (compat)` cho TiDB, dòng 48 `F26-multi-database.md`).

### 2. Credential rotation qua Vault cho dialect mới

- `04-tech-stack.md:34`: pool Postgres hiện tại lấy credential động qua Vault's dynamic secrets engine, không dùng static password. Cần xác nhận Vault's MySQL secrets engine (Vault hỗ trợ sẵn — `database/mysql-aurora`, `database/mysql-rds` plugin) được cấu hình tương đương, không hard-code credential cho path mới.

### 3. Pilot 1 service trước khi nhân rộng

- Bắt đầu từ service pilot đã chọn ở CR-DB-002 (khuyến nghị `usage-service`/`annotation-service` — traffic thấp nhất theo `05-data-architecture.md:17`).
- Chạy `impact()` cho constructor/interface cụ thể của service pilot **trước khi sửa** (bắt buộc theo CLAUDE.md) — chưa chạy ở CR này vì chưa chọn service cụ thể, xem §Impact analysis.

### 4. Migration + CI

- Áp dụng migration dialect-safe đã chuẩn bị ở CR-DB-002 cho service pilot, chạy qua CI matrix Postgres + MySQL/TiDB (`testcontainers-go`, mở rộng theo pattern CR-DB-002 §Changes Required).

## Changes Required

| File / Khu vực | Thay đổi |
|---|---|
| `backend-go/common/dbcapability/mysql/` (mới) | Adapter implement `Capabilities` (CR-DB-002) cho MySQL/TiDB |
| Service pilot's `internal/adapter/mysql/*.go` (mới) | Implement lại `ports.go` interface của service đó cho MySQL |
| Service pilot's `cmd/server/main.go` | Factory chọn dialect theo `ORCA_DB_URL` (đã có ở CR-DB-002, chỉ wire thêm nhánh MySQL) |
| Vault config (`06-secrets-vault-architecture.md` liên quan) | Bật MySQL dynamic secrets engine cho service pilot |
| `backend-go/deploy/*.yml` (dev/staging) | Thêm service MySQL/TiDB container song song Postgres cho môi trường test |

## Không thuộc phạm vi CR này

- Chuyển toàn bộ 17 service sang hỗ trợ MySQL/TiDB đồng thời — CR này chỉ pilot 1 service, nhân rộng là CR tiếp theo sau khi pilot ổn định.
- SQLite cho `backend-go` — xem lý do loại trừ ở [CR-DB-002 §Không thuộc phạm vi](./CR-DB-002-dialect-capability-layer-foundation.md).
- Data migration tool (chuyển dữ liệu thật từ Postgres sang MySQL/TiDB cho khách hàng hiện tại) — ngoài phạm vi, đây là 1 khách hàng mới on-prem chọn dialect từ đầu, không phải chuyển đổi khách hàng đang chạy Postgres.

## Tiêu chí chấp nhận

- [ ] Service pilot chạy được với `ORCA_DB_URL=mysql://...` hoặc `tidb://...`, mọi RPC hiện có của service đó pass test với DB backend là MySQL/TiDB.
- [ ] Vault dynamic secrets engine cấp credential MySQL cho service pilot, không có static password trong config/env.
- [ ] CI chạy migration matrix Postgres + MySQL cho service pilot xanh.
- [ ] Tenant isolation test (từ CR-DB-002) pass trên MySQL/TiDB (không có RLS).
- [ ] `impact()` đã chạy cho mọi symbol sửa trong service pilot, risk được ghi nhận trước khi merge; `detect_changes()` chạy trước khi commit.

## Impact analysis (gitnexus)

**Chưa chạy** — CR này Backlog, chưa chọn service pilot cụ thể nên chưa có symbol để chạy `impact()` có ý nghĩa. Khi kích hoạt CR này, bước đầu tiên bắt buộc là chạy `impact({target: "<Repository interface của service pilot>", direction: "upstream"})` và `impact({target: "New<X>Repository", direction: "upstream"})` cho từng constructor sẽ có thêm implementation MySQL song song — theo đúng mẫu đã minh hoạ ở CR-DB-001 (`CompanyRepository`/`NewCompanyRepository`, risk LOW).

## Liên quan

- [CR-DB-001](./CR-DB-001-postgres-lockin-gap-analysis-and-decision.md), [CR-DB-002](./CR-DB-002-dialect-capability-layer-foundation.md) — phụ thuộc cứng
- `docs/crs/v1/sql-server/CR-001-database-provider-abstraction.md` — MySQL adapter tương đương ở `backend/` legacy (TypeScript, `mysql2`)
- `docs/features/F26-multi-database.md:28,48` — use case "Enterprise: dùng PostgreSQL/MySQL cluster sẵn có", `tidb: mysql2 (compat)`
- `specs/backend-go/tdd/architecture/06-secrets-vault-architecture.md` — Vault dynamic secrets, cần mở rộng cho MySQL
