# BE-DB-SOL-006: Adapter MySQL/TiDB thật cho `credential-broker-service`

> **✅ Implemented.** Batch 1 rollout của pattern BE-DB-SOL-001/002 ra
> `credential-broker-service`.

**CR:** [CR-DB-002](../../../../../../docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md), [CR-DB-003](../../../../../../docs/crs/v4/multi-database/CR-DB-003-mysql-tidb-adapter-backend-go.md)
**Service:** `credential-broker-service`
**Task:** [TASK-BE-DB-011](../tasks/TASK-BE-DB-011-credential-broker-service-mysql-rollout.md)
**Phụ thuộc cứng:** [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) — pattern dùng lại nguyên vẹn, không thiết kế lại

---

## 1. `credential-broker-service` khác `usage-service` (pilot) ở đâu — đọc trước khi implement

Audit trực tiếp (`Read` đầy đủ `internal/usecase/ports.go`,
`internal/adapter/postgres/repository.go`, 6 file migration,
`cmd/server/main.go`, `internal/config/config.go`, `internal/domain/*.go`)
xác nhận:

- **Không có logic mã hoá/giải mã nào trong tầng SQL.** `internal/domain`'s
  package doc comment tự khẳng định "NONE of them has a field capable of
  holding a secret value" — mọi cột trong `credential_metadata` là con
  trỏ/enum/timestamp, không có ciphertext. Mã hoá thật (Transit
  encrypt/decrypt, KV read/write) sống hoàn toàn ở `internal/adapter/vault`
  qua `usecase.SecretStore` — **không đụng vào** khi viết adapter MySQL,
  đúng yêu cầu "đừng làm yếu logic mã hoá/kiểm soát truy cập, chỉ đổi SQL
  dialect". Adapter MySQL này chỉ xử lý con trỏ (`vault_path`), không bao
  giờ thấy giá trị bí mật.
- **1 repository nhưng 3 port**: `usecase.CredentialMetadataRepository`
  (5 method: `Create`, `Get`, `UpdateStatus`, `GetByOwner`,
  `ListByCategory`), `usecase.AuditRepository` (`Append`), và
  `usecase.TxRunner` (`RunInTx`) — đều được implement bởi CÙNG 1 struct
  `Repository`, khác `usage-service`'s 2-port shape
  (`Repository`+`outbox.Store`). `RunInTx` mở 1 transaction thật và bọc
  cả metadata mutation lẫn audit append — pattern mới so với pilot
  (pilot không có transaction-wrapping port), dịch bằng
  `database/sql`'s `BeginTx`/`Commit`/`Rollback` thay
  `pgx.BeginFunc`.
- **CÓ RLS thật trong migration** (khác `usage-service`'s finding, khác
  `issue-status-sync` hoàn toàn không có RLS): `migrations/postgres/0001_init.up.sql`
  có `ALTER TABLE credential.credential_metadata ENABLE ROW LEVEL
  SECURITY` + `CREATE POLICY tenant_isolation`. → **TASK-BE-DB-003's
  tenant-isolation-without-RLS pattern ÁP DỤNG cho service này** — viết 2
  test mới (`GetByOwner`/`ListByCategory`, 2 method có filter
  `tenant_id`) mirror pilot. Cùng phát hiện như BE-DB-SOL-001 §4: không
  có `SET LOCAL app.tenant_id` nào trong `backend-go`, không migration nào
  dùng `FORCE ROW LEVEL SECURITY` → RLS này chưa từng thực sự active,
  application-layer scoping (`WHERE tenant_id = $1` tường minh ở mọi
  query) đã là cơ chế bảo vệ duy nhất có thật, kể cả trên Postgres hôm
  nay.
- **`owner_id` là TEXT/VARCHAR (không phải UUID/CHAR(36))** — migration
  0003 đã đổi từ UUID sang TEXT (bug thật phát hiện ở TASK-043, xem
  migration's comment): caller thật truyền `"bitbucket"`, `"jira"`, không
  phải UUID. MySQL migration dùng `VARCHAR(255)` ngay từ 0001 (không
  `CHAR(36)`) để tránh lặp lại đúng bug này ở dialect mới.
- **`cmd/server/main.go` TRƯỚC task này dùng thẳng `cfg.DatabaseDSN`** +
  tự check `dsn == ""` bằng `errors.New(...)` — giống `issue-status-sync`
  TRƯỚC BE-DB-SOL-003, khác `usage-service`. Task này thêm cả
  dialect-detection lẫn Vault-credentials-file resolution cùng lúc, thêm
  field `DatabaseCredentialsFile` mới vào `internal/config/config.go`
  (mirror `usage-service`'s field/default y hệt: env
  `DATABASE_CREDENTIALS_FILE`, default `/vault/secrets/database-credentials`).

## 2. Chọn driver — giống hệt BE-DB-SOL-002 §2

`github.com/go-sql-driver/mysql` qua `database/sql`, TiDB dùng chung
nhánh MySQL (wire protocol, ADR-021). Test container `mysql:8` qua
`common/testutil.StartMySQL` (đã có sẵn từ pilot, không sửa).

## 3. `internal/adapter/mysql/repository.go`

Dịch 1:1 cả 6 method từ bản Postgres. Khác biệt chính so với dịch thẳng
`$N` → `?`:

- **`RunInTx`**: `pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {...})`
  → `db.BeginTx(ctx, nil)` + `fn(...)` + `tx.Commit()`/`tx.Rollback()` thủ
  công (khác pilot, vì `usage-service` không có method này) — cùng ngữ
  nghĩa: `fn` trả lỗi → rollback + trả lỗi gốc; `fn` trả `nil` → commit,
  lỗi commit được bọc riêng.
- `dbtx` interface local (mirror bản Postgres's `dbtx`) trừu tượng hoá
  `*sql.DB`/`*sql.Tx` — cả 2 đều thoả `ExecContext`/`QueryRowContext`/
  `QueryContext`, nên mọi method chạy y hệt dù `conn` là pool hay tx.
- `errors.Is(err, pgx.ErrNoRows)` → `errors.Is(err, sql.ErrNoRows)` ở
  `Get`/`GetByOwner`.
- `UpdateStatus` giữ nguyên logic `RowsAffected() == 0` →
  `ErrCredentialNotFound` (bản Postgres dùng `pgconn.CommandTag.RowsAffected()`,
  bản MySQL dùng `sql.Result.RowsAffected()`, cùng ngữ nghĩa).
- Không có `ON CONFLICT`/`RETURNING`/`id = ANY($1)` nào trong bản gốc —
  khác `usage-service`, service này không cần các bản dịch phức tạp đó.
- **Không đổi tenant-scoping**: mọi `WHERE tenant_id = ?` giữ nguyên vị
  trí/logic hệt bản Postgres — compensating control duy nhất khi MySQL
  không có RLS (xem BE-DB-SOL-001 §4, mục 1 ở trên).
- Tên bảng theo quyết định đã chốt ở BE-DB-SOL-002 §3 (database MySQL
  tên `credential`, bảng không prefix: `credential_metadata`,
  `access_audit_log`).

## 4. Compensating control cho tenant isolation — CÓ áp dụng (khác issue-status-sync)

Khác `issue-status-sync` (không có cột `tenant_id`), `credential_metadata`
CÓ `tenant_id` + RLS policy trên Postgres. 2 test mới trong
`internal/adapter/mysql/repository_test.go`
(`TestRepository_GetByOwner_DoesNotLeakAcrossTenants`,
`TestRepository_ListByCategory_DoesNotLeakAcrossTenants`) chứng minh
application-layer `WHERE tenant_id = ?` một mình đủ cách ly 2 tenant trên
MySQL — dialect không có bất kỳ RLS-tương đương nào, khác Postgres nơi ít
nhất còn 1 policy khai báo (dù chưa từng active thật, xem mục 1).

## 5. Migration dialect-safe

```
backend-go/services/credential-broker-service/migrations/
├── postgres/            (nội dung y hệt 6 file gốc, chỉ ĐỔI VỊ TRÍ)
│   ├── 0001_init.{up,down}.sql
│   ├── 0002_config_json.{up,down}.sql
│   └── 0003_owner_id_text.{up,down}.sql
└── mysql/               (mới)
    ├── 0001_init.{up,down}.sql      — bỏ CREATE/DROP SCHEMA, bỏ RLS
    │                                  (comment giải thích thay thế),
    │                                  UUID→CHAR(36), BIGINT GENERATED
    │                                  ALWAYS AS IDENTITY→BIGINT
    │                                  AUTO_INCREMENT, CHECK constraint
    │                                  giữ nguyên (MySQL 8.0.16+ hỗ trợ)
    ├── 0002_config_json.{up,down}.sql  — TEXT→TEXT (không đổi, cột này
    │                                     chưa từng là JSONB)
    └── 0003_owner_id_text.{up,down}.sql — CHAR(36)→VARCHAR(255), down
                                            KHÔNG fail-loudly như bản
                                            Postgres's `::uuid` cast
                                            (ghi rõ trong file, xem mục 1)
```

Không có `gen_random_uuid()` trong bản gốc (ID sinh ở Go,
`uuid.NewString()` — xem `write_credential.go`) nên không có gap
ID-generation. Không có `RETURNING` trong bản gốc.

## 6. Wiring `cmd/server/main.go`

Theo đúng `switch caps.Dialect` pattern của `usage-service/cmd/server/main.go`'s
`run()`, bao gồm `toMySQLDriverDSN` helper copy nguyên văn (không share
được qua import giữa 2 `cmd/main` package khác nhau). `repo` được khai
báo với 1 interface ẩn danh gộp cả 3 port
(`CredentialMetadataRepository`+`AuditRepository`+`TxRunner`) vì cả 3
constructor usecase cần các tổ hợp khác nhau của cùng 1 biến — khác
`usage-service` nơi `repo`/`outboxStore` tách biệt 2 biến vì 2 interface
độc lập (`usecase.Repository`, `outbox.Store`). `healthSrv.Register` đổi
tên theo `caps.Dialect` (`"postgres"`/`"mysql"`) giống pilot; `"vault"`
health check giữ nguyên không đổi (Vault không phụ thuộc dialect DB).

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| `impact()` cho `run`/`Load`/`CredentialMetadataRepository`/`Repository` | Đã chạy — LOW cả 4 (impactedCount 1/2/3/4) | Xem TASK-BE-DB-011's "Kết quả thực tế" |
| Không có RLS-tương đương ở MySQL, khác Postgres | Trung bình, đã compensat qua test | Xem mục 4 — 2 test PASS thật trên MySQL 8 |
| `0003_owner_id_text.down.sql`'s MySQL variant silently truncates thay vì fail loudly | Thấp, đã ghi nhận rõ trong file | Khác biệt hành vi thật giữa 2 dialect, không phải bug — MySQL không có type cast tương đương `::uuid` |
| Vault MySQL dynamic secrets | Không có (ngoài phạm vi code) | Giống BE-DB-SOL-002 §1 — ops-side |

## Không thuộc phạm vi solution này

- 14 service còn lại của rollout — xem `ROLLOUT-TRACKING.md`.
- Sửa `internal/adapter/vault`/`common/secrets` — không đụng vào, đúng
  yêu cầu "preserve encryption-at-rest logic exactly".
- Data migration tool / Vault infra thật — như BE-DB-SOL-002 §"Không thuộc
  phạm vi".

## Liên quan

- `backend-go/services/credential-broker-service/internal/usecase/ports.go` (`CredentialMetadataRepository`, `AuditRepository`, `TxRunner`)
- `backend-go/services/credential-broker-service/internal/adapter/postgres/repository.go` (bản gốc để dịch)
- `backend-go/services/credential-broker-service/internal/adapter/mysql/repository.go` (mới)
- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (phụ thuộc cứng)
