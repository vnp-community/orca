# BE-DB-SOL-009: Adapter MySQL/TiDB thật cho `ai-provider-service`

> **✅ Implemented.** Batch 2 rollout của pattern BE-DB-SOL-001/002 ra
> `ai-provider-service`.

**CR:** [CR-DB-002](../../../../../../docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md), [CR-DB-003](../../../../../../docs/crs/v4/multi-database/CR-DB-003-mysql-tidb-adapter-backend-go.md)
**Service:** `ai-provider-service`
**Task:** [TASK-BE-DB-014](../tasks/TASK-BE-DB-014-ai-provider-service-mysql-rollout.md)
**Phụ thuộc cứng:** [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) — pattern dùng lại nguyên vẹn, không thiết kế lại

---

## 1. `ai-provider-service` khác `usage-service`/batch-1 ở đâu — đọc trước khi implement

Audit trực tiếp (`Read` đầy đủ `internal/usecase/ports.go`,
`internal/adapter/postgres/repository.go`, cả 10 file migration (5 cặp
up/down), `cmd/server/main.go`, `internal/config/config.go`,
`internal/domain/provider_account.go`) xác nhận:

- **1 repository (`Repository`) nhưng implement 5 interface** — nhiều hơn
  cả `usage-service` (2: `Repository`+`outbox.Store`) lẫn
  `credential-broker-service` (3): `usecase.ProviderAccountRepository` (7
  method), `usecase.UsageRepository` (2 method), `usecase.DueHealthCheckClaimer`
  (`ClaimDue`, dùng `SELECT...FOR UPDATE SKIP LOCKED`, pattern mới chưa
  gặp ở batch 1), `usecase.OutboxEnqueuer` (`Enqueue`), và `outbox.Store`
  (`FetchUnpublished`/`MarkPublished`). `cmd/server/main.go`'s `repo`
  biến khai báo với 1 interface ẩn danh gộp cả 5 (mirror
  credential-broker's 3-interface pattern, BE-DB-SOL-006 §6).
- **Không có logic mã hoá/giải mã trong tầng SQL** — `credential_ref` là
  con trỏ opaque tới `credential-broker-service`, không phải giá trị bí
  mật (package doc comment tự khẳng định "NO SECRET COLUMNS ANYWHERE").
  Adapter MySQL chỉ dịch SQL dialect, không đụng logic nào liên quan tới
  Vault/mã hoá — vì service này còn không có adapter Vault nào để mà đụng.
- **CÓ RLS thật trên CẢ 2 bảng** (`accounts` VÀ `usage`, khác
  `credential-broker-service` chỉ có RLS trên 1 bảng): cùng phát hiện
  BE-DB-SOL-001 §4 — không `SET LOCAL app.tenant_id` nào, không
  `FORCE ROW LEVEL SECURITY` nào trong `backend-go`, nên RLS này chưa từng
  active thật. → viết 2 test tenant-isolation-without-RLS mới, 1 cho mỗi
  bảng: `TestRepository_List_DoesNotLeakAcrossTenants` (accounts),
  `TestRepository_GetToday_DoesNotLeakAcrossTenants` (usage).
- **`models` là `TEXT[]` (Postgres array)** — dialect MySQL/TiDB không có
  kiểu mảng. Dịch sang cột `JSON` (MySQL 5.7+/TiDB native), Go
  marshal/unmarshal `[]string` ở tầng adapter (không có JSON operator nào
  cần giữ — outbox relay và mọi usecase chỉ đọc/ghi nguyên khối mảng, đúng
  khuôn "no operator lost" `usage-service`'s JSONB→JSON đã xác lập).
- **`uq_accounts_one_default_per_dev_server_provider` là UNIQUE PARTIAL
  INDEX** (`WHERE is_default AND deleted_at IS NULL`, migration 0003) —
  **case đầu tiên trong toàn bộ rollout CR-DB-002/003 có partial index
  KIỂU UNIQUE**, không phải chỉ non-unique như mọi index `WHERE ...` khác
  đã gặp ở batch 1 (`idx_annotations_worktree`,
  `idx_accounts_tenant_scope_user/project/server`, v.v. — những cái đó chỉ
  cần bỏ `WHERE`, thu hẹp độ chọn lọc chứ không đổi ngữ nghĩa). Một UNIQUE
  index bỏ `WHERE` ở đây SẼ sai: `UNIQUE(tenant_id, dev_server_id,
  provider_type, is_default)` sẽ từ chối 2 tài khoản không-default trong
  cùng nhóm (cả 2 có `is_default=false` trùng khoá) — điều Postgres's
  partial index chưa từng cấm. Giải pháp dùng: cột generated STORED
  (`default_slot_key`) chỉ có giá trị non-NULL khi điều kiện
  `WHERE` gốc đúng, NULL nếu không — rồi UNIQUE index thường trên cột đó.
  MySQL (giống Postgres) coi nhiều NULL trong UNIQUE index là phân biệt
  nhau, nên ngữ nghĩa "tối đa 1 dòng default mỗi nhóm" được giữ nguyên,
  không suy yếu. Xem `migrations/mysql/0003_account_registration_fields.up.sql`'s
  comment đầy đủ.
- **`RowsAffected()` matched-vs-changed pitfall áp dụng cho CẢ
  `UpdateStatus` LẪN `Update`** (khác `usage-service`/`credential-broker-service`,
  nơi pitfall này không xuất hiện vì không có method nào có thể no-op hợp
  lệ theo kiểu retry) — cả 2 method này nhận input có thể trùng giá trị
  hiện tại của dòng khi caller retry idempotent (ví dụ `UpdateStatus` gọi
  lại đúng `status`/`credential_ref` cũ). Áp dụng đúng pattern
  TASK-BE-DB-010 (`annotation-service`) đã xác lập: KHÔNG BAO GIỜ đọc
  `RowsAffected()` để quyết định not-found ở 2 method này — luôn UPDATE
  rồi SELECT lại theo đúng WHERE clause gốc. `Delete` (soft-delete) NGƯỢC
  LẠI an toàn để dùng `RowsAffected()` trực tiếp — vì `deleted_at` luôn
  chuyển từ NULL sang non-NULL khi WHERE khớp, nên "khớp" và "đổi" luôn
  đồng nhất ở method này; giải thích đầy đủ trong `Delete`'s doc comment.
  `TestRepository_UpdateStatus_NoopRetryStillSucceeds` (mới, không có ở
  bản Postgres) chứng minh fix bằng test thật.
- **`cmd/server/main.go` TRƯỚC task này dùng thẳng `cfg.DatabaseDSN`** +
  tự check `dsn == ""` bằng `errors.New(...)` — giống
  `credential-broker-service` TRƯỚC BE-DB-SOL-006, khác `usage-service`.
  Đã thay bằng `secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)`
  + `dbcapability.DetectDialectFromDSN`, đóng luôn README's "Known gap"
  "`common/secrets` (Vault) is not wired into `main.go`" như tác dụng phụ
  hợp lý (cùng đoạn code cần sửa để chọn dialect), không phải mở rộng
  phạm vi.

## 2. Chọn driver — giống hệt BE-DB-SOL-002 §2

`github.com/go-sql-driver/mysql` qua `database/sql`, TiDB dùng chung
nhánh MySQL (wire protocol, ADR-021). Test container `mysql:8` qua
`common/testutil.StartMySQL` (đã có sẵn từ pilot, không sửa).

## 3. `internal/adapter/mysql/repository.go`

Dịch 1:1 cả 13 method (`Create`, `FetchUnpublished`, `MarkPublished`,
`Enqueue`, `Get`, `List`, `UpdateStatus`, `Update`, `Delete`,
`MarkQuotaWarningSent`, `GetToday`, `IncrementUsage`, `ClaimDue`) từ bản
Postgres. Khác biệt chính so với dịch thẳng `$N` → `?`:

- `models []string` ↔ cột JSON: marshal trước INSERT, unmarshal sau SELECT
  (xem mục 1).
- `UpdateStatus`/`Update`: UPDATE rồi luôn SELECT lại theo cùng WHERE,
  không đọc `RowsAffected()` (xem mục 1's pitfall).
- `IncrementUsage`: `ON CONFLICT ... DO UPDATE ... RETURNING` →
  `ON DUPLICATE KEY UPDATE` + SELECT lại theo khoá chính
  `(account_id, date)` sau upsert (không có ambiguity not-found ở đây —
  upsert luôn khớp).
- `ClaimDue`: `SELECT ... FOR UPDATE SKIP LOCKED` giữ nguyên cú pháp
  (MySQL 8.0.1+ hỗ trợ) — `ORDER BY last_health_check_at` bỏ `NULLS
  FIRST` (MySQL không có cú pháp này) vì MySQL đã sắp NULL lên đầu trong
  `ASC` mặc định, cùng hiệu ứng.
- `MarkPublished`'s `id = ANY($1)` → `IN (?,...)` động, guard
  `len(ids) == 0`, cùng khuôn `usage-service`.
- `errors.Is(err, pgx.ErrNoRows)` → `errors.Is(err, sql.ErrNoRows)`.
- Tên bảng theo quyết định đã chốt ở BE-DB-SOL-002 §3 (database MySQL
  tên `ai_provider`, bảng không prefix: `accounts`, `usage`, `outbox`).

## 4. Compensating control cho tenant isolation — áp dụng cho CẢ 2 bảng

Khác `credential-broker-service` (RLS chỉ trên 1 bảng), `accounts` VÀ
`usage` đều có RLS thật trên Postgres. 2 test mới
(`TestRepository_List_DoesNotLeakAcrossTenants`,
`TestRepository_GetToday_DoesNotLeakAcrossTenants`) chứng minh
application-layer `WHERE tenant_id = ?` một mình đủ cách ly 2 tenant trên
MySQL cho cả 2 bảng.

## 5. Migration dialect-safe

```
backend-go/services/ai-provider-service/migrations/
├── postgres/            (nội dung y hệt 10 file gốc, chỉ ĐỔI VỊ TRÍ)
│   ├── 0001_init.{up,down}.sql
│   ├── 0002_dev_server_id.{up,down}.sql
│   ├── 0003_account_registration_fields.{up,down}.sql
│   ├── 0004_outbox.{up,down}.sql
│   └── 0005_health_and_usage_writes.{up,down}.sql
└── mysql/               (mới)
    ├── 0001_init.{up,down}.sql       — bỏ CREATE SCHEMA, bỏ RLS (2 bảng),
    │                                   UUID→CHAR(36), 3 index WHERE-partial
    │                                   không-unique thu hẹp thành index đầy
    │                                   đủ, NUMERIC(12,4)→DECIMAL(12,4)
    ├── 0002_dev_server_id.{up,down}.sql — TEXT NOT NULL DEFAULT ''→VARCHAR
    │                                      (MySQL không nhận DEFAULT literal
    │                                      trên TEXT/BLOB)
    ├── 0003_account_registration_fields.{up,down}.sql — TEXT[]→JSON,
    │                                      UNIQUE PARTIAL INDEX→generated
    │                                      column + UNIQUE index thường
    │                                      (xem mục 1, case đầu tiên trong
    │                                      rollout)
    ├── 0004_outbox.{up,down}.sql        — JSONB→JSON, index WHERE-partial
    │                                      thu hẹp
    └── 0005_health_and_usage_writes.{up,down}.sql — index WHERE-partial
                                           thu hẹp
```

Không có `gen_random_uuid()` được dùng thật (ID sinh ở Go,
`uuid.NewString()` qua `usecase.NewCreateAccount(..., uuid.NewString,
...)`) nên bỏ default đó không mất chức năng, cùng lý do
`usage-service`/`annotation-service` đã xác nhận.

## 6. Wiring `cmd/server/main.go`

Theo đúng `switch caps.Dialect` pattern của `usage-service`, bao gồm
`toMySQLDriverDSN` copy nguyên văn. `repo` khai báo với 1 interface ẩn
danh gộp **5** port (`ProviderAccountRepository`+`UsageRepository`+
`DueHealthCheckClaimer`+`OutboxEnqueuer`+`outbox.Store`) — nhiều hơn
`credential-broker-service`'s 3, vì `ai-provider-service` có nhiều
usecase constructor dùng `repo` ở nhiều vai trò khác nhau hơn
(`reconcileHealthUC`/`recordTokenUsageUC` cần `DueHealthCheckClaimer`/
`OutboxEnqueuer`, relay cần `outbox.Store`). `healthSrv.Register` đổi tên
theo `caps.Dialect` (`"postgres"`/`"mysql"`) giống pilot.

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| `impact()` cho `run`/`Load`/`Repository`/`ProviderAccountRepository` | Đã chạy — LOW cho 3/4 (`run` impactedCount 1, `Load` impactedCount 2, `Repository` struct impactedCount 3); `ProviderAccountRepository` interface **MEDIUM** (impactedCount 6, toàn bộ IMPORTS — không caller nào bị phá vì interface không đổi signature) | Xem TASK-BE-DB-014's "Kết quả thực tế" — không HIGH/CRITICAL nào |
| UNIQUE partial index không có tiền lệ trong rollout trước đó | Trung bình, đã giải quyết bằng generated column | Case đầu tiên — xem mục 1 và migration 0003's comment đầy đủ |
| `RowsAffected()` matched-vs-changed pitfall trên 2 method (`UpdateStatus`, `Update`) | Trung bình, đã fix + test | Xem mục 1 — `TestRepository_UpdateStatus_NoopRetryStillSucceeds` mới |
| `models TEXT[]` → JSON round-trip | Thấp | Không có JSON operator nào cần giữ, chỉ đọc/ghi nguyên khối |
| Driver TiDB thật chưa test — chỉ giả định "giống MySQL" qua wire protocol | Trung bình | Kế thừa từ BE-DB-SOL-002 §Rủi ro, chưa đóng ở service nào trong rollout tính đến batch 2 |
| Vault MySQL dynamic secrets | Không có (ngoài phạm vi code) | Giống BE-DB-SOL-002 §1 — ops-side |

## Không thuộc phạm vi solution này

- 13 service còn lại của rollout (sau batch 2) — xem `ROLLOUT-TRACKING.md`.
- Sửa `internal/adapter/grpcclient`/`internal/adapter/eventbus`/
  `internal/adapter/scheduler` — không đụng vào, không liên quan tới tầng
  DB.
- Data migration tool / Vault infra thật — như BE-DB-SOL-002 §"Không thuộc
  phạm vi".

## Liên quan

- `backend-go/services/ai-provider-service/internal/usecase/ports.go` (5 interface)
- `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go` (bản gốc để dịch)
- `backend-go/services/ai-provider-service/internal/adapter/mysql/repository.go` (mới)
- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (phụ thuộc cứng)
- [BE-DB-SOL-006](./BE-DB-SOL-006-credential-broker-service-mysql-tidb-adapter.md) (mẫu gần nhất cho service quản lý credential-shaped data + `RowsAffected()` pitfall's TASK-BE-DB-010 origin)
