# TASK-BE-DB-015: Multi-database rollout cho `orchestration-service`

**Solution:** [BE-DB-SOL-010](../solutions/BE-DB-SOL-010-orchestration-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `orchestration-service` (batch 2 của rollout 15 service)
**Depends on:** TASK-BE-DB-002 (`dbcapability`, dùng lại nguyên vẹn), pattern TASK-BE-DB-004~007 (`usage-service` pilot)
**Status:** ✅ DONE — 4 bug thật phát hiện tổng cộng, cả 4 đã sửa: 2 bug
lúc viết adapter MySQL (không phải pre-existing-out-of-scope) + **2 bug
tiền tồn tại KHÁC** ở `internal/adapter/postgres` (ban đầu ghi "ngoài phạm
vi, không sửa" — sau đó được yêu cầu trực tiếp sửa luôn, xem "Kết quả thực
tế" §Cập nhật, 2026-09-11)

## Mục tiêu

Nhân rộng pattern CR-DB-002/CR-DB-003 (`dbcapability` + adapter MySQL/TiDB
song song + migration dialect-safe + CI matrix) từ pilot `usage-service`
sang `orchestration-service` — service trong batch 2 của rollout 15 service
(`specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md`).

`orchestration-service` có **1 repository theo audit gốc** nhưng thực chất
implement **4 interface riêng biệt** (`OrchestrationTaskRepository`,
`DispatchContextRepository`, `GateRepository`, `CoordinatorRunRepository`)
trên 1 struct — khác pilot's 1 interface đơn — cộng `common/outbox.Store`
(bảng `orchestration.outbox_events`, service này coordinate workflow
execution nên có outbox pattern cho NATS publishing, đúng như gợi ý trong
context task gốc). Xem BE-DB-SOL-010 §1 cho phân tích đầy đủ.

## Files đã sửa

1. `backend-go/services/orchestration-service/migrations/postgres/*.sql` (MỚI — `git mv` nguyên văn từ `migrations/`, 12 file)
2. `backend-go/services/orchestration-service/migrations/mysql/*.sql` (MỚI — 12 file, dialect-safe)
3. `backend-go/services/orchestration-service/internal/adapter/mysql/repository.go` (MỚI)
4. `backend-go/services/orchestration-service/internal/adapter/mysql/repository_test.go` (MỚI)
5. `backend-go/services/orchestration-service/internal/config/config.go` (MODIFY — thêm `DatabaseCredentialsFile`)
6. `backend-go/services/orchestration-service/cmd/server/main.go` (MODIFY — dialect switch, `secrets.DatabaseCredentialsFromFile` thay `cfg.DatabaseDSN` trực tiếp, interface `repository` cục bộ gộp 4 port + `outbox.Store`)
7. `backend-go/services/orchestration-service/internal/adapter/postgres/repository_test.go` (MODIFY — `migrationsPath` trỏ `migrations/postgres`)
8. `backend-go/services/orchestration-service/README.md` (MODIFY — cập nhật đường dẫn migration, mục "Known gaps" đóng gap `common/secrets`)
9. `.github/workflows/backend-go-orchestration-service.yml` (MỚI)
10. `backend-go/services/orchestration-service/internal/adapter/postgres/repository.go` (MODIFY, 2026-09-11 — vá 2 bug tiền tồn tại: `ResolveGate` NULL-scan, `RecordHeartbeat` thiếu UUID validation)

## Test cases

`internal/adapter/mysql/repository_test.go` mirror toàn bộ 15 test case
của `internal/adapter/postgres/repository_test.go` (tên test giữ nguyên,
build tag `integration`, `common/testutil.StartMySQL`) cộng 2 test bổ sung
cho `common/outbox.Store` (`TestRepository_Outbox_EnqueueFetchMarkPublished`,
`TestRepository_MarkPublished_EmptyIDsIsNoop`) — 17 test tổng cộng.

## Verify

```bash
cd backend-go/services/orchestration-service
GOWORK=off go build ./... && GOWORK=off go vet ./...
gofmt -l internal/adapter/mysql/*.go cmd/server/main.go internal/config/config.go
GOWORK=off go test ./...
GOWORK=off go test -tags=integration ./internal/adapter/mysql/... -v
GOWORK=off go test -tags=integration ./internal/adapter/postgres/... -v
python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-orchestration-service.yml'))"
```

## gitnexus

`impact()` chạy thật cho `New` (`adapter/postgres/repository.go`), `run`
(`cmd/server/main.go`), và cả 4 interface `ports.go`
(`OrchestrationTaskRepository`/`DispatchContextRepository`/
`GateRepository`/`CoordinatorRunRepository`) trước khi sửa — 6 lần gọi,
tất cả **LOW** risk, chi tiết đầy đủ ở BE-DB-SOL-010 §2. Không HIGH/CRITICAL
nào. `detect_changes()` chạy trước khi hoàn tất task — xem mục "Kết quả
thực tế" bên dưới.

---

## Kết quả thực tế (2026-09-11)

Chạy thật, foreground, không mock — theo đúng lệnh ở mục "Verify" trên
(`GOWORK=off` thay bằng cwd trong module, tương đương):

```
cd backend-go/services/orchestration-service
go build ./...                                        # OK, không lỗi
go vet ./...                                           # OK, không lỗi
gofmt -l .                                             # rỗng — toàn bộ file đã format
go test ./...                                          # OK — mọi package có test đều PASS (cached),
                                                        #   các package adapter/mysql|postgres|cmd/server|
                                                        #   grpc|grpcclient|config không có unit test riêng
go test -tags=integration ./internal/adapter/postgres/... -v -timeout=15m
                                                        # 12/15 PASS, 3 FAIL — 167.7s
go test -tags=integration ./internal/adapter/mysql/...    -v -timeout=20m
                                                        # 17/17 PASS — chạy 2 lần độc lập để xác nhận
                                                        #   không flaky (775.5s và 633.0s, cả 2 exit 0)
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/backend-go-orchestration-service.yml'))"
                                                        # YAML hợp lệ
```

**MySQL adapter: 17/17 PASS**, tái lặp 2 lần (không flaky) — toàn bộ 15
test mirror từ bản Postgres cộng 2 test outbox mới
(`TestRepository_Outbox_EnqueueFetchMarkPublished`,
`TestRepository_MarkPublished_EmptyIDsIsNoop`). Không bug nào phát hiện
trong `internal/adapter/mysql`.

**Postgres baseline: 12/15 PASS, 3 FAIL** — xác nhận **2 bug tiền tồn tại
thật** trong `internal/adapter/postgres/repository.go`, độc lập với rollout
MySQL này (file test Postgres chỉ đổi `migrationsPath` sang
`migrations/postgres`, xác nhận bằng `git diff`; bug tồn tại trước rollout,
không phải do thay đổi trong task này gây ra):

1. **`ResolveGate` scan lỗi khi `resolution` còn NULL** (gate `pending`,
   chưa resolve lần nào) — dòng
   `internal/adapter/postgres/repository.go:694` scan trực tiếp
   `&gate.Resolution` (kiểu `string`, không phải `*string`) từ cột
   `g.resolution` nullable → panic-safe nhưng lỗi runtime
   `can't scan into dest[7] (col: resolution): cannot scan NULL into *string`
   ngay ở bước SELECT đầu tiên trong transaction, trước khi kịp UPDATE. Làm
   fail 2 test: `TestRepository_ResolveGate_CannotBeResolvedTwice` (gọi
   `ResolveGate` lần đầu trên gate pending) và
   `TestRepository_ListPending_ReturnsOnlyPendingRows` (dùng `ResolveGate`
   để setup 1 gate đã resolved).
2. **`RecordHeartbeat` không validate UUID trước khi query** —
   `internal/adapter/postgres/repository.go:1108-1124` truyền thẳng
   `dispatchContextID` (chuỗi bất kỳ từ caller) vào
   `WHERE id = $1` trên cột kiểu `uuid` mà không catch lỗi driver; với ID
   không phải UUID hợp lệ (`"does-not-exist"`), Postgres trả
   `SQLSTATE 22P02 invalid input syntax for type uuid`, không phải
   `pgx.ErrNoRows` → hàm trả lỗi driver thô thay vì
   `usecase.ErrDispatchContextNotFound` như test
   `TestRepository_RecordHeartbeat` kỳ vọng.

Cả 2 bug đều **ngoài phạm vi task này** (task chỉ nhân rộng adapter MySQL
song song, không sửa lại `internal/adapter/postgres` hiện có) — không sửa.
Adapter MySQL mới **không có bug tương ứng**: `ResolveGate`'s MySQL variant
dùng `nullableString`/scan qua `*string` đúng cho cột nullable (xem
BE-DB-SOL-010 §4), và `RecordHeartbeat`'s MySQL variant cũng không gặp lỗi
này trong test tương ứng (PASS) — vì `database/sql`'s driver MySQL không
strict-validate định dạng UUID ở tầng type hệt Postgres (chuỗi bất kỳ hợp
lệ như 1 `CHAR(36)`, đơn giản không match → `RowsAffected()==0` →
`ErrDispatchContextNotFound` đúng luồng).

Trong quá trình rollout (trước khi tới bước verify cuối này), 2 bug THẬT
khác đã được phát hiện và sửa trong phạm vi (chi tiết ở commit/diff, không
lặp lại ở đây) — không phải 2 bug tiền tồn tại nói trên.

**Kết luận ban đầu: ✅ DONE cho phạm vi rollout MySQL** (build/vet/fmt sạch,
17/17 MySQL integration test pass tái lặp được, CI YAML hợp lệ). 2 bug
Postgres tiền tồn tại nêu trên ban đầu được ghi nhận là gap đã biết, ngoài
phạm vi CR-DB-002/CR-DB-003, không sửa.

## Cập nhật (2026-09-11) — 2 bug Postgres tiền tồn tại đã được yêu cầu sửa

Theo yêu cầu trực tiếp (sau khi phát hiện + vá bug tương tự ở
`task-service`, xem TASK-BE-DB-020 mục 7), 2 bug tiền tồn tại ở
`internal/adapter/postgres` nêu trên đã được sửa, mở rộng phạm vi task
này ngoài "chỉ MySQL rollout":

1. **`ResolveGate`** (`internal/adapter/postgres/repository.go`) —
   `impact()` xác nhận LOW risk (0 caller nội bộ khác) trước khi sửa. Sửa
   bằng cách scan cột `resolution` nullable vào biến cục bộ `*string`
   (`existingResolution`) rồi gán có điều kiện vào `gate.Resolution`, thay
   vì scan trực tiếp vào field `string` không nullable.
2. **`RecordHeartbeat`** (`internal/adapter/postgres/repository.go`) —
   `impact()` xác nhận LOW risk trước khi sửa. Thêm `uuid.Parse(dispatchContextID)`
   validate trước khi query — id không hợp lệ trả `ErrDispatchContextNotFound`
   ngay (nhất quán với ý nghĩa "không tồn tại" thay vì lộ lỗi driver thô),
   dùng lại package `github.com/google/uuid` đã import sẵn trong file.

**Verify lại thật**: `go build ./...`/`go vet ./...`/`gofmt -l .` sạch; `go
test -tags=integration ./internal/adapter/postgres/... -v -timeout=15m` →
**15/15 PASS** (52.8s, tăng từ 12/15). MySQL adapter không đổi (không cần
chạy lại — không chạm file nào trong `internal/adapter/mysql`).

**Kết luận cuối: ✅ DONE — không còn bug/gap nào (đã biết) trong phạm vi
`orchestration-service`.** 15/15 Postgres + 17/17 MySQL integration test
PASS thật (32/32 tổng), build/vet/fmt sạch, CI YAML hợp lệ.
