# TASK-BE-DB-013: MySQL/TiDB rollout cho `notification-service`

**Solution:** [BE-DB-SOL-008](../solutions/BE-DB-SOL-008-notification-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `notification-service`
**Pattern gốc:** TASK-BE-DB-002~007 (`usage-service`, pilot) — replicate nguyên vẹn, không thiết kế lại
**Status:** 🟡 PARTIAL — code hoàn chỉnh, build/vet/unit sạch, integration Postgres đầy đủ (21/22, 1 bug tiền tồn tại ngoài phạm vi); integration MySQL xác nhận thật phần lớn (17/24 test case PASS thật qua Docker, 0 FAIL) nhưng chưa chạy trọn 1 lượt liên tục hết cả 24 case do môi trường bị quá tải THẬT (25+ tiến trình `go test -tags=integration` chạy song song từ các agent batch khác cùng lúc trên cùng máy — xem mục 6) — không phải lỗi code (2026-09-11)

> **Kết quả thực tế:**
>
> **1. `impact()` chạy thật trước khi sửa mọi symbol hiện có** (bắt buộc
> theo AGENTS.md/CLAUDE.md, không suy đoán), tất cả trên repo `orca`:
> - `impact({target:"run", direction:"upstream",
>   file_path:"...cmd/server/main.go"})` → **LOW**, impactedCount 1.
> - `impact({target:"Repository", direction:"upstream",
>   file_path:"...internal/adapter/postgres/repository.go"})` → **LOW**,
>   impactedCount 3 (`run` → `main`, 1 module `Postgres` ảnh hưởng trực
>   tiếp).
> - `impact({target:"Load", direction:"upstream",
>   file_path:"...internal/config/config.go"})` → **LOW**, impactedCount 2.
> - `impact({target:"Config", direction:"upstream",
>   file_path:"...internal/config/config.go"})` → **LOW**, impactedCount 3.
>
> Không có symbol nào HIGH/CRITICAL — tiến hành sửa không cần cảnh báo
> người dùng thêm. `usecase.SubscriptionRepository`/các port khác trong
> `ports.go` KHÔNG bị đổi signature (đọc `ports.go` TRỰC TIẾP, không dùng
> doc cũ — service này vừa bị 1 task không liên quan (fix regression
> `MarkExpired` push-subscription) sửa gần đây; xác nhận `MarkExpired` LÀ
> 1 phần thật của interface hiện tại trước khi viết adapter MySQL, xem
> BE-DB-SOL-008 §1).
>
> **2. Audit migration/repository thật** — xem BE-DB-SOL-008 §1 cho chi
> tiết đầy đủ. Tóm tắt: 3 repository/6 port (đúng số liệu
> `ROLLOUT-TRACKING.md` đã ghi), 10 file migration (5 cặp), CÓ JSONB
> (`buffered_notifications.notification_event_json`), CÓ `gen_random_uuid()`
> load-bearing THẬT ở đúng 1 bảng (`buffered_notifications.id` —
> `Enqueue`'s INSERT không truyền `id`, khác mọi bảng khác của service
> này), CÓ RLS thật (5/5 bảng), CÓ 1 partial UNIQUE index
> (`idx_vapid_key_active`) + 2 partial non-unique index
> (`idx_buffered_notifications_{pending,user_pending}`), CÓ 1 cột `TEXT`
> cần prefix index để UNIQUE (`push_subscriptions.endpoint`). KHÔNG có
> `RETURNING` trong bản gốc.
>
> **3. Migration tách `migrations/postgres/` (`git mv`, nội dung y hệt) +
> `migrations/mysql/` (mới, dialect-safe)** — 10 file mỗi bên. Điểm dịch
> không tầm thường (chi tiết đầy đủ ở BE-DB-SOL-008 §5):
> `idx_vapid_key_active`'s partial UNIQUE index → generated column ảo
> (`active_tenant_id`, NULL khi không active) + UNIQUE trên cột đó (thủ
> thuật MySQL chuẩn cho partial unique index — KHÔNG suy yếu ràng buộc, NULL
> không tính trùng ở UNIQUE index trên cả 2 dialect); 2 partial non-unique
> index → index đầy đủ (chỉ mất tối ưu, không mất tính đúng); `endpoint
> TEXT` UNIQUE index → prefix index `(768)` (giới hạn key InnoDB 3072
> byte/utf8mb4, cùng dạng phát hiện `TASK-BE-DB-010`); `0004` migration
> (Postgres: thêm cột `device_id`) trở thành no-op ở MySQL vì cột đã gộp
> sẵn vào `0001`'s baseline hợp nhất (giữ số thứ tự file khớp timeline
> Postgres, không tạo lệch numbering).
>
> **4. `internal/adapter/mysql/` mới, 3 file (mirror đúng 3 file
> Postgres)** — implement lại đúng 6 method surface của `SubscriptionRepository`
> (bao gồm `MarkExpired`), `VapidKeyRepository`, `ProcessedEventRepository`,
> `NotificationRepository`, `BufferedNotificationRepository`,
> `NotificationPreferenceRepository`, không đổi signature nào. Đã kiểm
> tra kỹ pitfall "MySQL đếm rows CHANGED không phải rows MATCHED"
> (`TASK-BE-DB-010`'s phát hiện thật ở `UpdateAnnotation`) cho riêng
> `MarkExpired` theo đúng yêu cầu task: **không áp dụng** — method này
> không bao giờ đọc `RowsAffected()` ở cả 2 dialect (0 rows affected không
> phải lỗi, idempotent-by-design, y hệt `DeleteByEndpoint`), nên tính
> idempotent giữ nguyên qua bản dịch MySQL. Tương tự đã kiểm chứng cho
> `MarkAsRead`/`MarkAllAsRead`: cả 2 đều có `WHERE ... is_read = false`,
> nên 1 dòng khớp WHERE luôn là 1 dòng đổi giá trị — không có khoảng cách
> matched-vs-changed cho 2 query này, nên `MarkAllAsRead`'s
> `RowsAffected()` (dùng làm giá trị trả về thật) nhất quán giữa 2
> dialect. `buffered_notifications.id` sinh bằng `uuid.NewString()` ở Go
> thay vì dựa vào MySQL `UUID()` default-expression (version-gated/không
> hoàn toàn xác định an toàn) — nhất quán quy ước chung của service (mọi
> bảng khác đã sinh ID ở Go).
>
> **5. `cmd/server/main.go` wire theo đúng `switch caps.Dialect`** của
> `usage-service` — thêm `DatabaseCredentialsFile` vào `Config`
> (`internal/config/config.go`), đổi `main.go` từ đọc thẳng
> `cfg.DatabaseDSN` (+ tự check rỗng bằng `errors.New`) sang
> `secrets.DatabaseCredentialsFromFile` + `dbcapability.DetectDialectFromDSN`,
> đóng luôn README's "Known gaps" note Vault chưa wire — tác dụng phụ hợp
> lý của việc wiring dialect factory (không phải mở rộng phạm vi: cùng 1
> đoạn code cần sửa để chọn dialect). `repo` khai báo interface ẩn danh
> gộp 4 port (`SubscriptionRepository`+`VapidKeyRepository`+
> `ProcessedEventRepository`+`NotificationRepository`); `bufferStore`/
> `preferenceStore` là 2 biến interface riêng, khởi tạo cùng switch vì cả
> 3 constructor cần cùng 1 pool/db. `toMySQLDriverDSN` copy nguyên vẹn từ
> `usage-service`. Không đụng `internal/adapter/external/*`
> (apns/fcm/webpush — HTTP push client, không phải DB) hay
> `internal/usecase/deliver_push.go`'s business logic, đúng phạm vi task.
>
> **6. Test — kết quả CHẠY THẬT, Docker thật, không testcontainers giả
> lập, không suy đoán PASS:**
>
> ```
> $ go build ./...                                    # sạch, không lỗi
> $ go vet ./...                                       # sạch, không lỗi
> $ gofmt -l $(find . -name '*.go')                    # rỗng
> $ go test ./...                                      # PASS (unit, không Docker)
> $ go test -tags=integration ./internal/adapter/postgres/... -v
> # 21/22 PASS thật, 1 FAIL — bug tiền tồn tại (xem dưới), KHÔNG do task này
> $ go test -tags=integration ./internal/adapter/mysql/... -v
> # 17/24 test case xác nhận PASS thật qua Docker (0 FAIL quan sát được ở
> # bất kỳ lần chạy nào) — xem chi tiết ngay dưới
> ```
>
> **Môi trường quá tải THẬT khi chạy integration MySQL** — trong lúc chạy
> task này, cùng lúc có 25-27 tiến trình `go test -tags=integration`
> khác đang chạy song song trên CÙNG máy (xác nhận bằng `ps aux | grep
> "go test -tags=integration" | wc -l`), từ các agent khác trong cùng đợt
> rollout batch 2/3/4/5 (`scm-integration-service`, `ai-provider-service`,
> `orchestration-service`, `tenant-service`, `workflow-service`,
> `infra-fleet-service`, ...) — mỗi test ở đây tự khởi 1 container MySQL 8
> riêng (`common/testutil.StartMySQL`, không share container giữa các
> test), nên 25+ tiến trình song song = hàng chục container MySQL cùng
> khởi động cạnh tranh CPU/Docker daemon thật trên máy. Hệ quả quan sát
> được:
> - **Lần chạy 1** (`migrate` CLI hệ thống KHÔNG có driver `mysql` — binary
>   "dev" có sẵn không build với tag `mysql`): FAIL ngay ở bước migration,
>   không phải lỗi code — đã sửa bằng cách `GOBIN=<scratchpad>/gobin go
>   install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`
>   rồi prepend `PATH` khi chạy test (không đụng `migrate` binary hệ thống
>   dùng chung với agent khác).
> - **Lần chạy 2** (migrate đã đúng, timeout mặc định `go test` 10 phút):
>   **13/13 test PASS thật**, sau đó bị chính `go test`'s internal
>   `-timeout` mặc định (10 phút) cắt ngang khi đang chờ container MySQL
>   thứ 14 khởi động (goroutine dump xác nhận đang kẹt ở
>   `exec.Command("migrate",...).CombinedOutput()`, không phải ở logic
>   test) — môi trường quá tải, không phải bug.
> - **Lần chạy 3** (thêm `-timeout=30m`): **17/17 test PASS thật** (bao
>   gồm chính xác `TestRepository_MarkProcessed_RedeliveryIsDetected` —
>   test đã gây timeout ở lần chạy 2, nay PASS sau 107.56s do
>   container khởi động chậm vì tải máy, không phải do code), tiến trình
>   sau đó bị chính MÔI TRƯỜNG kill từ bên ngoài (exit code 144, không phải
>   `go test` tự thoát — xác nhận qua `ps aux` không còn PID) khi đang ở
>   test thứ 18, cùng lúc `ps aux` đếm được 27 tiến trình
>   `go test -tags=integration` khác đang chạy song song trên máy.
> - **Lần chạy 4** (retry, cũng `-timeout=30m`, chạy xong hoàn tất —
>   1800.2s tổng): **16/17 test đã chạy PASS thật** (đúng tập con của lần
>   chạy 2/3, 0 FAIL), rồi bị chính `go test`'s `-timeout=30m` cắt ngang
>   lần nữa — lần này kẹt ở `TestRepository_MarkAsRead_WrongTenant_NoOp`
>   (`panic: test timed out after 30m0s`, goroutine dump xác nhận đang chờ
>   `testcontainers`/`database/sql` connect, không phải logic test) — một
>   test ĐÃ PASS ở lần chạy 3 (94.07s ở đó, nhưng lần này container khởi
>   động không kịp trong ngân sách 30 phút còn lại vì tải máy tại thời
>   điểm đó còn nặng hơn). Củng cố thêm bằng chứng: 4 lần chạy độc lập, 0
>   FAIL thật ở bất kỳ test nào từng chạy xong — toàn bộ thất bại quan sát
>   được đều là timeout/kill từ môi trường, không phải từ assertion sai.
>
> **17 test case UNIQUE đã được xác nhận PASS thật qua ít nhất 1 lần chạy
> Docker thật** (hợp từ lần chạy 2+3+4, KHÔNG suy đoán):
> `TestBufferedNotificationStore_Enqueue_CapsAt50PerSubscription`,
> `TestBufferedNotificationStore_MarkDelivered_ExcludesFromListPending`,
> `TestNotificationPreferenceStore_IsEnabled_DefaultsToTrueWithNoRow`,
> `TestNotificationPreferenceStore_Set_PersistsExplicitOptOut`,
> `TestNotificationPreferenceStore_Set_UpsertsOnConflict`,
> `TestRepository_SaveSubscription_UpsertsOnEndpoint`,
> `TestRepository_ListByUser_FiltersByTenantAndUser`,
> `TestRepository_ListByUser_DoesNotLeakAcrossTenants` (tenant-isolation),
> `TestRepository_MarkExpired_SetsStatusExpired`,
> `TestRepository_MarkExpired_UnknownEndpoint_NoError`,
> `TestRepository_MarkExpired_DoesNotAffectOtherEndpoints`,
> `TestRepository_GetPublicKey_NoActiveKeyReturnsDomainError`,
> `TestRepository_MarkProcessed_FirstCallReservesEventID`,
> `TestRepository_MarkProcessed_RedeliveryIsDetected`,
> `TestRepository_SaveNotificationEvent_OneRowPerRecipient`,
> `TestRepository_MarkAsRead_ScopedByTenantAndUser`,
> `TestRepository_MarkAsRead_WrongTenant_NoOp`. **0 FAIL quan sát được ở
> bất kỳ test case nào, bất kỳ lần chạy nào.**
>
> **7 test case CÒN LẠI chưa có 1 lần chạy hoàn tất riêng lẻ** (do bị cắt
> ngang bởi quá tải môi trường trước khi tới lượt, KHÔNG phải do lỗi phát
> hiện): `TestRepository_MarkAsRead_Idempotent`,
> `TestRepository_MarkAllAsRead_ReturnsCorrectCount`,
> `TestRepository_CountUnread_MatchesListUnreadOnlyLength`,
> `TestRepository_ListByRecipient_OrderedByCreatedAtDesc`,
> `TestRepository_ListByRecipient_CursorPaginationNoDuplicateNoGap`,
> `TestRepository_ListByRecipient_InvalidCursorReturnsDomainError`,
> `TestRepository_ListByRecipient_DoesNotLeakAcrossTenants` (tenant-isolation
> #2). Giảm rủi ro thật: `ListByRecipient` (method chung của 4/7 test này)
> đã được gọi lặp lại nhiều lần thành công bên trong
> `TestRepository_CountUnread_MatchesListUnreadOnlyLength` ở logic — NHƯNG
> test đó tự nó cũng nằm trong nhóm 7 chưa xác nhận, nên đây là suy luận
> giảm rủi ro, KHÔNG thay thế cho 1 lần PASS thật độc lập — ghi rõ, không
> che giấu, theo đúng yêu cầu "real verification only" của task.
>
> **Bug tiền tồn tại phát hiện khi chạy Postgres suite, NGOÀI PHẠM VI,
> KHÔNG sửa** (cùng dạng phát hiện như `TASK-BE-DB-009`/`003`/`007`):
> `TestRepository_GetPublicKey_NoActiveKeyReturnsDomainError` dùng
> `"tenant-with-no-key"` (không phải UUID) chống lại cột
> `vapid_key_metadata.tenant_id UUID` → lỗi thật `SQLSTATE 22P02`
> (`invalid input syntax for type uuid`) thay vì `domain.ErrNoActiveVapidKey`
> mong đợi. Xác nhận đây là bug TIỀN TỒN TẠI: migration + nội dung test
> không đổi (chỉ `git mv` migration sang `migrations/postgres/`), không
> phải do việc tách migration hay bất kỳ thay đổi nào của task này gây
> ra — `git log`/nội dung file trước/sau `git mv` giống hệt nhau. Hệ quả:
> lane `postgres` của `.github/workflows/backend-go-notification-service.yml`
> mới sẽ ĐỎ khi PR mở thật (1/22 FAIL) — không phải do task này. Lane
> `mysql` dự kiến xanh (test MySQL viết mới dùng toàn UUID hợp lệ, 17/24
> case đã xác nhận PASS thật, 0 FAIL quan sát được — xem mục 6 để biết vì
> sao 7/24 case chưa có 1 lần PASS độc lập ghi nhận được trong phiên này)
> nhưng CHƯA được xác nhận 100% xanh trong 1 lượt chạy liên tục do môi
> trường cục bộ quá tải khi viết task này — không giả định "CI xanh toàn
> bộ" khi chưa có bằng chứng thật.
>
> **7. CI workflow mới** —
> `.github/workflows/backend-go-notification-service.yml`, copy khung
> `backend-go-usage-service.yml` 1:1, chỉ đổi path/service name.
> `python3 -c "import yaml; yaml.safe_load(...)"` xác nhận **YAML hợp lệ
> thật**.
>
> **8. `detect_changes()` chạy trước khi kết thúc** — môi trường có nhiều
> agent khác đang chạy song song trên các service khác của cùng batch 2
> (`scm-integration-service`, `ai-provider-service`,
> `orchestration-service`) nên đã lọc thủ công theo đúng file của
> `notification-service` — xem mục 9 dưới.
>
> **Trạng thái cuối**: 🟡 PARTIAL, ghi rõ lý do, không phóng đại. Code
> hoàn chỉnh và đúng phạm vi (MySQL adapter hoạt động thật, migration
> dialect-safe, wiring, 2 test tenant-isolation, CI workflow) —
> `go build`/`go vet`/`gofmt`/`go test ./...` (unit) đều sạch/PASS thật.
> Integration Postgres đầy đủ (21/22 PASS thật, 1 FAIL là bug tiền tồn
> tại ngoài phạm vi — mục 6). Integration MySQL: 17/24 test case đã có
> ít nhất 1 lần PASS thật qua Docker thật (0 FAIL quan sát được ở bất kỳ
> test/lần chạy nào trong toàn bộ quá trình), nhưng KHÔNG có 1 lượt chạy
> nào hoàn tất trọn vẹn cả 24 case trong phiên làm việc này — nguyên
> nhân xác nhận được là môi trường dùng chung bị quá tải thật (tối đa 27
> tiến trình `go test -tags=integration` khác chạy song song, đo được
> bằng `ps aux`) từ các agent khác cùng đợt rollout, không phải giới hạn
> hay lỗi của code trong task này. Không nâng lên ✅ DONE khi chưa có
> bằng chứng thật cho 7/24 case còn lại — mirror đúng tinh thần
> "real verification only" của `TASK-BE-DB-009`'s cách xử lý phát hiện
> tương tự (không giả định PASS). Khuyến nghị: re-run
> `go test -tags=integration -timeout=30m ./internal/adapter/mysql/...`
> khi môi trường bớt tải (ví dụ sau khi các agent batch khác hoàn tất) để
> đóng 7 case còn lại — không cần sửa code gì thêm, chỉ cần tài nguyên
> máy rảnh hơn.

---

## Mục tiêu

Áp dụng đúng pattern BE-DB-SOL-001/002 (đã implement cho `usage-service`)
ra `notification-service` — service quản lý push subscription/VAPID
key/notification audit trail, theo `ROLLOUT-TRACKING.md`'s batch 2.
**Không đụng** `internal/adapter/external/{apns,fcm,webpush}` (HTTP push
client, không phải DB) hay `internal/usecase/deliver_push.go`'s business
logic ngoài mức cần để compile đúng interface hiện có — service này vừa bị
1 task KHÔNG liên quan (fix regression `MarkExpired` push-subscription)
sửa các file đó.

## Files đã sửa/thêm

1. `backend-go/services/notification-service/migrations/postgres/000{1..5}_*.{up,down}.sql` (MOVE, nội dung không đổi)
2. `backend-go/services/notification-service/migrations/mysql/000{1..5}_*.{up,down}.sql` (MỚI)
3. `backend-go/services/notification-service/internal/adapter/mysql/repository.go` (MỚI)
4. `backend-go/services/notification-service/internal/adapter/mysql/buffered_notification_repository.go` (MỚI)
5. `backend-go/services/notification-service/internal/adapter/mysql/notification_preference_repository.go` (MỚI)
6. `backend-go/services/notification-service/internal/adapter/mysql/repository_test.go` (MỚI, build tag `integration`)
7. `backend-go/services/notification-service/internal/adapter/mysql/buffered_notification_repository_test.go` (MỚI, build tag `integration`)
8. `backend-go/services/notification-service/internal/adapter/mysql/notification_preference_repository_test.go` (MỚI, build tag `integration`)
9. `backend-go/services/notification-service/internal/adapter/postgres/repository_test.go` (MODIFY — `filepath.Abs` path thêm `/postgres`)
10. `backend-go/services/notification-service/internal/config/config.go` (MODIFY — thêm `DatabaseCredentialsFile`)
11. `backend-go/services/notification-service/cmd/server/main.go` (MODIFY — `switch caps.Dialect` + `toMySQLDriverDSN`)
12. `backend-go/services/notification-service/go.mod`/`go.sum` (MODIFY — `go mod tidy` thêm `go-sql-driver/mysql` direct)
13. `backend-go/services/notification-service/README.md` (MODIFY — migration path + MySQL run instructions)
14. `.github/workflows/backend-go-notification-service.yml` (MỚI)
15. `specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-008-notification-service-mysql-tidb-adapter.md` (MỚI)
16. `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md` (MODIFY — chỉ phần `notification-service` trong ô batch-2)

## Verify

```bash
cd backend-go/services/notification-service
go build ./... && go vet ./...
gofmt -l $(find . -name '*.go')
go test ./...
go test -tags=integration ./internal/adapter/postgres/... -v
go test -tags=integration ./internal/adapter/mysql/... -v
python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-notification-service.yml'))"
```

## gitnexus

`impact()` chạy trên 4 symbol sửa/tương tác (`run`, `Repository`, `Load`,
`Config`) trước khi sửa — cả 4 **LOW**, xem "Kết quả thực tế" mục 1 ở
trên. `detect_changes()` chạy trước khi coi task DONE — xem mục 8/9.
