# BE-DB-SOL-002: Adapter MySQL/TiDB thật cho `usage-service`

> **🔲 Designed — chưa implement.** Phụ thuộc cứng BE-DB-SOL-001.

**CR:** [CR-DB-003](../../../../../../docs/crs/v4/multi-database/CR-DB-003-mysql-tidb-adapter-backend-go.md)
**Service:** `usage-service`
**TDD tham chiếu:** [`04-tech-stack.md`](../../../../tdd/architecture/04-tech-stack.md) §Data access, [`06-secrets-vault-architecture.md`](../../../../tdd/architecture/06-secrets-vault-architecture.md)

---

## 1. Trạng thái hiện tại — quan trọng, thay đổi thiết kế so với CR gốc

CR-DB-003 gốc đặt "Credential rotation qua Vault cho dialect mới" thành 1
mục việc riêng, ngụ ý cần code mới. Audit xác nhận **không đúng cho
`backend-go`**: `grep`/`find` xác nhận **không có file Vault
Terraform/HCL nào trong repo** (kể cả cho Postgres hiện tại) —
`secrets.DatabaseCredentialsFromFile(path)` chỉ đọc 1 chuỗi DSN đã
render sẵn từ file (hoặc fallback env `DATABASE_DSN`), không phân biệt
dialect ở tầng code. → **Bật Vault MySQL dynamic secrets cho
`usage-service` không cần sửa bất kỳ file `.go` nào** — đây thuần là việc
ops-side (bật `database/mysql-rds` hoặc tương đương plugin trong Vault,
tạo role trỏ tới MySQL instance của `usage-service`, cấu hình Vault Agent
render DSN dạng `mysql://...` ra cùng đường dẫn file hiện tại) — **ngoài
phạm vi code repo, không có task Go tương ứng** trong bộ tài liệu này.

## 2. Chọn driver

`github.com/go-sql-driver/mysql` qua `database/sql` — chuẩn, không cần
driver TiDB riêng vì TiDB dùng MySQL wire protocol (đã xác nhận ở
ADR-021 dòng 40, trích trong CR-DB-001). Test chạy trên container
`mysql:8` (đại diện MySQL) — container `pingcap/tidb` để xác nhận tương
thích TiDB thật sự là việc riêng, ghi nhận ở §Rủi ro (không giả định
"chắc chắn giống hệt MySQL" chỉ vì cùng wire protocol — vẫn cần 1 lần
chạy thật trên TiDB trước khi coi là done cho TiDB, không chỉ MySQL).

## 3. `internal/adapter/mysql/repository.go` — implement lại `usecase.Repository` + `outbox.Store`

Dịch từng query trong `internal/adapter/postgres/repository.go` (đã đọc
đầy đủ, không suy đoán shape) sang `database/sql`/MySQL syntax. Khác biệt
chính: placeholder `$N` → `?`, `ON CONFLICT` → `ON DUPLICATE KEY UPDATE`,
không có `RETURNING`, JSON column thay JSONB.

```go
// backend-go/services/usage-service/internal/adapter/mysql/repository.go
package mysql

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"

    _ "github.com/go-sql-driver/mysql"

    "github.com/stablyai/orca-go/common/outbox"
    "github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

type Repository struct {
    db *sql.DB
}

func New(db *sql.DB) *Repository {
    return &Repository{db: db}
}

func (r *Repository) SaveSession(ctx context.Context, s domain.UsageSession, event domain.OutboxEvent) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("mysql: begin tx: %w", err)
    }
    defer func() { _ = tx.Rollback() }()

    res, err := tx.ExecContext(ctx, `
        INSERT IGNORE INTO usage_sessions (
            id, tenant_id, user_id, provider, worktree_id,
            input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
            cost_usd, started_at, ended_at, request_id
        ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
    `, s.ID, s.TenantID, s.UserID, string(s.Provider), s.WorktreeID,
        s.InputTokens, s.OutputTokens, s.CacheReadTokens, s.CacheWriteTokens,
        s.CostUSD, nullableTime(s.StartedAt), nullableTime(s.EndedAt), s.RequestID)
    if err != nil {
        return fmt.Errorf("mysql: insert session: %w", err)
    }
    // INSERT IGNORE thay ON CONFLICT DO NOTHING: RowsAffected() == 0 nghĩa
    // là UNIQUE(tenant_id, request_id) đã tồn tại — cùng ngữ nghĩa
    // idempotent-replay như bản Postgres.
    n, err := res.RowsAffected()
    if err != nil {
        return fmt.Errorf("mysql: rows affected: %w", err)
    }
    if n == 0 {
        return tx.Commit()
    }

    day := domain.DayKey(s.StartedAt)
    _, err = tx.ExecContext(ctx, `
        INSERT INTO usage_daily_rollups (
            tenant_id, user_id, provider, day,
            total_input_tokens, total_output_tokens, total_cost_usd, session_count
        ) VALUES (?,?,?,?,?,?,?,1)
        ON DUPLICATE KEY UPDATE
            total_input_tokens = total_input_tokens + VALUES(total_input_tokens),
            total_output_tokens = total_output_tokens + VALUES(total_output_tokens),
            total_cost_usd = total_cost_usd + VALUES(total_cost_usd),
            session_count = session_count + 1
    `, s.TenantID, string(s.Provider), day, // giữ đúng thứ tự cột thật khi implement — mẫu rút gọn
        s.InputTokens, s.OutputTokens, s.CostUSD)
    if err != nil {
        return fmt.Errorf("mysql: upsert daily rollup: %w", err)
    }

    _, err = tx.ExecContext(ctx, `
        INSERT INTO usage_outbox_events (id, tenant_id, subject, occurred_at, version, payload)
        VALUES (?, ?, ?, ?, 1, ?)
    `, event.ID, s.TenantID, event.Subject, event.OccurredAt, event.PayloadJSON)
    if err != nil {
        return fmt.Errorf("mysql: insert outbox event: %w", err)
    }
    return tx.Commit()
}

func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) { /* SELECT ... LIMIT ? — cùng khuôn bản Postgres */ return nil, nil }
func (r *Repository) MarkPublished(ctx context.Context, ids []string) error { /* UPDATE ... WHERE id IN (?,?,...) — MySQL không có = ANY($1), phải build placeholder list */ return nil }
func (r *Repository) GetDailyRollup(ctx context.Context, tenantID, userID string, provider domain.Provider, day time.Time) (domain.DailyUsageRollup, error) { /* cùng khuôn, errors.Is(err, sql.ErrNoRows) thay pgx.ErrNoRows */ return domain.DailyUsageRollup{}, nil }
func (r *Repository) ListSessions(ctx context.Context, tenantID, userID, pageToken string, pageSize int32) ([]domain.UsageSession, string, error) { return nil, "", nil }
func (r *Repository) RecomputeDailyRollup(ctx context.Context, tenantID, userID string, provider domain.Provider, day time.Time) error { return nil }

func nullableTime(t time.Time) *time.Time {
    if t.IsZero() {
        return nil
    }
    return &t
}
```

**Quyết định tên bảng/schema MySQL** (chốt ở đây, TASK-BE-DB-004/005 dùng
lại, không tự chọn khác): Postgres dùng `CREATE SCHEMA usage; CREATE TABLE
usage.sessions` bên trong database `usage-service` sở hữu riêng
(database-per-service). MySQL không có `SCHEMA` tách khỏi `DATABASE`
(2 từ đồng nghĩa) — mapping tự nhiên nhất là: **DSN của `usage-service`
trỏ thẳng tới 1 database MySQL tên `usage`** (giữ song song với schema
Postgres cùng tên), và bên trong đó **dùng tên bảng KHÔNG prefix**:
`sessions`, `daily_rollups`, `outbox_events` — vì bản thân database đã là
đơn vị cách ly tương đương `usage.` schema-qualifier của Postgres, prefix
thêm `usage_` là dư thừa. Code mẫu trên dùng tên có prefix chỉ vì viết
tắt khi phác thảo — khi implement thật (TASK-BE-DB-005) dùng
`sessions`/`daily_rollups`/`outbox_events`, khớp đúng migration MySQL ở
TASK-BE-DB-004.

`MarkPublished`'s `id = ANY($1)` (Postgres array) không có tương đương
trực tiếp ở MySQL — phải build `IN (?, ?, ...)` động theo `len(ids)`.

## 4. Wiring `cmd/server/main.go`

Theo đúng factory đã thiết kế ở BE-DB-SOL-001 §3 — chỉ còn thêm nhánh
`case dbcapability.DialectMySQL` thật (BE-DB-SOL-001 để trống chỗ này).
`healthSrv.Register(string(caps.Dialect), func() error { ... })` — với
MySQL dùng `db.PingContext(ctx)` thay `pool.Ping(ctx)`.

## 5. Migration + CI

Dùng migration MySQL đã viết ở TASK-BE-DB-004 (`migrations/mysql/`).
CI matrix cần `common/testutil.StartMySQL` (chưa tồn tại — thêm mới,
cùng khuôn `StartPostgres`, image `mysql:8`) — chi tiết ở
[TASK-BE-DB-007](../tasks/TASK-BE-DB-007-ci-matrix-postgres-mysql-usage-service.md).

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Phụ thuộc cứng BE-DB-SOL-001 | Cao | Cần `dbcapability.Capabilities`/factory tồn tại trước |
| Tên bảng/schema MySQL khác Postgres (`usage.sessions` → cần quyết định tên) | Trung bình | Chưa chốt ở solution này — chốt ở TASK-BE-DB-004, phải nhất quán với adapter |
| `id = ANY($1)` không có tương đương MySQL trực tiếp | Trung bình | Cần build `IN (...)` động — dễ sai nếu `len(ids) == 0` (đã có guard ở bản Postgres, giữ nguyên) |
| Driver TiDB thật chưa test — chỉ giả định "giống MySQL" qua wire protocol | Trung bình | Bắt buộc chạy ít nhất 1 lần trên container `pingcap/tidb` trước khi coi TiDB support là "done", không chỉ MySQL |
| Vault MySQL dynamic secrets | Không có (ngoài phạm vi code) | Xem §1 — việc ops-side, không phải task Go |

## Không thuộc phạm vi solution này

- 16 service còn lại — nhân rộng, ngoài phạm vi (xem solutions/README.md).
- Data migration tool chuyển dữ liệu khách hàng thật từ Postgres sang
  MySQL/TiDB — theo CR-DB-003's loại trừ, khách hàng on-prem chọn dialect
  từ đầu, không phải chuyển đổi khách hàng đang chạy Postgres.
- Vault infra config thật (Terraform/HCL bật MySQL secrets engine) — ops
  side, xem §1.

## Liên quan

- `backend-go/services/usage-service/internal/adapter/postgres/repository.go` (bản gốc để dịch)
- `backend-go/common/outbox/*.go` (`Store` interface, `Record`)
- `backend-go/common/secrets/*.go` (`DatabaseCredentialsFromFile` — không đổi)
- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md) (phụ thuộc cứng)
