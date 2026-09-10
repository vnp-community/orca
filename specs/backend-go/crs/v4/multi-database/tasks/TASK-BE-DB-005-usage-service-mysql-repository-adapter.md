# TASK-BE-DB-005: `internal/adapter/mysql/repository.go` cho `usage-service`

**Solution:** BE-DB-SOL-002 §2-3 | **CR:** CR-DB-003
**Service:** `usage-service`
**Depends on:** TASK-BE-DB-002 (`dbcapability`), TASK-BE-DB-004 (migration `mysql/` phải tồn tại để test được)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** `impact()` đã chạy thật trước khi thêm implementation
> mới — `mcp__gitnexus__impact({target:"Repository", direction:"upstream",
> file_path:"backend-go/services/usage-service/internal/usecase/ports.go",
> kind:"Interface"})` → risk **LOW**, impactedCount 2 (khớp đúng số liệu
> task doc đã ghi: `main.go`, `adapter/grpc/server.go`) — an toàn để thêm 1
> implementation song song, không đổi interface.
>
> Đã tạo đúng `internal/adapter/mysql/repository.go` dịch 1:1 cả 6 method
> (`SaveSession`, `FetchUnpublished`, `MarkPublished`, `GetDailyRollup`,
> `ListSessions`, `RecomputeDailyRollup`) từ bản Postgres, theo đúng mapping
> đã chỉ định: `ON CONFLICT ... DO NOTHING` → `INSERT IGNORE` + kiểm tra
> `RowsAffected()==0`; `ON CONFLICT ... DO UPDATE SET x = t.x + EXCLUDED.x`
> → `INSERT ... ON DUPLICATE KEY UPDATE x = x + VALUES(x)`; `id = ANY($1)`
> → `IN (?,...)` động theo `len(ids)` với guard `len(ids)==0` giữ nguyên;
> `errors.Is(err, pgx.ErrNoRows)` → `errors.Is(err, sql.ErrNoRows)`. Mọi
> `WHERE tenant_id = ?` giữ nguyên vị trí/logic hệt bản Postgres — không
> nới lỏng tenant-scoping.
>
> **Lệch quan trọng phát hiện khi implement (không có trong bản phác thảo
> BE-DB-SOL-002 §3)**:
> 1. `started_at`/`ended_at` scan cần `sql.NullTime` (không phải
>    `time.Time` trực tiếp như bản Postgres) vì `ended_at` có thể NULL —
>    và scan `TIMESTAMP` MySQL vào `time.Time`/`sql.NullTime` đòi hỏi
>    DSN có `parseTime=true`, nếu không driver trả về `[]byte` thô. Đã
>    thêm `parseTime=true` vào driver DSN ở test setup, và ghi chú lại để
>    TASK-BE-DB-006's `toMySQLDriverDSN` cũng phải làm vậy cho production
>    (đã áp dụng — xem TASK-BE-DB-006's Kết quả thực tế).
> 2. `common/testutil.StartMySQL`'s wait strategy **ban đầu dùng
>    `wait.ForLog("ready for connections").WithOccurrence(2)`** theo thiết
>    kế thông thường cho MySQL testcontainers, nhưng chạy test thật cho
>    thấy đây là 1 race thật: `migrate` CLI fail `unexpected EOF`/`bad
>    connection` ngay sau khi container được báo "ready" (log xuất hiện
>    trước khi MySQL thực sự nhận kết nối TCP ổn định sau lần restart nội
>    bộ). Đã đổi sang `wait.ForSQL("3306/tcp", "mysql", ...)` (retry
>    `SELECT 1` thật qua `database/sql` cho tới khi thành công) — sửa
>    ngay trong `common/testutil/mysql.go` (thuộc phạm vi cho phép sửa của
>    task này), không thêm dependency mới vào `common/go.mod` (dùng tên
>    driver `"mysql"` runtime, dựa vào driver đã được blank-import sẵn ở
>    binary test gọi nó — `internal/adapter/mysql/repository_test.go`
>    blank-import `_ "github.com/go-sql-driver/mysql"`).
>
> Test cases thêm đúng 6 cái yêu cầu (`SaveSession_IsIdempotentOnRequestID`,
> `ListSessions_DoesNotLeakAcrossTenants`, `GetDailyRollup_DoesNotLeakAcrossTenants`,
> `RecomputeDailyRollup_MatchesSessionSums`, `FetchUnpublished_ReturnsOldestFirst`,
> `MarkPublished_EmptyIDsIsNoop`) cộng test outbox round-trip mirror từ bản
> Postgres (`Outbox_EnqueueFetchMarkPublished`) — 7 test tổng cộng. Tenant
> ID dùng hằng số UUID-dạng-hợp-lệ (`tenantAUUID`/`tenantBUUID`) thay vì
> chuỗi tuỳ ý như mẫu ban đầu — nhất quán với điều chỉnh đã áp dụng ở
> TASK-BE-DB-003 (dù MySQL's `CHAR(36)` không bắt buộc format UUID, giữ
> nhất quán dữ liệu test giữa 2 dialect).
>
> **Build/test thật đã chạy**: `go build ./... && go vet ./...` sạch;
> `gofmt -l internal/adapter/mysql/*.go` sạch (sau khi chạy `gofmt -w` sửa
> thứ tự import 1 lần); `go test -tags=integration ./internal/adapter/mysql/...
> -v` (Docker + testcontainers thật, MySQL 8) → **7/7 PASS thật** (lần
> chạy đầu tiên với wait strategy cũ FAIL toàn bộ 7/7 do bug ở mục trên;
> sau khi sửa `testutil.StartMySQL` sang `wait.ForSQL`, chạy lại **PASS
> toàn bộ 7/7**, ~157s). Đặc biệt xác nhận
> `TestRepository_ListSessions_DoesNotLeakAcrossTenants` **PASS trên MySQL**
> — khác với bản Postgres ở TASK-BE-DB-003 (FAIL do bug SQL tiền tồn tại
> trong `ListSessions`'s tham số tái sử dụng `$2`), vì MySQL's placeholder
> `?` không tái sử dụng và không có kiểu ép buộc tĩnh như Postgres extended
> protocol — adapter MySQL viết 2 dấu `?` riêng (`tenantID, userID, userID,
> pageToken, pageSize`) nên không gặp lại bug đó.

---

## Mục tiêu

Implement `usecase.Repository` (interface có sẵn, không đổi) +
`common/outbox.Store` cho MySQL/TiDB, khớp đúng hành vi
`internal/adapter/postgres/repository.go` (idempotency, tenant scoping,
transactional outbox) — dùng tên bảng đã chốt ở TASK-BE-DB-004
(`sessions`, `daily_rollups`, `outbox_events`, không prefix).

## Files cần sửa

1. `backend-go/services/usage-service/go.mod` (MODIFY — thêm `github.com/go-sql-driver/mysql`)
2. `backend-go/services/usage-service/internal/adapter/mysql/repository.go` (MỚI)
3. `backend-go/services/usage-service/internal/adapter/mysql/repository_test.go` (MỚI — integration test, mirror `adapter/postgres/repository_test.go`)
4. `backend-go/common/testutil/mysql.go` (MỚI — `StartMySQL(t *testing.T, dbName string) string`, cùng khuôn `StartPostgres` ở `common/testutil/postgres.go` nhưng image `mysql:8`, trả về DSN dạng `mysql://root:orca@tcp(host:port)/dbName`)
5. `backend-go/common/go.mod` (MODIFY — xác nhận `testcontainers-go` đã có sẵn cho module `postgres`, chỉ cần thêm submodule `testcontainers-go/modules/mysql` nếu dùng, hoặc dùng `GenericContainer` thuần như `StartPostgres` đã làm — ưu tiên cách 2 để không thêm dependency mới, đúng khuôn có sẵn)

## Nội dung `repository.go`

Dịch đầy đủ (không rút gọn như bản phác thảo ở BE-DB-SOL-002) 6 method:
`SaveSession`, `FetchUnpublished`, `MarkPublished`, `GetDailyRollup`,
`ListSessions`, `RecomputeDailyRollup`. Khung + import + `New`/struct đã
có ở BE-DB-SOL-002 §3 — khi implement thật:

- Đọc lại **toàn bộ** `internal/adapter/postgres/repository.go` trước khi
  viết, dịch từng câu SQL 1:1 về ngữ nghĩa (không suy đoán cột/thứ tự
  tham số) — đặc biệt `SaveSession`'s `INSERT ... VALUES (...)` 13 cột
  phải đúng thứ tự tham số hệt bản Postgres.
- `ON CONFLICT (...) DO NOTHING` → `INSERT IGNORE` (dựa vào UNIQUE KEY
  `uniq_tenant_request`), kiểm tra `RowsAffected() == 0` để phát hiện
  idempotent-replay, giữ nguyên logic "nếu đã tồn tại thì bỏ qua rollup +
  outbox, chỉ commit".
- `ON CONFLICT (...) DO UPDATE SET x = t.x + EXCLUDED.x` → `INSERT ...
  ON DUPLICATE KEY UPDATE x = x + VALUES(x)` (dựa vào PRIMARY KEY
  `(tenant_id, user_id, provider, day)` đã có ở migration MySQL).
- `id = ANY($1)` (`MarkPublished`) → build `IN (?, ?, ..., ?)` động theo
  `len(ids)`, giữ nguyên early-return khi `len(ids) == 0` (bản Postgres đã
  có guard này, không được bỏ).
- `errors.Is(err, pgx.ErrNoRows)` → `errors.Is(err, sql.ErrNoRows)`.
- **Không đổi tenant-scoping**: mọi `WHERE tenant_id = ?` phải giữ nguyên
  vị trí/logic hệt bản Postgres — đây là compensating control duy nhất
  khi MySQL không có RLS (xem BE-DB-SOL-001 §4, TASK-BE-DB-003).

## Test cases cần cover (`repository_test.go`, build tag `integration`)

Mirror chính xác các test đã có ở `adapter/postgres/repository_test.go`
(cùng tên, đổi package) cộng 2 test tenant-isolation mới từ
TASK-BE-DB-003 (áp dụng lại cho MySQL — đây là điểm quan trọng nhất của
CR-DB-002's tiêu chí chấp nhận "Test xác nhận tenant isolation vẫn đúng
khi RLS không khả dụng" — phải thực sự chạy trên MySQL thật, không chỉ
Postgres):

- `TestRepository_SaveSession_IsIdempotentOnRequestID`
- `TestRepository_ListSessions_DoesNotLeakAcrossTenants` (từ TASK-BE-DB-003, mirror cho MySQL)
- `TestRepository_GetDailyRollup_DoesNotLeakAcrossTenants` (từ TASK-BE-DB-003, mirror cho MySQL)
- `TestRepository_RecomputeDailyRollup_MatchesSessionSums`
- `TestRepository_FetchUnpublished_ReturnsOldestFirst`
- `TestRepository_MarkPublished_EmptyIDsIsNoop` (guard `len(ids) == 0`)

`setupRepository(t)` cho MySQL dùng `common/testutil.StartMySQL` (mới,
xem mục "Files cần sửa" #4) — TASK-BE-DB-007 chỉ còn việc wire CI
workflow gọi lại các test này, không cần viết lại `StartMySQL`.

## Verify

```bash
cd backend-go/services/usage-service && go build ./... && go vet ./...
go test -tags=integration ./internal/adapter/mysql/... -v
gofmt -l internal/adapter/mysql/*.go
```

## gitnexus

`impact({target: "Repository", direction: "upstream", file_path: "services/usage-service/internal/usecase/ports.go", kind: "Interface"})`
trước khi thêm implementation mới — **đã chạy, risk LOW, impacted 2**
(xem solutions/README.md). Thêm 1 implementation song song không đổi
interface, an toàn theo đúng kết quả này.
