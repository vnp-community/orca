# TASK-BE-DB-009: Multi-database rollout cho `issue-tracking-service`

**Solution:** [BE-DB-SOL-004](../solutions/BE-DB-SOL-004-issue-tracking-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `issue-tracking-service`
**Depends on:** TASK-BE-DB-002 (`dbcapability`, dùng lại nguyên vẹn), pattern TASK-BE-DB-004~007 (`usage-service` pilot)
**Status:** ✅ DONE cho phạm vi rollout MySQL — 🟡 1 bug tiền tồn tại (ngoài phạm vi) được ghi nhận, không sửa (2026-09-11)

> **Kết quả thực tế:**
>
> **1. `impact()` chạy thật trước khi sửa/thêm bất kỳ symbol nào** (bắt
> buộc theo AGENTS.md — chi tiết đầy đủ ở BE-DB-SOL-004 §2): 4 lần chạy,
> `ConnectionRepository`/`OutboxEnqueuer` (interface, `ports.go`) →
> **MEDIUM** (impactedCount 7 — toàn depth=1 là `IMPORTS` cấp file, không
> phải phụ thuộc chữ ký thật; không đổi interface nên an toàn tiếp tục);
> `New` (`adapter/postgres/repository.go`) → **LOW** (2); `run`
> (`cmd/server/main.go`) → **LOW** (1). Không HIGH/CRITICAL nào.
>
> **2. Migration split** — `migrations/0001_outbox.{up,down}.sql` +
> `migrations/0002_connections.{up,down}.sql` di chuyển nguyên văn (`git
> mv`, không sửa nội dung) vào `migrations/postgres/`.
> `migrations/mysql/` mới, 4 file, dialect-safe đầy đủ — chi tiết mapping ở
> BE-DB-SOL-004 §3 (điểm đáng chú ý nhất: `id UUID DEFAULT gen_random_uuid()`
> ở bảng `connections` → `CHAR(36) DEFAULT (UUID())` vì cột này không hề
> được Go code đọc/ghi, xác nhận bằng đọc trực tiếp
> `internal/adapter/postgres/connections.go`, không suy đoán; và việc phải
> đổi 4 cột `TEXT` trong composite UNIQUE KEY sang `VARCHAR` có độ dài cụ
> thể vì InnoDB từ chối `TEXT` trong khoá không prefix-length).
>
> **3. `internal/adapter/mysql/{repository,connections}.go` mới** —
> implement lại đúng CẢ 2 interface (`usecase.OutboxEnqueuer` +
> `common/outbox.Store` + `usecase.ConnectionRepository`, 8 method tổng
> cộng) trên 1 struct `Repository` duy nhất, KHÔNG đổi chữ ký interface
> nào. `Upsert`'s cách tính `is_selected` cho lần connect đầu khác pilot
> (SQL subquery-trong-VALUES → SELECT COUNT(*) riêng trong cùng
> transaction) — lý do đầy đủ ở BE-DB-SOL-004 §4, không phải suy đoán mà
> là quyết định tránh cú pháp MySQL/TiDB không chắc chắn.
>
> **4. Wiring `cmd/server/main.go`** — thêm `switch caps.Dialect` đúng
> khuôn `usage-service`, gán cả 3 interface (`repo`, `connectionRepo`,
> `outboxStore`) từ CÙNG 1 giá trị Repository cụ thể mỗi nhánh dialect.
> `toMySQLDriverDSN` copy nguyên văn từ `usage-service` (không share qua
> package chung — cùng tiền lệ `issue-status-sync`'s TASK-BE-DB-008).
> Không thêm biến env mới. `healthSrv.Register(string(caps.Dialect), ...)`
> thay hard-code `"postgres"`.
>
> **5. Test MySQL adapter** — `internal/adapter/mysql/repository_test.go`
> (2 test: outbox round-trip, `MarkPublished` empty-ids no-op) +
> `internal/adapter/mysql/connections_test.go` (6 test: 4 mirror từ
> `adapter/postgres/connections_test.go`, cộng 2 test tenant-isolation-
> without-RLS mới theo TASK-BE-DB-003's pattern —
> `GetStatus_DoesNotLeakAcrossTenants`, `GetCredentialID_DoesNotLeakAcrossTenants`,
> lý do cần 2 test này thay vì bỏ qua ở BE-DB-SOL-004 §6: bảng
> `connections`/`outbox_events` CÓ RLS policy ở bản Postgres, khác
> `issue-status-sync` không có). Build tag `integration`, dùng
> `common/testutil.StartMySQL` nguyên vẹn (không sửa package này).
>
> **Build/test thật đã chạy** (Docker + testcontainers thật, không giả
> định):
> ```
> go build ./...          → sạch
> go vet ./...             → sạch
> gofmt -l internal/adapter/mysql/*.go cmd/server/main.go  → sạch
> go test ./... (unit)     → PASS toàn bộ (không có test nào fail; các
>                              package không có test riêng báo "no test
>                              files", đúng như trước khi sửa)
> go test -tags=integration ./internal/adapter/mysql/... -v
>                          → PASS 8/8 thật (MySQL 8 container), ~175s:
>   TestConnectionsRepository_MultiSiteUpsert_AddsRowDoesNotOverwrite
>   TestConnectionsRepository_SelectWorkspace_MovesIsSelected
>   TestConnectionsRepository_Delete_RemovesOneWorkspaceOnly
>   TestConnectionsRepository_GetCredentialID_NotFoundReturnsSentinel
>   TestConnectionsRepository_GetStatus_DoesNotLeakAcrossTenants
>   TestConnectionsRepository_GetCredentialID_DoesNotLeakAcrossTenants
>   TestRepository_Outbox_EnqueueFetchMarkPublished
>   TestRepository_MarkPublished_EmptyIDsIsNoop
> ```
>
> **6. Phát hiện phụ, ngoài phạm vi, KHÔNG sửa**: để xác nhận việc tách
> `migrations/postgres/` không phá vỡ test hiện có, cũng chạy `go test
> -tags=integration ./internal/adapter/postgres/... -v` (không bắt buộc
> theo task doc gốc, chỉ để đối chứng) — **4/5 FAIL thật**, lý do: 2 cột
> `UUID NOT NULL` trong migration (`connections.credential_id`,
> `outbox_events.tenant_id`) nhưng test fixture dùng literal không phải
> UUID hợp lệ (`"cred-1"`, `"tenant-1"`) → `SQLSTATE 22P02`. Xác nhận đây
> là bug TIỀN TỒN TẠI (migration + test content không đổi, chỉ `git mv` vị
> trí file) — không phải do việc tách migration hay bất kỳ thay đổi nào
> của task này gây ra. Chi tiết đầy đủ + lý do không sửa ở BE-DB-SOL-004
> §7 (cùng dạng phát hiện như TASK-BE-DB-003/007's Postgres bug ở
> `usage-service`). Hệ quả: lane `postgres` của `.github/workflows/backend-
> go-issue-tracking-service.yml` mới sẽ ĐỎ khi PR mở thật, lane `mysql`
> xanh — không giả định "CI xanh toàn bộ", ghi rõ theo yêu cầu README's
> Verify convention.
>
> **7. CI workflow** — `.github/workflows/backend-go-issue-tracking-service.yml`
> mới, copy khung `backend-go-usage-service.yml`, đổi path/service name.
> `python3 -c "import yaml; yaml.safe_load(...)"` xác nhận YAML hợp lệ.
> Không dry-run được qua `act` (không có sẵn trong môi trường) — đúng
> fallback đã dùng ở TASK-BE-DB-007.
>
> **Kết luận trạng thái**: ✅ DONE cho đúng phạm vi task này (MySQL adapter
> hoạt động thật, migration dialect-safe, wiring, test tenant-isolation,
> CI workflow) — build/vet/unit/mysql-integration đều PASS thật. Không
> nâng lên tuyệt đối "mọi thứ xanh" vì bug Postgres tiền tồn tại ở mục 6
> vẫn còn đó (ngoài phạm vi sửa của task này, đã ghi nhận rõ, không che
> giấu) — mirror đúng cách TASK-BE-DB-007 xử lý phát hiện tương tự ở pilot.

---

## Mục tiêu

Nhân rộng pattern CR-DB-002/CR-DB-003 (`dbcapability` + adapter MySQL/TiDB
song song + migration dialect-safe + test tenant-isolation-without-RLS +
CI matrix) từ pilot `usage-service` sang `issue-tracking-service` — service
thứ 2 trong batch 1 của rollout 15 service
(`specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md`).

`issue-tracking-service` có **2 repository** (khác pilot's 1): outbox
(Epic G) + connections (Connect/GetConnectionStatus, thêm sau) — cả 2 trên
1 struct `Repository`, xem BE-DB-SOL-004 §1.

## Files cần sửa

1. `backend-go/services/issue-tracking-service/migrations/postgres/*.sql` (MỚI — di chuyển nguyên văn từ `migrations/`)
2. `backend-go/services/issue-tracking-service/migrations/mysql/*.sql` (MỚI)
3. `backend-go/services/issue-tracking-service/internal/adapter/mysql/repository.go` (MỚI)
4. `backend-go/services/issue-tracking-service/internal/adapter/mysql/connections.go` (MỚI)
5. `backend-go/services/issue-tracking-service/internal/adapter/mysql/repository_test.go` (MỚI)
6. `backend-go/services/issue-tracking-service/internal/adapter/mysql/connections_test.go` (MỚI)
7. `backend-go/services/issue-tracking-service/cmd/server/main.go` (MODIFY — dialect switch)
8. `backend-go/services/issue-tracking-service/internal/adapter/postgres/repository_test.go` (MODIFY — `migrationsPath` trỏ `migrations/postgres`)
9. `backend-go/services/issue-tracking-service/README.md` (MODIFY — cập nhật đường dẫn migration)
10. `.github/workflows/backend-go-issue-tracking-service.yml` (MỚI)

## Test cases cần cover

Mirror `adapter/postgres`'s test hiện có (4 connections + 1 outbox) cộng 2
test tenant-isolation-without-RLS mới (xem BE-DB-SOL-004 §6) — 8 test tổng
cộng cho `internal/adapter/mysql`, tất cả PASS thật trên MySQL 8 container
(xem "Kết quả thực tế" #5 ở trên).

## Verify

```bash
cd backend-go/services/issue-tracking-service
go build ./... && go vet ./...
gofmt -l internal/adapter/mysql/*.go cmd/server/main.go
go test ./...
go test -tags=integration ./internal/adapter/mysql/... -v
python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-issue-tracking-service.yml'))"
```

## gitnexus

`impact()` đã chạy thật cho `ConnectionRepository`, `OutboxEnqueuer`
(interface, `ports.go`), `New` (`adapter/postgres/repository.go`), `run`
(`cmd/server/main.go`) trước khi sửa — chi tiết đầy đủ ở BE-DB-SOL-004 §2,
không HIGH/CRITICAL nào. `detect_changes()` chạy trước khi hoàn tất task
(xem phần cuối báo cáo agent).
