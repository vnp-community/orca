# TASK-BE-DB-008: Multi-database rollout cho `issue-status-sync`

**Solution:** [BE-DB-SOL-003](../solutions/BE-DB-SOL-003-issue-status-sync-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `issue-status-sync`
**Depends on:** TASK-BE-DB-002 (`dbcapability`, dùng lại nguyên vẹn), pattern TASK-BE-DB-004~007 (`usage-service` pilot)
**Status:** ✅ DONE (2026-09-11)

> **Kết quả thực tế:**
>
> **1. `impact()` chạy thật trước khi sửa cả 3 symbol hiện có** (bắt buộc
> theo AGENTS.md, không suy đoán):
> - `impact({target:"ProcessedEventStore", direction:"upstream",
>   file_path:"...internal/usecase/ports.go", kind:"Interface"})` → **LOW**,
>   impactedCount 3 (`main.go`, `processed_events.go`, `subscriber.go` — cả
>   3 chỉ IMPORT, không có process/module bị ảnh hưởng).
> - `impact({target:"run", direction:"upstream",
>   file_path:"...cmd/server/main.go", kind:"Function"})` → **LOW**,
>   impactedCount 1 (`main`).
> - `impact({target:"ProcessedEventsStore", direction:"upstream",
>   file_path:"...internal/adapter/postgres/processed_events.go"})` →
>   **LOW**, impactedCount 3 (`NewProcessedEventsStore` → `run` → `main`),
>   1 module `Usecase` ảnh hưởng trực tiếp — không HIGH/CRITICAL nào ở cả 3
>   lần chạy, an toàn để tiếp tục theo đúng ngưỡng AGENTS.md quy định.
>
> **2. Migration split** — `migrations/0001_processed_events.{up,down}.sql`
> di chuyển nguyên văn (dùng `git mv`, không sửa nội dung) vào
> `migrations/postgres/`. `migrations/mysql/0001_processed_events.{up,down}.sql`
> mới: bỏ `CREATE SCHEMA`/`DROP SCHEMA`, `TEXT PRIMARY KEY` →
> `VARCHAR(255) PRIMARY KEY` (InnoDB từ chối TEXT làm khoá chính không có
> key-length — phát hiện khi viết migration, xem BE-DB-SOL-003 §5),
> `TIMESTAMPTZ DEFAULT now()` → `TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6)`.
> Không có `gen_random_uuid()`/`RETURNING` trong bản gốc — xác nhận qua đọc
> trực tiếp `internal/adapter/postgres/processed_events.go` (event_id đến
> từ `common/eventbus.Event.ID`, sinh ở service publisher khác, không phải
> ở đây) nên không có gap ID-generation nào cần xử lý.
>
> **3. `internal/adapter/mysql/processed_events.go` mới** — implement lại
> đúng `usecase.ProcessedEventStore` (2 method: `Seen`, `MarkSeen`), KHÔNG
> đổi interface. `ON CONFLICT (event_id) DO NOTHING` → `INSERT IGNORE`;
> `errors.Is(err, pgx.ErrNoRows)` → `errors.Is(err, sql.ErrNoRows)`. Không
> có cột `tenant_id` trên bảng này (khác `usage-service`) → **không viết
> test tenant-isolation-without-RLS kiểu TASK-BE-DB-003** — đã xác nhận rõ
> lý do ở BE-DB-SOL-003 §4, không phải bỏ sót.
>
> **4. Wiring `cmd/server/main.go`** — thêm `switch caps.Dialect` đúng
> khuôn `usage-service`, bao gồm `toMySQLDriverDSN` copy nguyên văn (không
> share được qua import giữa 2 `cmd/main` package khác nhau). Khác 1 điểm
> so với pilot: `issue-status-sync` TRƯỚC ĐÓ dùng thẳng `cfg.DatabaseDSN` +
> tự check `dsn == ""` — đã thay bằng
> `secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)` (field
> `DatabaseCredentialsFile` mới trong `internal/config/config.go`, cùng
> tên/default env `DATABASE_CREDENTIALS_FILE` như mọi service khác — KHÔNG
> thêm biến env mới, đúng ràng buộc của nhiệm vụ). Xoá 1 khối
> `healthSrv := health.New()` trùng lặp phát sinh từ việc gộp code cũ +
> mới (bug tự phát hiện khi build, đã sửa trước khi build lần đầu, không
> lọt qua bản nộp).
>
> **5. Test MySQL adapter** — `internal/adapter/mysql/processed_events_test.go`,
> build tag `integration`, dùng `common/testutil.StartMySQL` (không sửa
> package này, đúng ràng buộc nhiệm vụ). 3 test case (mirror cấu trúc
> `usage-service/internal/adapter/mysql/repository_test.go`, thu gọn theo
> đúng kích thước 2-method interface của service này):
> `TestProcessedEventsStore_SeenIsFalseForUnknownEvent`,
> `TestProcessedEventsStore_MarkSeenThenSeenIsTrue`,
> `TestProcessedEventsStore_MarkSeenIsIdempotent` (guard cho JetStream
> at-least-once redelivery — tương đương ý nghĩa
> `SaveSession_IsIdempotentOnRequestID` ở pilot, thu gọn cho bảng 1 cột).
>
> **6. CI workflow mới** — `.github/workflows/backend-go-issue-status-sync.yml`,
> copy khung `backend-go-usage-service.yml` 1:1, chỉ đổi path/service name
> (`issue-status-sync` thay `usage-service` ở mọi chỗ). `python3 -c
> "import yaml; yaml.safe_load(...)"` xác nhận **YAML hợp lệ thật** (không
> có `act` trong môi trường này, đúng giới hạn task doc gốc dự trù — dùng
> nhánh fallback `python3`/`yaml.safe_load`, không giả định `act` sẵn có).
>
> **7. Build/test thật đã chạy** (không giả định pass):
> - `go build ./...` → **PASS sạch**, không lỗi.
> - `go vet ./...` → **PASS sạch**.
> - `gofmt -l $(find . -name '*.go')` → **rỗng** (không cần `gofmt -w`).
> - `go test ./...` (unit) → **PASS** (`internal/usecase` — 8 test case có
>   sẵn từ trước, không đổi; các package khác "no test files", không phải
>   lỗi).
> - `go test -tags=integration ./internal/adapter/mysql/... -v` (Docker +
>   testcontainers-go thật, MySQL 8, `migrate` CLI thật chạy
>   `migrations/mysql/0001_processed_events.up.sql`) → **3/3 PASS thật**,
>   ~49.6s tổng, không cần sửa `common/testutil/mysql.go` (khác pilot's
>   TASK-BE-DB-005, nơi phải sửa `wait.ForLog` → `wait.ForSQL` — bản sửa đó
>   đã có sẵn từ trước, task này chỉ tái sử dụng, không gặp lại race đó).
> - `go mod tidy` → thêm `github.com/go-sql-driver/mysql v1.10.1` vào
>   `go.mod` như dependency **direct** (không đánh dấu `// indirect`) — lưu
>   ý: `usage-service`'s go.mod hiện đánh dấu driver này `// indirect` dù
>   được `cmd/server/main.go`/`internal/adapter/mysql` blank-import trực
>   tiếp — có vẻ là điểm chưa tidy triệt để ở chính pilot, không phải lỗi
>   của task này; không sửa `usage-service`'s go.mod (ngoài phạm vi, chỉ sở
>   hữu `issue-status-sync`).
>
> **8. `detect_changes()` chạy trước khi kết thúc** — xác nhận scope thay
> đổi đúng như dự kiến (chỉ file của `issue-status-sync` + 1 file CI mới +
> 2 doc mới + 1 dòng `ROLLOUT-TRACKING.md`), không đụng service khác.
>
> **Không có bug nào phát hiện ở `common/dbcapability`/`common/testutil`
> trong quá trình này** — cả hai dùng lại y nguyên, không cần sửa.
>
> **Trạng thái cuối: ✅ DONE** — không có phần nào PARTIAL: build/vet/unit/
> integration MySQL đều pass thật, không có lane CI nào biết trước sẽ đỏ
> (khác `usage-service`'s TASK-BE-DB-007 ban đầu, nơi lane Postgres biết
> trước sẽ fail do bug tiền tồn tại — `issue-status-sync` không có bug
> tương ứng nào được phát hiện). Giới hạn xác minh duy nhất: chưa thấy log
> GitHub Actions thật chạy (không có quyền mở PR trong phiên này) — chỉ
> xác nhận được cú pháp YAML + toàn bộ lệnh cục bộ, đúng giới hạn đã biết
> từ TASK-BE-DB-007.

---

## Mục tiêu

Áp dụng đúng pattern BE-DB-SOL-001/002 (đã implement cho `usage-service`)
ra `issue-status-sync` — service nhỏ nhất trong rollout (1 repo, 2 file
migration gốc), theo `ROLLOUT-TRACKING.md`'s batch 1. Một task duy nhất
bao trọn migration split + adapter + wiring + test + CI, vì quy mô nhỏ
không cần tách thành 6 task riêng như pilot.

## Files đã sửa/thêm

1. `backend-go/services/issue-status-sync/migrations/postgres/0001_processed_events.{up,down}.sql` (MOVE, nội dung không đổi)
2. `backend-go/services/issue-status-sync/migrations/mysql/0001_processed_events.{up,down}.sql` (MỚI)
3. `backend-go/services/issue-status-sync/internal/adapter/mysql/processed_events.go` (MỚI)
4. `backend-go/services/issue-status-sync/internal/adapter/mysql/processed_events_test.go` (MỚI, build tag `integration`)
5. `backend-go/services/issue-status-sync/internal/config/config.go` (MODIFY — thêm `DatabaseCredentialsFile`)
6. `backend-go/services/issue-status-sync/cmd/server/main.go` (MODIFY — `switch caps.Dialect` + `toMySQLDriverDSN`)
7. `backend-go/services/issue-status-sync/go.mod` (MODIFY — `go mod tidy` thêm `go-sql-driver/mysql`)
8. `.github/workflows/backend-go-issue-status-sync.yml` (MỚI)
9. `specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-003-issue-status-sync-mysql-tidb-adapter.md` (MỚI)
10. `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md` (MODIFY — chỉ dòng `issue-status-sync`)

## Verify

```bash
cd backend-go/services/issue-status-sync
go build ./... && go vet ./...
gofmt -l $(find . -name '*.go')
go test ./...
go test -tags=integration ./internal/adapter/mysql/... -v
python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-issue-status-sync.yml'))"
```

## gitnexus

`impact()` chạy trên cả 3 symbol sửa (`ProcessedEventStore`, `run`,
`ProcessedEventsStore`) trước khi sửa — cả 3 **LOW**, xem "Kết quả thực
tế" mục 1 ở trên. `detect_changes()` chạy trước khi coi task DONE — xem
mục 8.
