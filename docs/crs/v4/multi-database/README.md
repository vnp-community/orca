# Multi-Database (F26) — Change Requests (v4)

> **Bối cảnh:** `docs/roadmap/feature-completion-matrix.md` gắn cờ F26 (Multi-Database, P1, ✅ đã
> phát hành) là gap ưu tiên #1 ở `backend-go`: *"BG chỉ hỗ trợ Postgres — chưa có SQLite/MySQL/TiDB
> dialect abstraction như spec mô tả (regression so với `docs/features/F26`)"*. Bộ CR này rà soát
> gap đó bằng code thật + tài liệu kiến trúc thật của `backend-go`; audit ban đầu nghiêng về
> "đây KHÔNG phải regression mà là quyết định kiến trúc đã có sẵn" (ADR-021 +
> `specs/backend-go/tdd/architecture/05-data-architecture.md`), nhưng để ngỏ chờ Product Owner xác
> nhận chính thức trước khi effort lớn được đầu tư.
>
> **✅ Cập nhật 2026-09-09 — Quyết định chính thức: Option B (multi-dialect).** Product Owner xác
> nhận `backend-go` cần hỗ trợ multi-dialect thật, ghi đè khuyến nghị ban đầu của audit. CR-DB-002
> và CR-DB-003 (trước đó ở Backlog chờ quyết định) nay **đã kích hoạt** — xem trạng thái cập nhật
> trong từng CR và bảng dưới.

## Tổng quan gap đã xác nhận

| Gap | Trạng thái thật | CR |
|-----|-----------------|-----|
| `backend-go` 100% Postgres (`pgx/v5` native driver, 0 tham chiếu mysql/sqlite/tidb trong toàn bộ code) | ✅ Xác nhận đúng — nhưng nhiều khả năng **chủ đích**, không phải thiếu sót | CR-DB-001 |
| F26 gốc (SQLite/MySQL/PostgreSQL/TiDB qua `ORCA_DB_URL`) đã ✅ implement — nhưng ở `backend/` (Electron TS legacy), không phải `backend-go` | ✅ Xác nhận — F26/CR-000~006 (`v1/sql-server`) viết và triển khai trước khi `backend-go` tồn tại | CR-DB-001 |
| `backend-go`'s TDD (`04-tech-stack.md:33`) tuyên bố giữ "TiDB escape hatch" nhưng driver (`pgx` native, dòng 31) + RLS (`05-data-architecture.md:46-53`) đã khoá chặt Postgres | ❌ Mâu thuẫn nội tại trong chính tài liệu `backend-go` — nên sửa dù chọn hướng nào | CR-DB-001 |
| Repository interface abstraction (`ports.go`, 17 service) đã tồn tại sẵn — kỹ thuật khả thi để thêm dialect khác | ✅ Tin tốt, xác nhận qua `impact()` (blast radius LOW per-repository) | CR-DB-001 (ghi nhận), CR-DB-002 (dùng làm nền) |
| Nếu cần multi-dialect thật: phải gỡ JSONB/RLS/`gen_random_uuid()`/`$N`-placeholder lock-in trước (142 migration, 44 repository, 17 service) | Chưa triển khai — Backlog, chờ quyết định | CR-DB-002 |
| Adapter MySQL/TiDB cụ thể cho `backend-go` (nếu cần) | Chưa triển khai — Backlog, chờ CR-DB-002 | CR-DB-003 |

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-DB-001](./CR-DB-001-postgres-lockin-gap-analysis-and-decision.md) | Gap analysis + quyết định kiến trúc: Postgres-only ở `backend-go` là chủ đích hay thiếu sót | 🔴 P1 | Small (xác nhận + cập nhật tài liệu) | ✅ Quyết định: Option B (2026-09-09) |
| [CR-DB-002](./CR-DB-002-dialect-capability-layer-foundation.md) | Foundation: capability layer + gỡ Postgres-only feature lock-in | 🟠 P1 | XL | 🔲 Chưa triển khai — kích hoạt, sẵn sàng bắt đầu |
| [CR-DB-003](./CR-DB-003-mysql-tidb-adapter-backend-go.md) | Adapter MySQL/TiDB cụ thể, pilot 1 service | 🟠 P1 | L–XL | 🔲 Chưa triển khai — chờ CR-DB-002 xong (phụ thuộc kỹ thuật) |

## Thứ tự thực thi

```
CR-DB-001 (gap analysis + quyết định Option A/B) ── ✅ Option B đã chốt, 2026-09-09
        │
        ▼
CR-DB-002 (capability layer + gỡ Postgres lock-in, XL) ── kích hoạt, bắt đầu ngay
        │
        ▼
CR-DB-003 (MySQL/TiDB adapter, pilot 1 service, L–XL)
```

Cả 2 CR con nay là phụ thuộc **kỹ thuật tuần tự** (CR-DB-003 cần capability layer của CR-DB-002 tồn tại trước), không còn là phụ thuộc "chờ quyết định business" — quyết định đó đã có ở CR-DB-001.

## Bằng chứng cốt lõi (chi tiết đầy đủ ở CR-DB-001)

- **F26 gốc** (`docs/features/F26-multi-database.md`) mô tả `src/main/db/**` — Electron TS (`backend/` legacy), đã ✅ implement qua `docs/crs/v1/sql-server/CR-000~006` (2026-07-23/24).
- **`backend-go`** có tài liệu kiến trúc riêng, mới hơn: `specs/backend-go/tdd/architecture/05-data-architecture.md` (tiêu đề "Data Architecture — PostgreSQL") + `docs/adrs/v2/ADR-021-unified-postgres-microservices-platform.md` — quyết định Postgres tập trung, database-per-service, RLS làm lớp phòng vệ thứ 2, **được viết trước khi audit này chạy**.
- Code thật xác nhận triệt để: `grep -rli "mysql|sqlite|tidb" --include="*.go" backend-go/` → **0 matches**; driver `github.com/jackc/pgx/v5` trong toàn bộ 17 `go.mod`; 142 file migration dùng `JSONB`/`gen_random_uuid()`/`BIGSERIAL`/`$N`-placeholder không capability-gated; `backend-go/deploy/postgres-init-databases.sh:8` tạo 15 database Postgres riêng, không nhánh MySQL/SQLite.
- Bằng chứng độc lập (`specs/backend-go/bugs/task-v1/BUG-TASKV1-008-...md`) xác nhận yêu cầu nghiệp vụ đã có: *"mọi thông tin phải lưu Postgres tập trung, không SQLite"* — và Node backend legacy có sẵn multi-dialect nhưng **production không bật** (`deploy/prod/docker-compose.yml:39-42` — comment sẵn nhưng không dùng).
- Repository interface abstraction (`internal/usecase/ports.go`, 17 service) đã tồn tại — `impact()` xác nhận blast radius thấp (`CompanyRepository`/`NewCompanyRepository`: risk LOW, 1-2 impacted) — nếu Option B được chọn, kỹ thuật khả thi nhưng đắt về **bề rộng** (44 repository × 17 service × 142 migration), không phải độ sâu.

## Kết luận (2026-09-09 — đã chốt)

Audit ban đầu nghiêng về Option A (Postgres-only là chủ đích) — có ADR + TDD riêng quyết định trước, có bằng chứng độc lập về yêu cầu nghiệp vụ tập trung Postgres, và F26 gốc thực chất áp dụng cho một codebase khác (`backend/` legacy) đã hoàn thành đúng scope của nó. **Product Owner đã xem xét và quyết định ngược lại: Option B — `backend-go` sẽ hỗ trợ multi-dialect thật.** Việc cần làm tiếp theo: cập nhật `docs/features/F26-multi-database.md` và `docs/roadmap/feature-completion-matrix.md` để phản ánh đúng hướng multi-dialect (không phải Postgres-only theo chủ đích), và triển khai CR-DB-002 → CR-DB-003.

## Impact analysis (gitnexus, chạy trước khi sửa — bắt buộc theo CLAUDE.md)

| Symbol | Direction | Risk | Impacted | CR |
|---|---|---|---|---|
| `CompanyRepository` (`tenant-service/internal/usecase/ports.go`) | upstream | LOW | 2 (1 direct, module `Usecase`) | CR-DB-001 (mẫu sizing) |
| `NewCompanyRepository` (`tenant-service/internal/adapter/postgres/company_repository.go`) | upstream | LOW | 2 (1 direct: `main.go`'s `run`, 1 process) | CR-DB-001 (mẫu sizing) |

Đây là 2 symbol mẫu, chạy để **sizing khả năng kỹ thuật** cho Option B, không phải impact của một thay đổi đã lên kế hoạch cụ thể. CR-DB-002 và CR-DB-003 **chưa chạy** impact cho symbol thật của mình (chưa chọn service pilot) — mỗi CR đã ghi rõ nghĩa vụ chạy `impact()` lại ngay trước khi implement, theo đúng quy tắc bắt buộc của repo. Không CR nào trong bộ này đã thực thi code — cả 3 file là tài liệu đặc tả.

## Việc chưa làm ngoài bộ CR này

- Sửa gap "`sqlc` được `specs/backend-go/tdd/architecture/04-tech-stack.md:32` chọn làm query layer mặc định nhưng chưa có `sqlc.yaml` nào trong `backend-go/`, toàn bộ vẫn raw SQL viết tay" — phát hiện phụ trong lúc audit F26, không liên quan trực tiếp multi-dialect, cần CR riêng nếu team muốn đóng.
- Cutover Node/SQLite → backend-go/Postgres ở production cho Task-domain — đã có CR riêng (`docs/crs/v3/flow-task/CR-FLOW-TASK-004-...md`), theo dõi tiến độ ở `specs/backend-go/bugs/task-v1/`.
