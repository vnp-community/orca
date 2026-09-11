# TASK-BE-DB-011: Multi-database rollout cho `credential-broker-service`

**Solution:** [BE-DB-SOL-006](../solutions/BE-DB-SOL-006-credential-broker-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `credential-broker-service`
**Depends on:** TASK-BE-DB-002 (`dbcapability`, dùng lại nguyên vẹn), pattern TASK-BE-DB-004~007 (`usage-service` pilot)
**Status:** ✅ DONE (2026-09-11)

> **Kết quả thực tế:**
>
> **1. `impact()` chạy thật trước khi sửa cả 4 symbol hiện có** (bắt buộc
> theo AGENTS.md/CLAUDE.md, không suy đoán), tất cả trên repo `orca`:
> - `impact({target:"run", direction:"upstream",
>   file_path:"...cmd/server/main.go", kind:"Function"})` → **LOW**,
>   impactedCount 1 (`main`).
> - `impact({target:"Load", direction:"upstream",
>   file_path:"...internal/config/config.go", kind:"Function"})` → **LOW**,
>   impactedCount 2 (`run` → `main`), 1 module `Usecase` ảnh hưởng trực
>   tiếp.
> - `impact({target:"CredentialMetadataRepository", direction:"upstream",
>   file_path:"...internal/usecase/ports.go", kind:"Interface"})` →
>   **LOW**, impactedCount 3 (`main.go`, `adapter/postgres/repository.go`,
>   `adapter/grpc/server.go` — chỉ IMPORT, không caller nào bị phá vì
>   interface không đổi).
> - `impact({target:"Repository", direction:"upstream",
>   file_path:"...internal/adapter/postgres/repository.go",
>   kind:"Struct"})` → **LOW**, impactedCount 4 (`New`, `RunInTx` →
>   `run` → `main`), 1 module `Usecase` ảnh hưởng trực tiếp.
>
> Không có HIGH/CRITICAL nào ở cả 4 lần chạy — an toàn để tiếp tục theo
> đúng ngưỡng CLAUDE.md quy định. `internal/adapter/postgres/repository.go`
> **KHÔNG bị sửa** (chỉ dịch sang MySQL trong package mới, y hệt ràng buộc
> "preserve encryption-at-rest/access-control logic exactly, chỉ retarget
> SQL dialect" của nhiệm vụ — và thực tế không có logic mã hoá nào trong
> file này để mà làm yếu, xem BE-DB-SOL-006 §1).
>
> **2. Migration split** — 6 file `migrations/000{1,2,3}_*.{up,down}.sql`
> di chuyển nguyên văn (`git mv`, không sửa nội dung) vào
> `migrations/postgres/`. `migrations/mysql/` mới với 6 file tương ứng:
> - `0001_init.up.sql`: bỏ `CREATE SCHEMA`, bỏ 2 dòng RLS (comment giải
>   thích application-layer scoping thay thế, dẫn BE-DB-SOL-001 §4 —
>   **đã xác nhận migration Postgres gốc CÓ RLS thật** trước khi viết
>   comment này, không suy đoán), `UUID` → `CHAR(36)`,
>   `BIGINT GENERATED ALWAYS AS IDENTITY` → `BIGINT AUTO_INCREMENT`, `TEXT
>   CHECK (category IN (...))` → `VARCHAR(32) CHECK (...)` (MySQL 8.0.16+
>   hỗ trợ CHECK constraint enforced — xác nhận qua chạy test thật, xem
>   mục 5).
> - `0002_config_json.up.sql`: không đổi kiểu (`TEXT` cả 2 dialect — cột
>   này chưa từng là `JSONB` ở bản gốc, không có JSON-operator nào để mất).
> - `0003_owner_id_text.up.sql`: `CHAR(36)` → `VARCHAR(255)` (mirror đúng
>   widening đã làm ở Postgres). `.down.sql` ghi rõ 1 khác biệt hành vi
>   thật giữa 2 dialect: bản Postgres's `::uuid` cast fail loudly trên dữ
>   liệu không phải UUID, bản MySQL's `MODIFY COLUMN` sẽ **silently
>   truncate** thay vì lỗi — không có type cast tương đương ở MySQL, đã
>   ghi chú trực tiếp trong file thay vì che giấu.
>
> **3. `internal/adapter/mysql/repository.go` mới** — implement đúng cả 3
> port hiện có (`CredentialMetadataRepository` 5 method,
> `AuditRepository` 1 method, `TxRunner` 1 method `RunInTx`), KHÔNG đổi
> signature nào. Điểm khác biệt lớn nhất so với pilot: `RunInTx` không có
> tương đương trực tiếp trong `usage-service` (pilot không có port này) —
> dịch từ `pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {...})` sang
> `db.BeginTx(ctx, nil)` + gọi `fn` thủ công + `tx.Commit()`/
> `tx.Rollback()` theo kết quả `fn` trả về, giữ đúng ngữ nghĩa "cả metadata
> mutation lẫn audit append cùng rollback nếu 1 trong 2 lỗi". `dbtx`
> interface local (`ExecContext`/`QueryRowContext`/`QueryContext`) trừu
> tượng hoá `*sql.DB`/`*sql.Tx`, mirror đúng cấu trúc `dbtx` interface của
> bản Postgres. `errors.Is(err, pgx.ErrNoRows)` → `errors.Is(err,
> sql.ErrNoRows)` ở `Get`/`GetByOwner`. Không có `ON CONFLICT`/
> `RETURNING`/`id = ANY($1)` trong bản gốc nên không cần các bản dịch phức
> tạp `usage-service` từng cần.
>
> **Phát hiện quan trọng, khác BE-DB-SOL-003 (issue-status-sync)**:
> `credential_metadata` CÓ cột `tenant_id` + RLS policy trên migration
> Postgres gốc (khác `issue-status-sync`'s `processed_events`, hoàn toàn
> không có `tenant_id`). → **Viết 2 test tenant-isolation-without-RLS
> mới**, mirror TASK-BE-DB-003's pattern cho 2 method có filter
> `tenant_id` (`GetByOwner`, `ListByCategory`):
> `TestRepository_GetByOwner_DoesNotLeakAcrossTenants`,
> `TestRepository_ListByCategory_DoesNotLeakAcrossTenants`. Cả 2 dùng
> `owner_id` dạng chuỗi thật (`"bitbucket"`, `"jira"`, không phải UUID) —
> khớp đúng phát hiện TASK-043 đã ghi trong migration 0003's comment, để
> test phản ánh caller thật thay vì dữ liệu tuỳ tiện.
>
> **4. Wiring `cmd/server/main.go`** — thêm `switch caps.Dialect` đúng
> khuôn `usage-service`, bao gồm `toMySQLDriverDSN` copy nguyên văn. Khác
> 1 điểm so với pilot, giống `issue-status-sync` (BE-DB-SOL-003):
> `credential-broker-service` TRƯỚC ĐÓ dùng thẳng `cfg.DatabaseDSN` + tự
> check `dsn == ""` — đã thay bằng
> `secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)`
> (field `DatabaseCredentialsFile` mới trong `internal/config/config.go`,
> cùng tên/default env `DATABASE_CREDENTIALS_FILE` =
> `/vault/secrets/database-credentials` như mọi service khác — KHÔNG thêm
> biến env mới). `repo` khai báo với 1 interface ẩn danh gộp cả 3 port
> (`CredentialMetadataRepository`+`AuditRepository`+`TxRunner`) vì các
> usecase constructor cần các tổ hợp khác nhau của cùng 1 biến — khác
> `usage-service`'s 2-biến-tách-biệt (`repo`/`outboxStore`). Vault wiring
> (`secrets.NewClient()`, health check `"vault"`) giữ nguyên hoàn toàn
> không đổi, đúng yêu cầu không đụng logic mã hoá/Vault.
>
> **5. Test MySQL adapter** —
> `internal/adapter/mysql/repository_test.go`, build tag `integration`,
> dùng `common/testutil.StartMySQL` (không sửa package này). 7 test case:
> mirror 5 test đã có ở bản Postgres
> (`CreateGetUpdateStatus_RoundTrip`, `Get_NotFound`,
> `RunInTx_CommitsBothOnSuccess`, `RunInTx_RollsBackBothOnFailure`,
> `Append_RequiresExistingCredential`) cộng 2 test tenant-isolation mới ở
> mục 3.
>
> **6. CI workflow mới** —
> `.github/workflows/backend-go-credential-broker-service.yml`, copy
> khung `backend-go-usage-service.yml` 1:1, chỉ đổi path/service name.
> `python3 -c "import yaml; yaml.safe_load(...)"` xác nhận **YAML hợp lệ
> thật**.
>
> **7. Build/test thật đã chạy** (không giả định pass):
> - `go build ./...` → **PASS sạch**.
> - `go vet ./...` → **PASS sạch**.
> - `gofmt -l $(find . -name '*.go')` → **rỗng**.
> - `go test ./...` (unit) → **PASS** (`internal/domain`,
>   `internal/usecase`, `internal/adapter/vault` — test có sẵn, không
>   đổi; các package khác "no test files", không phải lỗi).
> - `go test -tags=integration ./internal/adapter/postgres/... -v`
>   (Docker + testcontainers thật, Postgres 16-alpine, xác nhận việc di
>   chuyển thư mục migration không phá gì) → **5/5 PASS thật**, 18.5s
>   tổng.
> - `go test -tags=integration ./internal/adapter/mysql/... -v` (Docker +
>   testcontainers thật, MySQL 8, `migrate` CLI thật chạy 3 file migration
>   MySQL) → **7/7 PASS thật**, 154.3s tổng. Không gặp lại race
>   `wait.ForLog`/`wait.ForSQL` đã biết ở TASK-BE-DB-005 — bản sửa đó đã
>   có sẵn từ trước trong `common/testutil/mysql.go`, task này chỉ tái sử
>   dụng, không sửa.
> - `go mod tidy` → thêm `github.com/go-sql-driver/mysql v1.10.1` vào
>   `go.mod` như dependency **direct** (không đánh dấu `// indirect`) —
>   `go get` ban đầu đánh dấu indirect, `go mod tidy` sửa lại đúng vì
>   package này được `cmd/server/main.go` và
>   `internal/adapter/mysql/repository.go` blank-import trực tiếp.
>
> **8. `detect_changes(scope: "all")` chạy trước khi kết thúc** — môi
> trường này có nhiều agent khác đang chạy song song trên các service
> khác của cùng đợt rollout (batch 1: `issue-status-sync`,
> `issue-tracking-service`, `annotation-service`; batch 2:
> `notification-service`) nên `detect_changes(scope: "all")` trả về toàn
> bộ 44 file thay đổi trong working tree (không scoped theo service) —
> đã lọc thủ công, xác nhận đúng 4 file/symbol của
> `credential-broker-service` xuất hiện đúng như dự kiến
> (`cmd/server/main.go`'s `run`, `internal/config/config.go`'s `Load`,
> `internal/adapter/postgres/repository_test.go`'s `setupRepository`,
> `README.md`), không có symbol lạ nào của service này bị đụng ngoài dự
> kiến. `affected_processes` chỉ có 2 process cross-community
> ("Run → IntEnv", "Run → Base") với `changed_steps` là chính `run`
> (bước 1) — kỳ vọng đúng, vì `run` là nơi duy nhất bị sửa logic thật.
> Không sửa file nào ngoài `backend-go/services/credential-broker-service/`
> + 1 file CI workflow mới + 2 doc mới + 1 dòng `ROLLOUT-TRACKING.md`.
>
> **Không có bug nào phát hiện ở `common/dbcapability`/
> `common/testutil`/`internal/adapter/vault`/`internal/adapter/grpc`
> trong quá trình này** — tất cả dùng lại y nguyên hoặc không đụng tới,
> đúng ràng buộc "flag don't fix" nếu có bug (không phát hiện bug nào cần
> flag).
>
> **Trạng thái cuối: ✅ DONE** — build/vet/unit/integration cả 2 dialect
> đều pass thật (12/12 test integration: 5 Postgres + 7 MySQL), không có
> lane CI nào biết trước sẽ đỏ. Giới hạn xác minh duy nhất: chưa thấy log
> GitHub Actions thật chạy (không có quyền mở PR trong phiên này) — chỉ
> xác nhận được cú pháp YAML + toàn bộ lệnh cục bộ.

---

## Mục tiêu

Áp dụng đúng pattern BE-DB-SOL-001/002 (đã implement cho `usage-service`)
ra `credential-broker-service` — service quản lý secrets/credentials,
theo `ROLLOUT-TRACKING.md`'s batch 1. Yêu cầu đặc biệt: **không được làm
yếu bất kỳ logic mã hoá-tại-rest hay access-control nào** khi viết adapter
MySQL — chỉ retarget SQL dialect cho tầng con trỏ/metadata, giữ nguyên mọi
lời gọi Vault Transit/KV.

## Files đã sửa/thêm

1. `backend-go/services/credential-broker-service/migrations/postgres/000{1,2,3}_*.{up,down}.sql` (MOVE, nội dung không đổi)
2. `backend-go/services/credential-broker-service/migrations/mysql/000{1,2,3}_*.{up,down}.sql` (MỚI)
3. `backend-go/services/credential-broker-service/internal/adapter/mysql/repository.go` (MỚI)
4. `backend-go/services/credential-broker-service/internal/adapter/mysql/repository_test.go` (MỚI, build tag `integration`, 7 test)
5. `backend-go/services/credential-broker-service/internal/adapter/postgres/repository_test.go` (MODIFY — `filepath.Abs` path thêm `/postgres`)
6. `backend-go/services/credential-broker-service/internal/config/config.go` (MODIFY — thêm `DatabaseCredentialsFile`)
7. `backend-go/services/credential-broker-service/cmd/server/main.go` (MODIFY — `switch caps.Dialect` + `toMySQLDriverDSN`)
8. `backend-go/services/credential-broker-service/go.mod`/`go.sum` (MODIFY — `go mod tidy` thêm `go-sql-driver/mysql` direct)
9. `backend-go/services/credential-broker-service/README.md` (MODIFY — 1 dòng lệnh `migrate` thêm `/postgres`)
10. `.github/workflows/backend-go-credential-broker-service.yml` (MỚI)
11. `specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-006-credential-broker-service-mysql-tidb-adapter.md` (MỚI)
12. `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md` (MODIFY — chỉ dòng `credential-broker-service`)

## Verify

```bash
cd backend-go/services/credential-broker-service
go build ./... && go vet ./...
gofmt -l $(find . -name '*.go')
go test ./...
go test -tags=integration ./internal/adapter/postgres/... -v
go test -tags=integration ./internal/adapter/mysql/... -v
python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-credential-broker-service.yml'))"
```

## gitnexus

`impact()` chạy trên cả 4 symbol sửa/tương tác (`run`, `Load`,
`CredentialMetadataRepository`, `Repository`) trước khi sửa — cả 4 **LOW**,
xem "Kết quả thực tế" mục 1 ở trên. `detect_changes(scope: "all")` chạy
trước khi coi task DONE — xem mục 8 (đã lọc scope thủ công do môi trường
có nhiều agent chạy song song trên các service khác).
