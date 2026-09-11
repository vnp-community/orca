# BE-DB-SOL-003: Adapter MySQL/TiDB thật cho `issue-status-sync`

> **✅ Implemented.** Rollout đầu tiên (batch 1) của pattern BE-DB-SOL-001/002
> ra ngoài pilot `usage-service`, áp dụng cho `issue-status-sync`.

**CR:** [CR-DB-002](../../../../../../docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md), [CR-DB-003](../../../../../../docs/crs/v4/multi-database/CR-DB-003-mysql-tidb-adapter-backend-go.md)
**Service:** `issue-status-sync`
**Task:** [TASK-BE-DB-008](../tasks/TASK-BE-DB-008-issue-status-sync-mysql-rollout.md)
**Phụ thuộc cứng:** [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) — pattern dùng lại nguyên vẹn, không thiết kế lại

---

## 1. `issue-status-sync` khác `usage-service` (pilot) ở đâu — đọc trước khi implement

Audit trực tiếp (`Read` đầy đủ `internal/usecase/ports.go`,
`internal/adapter/postgres/processed_events.go`,
`migrations/0001_processed_events.{up,down}.sql`, `cmd/server/main.go`)
xác nhận service này **nhỏ hơn và đơn giản hơn pilot đáng kể**:

- **1 repository, 1 bảng, 1 cột không phải khoá**: `usecase.ProcessedEventStore`
  chỉ có 2 method (`Seen`, `MarkSeen`) chống lại bảng
  `issuestatussync.processed_events (event_id TEXT PRIMARY KEY,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now())` — không transaction đa
  bảng, không outbox, không rollup.
- **Không có RLS, không có cột `tenant_id`** — khác `usage-service`'s
  `usage.sessions`/`usage.daily_rollups` (2 policy RLS + `tenant_id` trên
  mọi bảng). `processed_events` là dedup cache thuần theo `event_id` toàn
  cục, không phân theo tenant. → **TASK-BE-DB-003's tenant-isolation-
  without-RLS test pattern KHÔNG áp dụng được** cho service này — không có
  gì để cách ly, xác nhận bằng đọc trực tiếp migration, không suy đoán.
- **Không dùng `gen_random_uuid()` lẫn `RETURNING`** — `MarkSeen` chỉ
  `INSERT ... ON CONFLICT (event_id) DO NOTHING`, không cần ID sinh phía
  DB (khoá chính chính là `event_id` truyền vào từ caller —
  `common/eventbus.Event.ID`, một chuỗi opaque service khác đã sinh). Cùng
  kết luận như `usage-service`: không có lock-in `gen_random_uuid()`.
- **`cmd/server/main.go` TRƯỚC task này chưa hề dùng `secrets.DatabaseCredentialsFromFile`
  hay `dbcapability`** — dùng thẳng `cfg.DatabaseDSN` (từ `commonconfig.Base`)
  qua `pgxpool.New(ctx, dsn)`, kèm 1 check `dsn == ""` trả lỗi tường minh.
  Task này thêm **cả** dialect-detection **và** Vault-credentials-file
  resolution cùng lúc — khác `usage-service`, nơi Vault-file resolution đã
  có sẵn từ trước (Epic G) và BE-DB-SOL-001 chỉ thêm dialect-detection lên
  trên nó. `internal/config/config.go` được thêm field
  `DatabaseCredentialsFile` mới (mirror `usage-service/internal/config/config.go`),
  KHÔNG thêm biến env mới (`DATABASE_CREDENTIALS_FILE`, cùng tên/default
  `/vault/secrets/database-credentials` như mọi service khác).

## 2. Chọn driver — giống hệt BE-DB-SOL-002 §2

`github.com/go-sql-driver/mysql` qua `database/sql`, TiDB dùng chung
nhánh MySQL (wire protocol, ADR-021). Test container `mysql:8` qua
`common/testutil.StartMySQL` (đã có sẵn từ pilot, không sửa).

## 3. `internal/adapter/mysql/processed_events.go`

Dịch 1:1 `ON CONFLICT (event_id) DO NOTHING` → `INSERT IGNORE` (dựa vào
PRIMARY KEY `event_id`, không cần kiểm tra `RowsAffected()` vì `MarkSeen`
không có logic phụ thuộc kết quả — khác `usage-service`'s `SaveSession`
nơi `RowsAffected()==0` quyết định có chạy tiếp rollup/outbox hay không);
`errors.Is(err, pgx.ErrNoRows)` → `errors.Is(err, sql.ErrNoRows)`. Tên
bảng theo đúng quyết định đã chốt ở BE-DB-SOL-002 §3 (database MySQL tên
`issuestatussync`, bảng không prefix: `processed_events`).

```go
// backend-go/services/issue-status-sync/internal/adapter/mysql/processed_events.go
type ProcessedEventsStore struct{ db *sql.DB }

func New(db *sql.DB) *ProcessedEventsStore { return &ProcessedEventsStore{db: db} }

func (s *ProcessedEventsStore) Seen(ctx context.Context, eventID string) (bool, error) {
    var id string
    err := s.db.QueryRowContext(ctx, `SELECT event_id FROM processed_events WHERE event_id = ?`, eventID).Scan(&id)
    if errors.Is(err, sql.ErrNoRows) { return false, nil }
    if err != nil { return false, fmt.Errorf("mysql: query processed events: %w", err) }
    return true, nil
}

func (s *ProcessedEventsStore) MarkSeen(ctx context.Context, eventID string) error {
    _, err := s.db.ExecContext(ctx, `INSERT IGNORE INTO processed_events (event_id) VALUES (?)`, eventID)
    if err != nil { return fmt.Errorf("mysql: mark event processed: %w", err) }
    return nil
}
```

## 4. Compensating control cho tenant isolation — KHÔNG áp dụng, đã kiểm tra

Khác mọi service khác trong đợt rollout này (dự kiến), `processed_events`
**không có cột `tenant_id`** ở bất kỳ dialect nào — dedup cache theo
`event_id` toàn cục (JetStream event ID đã unique toàn hệ thống, không
cần scope theo tenant để chống trùng). Do đó:

- Không viết test tenant-isolation-without-RLS kiểu TASK-BE-DB-003 cho
  service này — không có gì để rò rỉ giữa các tenant qua bảng này.
- Đây là kết luận rút ra từ việc đọc trực tiếp migration/repository, không
  phải giả định — nếu 1 service khác trong rollout có cột `tenant_id` trên
  bảng của nó, task tương ứng của service đó PHẢI viết test này (theo
  README.md's checklist gốc), không được bỏ qua theo lệ ngoại của tài
  liệu này.

## 5. Migration dialect-safe

```
backend-go/services/issue-status-sync/migrations/
├── postgres/            (nội dung y hệt file gốc, chỉ ĐỔI VỊ TRÍ)
│   └── 0001_processed_events.up.sql / .down.sql
└── mysql/               (mới)
    └── 0001_processed_events.up.sql / .down.sql
```

Khác biệt dialect duy nhất: bỏ `CREATE SCHEMA`/`DROP SCHEMA` (MySQL dùng
DATABASE làm đơn vị cách ly, giống quyết định BE-DB-SOL-002 §3); `TEXT
PRIMARY KEY` → `VARCHAR(255) PRIMARY KEY` (InnoDB từ chối `TEXT`/`BLOB`
làm khoá chính nếu không khai độ dài prefix — MySQL-specific constraint
không tồn tại ở Postgres, phát hiện khi viết migration, không có trong
BE-DB-SOL-001/002 vì `usage-service` không có cột `TEXT PRIMARY KEY` nào
kiểu này — `usage.sessions.id` cũng là `TEXT PRIMARY KEY` ở Postgres
nhưng bản MySQL của nó đã dùng `VARCHAR(64)` sẵn, cùng lý do); `TIMESTAMPTZ
DEFAULT now()` → `TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6)` (cùng mapping
đã dùng ở `usage-service`'s migration).

## 6. Wiring `cmd/server/main.go`

Theo đúng `switch caps.Dialect` pattern của `usage-service/cmd/server/main.go`'s
`run()` — bao gồm cả `toMySQLDriverDSN` helper (copy nguyên văn, không thể
share qua import vì `cmd/main` không import được giữa các service). Khác
1 điểm: `issue-status-sync` trước đây validate `dsn == ""` thủ công bằng
`errors.New(...)` — thay bằng `secrets.DatabaseCredentialsFromFile` (đã tự
fallback `DATABASE_DSN` env, xem `common/secrets/vault.go`), bỏ check thủ
công vì hàm này đã trả lỗi rõ ràng khi cả file lẫn env đều thiếu.

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| `impact()` cho `ProcessedEventStore`/`ProcessedEventsStore`/`run` | Đã chạy — LOW cả 3 (impactedCount 3/1/3, không process/module HIGH nào) | Xem TASK-BE-DB-008's "Kết quả thực tế" |
| `TEXT PRIMARY KEY` không hợp lệ trực tiếp ở MySQL InnoDB | Thấp, đã xử lý | `VARCHAR(255)` đủ rộng cho `event.ID` dạng UUID thực tế |
| Không có test integration nào tồn tại từ trước cho `internal/adapter/postgres` | Thấp, ngoài phạm vi | Gap tiền tồn tại — CI's lane `postgres` sẽ chỉ chạy build/vet/unit + "no test files" ở bước integration, không phải lỗi mới do task này gây ra; không tự thêm test Postgres mới vì task chỉ yêu cầu adapter/test MySQL |
| Vault MySQL dynamic secrets | Không có (ngoài phạm vi code) | Giống BE-DB-SOL-002 §1 — ops-side |

## Không thuộc phạm vi solution này

- 14 service còn lại của rollout — xem `ROLLOUT-TRACKING.md`.
- Thêm integration test cho `internal/adapter/postgres` (gap tiền tồn tại,
  không phải do CR-DB này gây ra — ghi nhận, không tự ý mở rộng phạm vi).
- Data migration tool / Vault infra thật — như BE-DB-SOL-002 §"Không thuộc
  phạm vi".

## Liên quan

- `backend-go/services/issue-status-sync/internal/usecase/ports.go` (`ProcessedEventStore`)
- `backend-go/services/issue-status-sync/internal/adapter/postgres/processed_events.go` (bản gốc để dịch)
- `backend-go/services/issue-status-sync/internal/adapter/mysql/processed_events.go` (mới)
- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (phụ thuộc cứng)
