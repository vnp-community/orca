# TASK-BE-DB-018: MySQL/TiDB rollout cho `workflow-service`

**Solution:** [BE-DB-SOL-013](../solutions/BE-DB-SOL-013-workflow-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `workflow-service`
**Pattern gốc:** TASK-BE-DB-002~007 (`usage-service`, pilot) — replicate nguyên vẹn, không thiết kế lại
**Status:** ✅ DONE (2026-09-11)

> **Kết quả thực tế:**
>
> **1. `impact()` chạy thật trước khi sửa mọi symbol hiện có** (bắt buộc
> theo CLAUDE.md/AGENTS.md), repo `orca`: `TemplateRepository`/
> `ApprovalRepository`/`ExecutionRepository`/`StepExecutionRepository`
> (interface, `ports.go`) → **HIGH** (impactedCount 16 mỗi cái) — kiểm tra
> chi tiết (không dừng ở summary) xác nhận toàn bộ depth=1 là quan hệ
> `IMPORTS` cấp FILE (mọi file trong `internal/adapter/*` import package
> `usecase`), không phải phụ thuộc chữ ký method cụ thể — cùng dạng
> false-positive-do-file-fan-out đã ghi nhận ở TASK-BE-DB-009. Không đổi
> signature interface nào → an toàn tiếp tục, đã CẢNH BÁO rõ trong
> BE-DB-SOL-013 §2 theo đúng yêu cầu "MUST warn user if HIGH/CRITICAL".
> `New` (`adapter/postgres/repository.go`) → LOW (2). `NewApprovalStore`
> (`adapter/postgres/approval_repository.go`) → LOW (2). `run`
> (`cmd/server/main.go`) → LOW (1).
>
> **2. Migration split** — 24 file `migrations/*.sql` → `git mv` nguyên
> vẹn vào `migrations/postgres/`; 24 file mới trong `migrations/mysql/`.
> Điểm dịch không tầm thường nhất trong cả rollout tới nay: `tags TEXT[]`
> → `tags JSON` + `JSON_CONTAINS`-per-tag (không có tiền lệ array type ở
> service nào trước đó), partial unique index cho
> `idx_workflow_approvals_one_pending_per_template` → generated column +
> UNIQUE INDEX, `WITH RECURSIVE`/row-value subquery đều dịch gần như
> nguyên văn (MySQL 8/TiDB hỗ trợ sẵn). Chi tiết đầy đủ ở BE-DB-SOL-013 §3.
>
> **2 bug thật phát hiện VÀ SỬA khi verify thật** (không phải suy đoán —
> tái hiện trực tiếp bằng container `mysql:8` độc lập trước khi tin vào
> kết quả `go test`):
> - **Error 1215 "Cannot add foreign key constraint"**: MySQL/InnoDB (bản
>   `mysql:8` resolve ra 8.4.11) từ chối `ON DELETE CASCADE` trên 1 FK mà
>   cột của nó cũng là base column của 1 STORED generated column khác
>   trong cùng bảng (`approvals.template_id` vừa là cột FK vừa là input
>   của `pending_template_id`). Sửa: bỏ `ON DELETE CASCADE` (dùng default
>   RESTRICT) — không mất chức năng thật, `TemplateRepository` không có
>   method Delete nào (grep xác nhận), CASCADE này chưa từng có đường kích
>   hoạt. Xem `migrations/mysql/0009_template_visibility_sharing.up.sql`'s
>   comment đầy đủ.
> - **Error 1553 "Cannot drop index ... needed in a foreign key
>   constraint"**: `down` migration gốc của `0003_template_parent_chain`
>   DROP INDEX trước khi DROP FOREIGN KEY phụ thuộc index đó. Sửa: đổi thứ
>   tự (FK trước, index sau). Xác nhận bằng cách chạy trọn `migrate up` rồi
>   `migrate down -all` trên cùng 1 container thật, cả 2 chiều pass.
>
> **3. `internal/adapter/mysql/{repository,template_tx,approval_repository}.go`
> mới** — implement lại đúng cả 4 interface (16 method tổng cộng dồn trên
> `Repository`/`templateTx`/`ApprovalStore`/`approvalTx`), KHÔNG đổi
> signature nào. MySQL RowsAffected() pitfall (BE-DB-SOL-005 §3.1's phát
> hiện) áp dụng RỘNG ở service này (nhiều write-by-key hơn mọi service đã
> rollout) — giải quyết bằng `SELECT ... FOR UPDATE` bên trong transaction
> trước mỗi UPDATE thay vì UPDATE-rồi-SELECT-lại của BE-DB-SOL-005 (mạnh
> hơn — không có khoảng hở TOCTOU). `approvalTx.CreateTx` map MySQL error
> 1062 (`ER_DUP_ENTRY`) → `domain.ErrApprovalAlreadyPending`, tương đương
> `pgUniqueViolation` (SQLSTATE 23505) của bản Postgres.
>
> **4. Wiring `cmd/server/main.go`** — `switch caps.Dialect` đúng khuôn
> `usage-service`; đóng gap "Vault wiring not wired in this scaffold" như
> tác dụng phụ hợp lý (thêm `DatabaseCredentialsFile` vào
> `internal/config/config.go`, dùng `secrets.DatabaseCredentialsFromFile`
> thay đọc thẳng `cfg.DatabaseDSN`). `repo` cần thoả đồng thời 4 interface
> (`TemplateRepository`+`ExecutionRepository`+`StepExecutionRepository`+
> `outbox.Store`) — định nghĩa interface cục bộ `workflowStore` gộp cả 4,
> khác pilot's đơn giản hơn (1 `usecase.Repository`). `toMySQLDriverDSN`
> copy nguyên vẹn từ `usage-service`.
>
> **5. Test MySQL adapter — 35 test mới**, 4 file
> (`repository_test.go` 20, `template_tx_test.go` 4, `list_templates_test.go`
> 6, `approval_repository_test.go` 5 — file cuối KHÔNG có tiền lệ Postgres,
> viết mới hoàn toàn để xác nhận generated-column unique index hoạt động
> đúng cả 2 chiều), mirror phần lớn test Postgres hiện có + 4 test
> `DoesNotLeakAcrossTenants`/`_NotFound` mới theo TASK-BE-DB-003's pattern
> (`GetTemplate`, `ListTemplates`, `ListPending`, `UpdateVisibility`).
>
> **Build/test thật đã chạy** (Docker + testcontainers thật, môi trường
> đang chịu tải RẤT nặng — ~10 agent song song khác cũng chạy
> testcontainers MySQL/Postgres của riêng họ cùng lúc, mỗi container khởi
> động 30-100s thay vì ~5-10s bình thường — không giả định PASS, chờ hoàn
> tất thật từng lần, 1 lần đầu bị Go's default `-timeout=10m` cắt ngang
> giữa chừng do tải, chạy lại với `-timeout=90m` để có kết quả thật đầy
> đủ):
> ```
> go build ./...                                     → sạch
> go vet ./...                                        → sạch
> gofmt -l internal/adapter/mysql/*.go cmd/server/main.go internal/config/config.go → sạch
> go test ./... (unit)                                → PASS toàn bộ (domain/usecase/adapter khác — cached, không đổi)
> go mod tidy                                         → +github.com/go-sql-driver/mysql v1.10.1 (go.mod/go.sum)
>
> go test -tags=integration -timeout=30m ./internal/adapter/postgres/... -v
> ok  	.../workflow-service/internal/adapter/postgres	657.492s
> → 27/27 PASS thật (Postgres 16-alpine, testcontainers-go)
>
> go test -tags=integration -timeout=90m ./internal/adapter/mysql/... -v
> ok  	.../workflow-service/internal/adapter/mysql	1518.788s
> → 35/35 PASS thật (MySQL 8 container, resolve 8.4.11)
>
> python3 -c "import yaml; yaml.safe_load(open('.github/workflows/backend-go-workflow-service.yml'))" → PASS, YAML hợp lệ
> ```
>
> 62/62 integration test PASS thật trên cả 2 dialect (27 Postgres + 35
> MySQL — MySQL nhiều hơn vì có `approval_repository_test.go` mới không
> tồn tại ở Postgres). Không có test nào FAIL sau khi 2 bug migration ở
> mục 2 được sửa.
>
> **6. `git status --porcelain` xác nhận scope** — đúng các file trong
> `workflow-service/{cmd/server/main.go, internal/config/config.go,
> internal/adapter/mysql/, internal/adapter/postgres/repository_test.go,
> migrations/, README.md, go.mod, go.sum}` +
> `.github/workflows/backend-go-workflow-service.yml` + 2 file spec (solution
> + task doc này) — không đụng file nào của 6 service khác đang chạy song
> song trong cùng batch 3+4+5 (`tenant-service`, `automation-service`,
> `auth-service`, `task-service`, `project-service`, `infra-fleet-service`)
> hay batch 2 đang chạy (`scm-integration-service`, `notification-service`,
> `ai-provider-service`, `orchestration-service`) — xác nhận qua `ps aux`
> thấy các process test riêng của họ chạy song song, không do agent này
> tạo ra.
>
> **Không phát hiện bug nào trong shared code** (`common/dbcapability`,
> `common/testutil`) — không cần sửa gì ở đó, đúng chỉ dẫn "flag don't
> fix" (không có gì để flag).
>
> **`detect_changes()`** chạy thật (scope `unstaged`, repo-wide do nhiều
> agent khác đang sửa song song — lọc riêng phần `workflow-service`): các
> symbol tracked-file bị đổi đúng như kỳ vọng (`README.md`'s section,
> `main.go`'s `run`, `postgres/repository_test.go`'s `setupRepository`,
> `config.go`'s `Load`/`splitCSV`) — không có symbol bất ngờ nào. File mới
> (untracked: `internal/adapter/mysql/*.go`, `migrations/mysql/*.sql`)
> không hiện trong git-diff-based tool này (dự kiến, xác nhận đúng đắn
> bằng build/vet/test thật thay vì công cụ này).
>
> **Final status: ✅ DONE** — build/vet/unit sạch, 62/62 integration test
> PASS thật trên cả 2 dialect (kể cả sau khi tìm và sửa 2 bug migration
> thật), CI workflow mới hợp lệ cú pháp, docs + tracking cập nhật, không
> mở rộng phạm vi ngoài `workflow-service`'s DB layer. 2 bug MySQL/InnoDB
> tìm thấy (generated-column + FK CASCADE restriction, FK/index drop
> order) là phát hiện MỚI, chưa từng gặp ở 8 service đã rollout trước đó
> trong CR-DB-002/003 — ghi lại đầy đủ ở BE-DB-SOL-013 cho các service sau
> tham khảo nếu gặp pattern tương tự (partial-unique-index-via-generated-
> column + FK CASCADE trên cùng cột).

---

## Mục tiêu

Mở rộng Multi-Database pattern (F26, CR-DB-002/003) đã xác lập ở pilot
`usage-service` sang `workflow-service` — 1 trong 15 service của rollout
(xem `ROLLOUT-TRACKING.md`, batch 3+4+5, chạy song song với 6 service khác
qua các agent riêng biệt). `workflow-service` là service phức tạp nhất
được rollout tới nay trong đợt này: 24 migration, 4 interface repository
(`TemplateRepository`+tx, `ApprovalRepository`+tx, `ExecutionRepository`,
`StepExecutionRepository`) dồn trên 3 file adapter.

## Files đã sửa/thêm

1. `backend-go/services/workflow-service/migrations/postgres/*.sql` (MOVED — `git mv`, nội dung không đổi, 24 file)
2. `backend-go/services/workflow-service/migrations/mysql/*.sql` (MỚI, 24 file)
3. `backend-go/services/workflow-service/internal/adapter/mysql/repository.go` (MỚI)
4. `backend-go/services/workflow-service/internal/adapter/mysql/template_tx.go` (MỚI)
5. `backend-go/services/workflow-service/internal/adapter/mysql/approval_repository.go` (MỚI)
6. `backend-go/services/workflow-service/internal/adapter/mysql/{repository,template_tx,list_templates,approval_repository}_test.go` (MỚI, 4 file)
7. `backend-go/services/workflow-service/internal/adapter/postgres/repository_test.go` (MODIFY — `migrationsPath` trỏ `migrations/postgres`)
8. `backend-go/services/workflow-service/internal/config/config.go` (MODIFY — `+DatabaseCredentialsFile`)
9. `backend-go/services/workflow-service/cmd/server/main.go` (MODIFY — dialect factory, `workflowStore` union interface, `toMySQLDriverDSN`, đóng gap Vault wiring)
10. `backend-go/services/workflow-service/go.mod`/`go.sum` (MODIFY — `+github.com/go-sql-driver/mysql`, via `go mod tidy`)
11. `backend-go/services/workflow-service/README.md` (MODIFY — migration paths, MySQL run instructions)
12. `.github/workflows/backend-go-workflow-service.yml` (MỚI)
13. `specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-013-workflow-service-mysql-tidb-adapter.md` (MỚI)
14. `specs/backend-go/crs/v4/multi-database/tasks/TASK-BE-DB-018-workflow-service-mysql-rollout.md` (MỚI, doc này)
15. `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md` (MODIFY — hàng `workflow-service` trong batch 3+4+5, chỉ nối thêm)

## Test cases cover

Mirror phần lớn `adapter/postgres`'s test hiện có (repository CRUD/pagination/
version-conflict/step-execution/outbox, `list_templates_test.go`'s FTS/tags/
trending/recent/rating, `template_tx_test.go`'s WithTx commit/rollback) +
test MỚI không có ở Postgres: `approval_repository_test.go` (không có tiền
lệ Postgres — CreateTx/Get/Update/ListPending + xác nhận generated-column
unique index hoạt động đúng cả 2 chiều) + các test
`DoesNotLeakAcrossTenants` (TASK-BE-DB-003 pattern) cho `templates`/
`approvals` (2 trong 4 bảng có RLS ở Postgres, `step_executions`'s tenant-
join test đã có sẵn từ trước, mirror lại).

## Verify

```bash
cd backend-go/services/workflow-service
go build ./... && go vet ./...
gofmt -l internal/adapter/mysql/*.go cmd/server/main.go internal/config/config.go
go test ./...
go test -tags=integration -timeout=30m ./internal/adapter/mysql/... -v
go test -tags=integration -timeout=30m ./internal/adapter/postgres/... -v
python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-workflow-service.yml'))"
```

Xem "Kết quả thực tế" ở đầu file cho output thật của các lệnh này.

## gitnexus

`impact()` chạy thật trước khi sửa `TemplateRepository`/`ApprovalRepository`/
`ExecutionRepository`/`StepExecutionRepository` (`ports.go`), `New`
(`adapter/postgres/repository.go`), `NewApprovalStore`
(`adapter/postgres/approval_repository.go`), `run` (`cmd/server/main.go`)
— 4 interface báo **HIGH** (impactedCount 16 mỗi cái), `New`/
`NewApprovalStore` **LOW** (2), `run` **LOW** (1). Chi tiết đầy đủ + giải
thích vì sao HIGH ở đây an toàn tiếp tục (toàn bộ depth=1 là `IMPORTS` cấp
file, không phải phụ thuộc chữ ký — không đổi signature interface nào) ở
BE-DB-SOL-013 §2. `detect_changes()` chạy trước khi coi task DONE — xem
báo cáo cuối của agent.
