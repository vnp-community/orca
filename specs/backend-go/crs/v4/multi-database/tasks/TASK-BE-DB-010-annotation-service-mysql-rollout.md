# TASK-BE-DB-010: MySQL/TiDB rollout cho `annotation-service`

**Solution:** [BE-DB-SOL-005](../solutions/BE-DB-SOL-005-annotation-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `annotation-service`
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
> - `Repository` (interface, `internal/usecase/ports.go`) — risk **LOW**,
>   impactedCount 2.
>
> Không có symbol nào HIGH/CRITICAL — tiến hành sửa không cần cảnh báo
> người dùng thêm. Không đổi signature `Repository` interface nào (adapter
> MySQL implement y hệt interface hiện có).
>
> **2. Audit migration/repository thật** (annotation-service được ghi
> "chưa audit" trong CR-DB-001/002 gốc) — xem BE-DB-SOL-005 §1 cho chi
> tiết đầy đủ. Tóm tắt: 1 repository, 6 file migration (3 cặp), KHÔNG
> JSONB, CÓ RLS (1 policy), CÓ `gen_random_uuid()` (nhưng không được ứng
> dụng dùng tới — id sinh ở Go, giống phát hiện của `usage-service`), CÓ
> `RETURNING` ở `UpdateAnnotation`/`MarkSent` (không có ở `CreateAnnotation`/
> `DeleteAnnotation`).
>
> **3. Migration tách `migrations/postgres/` (`git mv`, nội dung y hệt) +
> `migrations/mysql/` (mới, dialect-safe)** — 3 cặp file mỗi bên. Điểm
> dịch không tầm thường (không có trong `usage-service`'s migration):
> `TEXT` không giới hạn độ dài trên `repo_id`/`file_path` không index được
> trực tiếp trên MySQL/InnoDB (giới hạn 3072 byte/key) → dùng prefix index
> `(255)`; không có partial index (`WHERE worktree_id IS NOT NULL`) →
> index đầy đủ thay thế. Chi tiết đầy đủ ở BE-DB-SOL-005 §5.
>
> **4. `internal/adapter/mysql/repository.go` mới** — implement lại đúng
> `usecase.Repository` (7 method), không đổi signature. Phát hiện quan
> trọng khi dịch `UpdateAnnotation`/`MarkSent` (không có RETURNING):
> dịch ngây thơ "check `RowsAffected() == 0` rồi SELECT lại" SAI vì MySQL
> driver mặc định đếm "dòng đổi giá trị", không phải "dòng khớp WHERE" —
> một retry hợp lệ resend cùng `content`/`resolved` sẽ bị báo nhầm
> `ErrAnnotationNotFound`. Đã tránh bằng cách luôn UPDATE rồi SELECT lại
> theo `(tenant_id, id)` mà không đọc `RowsAffected()` để quyết định
> not-found — xem BE-DB-SOL-005 §3.1 và test
> `TestRepository_UpdateAnnotation_NoopRetryStillSucceeds` (PASS thật,
> mục 6 dưới đây). `DeleteAnnotation` không bị ảnh hưởng (DELETE's
> RowsAffected đếm theo WHERE-match ở mọi dialect, không có ambiguity
> này). `MarkSent`'s `id = ANY($1)` → `IN (?,...)` động, guard
> `len(ids) == 0` (MySQL `IN ()` là lỗi cú pháp).
>
> **5. `cmd/server/main.go` wire theo đúng `switch caps.Dialect`** của
> `usage-service` — thêm `DatabaseCredentialsFile` vào `Config`
> (`internal/config/config.go`), đổi `main.go` từ đọc thẳng
> `cfg.DatabaseDSN` sang `secrets.DatabaseCredentialsFromFile` +
> `dbcapability.DetectDialectFromDSN`, đóng luôn README's "Known gap"
> "`common/secrets` (Vault) is not wired into `main.go`" — gap này đã tồn
> tại từ trước, đóng như một tác dụng phụ hợp lý của việc wiring dialect
> factory (không phải mở rộng phạm vi: cùng 1 đoạn code cần sửa để chọn
> dialect). `toMySQLDriverDSN` copy nguyên vẹn từ `usage-service` (thuần
> DSN-plumbing, không đặc thù service nào).
>
> **6. Test — kết quả CHẠY THẬT, Docker thật, không testcontainers giả
> lập, không suy đoán PASS:**
>
> ```
> $ go build ./...                                    # sạch, không lỗi
> $ go vet ./...                                       # sạch, không lỗi
> $ go test ./...                                      # ok (domain, usecase — cached từ trước, không đổi)
> $ go test -tags=integration ./internal/adapter/postgres/... -v
> --- PASS: TestRepository_CreateAndListAnnotations_FiltersByTenant (6.97s)
> --- PASS: TestRepository_UpdateAnnotation_NotFoundReturnsSentinel (5.04s)
> --- PASS: TestRepository_DeleteAnnotation_NotFoundReturnsSentinel (8.12s)
> --- PASS: TestRepository_FindByRequestID_RoundTripsAndScopesToTenant (6.16s)
> --- PASS: TestRepository_MarkSent_UpdatesExactlyGivenIDsAndSkipsMissing (4.78s)
> ok  	.../annotation-service/internal/adapter/postgres	31.152s
>
> $ go test -tags=integration ./internal/adapter/mysql/... -v
> --- PASS: TestRepository_CreateAndListAnnotations_FiltersByTenant (22.60s)
> --- PASS: TestRepository_ListAnnotations_DoesNotLeakAcrossTenants (22.83s)
> --- PASS: TestRepository_UpdateAnnotation_NotFoundReturnsSentinel (17.84s)
> --- PASS: TestRepository_UpdateAnnotation_NoopRetryStillSucceeds (21.83s)
> --- PASS: TestRepository_DeleteAnnotation_NotFoundReturnsSentinel (16.83s)
> --- PASS: TestRepository_FindByRequestID_RoundTripsAndScopesToTenant (16.46s)
> --- PASS: TestRepository_MarkSent_UpdatesExactlyGivenIDsAndSkipsMissing (19.11s)
> --- PASS: TestRepository_MarkSent_EmptyIDsIsNoop (15.61s)
> ok  	.../annotation-service/internal/adapter/mysql	153.216s
> ```
>
> 5/5 Postgres + 8/8 MySQL — tất cả PASS thật (Postgres 16-alpine, MySQL
> 8, testcontainers-go, container thật khởi/dừng mỗi test, log xác nhận
> trong output gốc). 8 test MySQL = 5 test mirror 1:1 từ Postgres + 3 mới:
> `TestRepository_ListAnnotations_DoesNotLeakAcrossTenants` (TASK-BE-DB-003
> pattern — dữ liệu 2 tenant CÙNG `repo_id`/`file_path`, xác nhận
> `ListAnnotations` VÀ `GetAnnotation` không rò rỉ chéo tenant),
> `TestRepository_UpdateAnnotation_NoopRetryStillSucceeds` (mục 4),
> `TestRepository_MarkSent_EmptyIDsIsNoop` (mirror `usage-service`'s
> `MarkPublished` empty-guard test). Chạy lại lần 2 (`go test` cache) cho
> kết quả nhất quán, không có test nào flake.
>
> **7. `.github/workflows/backend-go-annotation-service.yml` mới** — copy
> khung `backend-go-usage-service.yml`, đổi path/service name. YAML hợp
> lệ xác nhận qua `python3 -c "import yaml; yaml.safe_load(...)"` — PASS,
> không lỗi cú pháp. Không thể verify "Actions tab chạy thật" trong phiên
> này (không có quyền mở PR/push) — nhưng khác với `usage-service`'s ban
> đầu PARTIAL (bug SQL tiền tồn tại chặn lane Postgres), **annotation-service
> không có bug tương tự đã biết** — cả 2 lane dùng đúng lệnh đã chạy PASS
> thật cục bộ ở mục 6, nên rủi ro CI đỏ khi mở PR thật là thấp, dù chưa
> quan sát trực tiếp Actions log.
>
> **8. Scope xác nhận qua `git status --porcelain`**: đúng các file trong
> `annotation-service/{cmd/server/main.go, internal/config/config.go,
> internal/adapter/mysql/, internal/adapter/postgres/repository_test.go,
> migrations/, README.md, go.mod, go.sum}` +
> `.github/workflows/backend-go-annotation-service.yml` + 2 file spec
> (solution + task doc này) + `ROLLOUT-TRACKING.md`'s hàng
> `annotation-service` — không đụng file nào của 3 service khác đang chạy
> song song trong cùng batch (`issue-status-sync`, `issue-tracking-service`,
> `credential-broker-service` — xác nhận qua `git status` thấy các thư mục
> đó cũng có thay đổi nhưng không do agent này tạo ra) hay
> `api-gateway`/`notification-service`/frontend (đang bị task khác — F08
> Annotate AI Diffs — chạm vào cùng lúc, ngoài phạm vi task này, không
> đụng tới).
>
> **Không phát hiện bug nào trong shared code (`common/dbcapability`,
> `common/testutil`)** khi dùng cho service thứ 2 — không cần sửa gì ở
> đó, đúng chỉ dẫn "flag don't fix" (không có gì để flag).
>
> **Deviation duy nhất so với pattern pilot**: task này gộp 6 bước tương
> đương TASK-BE-DB-002~007 vào 1 task doc thay vì 6 file riêng — theo yêu
> cầu của nhiệm vụ gốc (BE-DB-SOL-005 + TASK-BE-DB-010, không nhân bản 6
> task number cho mỗi service trong rollout 15-service, tránh bùng nổ số
> lượng file spec).
>
> **Final status: ✅ DONE** — build/vet/unit sạch, 13/13 integration test
> PASS thật trên cả 2 dialect, CI workflow mới hợp lệ cú pháp, docs +
> tracking cập nhật, không mở rộng phạm vi ngoài annotation-service's DB
> layer.

---

## Mục tiêu

Mở rộng Multi-Database pattern (F26, CR-DB-002/003) đã xác lập ở pilot
`usage-service` sang `annotation-service` — 1 trong 15 service còn lại của
rollout (xem `ROLLOUT-TRACKING.md`, batch 1).

## Files đã sửa/thêm

1. `backend-go/services/annotation-service/migrations/postgres/*.sql` (MOVED, nội dung không đổi)
2. `backend-go/services/annotation-service/migrations/mysql/*.sql` (MỚI, 6 file)
3. `backend-go/services/annotation-service/internal/adapter/mysql/repository.go` (MỚI)
4. `backend-go/services/annotation-service/internal/adapter/mysql/repository_test.go` (MỚI)
5. `backend-go/services/annotation-service/internal/adapter/postgres/repository_test.go` (MODIFY — migrations path only)
6. `backend-go/services/annotation-service/internal/config/config.go` (MODIFY — `+DatabaseCredentialsFile`)
7. `backend-go/services/annotation-service/cmd/server/main.go` (MODIFY — dialect factory + `toMySQLDriverDSN`)
8. `backend-go/services/annotation-service/go.mod`/`go.sum` (MODIFY — `+github.com/go-sql-driver/mysql`, via `go mod tidy`)
9. `backend-go/services/annotation-service/README.md` (MODIFY — migration paths, MySQL run instructions, closes Vault "Known gap")
10. `.github/workflows/backend-go-annotation-service.yml` (MỚI)
11. `specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-005-annotation-service-mysql-tidb-adapter.md` (MỚI)
12. `specs/backend-go/crs/v4/multi-database/tasks/TASK-BE-DB-010-annotation-service-mysql-rollout.md` (MỚI, doc này)
13. `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md` (MODIFY — hàng `annotation-service` only)

## Verify

```bash
cd backend-go/services/annotation-service
go build ./...
go vet ./...
go test ./...
go test -tags=integration ./internal/adapter/postgres/... -v
go test -tags=integration ./internal/adapter/mysql/... -v   # requires Docker

python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-annotation-service.yml'))"
```

Xem "Kết quả thực tế" ở trên cho output thật của các lệnh này.

## gitnexus

`impact()` chạy trước khi sửa `run`/`Config`/`Load`/`Repository` — cả 4
risk **LOW** (chi tiết ở "Kết quả thực tế" mục 1). `detect_changes()` chạy
trước khi coi task DONE — xem kết quả trong báo cáo cuối của agent (không
lặp lại ở đây để tránh lệch nếu chạy lại).
