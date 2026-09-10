# BE-DB-SOL-001: Dialect capability layer (`common/dbcapability`) + áp dụng cho `usage-service`

> **🔲 Designed — chưa implement.** Nền tảng cho BE-DB-SOL-002.

**CR:** [CR-DB-002](../../../../../../docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md)
**Service:** `backend-go/common/dbcapability` (package mới, dùng chung), `usage-service` (pilot)
**TDD tham chiếu:** [`04-tech-stack.md`](../../../../tdd/architecture/04-tech-stack.md) §Data access, [`05-data-architecture.md`](../../../../tdd/architecture/05-data-architecture.md) §Multi-tenancy, §Migration conventions

---

## 1. Trạng thái hiện tại — quan trọng, khác giả định gốc của CR

Audit khi viết solution xác nhận 3 điểm CR-DB-002 gốc **không nhắc chính
xác**:

1. **`ORCA_DB_URL` không tồn tại trong `backend-go`.** Đây là biến của
   `backend/` legacy (`docs/features/F26-multi-database.md:54-57`).
   `backend-go` dùng `DATABASE_DSN` (`common/config/config.go:27,49`),
   qua `secrets.DatabaseCredentialsFromFile(path)` — hàm này **đã
   dialect-agnostic sẵn**: đọc file Vault-Agent render, fallback env
   `DATABASE_DSN`, trả về 1 chuỗi DSN thô, không parse/validate scheme.
   → Factory chọn dialect ở §3 dưới đây **dùng lại `DATABASE_DSN`**, không
   thêm biến env mới.
2. **`usage-service` không dùng `gen_random_uuid()`.** ID sinh ở Go
   (`uuid.NewString()`, `internal/usecase/record_usage_session.go:83`),
   xác nhận bằng `grep` — 1 trong 3 lock-in CR-DB-002 liệt kê (JSONB/RLS/
   `gen_random_uuid()`) **không áp dụng** cho service pilot này. Lock-in
   thật của `usage-service`: RLS (2 policy) + JSONB (1 cột) + placeholder
   `$N` (toàn bộ `repository.go`).
3. **Vault MySQL dynamic secrets không cần code mới ở `backend-go`.**
   `secrets.DatabaseCredentialsFromFile` chỉ đọc 1 chuỗi DSN từ file —
   không quan tâm dialect. Bật MySQL secrets engine là việc **ops-side
   Vault config** (ngoài repo code), không phải task Go — xem
   BE-DB-SOL-002 §1.

## 2. Giải pháp: `backend-go/common/dbcapability/`

```go
// backend-go/common/dbcapability/capability.go
package dbcapability

import (
    "fmt"
    "strings"
)

type Dialect string

const (
    DialectPostgres Dialect = "postgres"
    DialectMySQL    Dialect = "mysql" // TiDB dùng chung nhánh này — MySQL wire protocol (ADR-021 dòng 40)
)

// Capabilities describes what a dialect supports, so callers (migration
// selection, adapter factory) branch on capability, not on ad-hoc string
// comparison scattered across services.
type Capabilities struct {
    Dialect           Dialect
    PlaceholderStyle  string // "$N" (postgres/pgx) | "?" (mysql/database/sql)
    SupportsRLS       bool   // false cho mysql — tenant isolation phải là application-layer-only, xem TASK-BE-DB-003
    SupportsJSONB     bool   // false cho mysql — dùng cột JSON thường (mất operator ->/@>)
    SupportsReturning bool   // Postgres RETURNING; MySQL không có tương đương trực tiếp
}

var postgresCaps = Capabilities{
    Dialect: DialectPostgres, PlaceholderStyle: "$N",
    SupportsRLS: true, SupportsJSONB: true, SupportsReturning: true,
}

var mysqlCaps = Capabilities{
    Dialect: DialectMySQL, PlaceholderStyle: "?",
    SupportsRLS: false, SupportsJSONB: false, SupportsReturning: false,
}

// DetectDialectFromDSN reads the DSN's scheme — no new env var, works with
// the DATABASE_DSN string secrets.DatabaseCredentialsFromFile already
// returns for every service (Vault-Agent-rendered or local-dev fallback).
func DetectDialectFromDSN(dsn string) (Capabilities, error) {
    switch {
    case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
        return postgresCaps, nil
    case strings.HasPrefix(dsn, "mysql://"), strings.HasPrefix(dsn, "tidb://"):
        return mysqlCaps, nil
    default:
        return Capabilities{}, fmt.Errorf("dbcapability: unrecognized DSN scheme in %q", dsn)
    }
}
```

`tidb://` là 1 alias tường minh cho MySQL wire protocol — tránh người vận
hành phải biết TiDB "thực ra là MySQL" khi viết `DATABASE_DSN`.

## 3. Factory chọn adapter theo dialect — mẫu cho `usage-service/cmd/server/main.go`

```go
// Thay đoạn hiện tại:
//   pool, err := pgxpool.New(ctx, dsn)
//   repo := usagepostgres.New(pool)
// bằng:
caps, err := dbcapability.DetectDialectFromDSN(dsn)
if err != nil {
    return fmt.Errorf("detecting database dialect: %w", err)
}

var repo usecase.Repository
switch caps.Dialect {
case dbcapability.DialectPostgres:
    pool, err := pgxpool.New(ctx, dsn)
    if err != nil {
        return fmt.Errorf("connecting to postgres: %w", err)
    }
    defer pool.Close()
    repo = usagepostgres.New(pool)
case dbcapability.DialectMySQL:
    // BE-DB-SOL-002 — internal/adapter/mysql, chưa viết ở solution này
    db, err := sql.Open("mysql", toMySQLDSN(dsn))
    if err != nil {
        return fmt.Errorf("connecting to mysql: %w", err)
    }
    defer db.Close()
    repo = usagemysql.New(db)
}
```

`healthSrv.Register("postgres", ...)` hiện hard-code tên `"postgres"` —
đổi thành `caps.Dialect`-driven tên health-check khi wire thật (chi tiết ở
BE-DB-SOL-002, vì cần `usagemysql` tồn tại trước).

## 4. Compensating control cho tenant isolation khi RLS vắng mặt

Đọc lại `internal/adapter/postgres/repository.go` (đã Read đầy đủ, không
suy đoán): **mọi query đã có `tenant_id = $1` tường minh** —
`SaveSession` (insert), `GetDailyRollup`, `ListSessions`,
`RecomputeDailyRollup` đều filter theo `tenantID` do caller truyền vào (từ
usecase, lấy từ context đã xác thực — không phải input tự do). Tin tốt:
**không cần viết lại query nào** — RLS ở `usage-service` từ trước tới nay
đã đúng nghĩa "backstop", ứng dụng không phụ thuộc nó để đúng.

Việc cần làm khi RLS vắng mặt (dialect MySQL) không phải sửa code, mà là
**chứng minh bằng test** rằng scoping tường minh đủ để cách ly tenant —
xem [TASK-BE-DB-003](../tasks/TASK-BE-DB-003-usage-service-tenant-isolation-test-without-rls.md):
test ghi 2 session ở 2 `tenant_id` khác nhau, gọi `ListSessions`/
`GetDailyRollup` với `tenant_id` A, xác nhận không thấy dữ liệu B — chạy
được trên cả Postgres (RLS bật) lẫn MySQL (RLS không tồn tại) và phải pass
như nhau.

**Phát hiện phụ quan trọng, ảnh hưởng tới mức độ khẩn cấp của compensating
control này** (`grep` xác nhận, không suy đoán): **không có dòng code Go
nào trong toàn bộ `backend-go` gọi `SET LOCAL app.tenant_id`**
(`grep -rln "app.tenant_id" --include="*.go" backend-go/` → 0 kết quả), và
**không migration nào dùng `FORCE ROW LEVEL SECURITY`** (chỉ có
`ENABLE ROW LEVEL SECURITY`, 31 file). Hệ quả: policy RLS hiện tại của
`usage.sessions`/`usage.daily_rollups` (và nhiều khả năng toàn bộ 17
service khác, chưa audit hết — ngoài phạm vi CR này) **chưa từng thực sự
enforce** — role kết nối pool (chủ sở hữu bảng qua migration) mặc định
bypass RLS trừ khi có `FORCE`, và kể cả khi bị áp dụng, thiếu `SET LOCAL`
khiến `current_setting('app.tenant_id', true)` trả `NULL`, mọi hàng sẽ bị
lọc hết (ứng dụng sẽ hỏng hoàn toàn) — điều này **không xảy ra trong thực
tế**, xác nhận role kết nối đang bypass RLS. Nói cách khác: application-
layer scoping tường minh **đã là cơ chế bảo vệ tenant DUY NHẤT có thật**
ngay cả trên Postgres hôm nay — bỏ RLS khi chuyển sang MySQL không làm
giảm mức bảo vệ thực tế nào cả, chỉ là gỡ bỏ 1 lớp tài liệu/ý định chưa
bao giờ vận hành. Đây là gap thật, **ngoài phạm vi CR-DB này** (RLS không
hoạt động là vấn đề tồn tại độc lập ở mọi service, không riêng multi-
dialect) — ghi nhận lại cho CR riêng nếu team muốn đóng nó cho Postgres
nói chung, không sửa ở đây.

## 5. Migration dialect-safe cho `usage-service`

Tách theo dialect thay vì cố viết 1 file chạy được cả 2 (khác nhau ở
`JSONB`/`JSON`, có/không `ENABLE ROW LEVEL SECURITY`, UUID column type
không đổi vì cả 2 dialect đều nhận `CHAR(36)`/`UUID`-as-string cho cột
`TEXT`/`UUID` hiện tại — `usage-service` không dùng `gen_random_uuid()`
nên không có gap này):

```
backend-go/services/usage-service/migrations/
├── postgres/            (nội dung y hệt 4 file hiện tại, chỉ ĐỔI VỊ TRÍ)
│   ├── 0001_init.up.sql / .down.sql
│   └── 0002_outbox.up.sql / .down.sql
└── mysql/               (mới)
    ├── 0001_init.up.sql / .down.sql     — bỏ 2 khối RLS, UUID cột giữ TEXT/CHAR(36)
    └── 0002_outbox.up.sql / .down.sql   — payload JSONB → JSON
```

`main.go`/CI chọn `-path services/usage-service/migrations/<dialect>`
theo `caps.Dialect` khi gọi `migrate` — chi tiết đầy đủ + nội dung SQL cụ
thể ở [TASK-BE-DB-004](../tasks/TASK-BE-DB-004-usage-service-migrations-dialect-safe.md)
(không lặp lại SQL đầy đủ ở đây, solution chỉ định hướng cấu trúc).

`Makefile`'s `migrate-all` target và `usage-service/README.md`'s hướng
dẫn `migrate -path services/usage-service/migrations ...` (dòng cứng,
không có subfolder) phải cập nhật theo cấu trúc mới — nằm trong phạm vi
TASK-BE-DB-004.

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| `impact()` cho `Repository`/`New` của `usage-service` | Đã chạy — LOW, impacted 2 mỗi symbol | Xem solutions/README.md — không phải suy đoán từ mẫu `tenant-service` của CR-DB-001 |
| Đổi migration path (`migrations/` → `migrations/<dialect>/`) | Trung bình | Phá `Makefile`'s `migrate-all` echo-hint + `usage-service/README.md` + `repository_test.go`'s `filepath.Abs("../../../migrations")` (test hiện trỏ path cũ, phải sửa cùng lúc — xem TASK-BE-DB-004) |
| Compensating control chỉ có test, không có code mới | Thấp | Đã xác nhận mọi query đã tenant-scope tường minh — nếu audit sau này tìm thấy 1 query thiếu `tenant_id`, đó là bug thật cần fix riêng, ngoài phạm vi solution này |
| `sql.Open("mysql", ...)` cần driver `go-sql-driver/mysql` chưa có trong `go.mod` | Thấp | Thêm ở BE-DB-SOL-002 (nơi thật sự cần import), không thêm dependency chưa dùng ở solution này |

## Không thuộc phạm vi solution này

- Adapter MySQL/TiDB thật (`internal/adapter/mysql/repository.go`) — xem
  [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md).
- Áp dụng `dbcapability` cho 16 service còn lại — nhân rộng, ngoài phạm vi
  bộ tài liệu này (xem solutions/README.md "Quy mô thật").
- SQLite adapter — theo CR-DB-002's loại trừ (`backend-go` là stack SaaS
  multi-writer, không khớp mô hình SQLite).

## Liên quan

- `backend-go/common/config/config.go:27,49` (`DatabaseDSN`)
- `backend-go/common/secrets/*.go` (`DatabaseCredentialsFromFile` — đã dialect-agnostic)
- `backend-go/services/usage-service/internal/usecase/ports.go` (`Repository` interface)
- `backend-go/services/usage-service/internal/adapter/postgres/repository.go` (`New`, mọi query đã tenant-scope)
- `backend-go/Makefile:88-91` (`migrate-all`, xác nhận không có CI wiring)
- [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (phụ thuộc CỨNG vào solution này)
