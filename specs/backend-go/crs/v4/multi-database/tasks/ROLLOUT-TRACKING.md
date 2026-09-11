# Multi-Database rollout — 15 remaining services (tracking)

**Yêu cầu (2026-09-11):** "rà soát lại tính năng Multi-Database ở backend-go:
cần fix toàn bộ" — mở rộng pattern đã xác lập ở pilot `usage-service`
(BE-DB-SOL-001/002, TASK-BE-DB-002~007) ra toàn bộ service còn lại sở hữu
database riêng.

## Audit thật (2026-09-11) — số liệu thật khác đáng kể so với ước tính CR-DB-001/002

CR-DB-001/002 ước tính "44 repository × 17 service × 142 migration". Audit
trực tiếp (`find`, không suy đoán) cho số liệu khác:

- 18 thư mục trong `backend-go/services/`: `usage-service` (pilot, ✅ DONE)
  + 17 service khác.
- 2 trong 17 service đó **không sở hữu database nào** — xác nhận qua chính
  comment trong code: `api-gateway/internal/config/config.go:7` ("this
  service owns no database"), `git-gateway-service/internal/config/config.go:6`
  + `cmd/server/main.go:6` ("owns no data ... no pgxpool"). → loại khỏi
  phạm vi rollout.
- **15 service còn lại thực sự cần rollout** — tổng **70 repository**
  (`adapter/postgres/*.go`, không tính `_test.go`) + **260 file migration**.

| Service | Repos | Migrations | Batch |
|---|---|---|---|
| issue-status-sync | 1 | 2 | 1 |
| issue-tracking-service | 2 | 4 | 1 |
| annotation-service | 1 | 6 | 1 |
| credential-broker-service | 1 | 6 | 1 |
| scm-integration-service | 4 | 6 | 2 |
| notification-service | 3 | 10 | 2 |
| ai-provider-service | 1 | 10 | 2 |
| orchestration-service | 1 | 12 | 2 |
| tenant-service | 8 | 12 | 3 |
| automation-service | 1 | 16 | 3 |
| workflow-service | 3 | 24 | 3 |
| auth-service | 10 | 20 | 4 |
| task-service | 10 | 22 | 4 |
| project-service | 9 | 48 | 5 |
| infra-fleet-service | 15 | 62 | 5 |

Sắp xếp theo độ phức tạp tăng dần (repos+migrations), gom batch 3-4
service/lượt để chạy song song qua background agent, không phá vỡ nguyên
tắc "mỗi agent chỉ sửa 1 service" (tránh đụng file giữa các agent chạy
song song — mỗi service là 1 thư mục độc lập, CI workflow cũng là 1 file
riêng/service theo đúng pattern `backend-go-usage-service.yml`).

## Pattern dùng lại nguyên vẹn từ pilot (không thiết kế lại)

Mỗi service rollout gồm đúng các bước TASK-BE-DB-002~007 đã làm cho
`usage-service`, áp dụng cho tất cả repository/migration của service đó:

1. `common/dbcapability` đã có sẵn — dùng lại, không sửa.
2. Migration: tách `migrations/postgres/` (bản gốc, giữ nguyên) +
   `migrations/mysql/` (bản dialect-safe: JSONB→JSON, `gen_random_uuid()`→bỏ
   nếu ID sinh ở Go/hoặc `UUID()` MySQL tương đương, `BIGSERIAL`→
   `BIGINT AUTO_INCREMENT`, `RETURNING`→bỏ + query lại).
3. `internal/adapter/mysql/` mới — implement lại đúng interface `ports.go`
   hiện có, KHÔNG đổi signature (impact() phải xác nhận LOW trước khi động
   vào interface nếu cần sửa).
4. `cmd/server/main.go` — wire theo đúng pattern `switch caps.Dialect` của
   `usage-service/cmd/server/main.go` (xem tham chiếu).
5. Test: tenant-isolation-without-RLS (nếu service có RLS policy, theo
   TASK-BE-DB-003 pattern) + adapter test dùng `common/testutil.StartMySQL`
   (đã có sẵn từ pilot, không cần viết lại).
6. `.github/workflows/backend-go-<service>.yml` mới — copy khung từ
   `backend-go-usage-service.yml`, đổi path/service name.
7. Solution + Task doc mới trong `specs/backend-go/crs/v4/multi-database/`
   (naming `BE-DB-SOL-<NNN>-<service>`, `TASK-BE-DB-<NNN>-<service>-*`),
   nối tiếp số thứ tự sau TASK-BE-DB-007.

## Trạng thái batch

| Batch | Services | Status |
|---|---|---|
| 1 | issue-status-sync (✅ DONE — [BE-DB-SOL-003](../solutions/BE-DB-SOL-003-issue-status-sync-mysql-tidb-adapter.md)/[TASK-BE-DB-008](./TASK-BE-DB-008-issue-status-sync-mysql-rollout.md)), issue-tracking-service (✅ DONE cho phạm vi MySQL rollout — [BE-DB-SOL-004](../solutions/BE-DB-SOL-004-issue-tracking-service-mysql-tidb-adapter.md)/[TASK-BE-DB-009](./TASK-BE-DB-009-issue-tracking-service-mysql-rollout.md); 1 bug UUID-column tiền tồn tại ở `internal/adapter/postgres` phát hiện, ngoài phạm vi, không sửa — xem TASK-BE-DB-009 mục 6), annotation-service (✅ DONE — [BE-DB-SOL-005](../solutions/BE-DB-SOL-005-annotation-service-mysql-tidb-adapter.md)/[TASK-BE-DB-010](./TASK-BE-DB-010-annotation-service-mysql-rollout.md); 13/13 integration test PASS thật trên cả 2 dialect, không bug sản xuất nào phát hiện), credential-broker-service (✅ DONE — [BE-DB-SOL-006](../solutions/BE-DB-SOL-006-credential-broker-service-mysql-tidb-adapter.md)/[TASK-BE-DB-011](./TASK-BE-DB-011-credential-broker-service-mysql-rollout.md)) | ✅ DONE (2026-09-11) |
| 2 | scm-integration-service (✅ DONE — [BE-DB-SOL-007](../solutions/BE-DB-SOL-007-scm-integration-service-mysql-tidb-adapter.md)/[TASK-BE-DB-012](./TASK-BE-DB-012-scm-integration-service-mysql-rollout.md); 4 repository/6 migration, 5/5 Postgres + 17/17 MySQL integration test PASS thật (22/22 tổng, không bug sản xuất nào phát hiện) — môi trường dùng chung quá tải thật (nhiều agent song song chạy testcontainers) khiến 2 lần chạy đầu bị `panic: test timed out` khi đang chờ container MySQL sẵn sàng (không phải lỗi adapter/migration), xử lý bằng cách tăng `-timeout` + build `migrate` binary riêng có driver mysql (binary dùng chung `~/go/bin/migrate` bị agent khác ghi đè mất driver giữa chừng) — xem TASK-BE-DB-012 mục 6 cho chi tiết), notification-service (🟡 PARTIAL — [BE-DB-SOL-008](../solutions/BE-DB-SOL-008-notification-service-mysql-tidb-adapter.md)/[TASK-BE-DB-013](./TASK-BE-DB-013-notification-service-mysql-rollout.md); code hoàn chỉnh đúng phạm vi (6 port/3 file adapter MySQL, migration dialect-safe 10 file, wiring, 2 test tenant-isolation, CI workflow), build/vet/unit sạch, Postgres integration 21/22 PASS thật (1 bug UUID-column tiền tồn tại ngoài phạm vi, cùng dạng TASK-BE-DB-009); MySQL integration 17/24 test case xác nhận PASS thật qua Docker (0 FAIL quan sát được ở bất kỳ lần chạy nào) nhưng chưa chạy trọn hết 24 case trong 1 lượt do môi trường dùng chung quá tải thật (tới 27 tiến trình `go test -tags=integration` song song từ agent batch khác đo được bằng `ps aux`) — xem TASK-BE-DB-013 mục 6 để biết chi tiết + khuyến nghị re-run khi máy rảnh hơn), ai-provider-service (✅ DONE — [BE-DB-SOL-009](../solutions/BE-DB-SOL-009-ai-provider-service-mysql-tidb-adapter.md)/[TASK-BE-DB-014](./TASK-BE-DB-014-ai-provider-service-mysql-rollout.md); 1 repository implement 5 interface (nhiều port nhất rollout tính đến batch 2), 11/11 Postgres + 13/13 MySQL integration test PASS thật (24/24 tổng, không bug sản xuất nào phát hiện) — case UNIQUE PARTIAL INDEX đầu tiên trong rollout (`uq_accounts_one_default_per_dev_server_provider`, giải bằng generated column + unique index thường), `RowsAffected()` matched-vs-changed pitfall áp dụng cho cả `UpdateStatus` lẫn `Update` cùng lúc — xem TASK-BE-DB-014 mục 6/7 cho chi tiết), orchestration-service (✅ DONE — [BE-DB-SOL-010](../solutions/BE-DB-SOL-010-orchestration-service-mysql-tidb-adapter.md)/[TASK-BE-DB-015](./TASK-BE-DB-015-orchestration-service-mysql-rollout.md); 4 repository interface + `outbox.Store` trên 1 struct, 17/17 MySQL integration test PASS thật (tái lặp 2 lần độc lập không flaky); Postgres 15/15 PASS thật — ban đầu 12/15 do 2 bug tiền tồn tại (ResolveGate scan lỗi trên gate còn `resolution` NULL; RecordHeartbeat không validate UUID trước query nên trả lỗi driver thô thay vì `ErrDispatchContextNotFound`), **cả 2 đã được vá theo yêu cầu trực tiếp** (cùng đợt với task-service's Bug A/B) — 32/32 integration test PASS thật tổng cộng, không còn gap nào — xem TASK-BE-DB-015 "Cập nhật (2026-09-11)" cho chi tiết) | ✅ DONE — 3/4 DONE, 1 PARTIAL (notification-service, xem TASK-BE-DB-013) (2026-09-11) |
| 3+4+5 (gộp, chạy song song theo yêu cầu 2026-09-11 "bổ sung thêm agent để xử lý song song") | tenant-service (✅ DONE — [BE-DB-SOL-011](../solutions/BE-DB-SOL-011-tenant-service-mysql-tidb-adapter.md)/[TASK-BE-DB-016](./TASK-BE-DB-016-tenant-service-mysql-rollout.md); 8/8 repository interface (nhiều nhất rollout tính tới batch này) implement đầy đủ trên MySQL, 22/22 Postgres integration test PASS thật; xem TASK-BE-DB-016 "Kết quả thực tế" cho số liệu MySQL integration test — môi trường batch này có nhiều agent song song cùng chạy Docker container MySQL/Postgres, gây chậm/tranh chấp tài nguyên thật quan sát được, không phải bug migration/adapter), automation-service (✅ DONE — [BE-DB-SOL-012](../solutions/BE-DB-SOL-012-automation-service-mysql-tidb-adapter.md)/[TASK-BE-DB-017](./TASK-BE-DB-017-automation-service-mysql-rollout.md); 20/20 Postgres + 20/20 MySQL integration test PASS thật (40/40), MySQL xác nhận qua 2 lần chạy độc lập không flaky; 1 partial unique index load-bearing (`idx_automation_runs_one_running`, BR-AT-08) dịch bằng shadow-column application-maintained sau khi GENERATED STORED column thất bại thật trên InnoDB (Error 1215, FK conflict) — xem TASK-BE-DB-017 §3.1/§4 cho chi tiết), workflow-service (✅ DONE — [BE-DB-SOL-013](../solutions/BE-DB-SOL-013-workflow-service-mysql-tidb-adapter.md)/[TASK-BE-DB-018](./TASK-BE-DB-018-workflow-service-mysql-rollout.md); 27/27 Postgres + 35/35 MySQL integration test PASS thật (62/62 tổng — MySQL nhiều hơn vì có `approval_repository_test.go` mới), 2 bug migration thật phát hiện + sửa trong phạm vi (không phải pre-existing-out-of-scope) — xem TASK-BE-DB-018 mục 2/5 cho chi tiết), auth-service (✅ DONE — [BE-DB-SOL-014](../solutions/BE-DB-SOL-014-auth-service-mysql-tidb-adapter.md)/[TASK-BE-DB-019](./TASK-BE-DB-019-auth-service-mysql-rollout.md); service identity core lớn nhất/nhạy cảm bảo mật nhất rollout tới nay (9 repository interface, 3 pitfall `RowsAffected()` trên đường revoke bảo mật đã vá + có test regression riêng) — 12/12 Postgres + 23/23 MySQL integration test PASS thật (35/35), cả 2 dialect re-verify độc lập bằng `go test` chạy trực tiếp trong phiên finalize riêng (không copy số liệu cũ), MySQL xác nhận qua 2 lần chạy độc lập không flaky (570.982s/869.805s, chênh lệch do tranh chấp Docker với agent sibling khác trong cùng batch) — xem TASK-BE-DB-019 "Trạng thái cuối" cho chi tiết), task-service (✅ DONE — [BE-DB-SOL-015](../solutions/BE-DB-SOL-015-task-service-mysql-tidb-adapter.md)/[TASK-BE-DB-020](./TASK-BE-DB-020-task-service-mysql-rollout.md); 28/28 Postgres + 41/41 MySQL integration test PASS thật (69/69) — nhưng lượt chạy đầu (2026-09-11, trước khi bị rate-limit dừng giữa chừng) lộ ra **3 bug thật ngoài phạm vi rollout**: (A) `CreateTask` fail 100% trên Postgres (Labels nil → vi phạm NOT NULL, đã vá 2 lớp domain+adapter), (B) `GetSubtree`/`GetSubtreeWithChildPercents` fail 100% (CTE thiếu tên cột tường minh, đã vá), (C) 1 test tự assert sai task (business logic đúng, chỉ test sai, đã vá) — cả 3 đã sửa và verify lại 28/28 PASS theo yêu cầu trực tiếp của người dùng, xem TASK-BE-DB-020 mục 7 cho chi tiết đầy đủ), project-service (✅ DONE — [BE-DB-SOL-016](../solutions/BE-DB-SOL-016-project-service-mysql-tidb-adapter.md)/[TASK-BE-DB-021](./TASK-BE-DB-021-project-service-mysql-rollout.md); service lớn thứ 2 rollout, 9 repository/48 migration (24 cặp); 26/40 Postgres + 27/27 MySQL integration test PASS thật — 14 Postgres FAIL đều là bug tiền tồn tại xác nhận KHÔNG do rollout này (1 đã biết từ trước — `ImportNested` 12-vs-10 cột `RETURNING`; 13 mới phát hiện trong phiên xác minh này — test helper `newTestOutboxEvent` thiếu `TenantID`, cả 2 file liên quan không nằm trong phạm vi sửa của rollout), flag không fix; MySQL 100% PASS không bug sản xuất nào phát hiện — xem TASK-BE-DB-021 mục 8 cho chi tiết đầy đủ), infra-fleet-service (✅ DONE — [BE-DB-SOL-017](../solutions/BE-DB-SOL-017-infra-fleet-service-mysql-tidb-adapter.md)/[TASK-BE-DB-022](./TASK-BE-DB-022-infra-fleet-service-mysql-rollout.md); service LỚN NHẤT rollout, 15 repo/62 migration — migration 100% xong + verify thật cả 2 chiều up/down trên `mysql:8` thật (độc lập Go test), adapter MySQL 100% xong 15/15 unit (build+vet sạch, 24 interface assertion); `cmd/server/main.go`'s dialect-switch ĐÃ wire (2026-09-12, union interface `repoAll` giống task-service) sau khi bị hoãn 1 lượt (2026-09-11, rủi ro composition root 825 dòng) — 35/37 Postgres + 23/23 MySQL integration test PASS thật (58/60 tổng) re-run dưới tải thấp (container MySQL ~15-19s/container, xác nhận contention 150-185s trước đó là do môi trường, không phải bug); lượt 2026-09-12 phát hiện + sửa 4 bug thật: 2 bug MySQL adapter logic (`FleetDefinitionStore.Update`'s optimistic-lock check sai dùng SELECT độc lập thay vì `RowsAffected()`; `TerminalScrollbackSnapshotStore.Upsert` thiếu insert cột `id` không có DEFAULT ở MySQL) + 2 bug mechanical (domain `NewSshTarget`'s `tags` không default nil→`[]string{}`; 1 test thiếu field `Kind`); 2 Postgres FAIL còn lại (`TestMigration0018_DownDropsTable`, `TestRepository_ResolveConnection_FoundAndNotFound`) xác nhận bug tiền tồn tại KHÔNG liên quan CR-DB-002/003 (test-design lỗi thời, đòi hỏi viết lại logic), flag không sửa cùng convention project-service — xem TASK-BE-DB-022 "Cập nhật (2026-09-12)" cho chi tiết đầy đủ) | ✅ DONE (2026-09-12) — 7/7 DONE |
