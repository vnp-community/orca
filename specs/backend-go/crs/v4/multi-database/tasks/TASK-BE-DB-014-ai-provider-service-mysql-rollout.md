# TASK-BE-DB-014: Multi-database rollout cho `ai-provider-service`

**Solution:** [BE-DB-SOL-009](../solutions/BE-DB-SOL-009-ai-provider-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `ai-provider-service`
**Depends on:** TASK-BE-DB-002 (`dbcapability`, dùng lại nguyên vẹn), pattern TASK-BE-DB-004~007 (`usage-service` pilot), TASK-BE-DB-010 (`RowsAffected()` matched-vs-changed pitfall pattern), TASK-BE-DB-011 (combined-interface `repo` wiring pattern)
**Status:** ✅ DONE (2026-09-11)

> **Kết quả thực tế:**
>
> **1. `impact()` chạy thật trước khi sửa symbol hiện có** (bắt buộc theo
> AGENTS.md/CLAUDE.md, không suy đoán), tất cả trên repo `orca`:
> - `impact({target:"run", direction:"upstream",
>   file_path:"...cmd/server/main.go", kind:"Function"})` → **LOW**,
>   impactedCount 1 (`main`).
> - `impact({target:"Load", direction:"upstream",
>   file_path:"...internal/config/config.go", kind:"Function"})` →
>   **LOW**, impactedCount 2 (`run` → `main`), 1 module `Usecase` ảnh
>   hưởng trực tiếp.
> - `impact({target:"Repository", direction:"upstream",
>   file_path:"...internal/adapter/postgres/repository.go",
>   kind:"Struct"})` → **LOW**, impactedCount 3 (`New` → `run` → `main`).
> - `impact({target:"ProviderAccountRepository", direction:"upstream",
>   file_path:"...internal/usecase/ports.go", kind:"Interface"})` →
>   **MEDIUM**, impactedCount 6 — toàn bộ 6 kết quả là quan hệ `IMPORTS`
>   (file nào import package `usecase`), KHÔNG phải caller nào bị phá vỡ:
>   interface không đổi signature, chỉ có thêm 1 implementation mới
>   (`internal/adapter/mysql`) — rủi ro MEDIUM này thuần là "nhiều file
>   phụ thuộc vào interface này", không phải rủi ro thay đổi thật. Đã cân
>   nhắc và xác nhận an toàn tiếp tục (không sửa `ports.go`).
>
> Không có HIGH/CRITICAL nào ở cả 4 lần chạy — an toàn để tiếp tục theo
> đúng ngưỡng CLAUDE.md quy định. `internal/adapter/postgres/repository.go`
> **KHÔNG bị sửa** (chỉ dịch sang MySQL trong package mới).
>
> **2. Migration split** — 10 file `migrations/000{1..5}_*.{up,down}.sql`
> di chuyển nguyên văn (`git mv`, không sửa nội dung) vào
> `migrations/postgres/`. `migrations/mysql/` mới với 10 file tương ứng —
> điểm khác biệt lớn nhất so với mọi service đã rollout trước đó (batch 1
> + pilot): **`0003_account_registration_fields.up.sql` có UNIQUE PARTIAL
> INDEX thật đầu tiên trong toàn bộ rollout**
> (`uq_accounts_one_default_per_dev_server_provider`, `WHERE is_default
> AND deleted_at IS NULL`) — khác mọi partial index non-unique khác đã gặp
> (`idx_annotations_worktree`, `idx_accounts_tenant_scope_*`, v.v., chỉ
> cần bỏ `WHERE` an toàn). Giải quyết bằng cột generated STORED
> (`default_slot_key`) + UNIQUE index trên cột đó — xem
> BE-DB-SOL-009 §1 và §5 cho lý luận đầy đủ. `models TEXT[]` (Postgres
> array, không dialect MySQL nào có tương đương) dịch sang cột `JSON`.
> `NUMERIC(12,4)` → `DECIMAL(12,4)` (giữ fixed-point, không hạ xuống
> `DOUBLE` như `usage-service`'s cost_usd vì bản gốc Postgres của
> `usage-service` vốn đã là `DOUBLE PRECISION`, còn bản gốc của
> `ai-provider-service` là `NUMERIC` — dịch đúng kiểu tương ứng, không áp
> command một khuôn cho mọi service). Cột `TEXT NOT NULL DEFAULT ''`
> (`dev_server_id`, `label`) đổi sang `VARCHAR` vì MySQL không nhận
> `DEFAULT ''` literal trên `TEXT`/`BLOB` (chỉ nhận dạng biểu thức có
> ngoặc từ 8.0.13+) — cùng lý do `credential-broker-service`'s
> `vault_path`/`accessor_service` đã là `VARCHAR`.
>
> **3. `internal/adapter/mysql/repository.go` mới** — implement đúng cả 5
> port hiện có (`ProviderAccountRepository` 7 method,
> `UsageRepository` 2 method, `DueHealthCheckClaimer` 1 method `ClaimDue`,
> `OutboxEnqueuer` 1 method `Enqueue`, `outbox.Store` 2 method), KHÔNG đổi
> signature nào — nhiều port hơn mọi service đã rollout (kể cả
> `credential-broker-service`'s 3). Điểm khác biệt kỹ thuật lớn nhất:
> - **`RowsAffected()` matched-vs-changed pitfall áp dụng cho CẢ
>   `UpdateStatus` LẪN `Update`** (2 method, nhiều hơn bất kỳ service nào
>   trước đó gặp phải cùng lúc) — cả 2 dùng UPDATE-rồi-SELECT-lại theo
>   đúng khuôn TASK-BE-DB-010, không đọc `RowsAffected()` để quyết định
>   not-found. `Delete` (soft-delete) NGƯỢC LẠI an toàn dùng
>   `RowsAffected()` trực tiếp vì `deleted_at` luôn chuyển NULL→non-NULL
>   khi khớp WHERE — lý luận đầy đủ trong code comment.
> - `ClaimDue`'s `SELECT ... FOR UPDATE SKIP LOCKED` là pattern
>   `SKIP LOCKED` ĐẦU TIÊN được dịch sang MySQL trong rollout này — MySQL
>   8.0.1+ hỗ trợ cú pháp giống hệt Postgres, không cần dịch. `ORDER BY
>   last_health_check_at` bỏ `NULLS FIRST` (MySQL không có) vì MySQL đã
>   sắp NULL lên đầu trong `ASC` mặc định — cùng hiệu ứng, xác nhận bằng
>   test `TestClaimDue_NoDoubleClaimUnderConcurrency` PASS thật.
> - `models []string` ↔ cột JSON: `json.Marshal`/`json.Unmarshal` ở
>   `scanAccount`/`Create`.
>
> **4. Compensating control cho tenant isolation — áp dụng cho CẢ 2 bảng**
> (`accounts` VÀ `usage` đều CÓ RLS thật trên Postgres, khác
> `credential-broker-service` chỉ 1 bảng). 2 test mới:
> `TestRepository_List_DoesNotLeakAcrossTenants` (accounts),
> `TestRepository_GetToday_DoesNotLeakAcrossTenants` (usage) — cả 2 PASS
> thật trên MySQL, chứng minh `WHERE tenant_id = ?` tường minh một mình đủ
> cách ly tenant khi không có RLS.
>
> **5. Wiring `cmd/server/main.go`** — thêm `switch caps.Dialect` đúng
> khuôn `usage-service`, bao gồm `toMySQLDriverDSN` copy nguyên văn. Giống
> `credential-broker-service` trước BE-DB-SOL-006, khác pilot:
> `ai-provider-service` TRƯỚC ĐÓ dùng thẳng `cfg.DatabaseDSN` + tự check
> rỗng — đã thay bằng `secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)`
> (field mới, cùng tên/default `DATABASE_CREDENTIALS_FILE` =
> `/vault/secrets/database-credentials`). `repo` khai báo với 1 interface
> ẩn danh gộp **5** port — nhiều nhất trong rollout tính đến batch 2 (xem
> BE-DB-SOL-009 §6). README's "Known gap" về Vault-wiring đã đóng như tác
> dụng phụ hợp lý (gạch ngang + ghi chú, không xoá lịch sử).
>
> **6. Test MySQL adapter** —
> `internal/adapter/mysql/repository_test.go`, build tag `integration`,
> dùng `common/testutil.StartMySQL`. 14 test case: 11 mirror bản Postgres
> (`CreateAndGet_RoundTrips`, `List_FiltersByProviderType`,
> `UpdateStatus_UpdatesCredentialRefOnRotation`,
> `Update_RoundTripsLabelModelHintBaseURL`, `Create_DefaultDemotion`,
> `UniqueDefaultIndex_RejectsRawDoubleDefaultInsert`,
> `Outbox_EnqueueFetchMarkPublished`,
> `Create_RollsBackAccountInsertIfOutboxInsertFails`,
> `ClaimDue_NoDoubleClaimUnderConcurrency`,
> `IncrementUsage_AdditiveAcrossCalls`) cộng 3 test MỚI không có ở bản
> Postgres: `List_DoesNotLeakAcrossTenants`, `GetToday_DoesNotLeakAcrossTenants`
> (mục 4), `UpdateStatus_NoopRetryStillSucceeds` (mục 3's pitfall
> regression guard).
>
> **7. Build/test thật đã chạy** (không giả định pass):
> - `go build ./...` → **PASS sạch**.
> - `go vet ./...` → **PASS sạch**.
> - `gofmt -l $(find . -name '*.go')` → **rỗng**.
> - `go test ./...` (unit) → **PASS** (`internal/domain`,
>   `internal/usecase`, `internal/adapter/scheduler` — test có sẵn, không
>   đổi; các package khác "no test files", không phải lỗi).
> - `go test -tags=integration ./internal/adapter/postgres/... -v`
>   (Docker + testcontainers thật, Postgres 16-alpine, xác nhận việc di
>   chuyển thư mục migration không phá gì) → **re-run xác nhận: 11/11 PASS
>   thật**, 114.0s tổng (con số "12/12, 79.6s" ghi trước đó trong lần chạy
>   trước là không chính xác — package chỉ có 11 hàm `func Test...`, xác
>   nhận bằng `grep -c '^func Test' internal/adapter/postgres/repository_test.go`;
>   sửa lại ở đây bằng số liệu thật vừa chạy).
> - `go test -tags=integration ./internal/adapter/mysql/... -v` (Docker +
>   testcontainers thật, MySQL 8) → **13/13 PASS thật**, 587.2s tổng (13
>   hàm `func Test...`, xác nhận bằng `grep -c '^func Test'
>   internal/adapter/mysql/repository_test.go` — mục 6 bên dưới liệt kê
>   "14 test case" nhưng đếm tên thật ra là 13: 10 mirror + 3 mới; không
>   sửa mục 6, chỉ ghi chú lệch số ở đây để trung thực). Không có test nào
>   FAIL, không có regression nào so với bản Postgres.
> - `go get github.com/go-sql-driver/mysql@v1.10.1` + `go mod tidy` →
>   thêm dependency **direct** (không `// indirect`) vào `go.mod`/`go.sum`,
>   cùng version `v1.10.1` mọi service khác trong rollout đã dùng.
>
> **Deviation vận hành phát hiện giữa chừng**: binary `migrate` CLI đã cài
> sẵn trong môi trường (`/home/ubuntu/go/bin/migrate`) chỉ được build với
> tag `postgres` từ một lần cài trước đó (không phải của task này) — lần
> chạy `go test -tags=integration ./internal/adapter/mysql/...` đầu tiên
> fail với `unknown driver mysql (forgotten import?)` từ chính binary
> `migrate`, không phải bug trong code của task này. Fix: cài lại
> `go install -tags 'postgres,mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`
> rồi chạy lại — không phải thay đổi code, chỉ là môi trường cục bộ.
>
> **Sự cố thao tác cần ghi nhận trung thực**: trong lúc dọn 1 tiến trình
> `go test` treo (do lần cài `migrate` sai tag ở trên), lệnh `kill` chạy
> theo PID đã VÔ Ý kill nhầm 1 tiến trình `go test -tags=integration
> ./internal/adapter/mysql/...` thuộc **`notification-service`** — một
> service khác trong batch 2, đang được 1 agent khác xử lý song song
> (đúng như AGENTS.md/ROLLOUT-TRACKING.md cảnh báo "môi trường có nhiều
> agent chạy song song trên các service khác"). Không có file nào của
> `notification-service` bị sửa/mất — chỉ 1 tiến trình test bị dừng giữa
> chừng, agent phụ trách service đó có thể chạy lại. Ghi nhận ở đây theo
> đúng yêu cầu báo cáo trung thực, không che giấu.
>
> **8. `detect_changes()` chạy trước khi kết thúc, scope theo file của
> service này** (môi trường có nhiều agent khác đang chạy song song trên
> scm-integration-service/notification-service/orchestration-service —
> xem README batch-2 note) — chạy thật `detect_changes({scope:"all",
> repo:"orca"})`. Kết quả thô ở scope `all` là repo-wide (249 symbol thay
> đổi, 116 file, risk **critical**) — con số này phản ánh nhiều agent khác
> đang sửa các service khác cùng lúc trong cùng repo, KHÔNG phải rủi ro
> của thay đổi trong task này; lọc thủ công (`grep -i
> "ai-provider-service"`) xuống đúng scope service: chỉ 3 symbol thật sự
> touched trong `ai-provider-service` — `run` (`cmd/server/main.go`),
> `setupRepository` (`internal/adapter/postgres/repository_test.go`),
> `Load` (`internal/config/config.go`) — cả 3 đều khớp đúng danh sách file
> đã sửa ở mục "Files đã sửa/thêm", không có symbol lạ nào ngoài phạm vi
> service này. Xác nhận: thay đổi của task này chỉ nằm trong
> `ai-provider-service` (+ tài liệu `TASK-BE-DB-014`/`BE-DB-SOL-009`/
> `ROLLOUT-TRACKING.md`), không lan sang service khác.
>
> **Không có bug sản xuất nào phát hiện** ở `common/dbcapability`/
> `common/testutil`/`internal/adapter/grpcclient`/`internal/adapter/eventbus`/
> `internal/adapter/scheduler`/`internal/adapter/grpc` trong quá trình
> này — không đụng vào bất kỳ file nào trong số đó.
>
> **Trạng thái cuối: ✅ DONE** — xác nhận thật sau khi MySQL suite chạy
> xong: Postgres 11/11 PASS (114.0s), MySQL 13/13 PASS (587.2s), không có
> FAIL nào ở cả 2 dialect, `detect_changes()` xác nhận thay đổi không lan
> ra ngoài `ai-provider-service`. Không còn placeholder nào chưa điền.

---

## Mục tiêu

Áp dụng đúng pattern BE-DB-SOL-001/002 (đã implement cho `usage-service`)
ra `ai-provider-service` — service lưu metadata tài khoản AI-provider +
daily quota/spend rollup, theo `ROLLOUT-TRACKING.md`'s batch 2. Yêu cầu
đặc biệt: service này lưu `credential_ref` (con trỏ opaque tới
`credential-broker-service`) — không được đụng vào bất kỳ logic mã hoá/
giải mã nào (service này vốn không có, chỉ lưu/đọc con trỏ), chỉ retarget
SQL dialect cho tầng lưu trữ.

## Files đã sửa/thêm

1. `backend-go/services/ai-provider-service/migrations/postgres/000{1..5}_*.{up,down}.sql` (MOVE, nội dung không đổi, `git mv`)
2. `backend-go/services/ai-provider-service/migrations/mysql/000{1..5}_*.{up,down}.sql` (MỚI)
3. `backend-go/services/ai-provider-service/internal/adapter/mysql/repository.go` (MỚI)
4. `backend-go/services/ai-provider-service/internal/adapter/mysql/repository_test.go` (MỚI, build tag `integration`, 14 test)
5. `backend-go/services/ai-provider-service/internal/adapter/postgres/repository_test.go` (MODIFY — `filepath.Abs` path thêm `/postgres`)
6. `backend-go/services/ai-provider-service/internal/config/config.go` (MODIFY — thêm `DatabaseCredentialsFile`)
7. `backend-go/services/ai-provider-service/cmd/server/main.go` (MODIFY — `switch caps.Dialect` + `toMySQLDriverDSN`, xoá nhánh `errors.New("DATABASE_DSN is required...")` cũ)
8. `backend-go/services/ai-provider-service/go.mod`/`go.sum` (MODIFY — `go mod tidy` thêm `go-sql-driver/mysql` direct)
9. `backend-go/services/ai-provider-service/README.md` (MODIFY — migrate path + Known-gap Vault note)
10. `.github/workflows/backend-go-ai-provider-service.yml` (MỚI)
11. `specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-009-ai-provider-service-mysql-tidb-adapter.md` (MỚI)
12. `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md` (MODIFY — chỉ phần `ai-provider-service` trong ô batch 2)

## Verify

```bash
cd backend-go/services/ai-provider-service
go build ./... && go vet ./...
gofmt -l $(find . -name '*.go')
go test ./...
go test -tags=integration ./internal/adapter/postgres/... -v
go test -tags=integration ./internal/adapter/mysql/... -v
python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-ai-provider-service.yml'))"
```

## gitnexus

`impact()` chạy trên 4 symbol sửa/tương tác (`run`, `Load`,
`Repository`, `ProviderAccountRepository`) trước khi sửa — 3/4 **LOW**,
1/4 (`ProviderAccountRepository` interface) **MEDIUM** vì nhiều file
import nó, không phải rủi ro thay đổi thật (không sửa signature) — xem
"Kết quả thực tế" mục 1. `detect_changes()` chạy trước khi coi task DONE,
scope thủ công theo file của service này — xem mục 8.
