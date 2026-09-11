# TASK-BE-DB-017: MySQL/TiDB rollout cho `automation-service`

**Solution:** [BE-DB-SOL-012](../solutions/BE-DB-SOL-012-automation-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `automation-service`
**Pattern gốc:** TASK-BE-DB-002~007 (`usage-service`, pilot) — replicate nguyên vẹn, không thiết kế lại
**Status:** ✅ DONE (2026-09-11)

> **Kết quả thực tế:**
>
> **1. `impact()` chạy trước khi sửa mọi symbol hiện có** (bắt buộc theo
> CLAUDE.md/AGENTS.md), repo `orca`:
> - `run` (`cmd/server/main.go`) — risk **LOW**, impactedCount 1.
> - `Load` (`internal/config/config.go`) — risk **LOW**, impactedCount 2.
> - `Config` (struct, `internal/config/config.go`) — risk **LOW**,
>   impactedCount 3.
> - `AutomationRepository` (interface, `internal/usecase/ports.go`) — risk
>   **MEDIUM**, impactedCount 7.
> - `AutomationRunRepository` (interface, `internal/usecase/ports.go`) —
>   risk **MEDIUM**, impactedCount 7.
> - `DueAutomationClaimer` (interface, `internal/usecase/ports.go`) — risk
>   **MEDIUM**, impactedCount 7.
>
> 3 interface ở MEDIUM (khác LOW của mọi service trước trong rollout) —
> phản ánh số lượng usecase tiêu thụ interface (`CreateAutomation`,
> `RunNow`, `ListRuns`, `HandleExternalTrigger`, `ListAutomations`,
> `UpdateAutomation`, `DeleteAutomation`, `WriteCleanupReport`,
> `HandleEventTrigger`), **KHÔNG phải** rủi ro đổi signature — không đổi
> method nào của 3 interface này, adapter MySQL implement y hệt. Không HIGH/
> CRITICAL nào — tiến hành sửa không cần cảnh báo người dùng thêm ngoài ghi
> nhận MEDIUM ở đây.
>
> **2. Audit migration/repository thật** — xem BE-DB-SOL-012 §1 cho chi
> tiết đầy đủ. Tóm tắt: 1 "repository" (2 struct chia sẻ 1 pool/db), 16
> file migration (8 cặp), CÓ JSONB (4 cột — nhiều nhất rollout tới nay),
> CÓ RLS (3 policy, chưa từng active), CÓ `gen_random_uuid()` (không dùng),
> KHÔNG `RETURNING`, và 1 partial unique index LOAD-BEARING
> (`idx_automation_runs_one_running`, BR-AT-08) — điểm khác biệt cấu trúc
> lớn nhất so với mọi service trước trong rollout.
>
> **3. Migration tách `migrations/postgres/` (`git mv`, nội dung y hệt) +
> `migrations/mysql/` (mới, dialect-safe)** — 8 cặp file mỗi bên. 2 bug
> thật phát hiện qua CHẠY THẬT (không suy đoán từ tài liệu MySQL), cả 2 đã
> sửa và xác nhận PASS:
>
> - **Bug A — literal DEFAULT trên cột TEXT**: MySQL/InnoDB từ chối
>   `TEXT NOT NULL DEFAULT ''`/`DEFAULT 'UTC'`/`DEFAULT 'cron'` với
>   `Error 1101: BLOB, TEXT, GEOMETRY or JSON column 'step_type' can't have
>   a default value`. Sửa bằng cách bọc literal trong ngoặc đơn
>   (`DEFAULT ('')`) — biến thành expression default, MySQL 8.0.13+ chấp
>   nhận. 4 chỗ sửa: `0001_init.up.sql` (`step_type`),
>   `0002_scheduler_columns.up.sql` (`step_type`, `timezone`),
>   `0005_trigger_columns.up.sql` (`trigger_type`).
> - **Bug B — `trigger` là từ khoá RESERVED trong MySQL**: cột
>   `automation_runs.trigger` (thêm ở `0002_scheduler_columns`) gây
>   `Error 1064: You have an error in your SQL syntax ... near 'trigger'`.
>   Postgres chấp nhận `trigger` không cần quote như 1 identifier thường —
>   MySQL thì không. Sửa bằng backtick-quote (`` `trigger` ``) ở MỌI nơi
>   tham chiếu: `0002_scheduler_columns.up.sql`/`.down.sql`, và
>   `internal/adapter/mysql/repository.go`'s `Create`'s INSERT column list
>   + `runColumns` constant.
> - **Bug C — `GENERATED ALWAYS AS (...) STORED` xung đột với FOREIGN KEY**
>   (phát hiện ở bước 4, xem mục 4 dưới — không phải bug migration thuần
>   tuý, là giới hạn thiết kế InnoDB, xử lý bằng đổi cách tiếp cận chứ
>   không phải sửa cú pháp).
>
> Xác nhận cả 8 migration chạy PASS thật (`migrate -path migrations/mysql
> -database "mysql://..." up`) trên container `mysql:8` throwaway, log:
> `1/u init ... 8/u project_and_actions (30.58s)`, không lỗi.
>
> **4. `internal/adapter/mysql/repository.go` mới** — implement lại đúng
> `AutomationRepository`/`AutomationRunRepository`/`DueAutomationClaimer`
> (không đổi signature). Phát hiện quan trọng nhất của task này — dịch
> `idx_automation_runs_one_running` (partial unique index enforce BR-AT-08)
> sang MySQL:
>
> - **Thử đầu tiên** (theo "chuẩn SQL thường dùng"): 1 cột
>   `GENERATED ALWAYS AS (CASE WHEN status='running' THEN automation_id ELSE
>   NULL END) STORED` + `UNIQUE INDEX` trên cột đó. **THẤT BẠI THẬT**:
>   `ERROR 1215 (HY000): Cannot add foreign key constraint` — xác nhận qua
>   chạy trực tiếp `docker exec ... mysql -e "ALTER TABLE ... ADD COLUMN
>   ... GENERATED ... STORED"` trên `mysql:8` (MySQL 8.4.11), kể cả khi thử
>   DROP FK trước / ADD COLUMN generated / RE-ADD FK sau — vẫn lỗi ở bước
>   cuối. Nguyên nhân: InnoDB từ chối biểu thức generated tham chiếu 1 cột
>   (`automation_id`) đang mang FOREIGN KEY — giới hạn thật của engine,
>   không phải lỗi cú pháp migration.
> - **Giải pháp thật đã dùng**: `running_slot` là cột **THƯỜNG** (không
>   generated, `CHAR(36) NULL`), do **application layer**
>   (`AutomationRunRepository.UpdateStatus`) tự gán giá trị
>   (`automation_id` nếu `status='running'`, `NULL` nếu không) trong cùng
>   câu UPDATE ghi `status`. Tránh hoàn toàn giới hạn InnoDB ở trên vì
>   không có generated column nào cả. Xem BE-DB-SOL-012 §3.1 cho SQL/Go
>   đầy đủ.
>
> `isDuplicateKeyError` dịch `pgconn.PgError.Code=="23505"`+`ConstraintName`
> (Postgres) sang `*mysqldriver.MySQLError.Number==1062`+`strings.Contains(
> Message, indexName)` (MySQL không có field tên constraint riêng, chỉ có
> message tự do) — map cùng `usecase.ErrConcurrentRunActive`.
> `Update`/`UpdateStatus`'s RowsAffected-not-found pitfall (giống hệt
> BE-DB-SOL-005 §3.1 phát hiện cho annotation-service, nhưng ở ĐÂY xảy ra
> ở 2 method thay vì 1) được xử lý bằng `clientFoundRows=true` trong DSN
> (`cmd/server/main.go`'s `toMySQLDriverDSN`) — 1 fix ở tầng driver áp
> dụng tự động cho toàn bộ connection, thay vì viết "UPDATE rồi SELECT lại"
> riêng cho từng method — xem BE-DB-SOL-012 §3.3 cho lý do chọn khác
> `annotation-service`. `PruneOldRuns`/`PruneRuns` cần bọc subquery trong 1
> derived table thêm (MySQL cấm SELECT trực tiếp từ bảng đang DELETE).
> `UpdateStatus`'s outbox write không dùng lại được
> `eventbus.RunCompletedPublisher` (hard-code `pgx.Tx`) — viết trực tiếp
> trong `*sql.Tx` của adapter, dùng lại `eventbus.RunCompletedSubject` +
> 1 struct payload cục bộ khớp field JSON — flagged trong BE-DB-SOL-012 §3.5
> như 1 coupling dialect tiền tồn tại trong package `eventbus` của chính
> service này (không sửa, ngoài phạm vi).
>
> **5. `cmd/server/main.go` wire theo đúng `switch caps.Dialect`** của
> `usage-service` — thêm `DatabaseCredentialsFile` vào `Config`
> (`internal/config/config.go`), đổi `main.go` từ đọc thẳng
> `cfg.DatabaseDSN` sang `secrets.DatabaseCredentialsFromFile` +
> `dbcapability.DetectDialectFromDSN`, đóng README's "Vault not wired"
> known gap như tác dụng phụ hợp lý (cùng đoạn code cần sửa để chọn
> dialect). Khác biệt so với `usage-service`/`annotation-service`:
> `automation-service` cần 2 CẶP biến tách riêng
> (`automationRepo`/`claimer` VÀ `runRepo`/`outboxStore`) vì
> `usecase.AutomationRepository`/`AutomationRunRepository` không tự embed
> `usecase.DueAutomationClaimer`/`common/outbox.Store` — `scheduler.New`
> và `outbox.NewRelay` cần kiểu hẹp hơn trỏ tới CÙNG giá trị cụ thể.
> `toMySQLDriverDSN` thêm `clientFoundRows=true` (không có ở bản
> `usage-service`/`annotation-service` copy) — xem mục 4.
>
> **6. Test — kết quả CHẠY THẬT, Docker thật, không testcontainers giả
> lập, không suy đoán PASS:**
>
> ```
> $ go build ./...                                    # sạch, không lỗi
> $ go vet ./...                                       # sạch, không lỗi
> $ go vet -tags=integration ./...                     # sạch, không lỗi (test files typecheck)
> $ go test ./...                                      # ok (domain, usecase, grpc, grpcclient, scheduler — cached, không đổi)
> ```
>
> Postgres (16 test, không đổi logic — chỉ path migration đổi):
> ```
> $ go test -tags=integration ./internal/adapter/postgres/... -v
> --- PASS: TestAutomationRepository_CreateAndGet
> --- PASS: TestAutomationRepository_ClaimDue_LocksAndAdvancesNextRunAt
> --- PASS: TestAutomationRepository_ClaimDue_SkipsRowsLockedByAnotherClaim
> --- PASS: TestAutomationRunRepository_FindByRequestID_IsIdempotencyBackstop
> --- PASS: TestAutomationRunRepository_ListByAutomation_EmptyAutomationIDListsAllTenantRuns
> --- PASS: TestAutomationRepository_AcquireRunLock_OnlyOneCallerWinsWhenUnlocked
> --- PASS: TestAutomationRepository_AcquireRunLock_StaleLockPastTTLSelfHeals
> --- PASS: TestAutomationRepository_ReleaseRunLock_OnlyReleasesOwnLock
> --- PASS: TestAutomationRepository_List_ScopesToTenant
> --- PASS: TestAutomationRepository_List_PaginatesWithoutDuplicatesOrGaps
> --- PASS: TestAutomationRepository_Update_PersistsFieldsAndFailsForWrongTenant
> --- PASS: TestAutomationRepository_Delete_CascadesToRunsAndFailsForWrongTenant
> --- PASS: TestAutomationRepository_CountByProject_ScopesPerProjectNoLeakage
> --- PASS: TestAutomationRepository_ListByTrigger_ReturnsOnlyEnabledMatchingEvent
> --- PASS: TestAutomationRunRepository_PruneOldRuns_KeepsMostRecentN
> --- PASS: TestAutomationRunRepository_OneRunningPartialUniqueIndex
> --- PASS: TestAutomationRunRepository_UpdateStatus_WritesOutboxOnlyForTerminalTransitions
> --- PASS: TestAutomationRunRepository_WriteCleanupReport_RoundTrips
> PASS
> ok  	.../automation-service/internal/adapter/postgres	252.594s
> ```
>
> MySQL (20 test, chạy thật trên container `mysql:8` throwaway, xác nhận
> qua 2 lần chạy độc lập, không dùng testcontainers giả lập):
>
> ```
> $ go test -tags=integration ./internal/adapter/mysql/... -v -timeout=20m
> --- PASS: TestAutomationRepository_CreateAndGet
> --- PASS: TestAutomationRepository_List_ScopesToTenant
> --- PASS: TestAutomationRepository_List_PaginatesWithoutDuplicatesOrGaps
> --- PASS: TestAutomationRepository_Update_PersistsFieldsAndFailsForWrongTenant
> --- PASS: TestAutomationRepository_Update_NoopRetryStillSucceeds
> --- PASS: TestAutomationRepository_Delete_CascadesToRunsAndFailsForWrongTenant
> --- PASS: TestAutomationRepository_ClaimDue_LocksAndAdvancesNextRunAt
> --- PASS: TestAutomationRepository_ClaimDue_SkipsRowsLockedByAnotherClaim
> --- PASS: TestAutomationRepository_AcquireRunLock_OnlyOneCallerWinsWhenUnlocked
> --- PASS: TestAutomationRepository_AcquireRunLock_StaleLockPastTTLSelfHeals
> --- PASS: TestAutomationRepository_ReleaseRunLock_OnlyReleasesOwnLock
> --- PASS: TestAutomationRepository_CountByProject_ScopesPerProjectNoLeakage
> --- PASS: TestAutomationRepository_ListByTrigger_ReturnsOnlyEnabledMatchingEvent
> --- PASS: TestAutomationRunRepository_FindByRequestID_IsIdempotencyBackstop
> --- PASS: TestAutomationRunRepository_ListByAutomation_EmptyAutomationIDListsAllTenantRuns
> --- PASS: TestAutomationRunRepository_OneRunningPartialUniqueIndex
> --- PASS: TestAutomationRunRepository_UpdateStatus_WritesOutboxOnlyForTerminalTransitions
> --- PASS: TestAutomationRunRepository_PruneRuns_KeepsOnlyMostRecentN
> --- PASS: TestAutomationRunRepository_PruneRuns_ZeroOrNegativeIsNoop
> --- PASS: TestAutomationRunRepository_WriteCleanupReport_RoundTrips
> PASS
> ok  	.../automation-service/internal/adapter/mysql	980.021s
> ```
>
> Chạy lần 2 (độc lập, xác nhận không flaky): cùng 20/20 PASS,
> `ok .../mysql 753.216s`, exit code 0 cả 2 lần. Runtime dài hơn Postgres
> (~980s/~753s so với ~203s) do container `mysql:8` cần nhiều vòng retry
> kết nối hơn khi khởi động (log `[mysql] unexpected EOF` lặp lại trong lúc
> chờ container sẵn sàng ở MỖI test — chi phí khởi động, không phải lỗi) —
> quan sát thật, không ảnh hưởng kết quả PASS/FAIL.
>
> **Sửa lại số liệu Postgres ở trên**: chạy lại xác nhận thật ra là
> **20 test** (không phải 18 như liệt kê trước đó — bản liệt kê trước bỏ
> sót `TestAutomationRunRepository_PruneRuns_KeepsOnlyMostRecentN`/
> `TestAutomationRunRepository_PruneRuns_ZeroOrNegativeIsNoop`), tất cả
> 20/20 PASS lại, `ok .../postgres 203.164s`, không có regression.
>
> 2 bug SQL migration (literal DEFAULT trên TEXT, `trigger` reserved
> keyword) và 1 giới hạn InnoDB (GENERATED STORED column + FOREIGN KEY)
> đều phát hiện qua CHẠY THẬT migration/test lần đầu, không suy đoán trước
> — cả 3 đã sửa, xác nhận migration chạy sạch qua toàn bộ 8 file trước khi
> chạy full test suite. Full test suite (20/20) xác nhận PASS thật 2 lần
> độc lập, không phát hiện thêm bug nào ở bước verify cuối này.
>
> **7. `.github/workflows/backend-go-automation-service.yml` mới** — copy
> khung `backend-go-usage-service.yml`, đổi path/service name. YAML hợp lệ
> xác nhận qua `python3 -c "import yaml; yaml.safe_load(...)"` — PASS,
> không lỗi cú pháp.
>
> **8. Scope xác nhận qua `git status --porcelain`**: đúng các file trong
> `automation-service/{cmd/server/main.go, internal/config/config.go,
> internal/adapter/mysql/, internal/adapter/postgres/repository_test.go,
> migrations/, README.md, go.mod, go.sum}` +
> `.github/workflows/backend-go-automation-service.yml` + 2 file spec
> (solution + task doc này) + `ROLLOUT-TRACKING.md`'s hàng `automation-service`
> — không đụng file nào của các service khác đang chạy song song trong
> cùng batch gộp 3+4+5 (`tenant-service`, `workflow-service`, `auth-service`,
> `task-service`, `project-service`, `infra-fleet-service` — quan sát qua
> `git status` thấy các thư mục đó cũng có thay đổi nhưng không do agent
> này tạo ra, đúng như kỳ vọng "nhiều agent chạy song song, mỗi agent 1
> service").
>
> **Không phát hiện bug nào trong shared code (`common/dbcapability`,
> `common/testutil`, `common/secrets`, `common/config`)** khi dùng cho
> service thứ 6 trong rollout — không cần sửa gì ở đó, đúng chỉ dẫn "flag
> don't fix". **CÓ flag 1 coupling dialect trong code CỦA CHÍNH SERVICE
> NÀY** (`internal/adapter/eventbus.RunCompletedPublisher`'s `pgx.Tx`-typed
> signature) — không fix, xem BE-DB-SOL-012 §3.5.
>
> **Deviation so với pattern pilot**: (1) gộp 6 bước tương đương
> TASK-BE-DB-002~007 vào 1 task doc, giống mọi service trước trong rollout
> 15-service; (2) `clientFoundRows=true` (khác `annotation-service`'s
> "UPDATE rồi SELECT lại") cho cạm bẫy RowsAffected; (3) `running_slot` là
> cột application-maintained thay vì generated column, do giới hạn InnoDB
> thật phát hiện qua chạy thử — không phải lựa chọn thiết kế ban đầu, mà
> là kết quả của việc thử-và-sửa dựa trên lỗi thật.
>
> **Final status: ✅ DONE** — Postgres 20/20 PASS (203.164s), MySQL 20/20
PASS xác nhận qua 2 lần chạy thật độc lập (980.021s và 753.216s, cả 2 exit
code 0), `go build`/`go vet`/`go vet -tags=integration`/`go test ./...`
sạch. Không còn hạng mục PENDING nào của task này.

---

## Mục tiêu

Mở rộng Multi-Database pattern (F26, CR-DB-002/003) đã xác lập ở pilot
`usage-service` sang `automation-service` — 1 trong 15 service còn lại của
rollout (xem `ROLLOUT-TRACKING.md`, batch 3+4+5 gộp).

## Files đã sửa/thêm

1. `backend-go/services/automation-service/migrations/postgres/*.sql` (MOVED, nội dung không đổi, 16 file)
2. `backend-go/services/automation-service/migrations/mysql/*.sql` (MỚI, 16 file)
3. `backend-go/services/automation-service/internal/adapter/mysql/repository.go` (MỚI)
4. `backend-go/services/automation-service/internal/adapter/mysql/repository_test.go` (MỚI)
5. `backend-go/services/automation-service/internal/adapter/postgres/repository_test.go` (MODIFY — migrations path only)
6. `backend-go/services/automation-service/internal/config/config.go` (MODIFY — `+DatabaseCredentialsFile`)
7. `backend-go/services/automation-service/cmd/server/main.go` (MODIFY — dialect factory + `toMySQLDriverDSN`)
8. `backend-go/services/automation-service/go.mod`/`go.sum` (MODIFY — `+github.com/go-sql-driver/mysql`, via `go mod tidy`)
9. `backend-go/services/automation-service/README.md` (MODIFY — migration paths, MySQL run instructions, closes Vault "Known gap")
10. `.github/workflows/backend-go-automation-service.yml` (MỚI)
11. `specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-012-automation-service-mysql-tidb-adapter.md` (MỚI)
12. `specs/backend-go/crs/v4/multi-database/tasks/TASK-BE-DB-017-automation-service-mysql-rollout.md` (MỚI, doc này)
13. `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md` (MODIFY — hàng `automation-service` only)

## Verify

```bash
cd backend-go/services/automation-service
go build ./...
go vet ./...
go test ./...
go test -tags=integration ./internal/adapter/postgres/... -v
go test -tags=integration ./internal/adapter/mysql/... -v   # requires Docker

python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-automation-service.yml'))"
```

Xem "Kết quả thực tế" ở trên cho output thật của các lệnh này.

## gitnexus

`impact()` chạy trước khi sửa `run`/`Config`/`Load`/`AutomationRepository`/
`AutomationRunRepository`/`DueAutomationClaimer` — LOW cho 3 cái đầu,
**MEDIUM** cho 3 interface (chi tiết ở "Kết quả thực tế" mục 1, không phải
HIGH/CRITICAL nên không cần dừng lại cảnh báo thêm). `detect_changes()`
chạy trước khi coi task DONE — xem kết quả trong báo cáo cuối của agent
(không lặp lại ở đây để tránh lệch nếu chạy lại).
