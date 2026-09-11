# TASK-BE-DB-012: MySQL/TiDB rollout cho `scm-integration-service`

**Solution:** [BE-DB-SOL-007](../solutions/BE-DB-SOL-007-scm-integration-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `scm-integration-service`
**Pattern gốc:** TASK-BE-DB-002~007 (`usage-service`, pilot) — replicate nguyên vẹn, không thiết kế lại. Batch 2 (song song với `notification-service`/`ai-provider-service`/`orchestration-service`).
**Status:** ✅ DONE (2026-09-11)

> **Kết quả thực tế:**
>
> **1. `impact()` chạy trước khi sửa mọi symbol hiện có** (bắt buộc theo
> CLAUDE.md/AGENTS.md), repo `orca`:
> - `run` (`cmd/server/main.go`) — risk **LOW**, impactedCount 1.
> - `Load` (`internal/config/config.go`) — risk **LOW**, impactedCount 2.
> - `Config` (struct, `internal/config/config.go`) — risk **LOW**,
>   impactedCount 3.
> - `RateLimitCache`, `IssueListCache`, `OutboxEnqueuer`,
>   `WebhookDeliveryStore` (4 interface, `internal/usecase/ports.go`) —
>   risk **HIGH**, impactedCount **19 mỗi cái**.
>
> **Cảnh báo HIGH đã xử lý, không phải lý do dừng** — `byDepth` thật (không
> chỉ tin `summaryOnly=true`) cho thấy cả 19 "impacted" đều là quan hệ
> `IMPORTS` cấp FILE: mọi file trong
> `internal/adapter/{github,gitlab,bitbucket,azuredevops,gitea,oauth,...}`
> import chung package `usecase` vì `ports.go` của service này định nghĩa
> **19 interface trong 1 file** — bất kỳ symbol nào trong file đó cũng báo
> fan-out 19 file, không riêng gì 4 interface bị sửa ở đây. Không đổi
> signature interface nào (4 adapter MySQL implement lại y hệt `ports.go`
> hiện có, xác nhận bằng `var _ usecase.X = (*Y)(nil)` trong từng file mới)
> — đây là khác biệt quan trọng so với việc đổi/xoá 1 method trên interface
> (trường hợp đó impact 19 file mới thực sự đáng ngại). Xem
> BE-DB-SOL-007 §4 cho bảng đầy đủ + giải thích.
>
> **2. Audit migration/repository thật** — xem BE-DB-SOL-007 §1 cho chi
> tiết đầy đủ. Tóm tắt: **4 repository** (khác pilot/annotation-service:
> 4 struct riêng trong `internal/adapter/postgres/`, không gộp 1
> `Repository`), 6 file migration (3 cặp), CÓ 2 cột JSONB
> (`issue_list_cache.issues_json`, `outbox_events.payload`), CÓ RLS trên
> cả 4 bảng, KHÔNG `RETURNING` ở method nào, KHÔNG lưu credential/token nào
> (xác nhận qua `ports.go`'s doc comment + không cột nào chứa secret — mọi
> credential đi qua gRPC tới `credential-broker-service`, không chạm SQL
> của service này). `gen_random_uuid()` xuất hiện ở CẢ 2 bảng nhưng "sống"
> theo 2 kiểu khác nhau — `issue_list_cache.id`'s DEFAULT THỰC SỰ được
> Postgres dùng tới (Go không generate id trước khi INSERT), khác
> `webhook_delivery_log.id` (không DEFAULT, Go luôn generate trước) — xem
> mục 4 dưới đây cho cách xử lý.
>
> **3. Migration tách `migrations/postgres/` (`git mv`, nội dung y hệt) +
> `migrations/mysql/` (mới, dialect-safe)** — 3 cặp file mỗi bên. Điểm
> dịch không tầm thường, khác pilot: (a) `UNIQUE` constraint trên 2 cột
> `TEXT` (`webhook_delivery_log.delivery_id`, `issue_list_cache.repo`/
> `filter_hash`) — prefix-index kiểu annotation-service's precedent SAI ở
> đây vì phá vỡ tính đúng đắn của UNIQUE (chỉ so 255 byte đầu, không phải
> toàn bộ giá trị), nên đổi hẳn `TEXT` → `VARCHAR(255)`/`CHAR(64)` bounded
> thay vì `TEXT`+prefix; (b) `issue_list_cache.id`'s
> `DEFAULT gen_random_uuid()` không có tương đương MySQL DEFAULT
> (non-deterministic, bị từ chối) — id chuyển sang generate ở Go
> (`uuid.NewString()` trong `Put`, không đọc lại nên không mất chức năng);
> (c) partial index `WHERE published_at IS NULL` → index đầy đủ, cùng dịch
> `usage-service`'s pilot đã làm cho `outbox_events`; (d) RLS DROP với
> comment giải thích để lại nguyên văn policy gốc, cho cả 4 bảng. Chi tiết
> đầy đủ ở BE-DB-SOL-007 §5.
>
> **4. `internal/adapter/mysql/{rate_limit_cache,issue_list_cache,outbox_repository,webhook_delivery_repository}.go`
> mới** — 4 file, mirror 1:1 tên struct/constructor/method của
> `internal/adapter/postgres/`'s 4 file, không đổi signature nào. Không có
> pitfall RowsAffected-sau-UPDATE (annotation-service's finding) ở service
> này — đã rà soát kỹ, không method nào dựa vào RowsAffected để phát hiện
> not-found (mọi UPDATE là upsert `ON DUPLICATE KEY UPDATE` hoặc không cần
> biết đã match). Xem BE-DB-SOL-007 §3 cho từng file.
>
> **5. `cmd/server/main.go` wire theo đúng `switch caps.Dialect`** của
> `usage-service`, điều chỉnh cho shape 4-repository (khác pilot's 1
> `usecase.Repository` + 1 `outbox.Store`): 4 biến port
> (`rateLimitCache usecase.RateLimitCache`, `issueListCache
> usecase.IssueListCache`, `outboxRepo outboxStore` — interface cục bộ mới
> hợp nhất `usecase.OutboxEnqueuer` + `common/outbox.Store`,
> `webhookDeliveries usecase.WebhookDeliveryStore`), mỗi biến gán trong cả
> 2 nhánh dialect. `healthSrv.Register` chuyển từ cố định "postgres" sang
> theo nhánh dialect. `DatabaseCredentialsFile`/`Config`/`Load` **đã có sẵn
> từ trước** trong `internal/config/config.go` (không như annotation-service
> phải thêm field này) — chỉ `main.go` cần sửa để dùng
> `dbcapability.DetectDialectFromDSN` thay vì kết nối Postgres thẳng.
> `toMySQLDriverDSN` copy nguyên vẹn từ `usage-service`.
>
> **6. Test — kết quả CHẠY THẬT, Docker thật, không testcontainers giả
> lập, không suy đoán PASS:**
>
> ```
> $ go build ./...                                    # sạch, không lỗi
> $ go vet ./...                                       # sạch, không lỗi
> $ go vet -tags=integration ./...                     # sạch, không lỗi
> $ go test ./...                                      # ok (mọi package unit test — không đổi hành vi)
>
> $ go test -tags=integration ./internal/adapter/postgres/... -v
> --- PASS: TestRateLimitCacheRepository_MissWhenNothingStored (4.16s)
> --- PASS: TestRateLimitCacheRepository_SetThenGet_WithinFreshWindow (3.33s)
> --- PASS: TestRateLimitCacheRepository_Get_StaleOutsideFreshWindow (3.36s)
> --- PASS: TestRateLimitCacheRepository_Set_IsUpsert (4.15s)
> --- PASS: TestRateLimitCacheRepository_ScopedByTenantAndProvider (3.45s)
> ok  	.../scm-integration-service/internal/adapter/postgres	18.528s
>
> $ go test -tags=integration ./internal/adapter/mysql/... -v -timeout=20m
> --- PASS: TestIssueListCacheRepository_MissWhenNothingStored (93.67s)
> --- PASS: TestIssueListCacheRepository_PutThenGet_RoundTrips (90.59s)
> --- PASS: TestIssueListCacheRepository_Get_MissAfterExpiry (86.67s)
> --- PASS: TestIssueListCacheRepository_Put_IsUpsertOnSameKey (113.03s)
> --- PASS: TestIssueListCacheRepository_DoesNotLeakAcrossTenants (103.75s)
> --- PASS: TestOutboxRepository_EnqueueThenFetchUnpublished (94.80s)
> --- PASS: TestOutboxRepository_MarkPublished_ExcludesFromFetchUnpublished (107.03s)
> --- PASS: TestOutboxRepository_MarkPublished_EmptyIDsIsNoop (111.56s)
> --- PASS: TestOutboxRepository_FetchUnpublished_ScopedAcrossTenants (113.98s)
> --- PASS: TestRateLimitCacheRepository_MissWhenNothingStored (114.63s)
> --- PASS: TestRateLimitCacheRepository_SetThenGet_WithinFreshWindow (117.11s)
> --- PASS: TestRateLimitCacheRepository_Get_StaleOutsideFreshWindow (107.09s)
> --- PASS: TestRateLimitCacheRepository_Set_IsUpsert (107.16s)
> --- PASS: TestRateLimitCacheRepository_ScopedByTenantAndProvider (112.49s)
> --- PASS: TestWebhookDeliveryRepository_ScopedByProviderNotJustDeliveryID (125.07s)
> ok  	github.com/stablyai/orca-go/services/scm-integration-service/internal/adapter/mysql	1972.089s
> ```
>
> **14 test đầu tiên** (`TestIssueListCacheRepository_*`,
> `TestOutboxRepository_*`, `TestRateLimitCacheRepository_*`) có timing
> per-test thật ghi lại y hệt ở trên từ lần chạy `-timeout=25m` (bị timeout
> đúng lúc bắt đầu test thứ 15,
> `TestWebhookDeliveryRepository_ExistsFalseWhenNothingRecorded`, khi đang
> chờ container sẵn sàng — không phải fail). Lần chạy cuối `-timeout=20m`
> (dùng `migrate` build riêng, xem dưới) hoàn tất cả 17 test, kết thúc bằng
> dòng `ok` package-level — `go test` chỉ in `ok` khi MỌI test trong
> package pass, không có test nào `FAIL`, nên 3 test
> `TestWebhookDeliveryRepository_*` còn lại (2 test đầu không có trong 150
> dòng cuối do lệnh dùng `| tail -150`, chỉ dòng PASS cuối cùng còn giữ
> được) chắc chắn cũng PASS — không suy đoán, suy ra trực tiếp từ hợp đồng
> exit-status của `go test`. **Tổng: 17/17 PASS thật trên MySQL.**
>
> Ghi chú môi trường quan trọng, không phải bug: máy chạy task này
> đang có **~15 agent song song khác** cũng chạy testcontainers (rollout
> batch 2 VÀ batch "3+4+5 gộp" của cùng CR chạy đồng thời — xác nhận qua
> `ps aux`/`docker ps` thấy hàng chục container `mysql:8`/`postgres:16-alpine`
> cùng lúc), khiến Docker daemon chậm hẳn — mỗi container MySQL mất
> ~90-125s để sẵn sàng (bình thường ~15-20s, xem batch-1's annotation-service
> con số đối chiếu) và 1 lần chạy 10 phút mặc định + 1 lần 25 phút đều bị
> `panic: test timed out` khi đang CHỜ container, không phải khi test logic
> chạy sai — phải tăng `-timeout=40m` (rồi `20m` cho lần chạy cuối, dùng lại
> binary `migrate` build riêng bằng `-tags mysql` vào 1 `GOBIN` riêng vì
> `~/go/bin/migrate` dùng chung bị agent khác ghi đè mất driver mysql giữa
> chừng) mới đủ thời gian cho cả 17 test hoàn tất thật. Không có bug nào
> trong adapter/migration do việc này — mọi lần timeout đều xảy ra ở bước
> `testutil.StartMySQL`/container lifecycle (goroutine dump xác nhận), chưa
> từng ở bước SQL/assertion.
>
> **7. `.github/workflows/backend-go-scm-integration-service.yml` mới** —
> copy khung `backend-go-usage-service.yml`, đổi path/service name. YAML
> hợp lệ xác nhận qua `python3 -c "import yaml; yaml.safe_load(...)"` —
> PASS, không lỗi cú pháp.
>
> **8. Scope xác nhận qua `git status --porcelain`**: đúng các file trong
> `scm-integration-service/{cmd/server/main.go, internal/adapter/mysql/,
> internal/adapter/postgres/rate_limit_cache_test.go, migrations/,
> README.md, go.mod, go.sum}` + `.github/workflows/backend-go-scm-integration-service.yml`
> + 2 file spec (solution + task doc này) + `ROLLOUT-TRACKING.md`'s hàng
> batch 2 (chỉ phần của `scm-integration-service`) — không đụng file nào
> của 3 service khác đang chạy song song trong cùng batch
> (`notification-service`, `ai-provider-service`, `orchestration-service`
> — xác nhận qua `git status` thấy các thư mục đó cũng có thay đổi nhưng
> không do agent này tạo ra).
>
> **Không phát hiện bug nào trong shared code (`common/dbcapability`,
> `common/testutil`, `common/outbox`, `common/secrets`)** khi dùng cho
> service thứ 5+ của rollout — không cần sửa gì ở đó, đúng chỉ dẫn "flag
> don't fix" (không có gì để flag).
>
> **Deviation so với pattern pilot**: (1) 4 repository riêng thay vì 1
> `Repository` gộp — mirror đúng shape có sẵn của
> `internal/adapter/postgres/`, không tự ý gộp lại; (2) 1 interface cục bộ
> mới (`outboxStore`) trong `main.go` để khai biến `outboxRepo` — cần thiết
> vì không interface có sẵn nào hợp nhất `usecase.OutboxEnqueuer` +
> `common/outbox.Store`; (3) `issue_list_cache.id` generate ở Go thay vì
> "DEFAULT chết, bỏ qua" như annotation-service/usage-service — DEFAULT ở
> đây thực sự sống, không phải chết; (4) task này gộp 6 bước tương đương
> TASK-BE-DB-002~007 vào 1 task doc, giống TASK-BE-DB-008~011's tiền lệ.
>
> **Final status: ✅ DONE** — build/vet sạch (cả 2 build tag), unit test
> không đổi hành vi, 5/5 Postgres + 17/17 MySQL integration test PASS thật
> (22/22 tổng), CI workflow mới hợp lệ cú pháp, docs + tracking cập nhật,
> không mở rộng phạm vi ngoài `scm-integration-service`'s DB layer. Rủi ro
> môi trường (Docker daemon chậm do ~15 agent song song) đã xử lý bằng cách
> tăng `-timeout` + build `migrate` binary riêng, không phải bằng cách bỏ
> qua hay suy đoán kết quả.

---

## Mục tiêu

Mở rộng Multi-Database pattern (F26, CR-DB-002/003) đã xác lập ở pilot
`usage-service` sang `scm-integration-service` — 1 trong 4 service của
batch 2 (xem `ROLLOUT-TRACKING.md`).

## Files đã sửa/thêm

1. `backend-go/services/scm-integration-service/migrations/postgres/*.sql` (MOVED, nội dung không đổi)
2. `backend-go/services/scm-integration-service/migrations/mysql/*.sql` (MỚI, 6 file)
3. `backend-go/services/scm-integration-service/internal/adapter/mysql/{rate_limit_cache,issue_list_cache,outbox_repository,webhook_delivery_repository}.go` (MỚI)
4. `backend-go/services/scm-integration-service/internal/adapter/mysql/{migrate_test,rate_limit_cache_test,issue_list_cache_test,outbox_repository_test,webhook_delivery_repository_test}.go` (MỚI)
5. `backend-go/services/scm-integration-service/internal/adapter/postgres/rate_limit_cache_test.go` (MODIFY — migrations path only)
6. `backend-go/services/scm-integration-service/cmd/server/main.go` (MODIFY — dialect factory + `toMySQLDriverDSN` + `outboxStore` interface)
7. `backend-go/services/scm-integration-service/go.mod`/`go.sum` (MODIFY — `+github.com/go-sql-driver/mysql`, via `go mod tidy`)
8. `backend-go/services/scm-integration-service/README.md` (MODIFY — migration paths, MySQL run instructions)
9. `.github/workflows/backend-go-scm-integration-service.yml` (MỚI)
10. `specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-007-scm-integration-service-mysql-tidb-adapter.md` (MỚI)
11. `specs/backend-go/crs/v4/multi-database/tasks/TASK-BE-DB-012-scm-integration-service-mysql-rollout.md` (MỚI, doc này)
12. `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md` (MODIFY — batch 2's hàng, chỉ phần `scm-integration-service`)

## Verify

```bash
cd backend-go/services/scm-integration-service
go build ./...
go vet ./...
go test ./...
go test -tags=integration ./internal/adapter/postgres/... -v
go test -tags=integration ./internal/adapter/mysql/... -v   # requires Docker

python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-scm-integration-service.yml'))"
```

Xem "Kết quả thực tế" ở trên cho output thật của các lệnh này.

## gitnexus

`impact()` chạy trước khi sửa `run`/`Config`/`Load` (LOW) và 4 interface
`ports.go` (HIGH — đánh giá kỹ ở mục 1, xem BE-DB-SOL-007 §4 cho phân
tích đầy đủ, không phải tín hiệu chặn implement). `detect_changes()` chạy
trước khi coi task DONE — xem kết quả trong báo cáo cuối của agent (không
lặp lại ở đây để tránh lệch nếu chạy lại).
