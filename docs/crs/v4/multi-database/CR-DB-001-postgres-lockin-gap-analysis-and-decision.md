# CR-DB-001 — Gap Analysis & Quyết định kiến trúc: `backend-go` chỉ hỗ trợ Postgres (F26 Multi-Database)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DB-001 |
| **Tên** | Xác nhận kiến trúc: Postgres-only ở `backend-go` là chủ đích hay thiếu sót so với F26 |
| **Loại** | Gap Analysis + Architectural Decision |
| **Priority** | 🔴 P1 |
| **Effort** | Small (xác nhận + cập nhật tài liệu) — **không** bao gồm effort triển khai multi-dialect (xem CR-DB-002/003, nay đã kích hoạt) |
| **Phiên bản** | v1.1 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | ✅ Quyết định: **Option B — multi-dialect** (xác nhận bởi Product Owner ngày 2026-09-09) — CR-DB-002/CR-DB-003 kích hoạt |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu rà soát F26 ở `backend-go` (gap #1, `docs/roadmap/feature-completion-matrix.md` §4) |
| **Tác động HLD** | ADR-002, ADR-021, `specs/backend-go/tdd/architecture/05-data-architecture.md` |
| **Tác động Features** | F26 (Multi-Database) |
| **Phụ thuộc** | Không — CR nền tảng, mọi CR con (CR-DB-002, CR-DB-003) phụ thuộc quyết định ở đây |

---

## Bối cảnh & Vấn đề

`docs/roadmap/feature-completion-matrix.md:64` gắn cờ F26 (Multi-Database, P1, ✅ đã phát hành theo `docs/features/F26-multi-database.md`) là:

> "BG chỉ hỗ trợ Postgres — chưa có SQLite/MySQL/TiDB dialect abstraction như spec mô tả (regression so với `docs/features/F26`)"

và §4 gap #1 đề xuất: "Xác nhận đây có phải là quyết định kiến trúc có chủ đích (chuẩn hoá về Postgres cho SaaS) hay là thiếu sót cần bổ sung." CR này thực hiện đúng việc xác nhận đó bằng cách đối chiếu 3 nguồn: (1) spec gốc F26, (2) tài liệu kiến trúc thật của `backend-go`, (3) code thật.

### 1. F26 gốc mô tả use case gì

`docs/features/F26-multi-database.md` (ID F26, ✅ Phát hành, CRs `v1/sql-server` CR-001~006) mô tả:

- 4 dialect: `sqlite` (mặc định, dev, **single-user**), `mysql`, `postgresql`, `tidb` — tất cả qua 1 `ORCA_DB_URL`.
- Use case nêu rõ ở §"Vấn đề cần giải quyết" (dòng 25-28): *"Nhiều users đồng thời"*, *"Production deployment trên cloud"*, và **"Enterprise: dùng PostgreSQL/MySQL cluster sẵn có"**.
- Yêu cầu kỹ thuật trỏ tới file TypeScript: `src/main/db/{types,provider,dsn-parser}.ts`, `src/main/db/{sqlite,mysql,postgresql}/*-adapter.ts` — đây là **Electron main process (`backend/` legacy)**, không phải `backend-go`.
- `docs/crs/v1/sql-server/CR-000-gap-analysis.md` (đã ✅ Implemented 2026-07-24) xác nhận: toàn bộ 6 CR con (CR-001~006) được implement **trong `src/main/db/**`, `src/main/repositories/**`** — cùng codebase TS đó. §7 của CR-000 ghi rõ ràng buộc: *"SQLite + JSON file mode PHẢI được giữ làm default cho Desktop Electron app. Multi-DB chỉ cần thiết cho Server mode."*

→ F26 gốc được viết và triển khai **cho `backend/` (Electron TS monolith cũ)**, không phải cho `backend-go`. `backend-go` chưa tồn tại khi F26/CR-000~006 được viết (2026-07-23/24).

### 2. `backend-go` là kiến trúc khác, có ADR/TDD riêng, và tài liệu đó nói gì

- `docs/adrs/v2/ADR-021-unified-postgres-microservices-platform.md` (Amends ADR-002) là quyết định **hợp nhất data-plane về 1 engine Postgres** cho server-mode — nhưng phạm vi ADR-021 vẫn là `backend/` legacy TS (`Code Ref: backend/src/main/db/migrations/0019-0022*.ts`). ADR-021 §1 quyết định *"triển khai thật trên Postgres bây giờ, nhưng MỌI migration mới bắt buộc... tránh tính năng riêng của Postgres (JSONB operators, arrays, RLS) trừ khi bọc sau capability check"* — để giữ khả năng chuyển TiDB.
- `specs/backend-go/tdd/architecture/05-data-architecture.md:1` — tiêu đề chính là **"Data Architecture — PostgreSQL"**. Tài liệu này (không phải ADR-021, mà là TDD riêng của `backend-go`) quyết định:
  - Database-per-service **vật lý** (dòng 3-19) — mỗi trong 13 service sở hữu 1 Postgres database riêng.
  - Multi-tenancy dùng **Postgres Row-Level Security** làm lớp phòng vệ thứ 2 (dòng 46-53): `ALTER TABLE ... ENABLE ROW LEVEL SECURITY`, policy theo `current_setting('app.tenant_id')` set qua `SET LOCAL`.
- `specs/backend-go/tdd/architecture/04-tech-stack.md:31` — chọn driver: *"`pgx` (v5)... native support for the Postgres wire protocol (**no `database/sql` abstraction tax**)"* — chọn **native driver**, không phải `database/sql` (generic, hỗ trợ đổi driver).
- Cùng file, dòng 33, khi chọn `golang-migrate` lại nói: *"dialect-agnostic enough to keep a **TiDB escape hatch** open the way ADR-002/ADR-021 did for the TS system"*.

**Mâu thuẫn nội tại đáng chú ý:** dòng 31 (chọn driver `pgx` native, từ chối `database/sql`) và dòng 46-53 của `05-data-architecture.md` (dùng RLS — tính năng chỉ Postgres có) **tự mâu thuẫn** với tuyên bố "giữ TiDB escape hatch" ở dòng 33 và với nguyên tắc "tránh tính năng riêng Postgres trừ khi capability-gated" mà chính ADR-021 (tài liệu `backend-go` tự trích dẫn) đặt ra. Nói cách khác: **`backend-go`'s TDD tự nhận mình giữ khả năng multi-dialect, nhưng lựa chọn driver + RLS + JSONB thật sự đã khoá chặt vào Postgres** — đây là gap thật giữa tài liệu kiến trúc nội bộ của chính `backend-go` và code, không chỉ giữa F26 và `backend-go`.

### 3. Bằng chứng code thật — khảo sát toàn bộ `backend-go/`

```
grep -rli "mysql\|sqlite\|tidb" --include="*.go" backend-go/   →  0 matches
```

- **Driver**: `github.com/jackc/pgx/v5 v5.10.0` khai trong **17/17 service `go.mod`** (vd. `backend-go/services/auth-service/go.mod:8`) — driver Postgres-native, không tương thích wire protocol MySQL/TiDB (TiDB dùng MySQL protocol, không phải Postgres — tự ADR-021 dòng 40 cũng ghi rõ khác biệt này).
- **Kết nối**: 15 file `cmd/server/main.go` (1 mỗi service data-owning) tự gọi `pgxpool.New(ctx, dsn)` trực tiếp (vd. `backend-go/services/tenant-service/cmd/server/main.go:74`) — **không có factory/abstraction dùng chung** giữa các service; mỗi service tự khởi tạo pool Postgres của riêng nó.
- **SQL đặc trưng Postgres, không capability-gated**:
  - `JSONB`: `backend-go/services/project-service/migrations/0013_worktree_metadata.up.sql:10` — `ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}'::jsonb`.
  - `gen_random_uuid()` (Postgres built-in ≥ 13, không có ở MySQL/TiDB): 25 file migration, vd. `backend-go/services/project-service/migrations/0001_init.up.sql:12`, `0016_sparse_presets.up.sql:9`, `0014_source_projects.up.sql:10`, `0004_worktrees.up.sql:6`.
  - `BIGSERIAL`: `backend-go/services/orchestration-service/migrations/0001_init.up.sql:102`.
  - Placeholder style `$1, $2, ...` (pgx-native, khác `?` của MySQL): 45 file repository, vd. `backend-go/services/tenant-service/internal/adapter/postgres/company_repository.go:31,41,95,123`.
  - Tổng **142 file migration `.sql`** trong `backend-go/`, không file nào theo cấu trúc dialect-tách-biệt (không có `*.mysql.sql`/`*.sqlite.sql` song song).
- **Hạ tầng**: `backend-go/docker-compose.yml:13-23` chỉ khai 1 service DB — `postgres:16-alpine` — cấu hình comment ở đầu file (dòng 1-4) ghi rõ: *"one shared Postgres instance (17 logical databases, one per service...)"*. `backend-go/deploy/postgres-init-databases.sh:8` liệt kê 15 database Postgres tạo tự động (biến `DATABASES="auth tenant project infra scmintegration issuetracking aiprovider workflow task orchestration automation annotation notification usage credential"`), không có nhánh MySQL/SQLite. (Lưu ý phụ: `docker-compose.yml:1-2` ghi "17 logical databases" — không khớp 15 database thật trong script; discrepancy nhỏ giữa comment và script, không phải trọng tâm CR này.)
- **`sqlc`** (query layer được `04-tech-stack.md:32` chọn làm mặc định) **chưa thực sự dùng** — không có file `sqlc.yaml`/`sqlc.yml` nào trong `backend-go/`; 11 file `.go` có chuỗi "sqlc" chỉ là comment tham chiếu, toàn bộ query là raw SQL string viết tay qua `pgx`. Không phải gap của CR này nhưng đáng ghi nhận: khoảng cách tài liệu-vs-code ở `backend-go` không chỉ có ở multi-dialect.

### 4. Abstraction đã tồn tại sẵn — tin tốt cho khả năng bổ sung sau này

Khác với lo ngại ban đầu, `backend-go` **không** rải SQL trực tiếp vào usecase layer. Mỗi trong 17 service có `internal/usecase/ports.go` định nghĩa interface repository (Dependency Inversion, theo `specs/backend-go/architecture/03-clean-architecture-guidelines.md`), implement bởi `internal/adapter/postgres/*.go`. Ví dụ `CompanyRepository` (`backend-go/services/tenant-service/internal/usecase/ports.go:19-34`) — usecase layer chỉ phụ thuộc interface, không phụ thuộc `pgx` trực tiếp.

Impact analysis xác nhận blast radius **theo từng repository là thấp** (xem §Impact analysis) — nghĩa là về mặt kiến trúc, thêm 1 `internal/adapter/mysql/` hoặc `internal/adapter/sqlite/` implement cùng interface **khả thi kỹ thuật**, không cần viết lại usecase layer. Cái đắt là **bề rộng** (44 file repository × 17 service × 142 migration cần viết lại dialect-safe hoặc song song), không phải độ sâu phụ thuộc.

### 5. Có bằng chứng độc lập cho hướng "Postgres tập trung là chủ đích", không phải sơ suất

`specs/backend-go/bugs/task-v1/BUG-TASKV1-008-dual-backend-routing-and-postgres-compliance.md` (điều tra độc lập, không liên quan trực tiếp CR này) xác nhận một **yêu cầu nghiệp vụ đã có từ trước**: *"mọi thông tin phải lưu Postgres tập trung, không SQLite"* — và coi việc `backend-go` 100% Postgres là **compliant**, không phải bug (mục (a)/(b) của file đó). Đồng thời, `deploy/prod/docker-compose.yml:39-42` (backend legacy) cho thấy: dù Node backend **có sẵn** hỗ trợ multi-dialect (comment sẵn `ORCA_DB_URL=mysql://...`, `postgresql://...`, `tidb://...`), production **không bật** — mặc định vẫn chạy SQLite (dòng 46-48) và chưa dùng multi-dialect thật trong bất kỳ deployment nào được audit thấy.

→ Xu hướng nghiệp vụ/kiến trúc quan sát được là **hội tụ về Postgres tập trung cho SaaS**, không phải mở rộng dialect.

---

## Quyết định chính thức (2026-09-09)

**Option B được chọn: `backend-go` sẽ hỗ trợ multi-dialect thật** (Postgres + tối thiểu 1 dialect thứ 2 — MySQL/TiDB, theo đúng use case "Enterprise: dùng PostgreSQL/MySQL cluster sẵn có" của F26 gốc), áp dụng cho chính `backend-go` chứ không chỉ `backend/` legacy. Quyết định này ghi đè khuyến nghị ban đầu của audit (nghiêng Option A) — Product Owner xác nhận trực tiếp có nhu cầu thật, không chỉ chuẩn hoá Postgres cho SaaS.

Hệ quả:

- [CR-DB-002](./CR-DB-002-dialect-capability-layer-foundation.md) (capability layer + gỡ lock-in) và [CR-DB-003](./CR-DB-003-mysql-tidb-adapter-backend-go.md) (adapter MySQL/TiDB) chuyển từ **Backlog** sang **kích hoạt** — xem trạng thái cập nhật trong từng file.
- `specs/backend-go/tdd/architecture/04-tech-stack.md:33`'s tuyên bố "keep a TiDB escape hatch open" **không còn mâu thuẫn** với hướng đi thật — nay là mục tiêu cần hiện thực hoá, không phải câu nói sai cần sửa/bỏ.
- Trade-off đã ghi nhận ở Option B (mục "Giải pháp đề xuất" bên dưới, giữ nguyên để tham khảo) vẫn áp dụng nguyên vẹn: mất RLS defense-in-depth trên dialect không phải Postgres, phải viết lại 142 migration dialect-safe, effort XL — CR-DB-002 phải làm compensating control cho tenant isolation **trước khi** CR-DB-003 thêm adapter thật.

## Giải pháp đề xuất (giữ nguyên để tham khảo — đã quyết định Option B ở trên)

### Quyết định kiến trúc cần Product Owner / Architecture Owner xác nhận (đã quyết định — xem trên)

**Option A — Xác nhận Postgres-only là chủ đích cho `backend-go` (SaaS microservices platform)** — *(không được chọn)*

- F26's phạm vi multi-dialect (`sqlite`/`mysql`/`postgresql`/`tidb`) tiếp tục áp dụng **chỉ cho `backend/` legacy** (nơi nó đã ✅ implement, CR-000~006) — không mở rộng sang `backend-go`.
- Cập nhật `docs/features/F26-multi-database.md` để tách rõ 2 phạm vi: "Desktop/legacy server mode (`backend/`)" giữ nguyên mô tả hiện tại; thêm mục "SaaS platform (`backend-go`)" ghi rõ: Postgres-only theo chủ đích (RLS multi-tenancy defense-in-depth, database-per-service), tham chiếu `specs/backend-go/tdd/architecture/05-data-architecture.md`.
- Cập nhật `docs/roadmap/feature-completion-matrix.md:64,117` — đổi ký hiệu F26/Backend-go từ 🟡 (kèm chú thích "regression") sang cách diễn đạt trung lập, vd. "Postgres-only theo chủ đích kiến trúc SaaS (ADR-021, `05-data-architecture.md`) — không áp dụng scope multi-dialect gốc của F26 (viết cho `backend/` legacy)".
- **Không cần đóng gap "TiDB escape hatch"** một cách triệt để, nhưng nên sửa `specs/backend-go/tdd/architecture/04-tech-stack.md:33` (bỏ hoặc làm rõ lại tuyên bố "keep a TiDB escape hatch open" — hiện đang sai so với driver/RLS/JSONB đã chọn) để tài liệu nội bộ `backend-go` không tự mâu thuẫn cho người đọc sau này.

**Option B — Xác nhận multi-dialect thật sự cần cho `backend-go`** *(✅ ĐÃ CHỌN — xem "Quyết định chính thức" ở trên)* (khách hàng enterprise on-prem yêu cầu MySQL/TiDB cluster sẵn có, đúng use case gốc F26 dòng 28 "Enterprise: dùng PostgreSQL/MySQL cluster sẵn có" áp dụng cho chính `backend-go`, không chỉ `backend/`)

- Cần đầu tư effort Lớn (XL) — xem CR-DB-002 (nền tảng: gỡ bỏ Postgres-only feature lock-in + capability layer) và CR-DB-003 (adapter MySQL/TiDB cụ thể) — **✅ đã kích hoạt** theo quyết định ở trên (không còn ở Backlog).
- Đánh đổi PO đã chấp nhận: mất RLS defense-in-depth (MySQL/TiDB không có RLS tương đương — tự ADR-021 dòng 71 cũng ghi nhận), phải viết lại 142 migration dialect-safe, và không có ORM/sqlc filter sẵn giúp giảm effort (vì `sqlc` chưa thực sự triển khai, xem mục 3). CR-DB-002 phải xây compensating control cho tenant isolation **trước khi** dialect thứ 2 được bật ở bất kỳ service nào.

### Khuyến nghị ban đầu của audit (đã bị ghi đè bởi quyết định PO — giữ lại để tham khảo lịch sử)

Dựa trên bằng chứng ở mục 2 và 5, audit ban đầu nghiêng về **Option A** — có ADR (ADR-021) + TDD riêng (`05-data-architecture.md`) đã quyết định Postgres tập trung *trước khi* audit này chạy, và có bằng chứng độc lập (BUG-TASKV1-008) về yêu cầu nghiệp vụ "Postgres tập trung, không SQLite". **Product Owner đã xem xét và chọn Option B** (2026-09-09) — nhu cầu multi-dialect cho `backend-go` được xác nhận là có thật, không chỉ giả thuyết. Từ thời điểm này, mọi tài liệu/CR con nên coi Option B là quyết định chính thức, không phải "chờ xác nhận".

## Changes Required

| Hạng mục | Thay đổi | Trạng thái |
|---|---|---|
| `docs/features/F26-multi-database.md` | Tách rõ phạm vi "Desktop/legacy (`backend/`)" vs "SaaS platform (`backend-go`)"; ghi rõ `backend-go` nay hỗ trợ multi-dialect (Postgres mặc định + MySQL/TiDB qua CR-DB-002/003), không còn Postgres-only | Cần làm — theo Option B |
| `docs/roadmap/feature-completion-matrix.md:64,117` | Cập nhật mô tả gap F26/Backend-go từ "regression, chưa xác nhận" sang "Option B đã quyết định (CR-DB-001), triển khai qua CR-DB-002/003" | Cần làm |
| `specs/backend-go/tdd/architecture/04-tech-stack.md:33` | Tuyên bố "keep a TiDB escape hatch open" nay là **mục tiêu cần hiện thực hoá thật** qua CR-DB-002/003, không phải câu nói sai cần xoá | Cần làm rõ lại theo hướng ngược (biến cam kết thành thật, không phải gỡ cam kết) |
| CR-DB-002 (nền tảng dialect layer), CR-DB-003 (MySQL/TiDB adapter) | Kích hoạt từ Backlog → triển khai | ✅ Đã kích hoạt |

## Không thuộc phạm vi CR này

- Triển khai code multi-dialect thật — nằm ở CR-DB-002 (capability layer, compensating tenant-isolation control) và CR-DB-003 (adapter MySQL/TiDB cụ thể, pilot 1 service).
- Sửa gap "`sqlc` được tài liệu hoá nhưng chưa triển khai" (mục 3) — không liên quan trực tiếp F26, nên tách CR riêng nếu cần.
- Cutover Node/SQLite → backend-go/Postgres ở production cho Task-domain (đã có CR riêng: `docs/crs/v3/flow-task/CR-FLOW-TASK-004-backend-go-cutover-and-node-sqlite-retirement.md`, theo dõi ở `specs/backend-go/bugs/task-v1/`).

## Tiêu chí chấp nhận

- [x] Product Owner / Architecture Owner xác nhận bằng văn bản: **Option B** (2026-09-09).
- [ ] `docs/features/F26-multi-database.md` và `docs/roadmap/feature-completion-matrix.md` được cập nhật đúng bảng ở "Changes Required" (phản ánh Option B, không phải Option A).
- [ ] CR-DB-002 hoàn thành: capability layer tồn tại, service pilot chạy được ≥ 2 dialect, tenant isolation test pass không cần RLS.
- [ ] CR-DB-003 hoàn thành: adapter MySQL/TiDB thật chạy production-ready cho ≥ 1 service pilot.
- [ ] `specs/backend-go/tdd/architecture/04-tech-stack.md:33`'s "TiDB escape hatch" phản ánh đúng trạng thái thật sau khi CR-DB-002/003 triển khai.

## Impact analysis (gitnexus)

CR này không tự thay đổi code — impact chỉ chạy để **sizing** khả năng triển khai Option B, không phải để merge ngay.

| Symbol | Direction | Risk | Impacted | Ghi chú |
|---|---|---|---|---|
| `CompanyRepository` (interface, `tenant-service/internal/usecase/ports.go`) | upstream | LOW | 2 (1 direct, module `Usecase`) | Mẫu đại diện: usecase layer chỉ phụ thuộc interface, không phụ thuộc `pgx` trực tiếp — tin tốt cho khả năng thêm dialect khác sau này |
| `NewCompanyRepository` (constructor, `adapter/postgres/company_repository.go`) | upstream | LOW | 2 (1 direct: `main.go`'s `run`, 1 process, module `Usecase`) | Mỗi constructor chỉ có 1 điểm gọi (service's `main.go`) — thay bằng factory theo dialect không phá vỡ nhiều điểm gọi |

**Kết luận sizing**: độ sâu phụ thuộc (depth) trên từng symbol thấp — kiến trúc port/adapter hiện tại **không cản trở** kỹ thuật việc thêm dialect. Chi phí thật nằm ở **bề rộng**: 44 file `adapter/postgres/*.go` (không tính test) × 17 `go.mod`/`main.go` × 142 file migration cần port hoặc viết dialect-safe. Impact chi tiết cho từng repository cụ thể **chưa chạy** — CR-DB-002 phải chạy `impact()` cho từng constructor/interface trước khi implement, theo đúng quy tắc bắt buộc của repo, nếu Option B được chọn.

## Liên quan

- [F26-multi-database.md](../../../features/F26-multi-database.md)
- [feature-completion-matrix.md §4 gap #1](../../../roadmap/feature-completion-matrix.md)
- [ADR-002](../../../adrs/v1/ADR-002-multi-database-iconnectionpool.md) — multi-dialect `IConnectionPool` cho `backend/` legacy (✅ Accepted, đã implement)
- [ADR-021](../../../adrs/v2/ADR-021-unified-postgres-microservices-platform.md) — hợp nhất Postgres cho server-mode data plane (`backend/`)
- `specs/backend-go/tdd/architecture/05-data-architecture.md` — TDD Postgres-only thật của `backend-go`
- `specs/backend-go/tdd/architecture/04-tech-stack.md` — lựa chọn driver `pgx` + tuyên bố mâu thuẫn về "TiDB escape hatch"
- `specs/backend-go/bugs/task-v1/BUG-TASKV1-008-dual-backend-routing-and-postgres-compliance.md` — bằng chứng độc lập về yêu cầu "Postgres tập trung"
- [CR-000-gap-analysis.md](../../v1/sql-server/CR-000-gap-analysis.md), [CR-006-db-health-monitoring.md](../../v1/sql-server/CR-006-db-health-monitoring.md) — bối cảnh thiết kế multi-dialect gốc (`backend/`)
- [CR-DB-002](./CR-DB-002-dialect-capability-layer-foundation.md), [CR-DB-003](./CR-DB-003-mysql-tidb-adapter-backend-go.md) — CR con, Backlog, chờ quyết định ở đây
