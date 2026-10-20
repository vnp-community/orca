# TASK-REQ-044-02: Migration `0101_plan_auto_generation`, repository `plan_auto_jobs` và cột `plan_auto_generate`, hai dialect

**From Solution:** [BE-REQ-SOL-044](../solutions/BE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/migrations/{postgres,mysql}/0101_plan_auto_generation.{up,down}.sql` (mới), `internal/usecase/ports_plan_auto.go` (mới), `internal/adapter/postgres/plan_auto_job_repository.go`, `internal/adapter/mysql/plan_auto_job_repository.go` (mới), `internal/adapter/postgres/project_engine_settings_repository.go`, `internal/adapter/mysql/project_engine_settings_repository.go` (sửa), `internal/adapter/contracttest/plan_auto_job_contract.go`, `internal/adapter/contracttest/expected_columns_plan_auto.go` (mới), `internal/adapter/contracttest/expected_columns.go`, `internal/adapter/contracttest/engine_settings_contract.go` (sửa), `internal/adapter/postgres/request_repository_contract_test.go`, `internal/adapter/mysql/request_repository_contract_test.go` (đăng ký)
**Depends on:** TASK-REQ-044-01
**Status:** [x] DONE (2026-10-20)

---

## Context

- Migration mới nhất hiện là `0100_analysis_runs_revision_index` (kiểm lại khi triển khai: CR khác có thể lấy `0101`).
- Mẫu bảng có RLS và khoá duy nhất theo trạng thái: `migrations/postgres/0006_analysis_runs.up.sql` (`analysis_runs_one_running`, chính sách `tenant_isolation`); MySQL dùng cột sinh `active_key` (`migrations/mysql/0006_analysis_runs.up.sql`). Mẫu thêm cột có mặc định: `0098_execution_mode.up.sql` (cả `ALTER TABLE ... ADD COLUMN ... CHECK`).
- Mẫu claim nhiều tenant với `FOR UPDATE SKIP LOCKED`: `AnalysisRunRepository.ClaimExpired` (`adapter/postgres/analysis_run_repository.go:194` dùng `withRelayTx`; `adapter/mysql/analysis_run_repository.go:227` mở transaction trên pool).
- Cài đặt dự án: `ProjectEngineSettingsRepository.Get/Upsert` (`adapter/postgres/project_engine_settings_repository.go`).

## Việc cần làm

1. Migration Postgres `0101_plan_auto_generation.up.sql`: bảng `request.plan_auto_jobs` — `tenant_id UUID NOT NULL`, `request_id UUID NOT NULL REFERENCES request.requests(id)`, `anchor TEXT NOT NULL`, `origin TEXT NOT NULL CHECK (origin IN ('event','retry'))`, `project_id UUID NOT NULL`, `state TEXT NOT NULL CHECK (state IN ('pending','running','committing','done','gave_up','skipped','superseded'))`, `attempt INT NOT NULL DEFAULT 1`, `max_attempts INT NOT NULL DEFAULT 1`, `run_id UUID NULL`, `reason_code TEXT NULL`, `commit_attempts INT NOT NULL DEFAULT 0`, `next_check_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `lease_owner TEXT NULL`, `lease_expires_at TIMESTAMPTZ NULL`, `requested_by UUID NULL`, `created_at`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `PRIMARY KEY (tenant_id, request_id, anchor)`; chỉ mục duy nhất một job hoạt động: `CREATE UNIQUE INDEX plan_auto_jobs_one_active ON request.plan_auto_jobs (tenant_id, request_id) WHERE state IN ('pending','running','committing')`; chỉ mục thăm dò `plan_auto_jobs_due (next_check_at) WHERE state IN ('pending','running','committing')`; chỉ mục `plan_auto_jobs_by_request (tenant_id, request_id, created_at DESC)`; RLS `ENABLE`/`FORCE` + chính sách `tenant_isolation` như `analysis_runs`. Cùng file: `ALTER TABLE request.project_engine_settings ADD COLUMN plan_auto_generate BOOLEAN NOT NULL DEFAULT true`. File `.down.sql` xoá cột và bảng.
2. Migration MySQL cùng nội dung: `VARCHAR(36)` cho id, `VARCHAR(128)` cho `anchor`, `ENUM`/`CHECK` cho `origin` và `state`, `TIMESTAMP(6)`, cột sinh `active_key VARCHAR(80) GENERATED ALWAYS AS (IF(state IN ('pending','running','committing'), request_id, NULL)) STORED` với `UNIQUE KEY plan_auto_jobs_one_active (tenant_id, active_key)`; `ALTER TABLE project_engine_settings ADD COLUMN plan_auto_generate TINYINT(1) NOT NULL DEFAULT 1`. Có `.down.sql`.
3. `usecase/ports_plan_auto.go`: `PlanAutoJobRepository` với `Insert(ctx, job) (inserted bool, err)` (xung đột khoá chính hoặc chỉ mục hoạt động trả `inserted=false`, không lỗi), `LatestByRequest(ctx, requestID) (PlanAutoJob, bool, error)`, `ActiveByRequest(ctx, requestID)`, `ClaimDue(ctx, owner string, ttl time.Duration, batch int) ([]PlanAutoJob, error)` (qua tenant, `SKIP LOCKED`, chỉ job hoạt động có `next_check_at <= now` và lease trống/hết hạn), `Update(ctx, job, owner string) (bool, error)` (ghi `state`, `attempt`, `run_id`, `reason_code`, `commit_attempts`, `next_check_at`, `updated_at` và **xoá lease**; điều kiện `lease_owner = owner`; `false` khi mất lease).
4. Hai repository (Postgres dùng `scoped` cho các lời gọi theo tenant và `withRelayTx` cho `ClaimDue`; MySQL dùng `scoped` và transaction pool cho `ClaimDue`). `ClaimDue` gán `lease_owner`, `lease_expires_at = now + ttl` cho các dòng nhận được.
5. `ProjectEngineSettingsRepository` (hai dialect): `Get` đọc cột mới, `Upsert` ghi cột mới (cả `INSERT` và `UPDATE`).
6. Test hợp đồng: `plan_auto_job_contract.go` (`RunPlanAutoJobContract`) và `expected_columns_plan_auto.go` (bảng `plan_auto_jobs` và cột `plan_auto_generate`), đăng ký ở hai file `request_repository_contract_test.go` (tag `integration`).

## Kiểm thử

- Hợp đồng: chèn rồi đọc lại đủ trường; chèn trùng khoá chính và chèn job thứ hai hoạt động cho cùng Request đều trả `inserted=false`; job cuối (`done`, `gave_up`, `skipped`, `superseded`) không chặn job mới; `ClaimDue` không trả dòng chưa đến hạn hoặc còn lease, hai lời gọi đồng thời không trả trùng một dòng; `Update` với `owner` sai trả `false` và không ghi; cách ly tenant (Postgres RLS; MySQL lọc theo `tenant_id`); `plan_auto_generate` mặc định `true` cho dòng cũ, ghi/đọc `false`.
- Migration xuôi, ngược, xuôi lại trên Postgres 16 và MySQL 8 (`migration_integration_test.go`, `schema_contract_test.go`).
- `cd backend-go/services/request-service && DOCKER_HOST=... go test -tags=integration ./internal/adapter/postgres/ ./internal/adapter/mysql/ -run 'PlanAuto|EngineSettings|Migration|Schema' -count=1 -p 1` (chưa chạy; dùng cấu hình Docker như các task trước).

## Tiêu chí hoàn thành

- [x] Migration xuôi và ngược sạch trên cả hai dialect; dữ liệu cũ đọc được. (`TestPostgres_Migration_UpDownUp`, `TestMySQL_Migration_UpDownUp`, `*_PlanAutoGenerateDefaultsOnForOldRows` xanh)
- [x] Chỉ mục "một job hoạt động" hoạt động ở cả hai dialect (đặc biệt cột sinh của MySQL cho nhiều trạng thái). (`*_PlanAutoJobContract/a_request_has_one_active_job_whatever_its_state` và `a_finished_job_never_blocks...` xanh trên cả hai dialect (cột sinh nhiều trạng thái của MySQL chạy đúng))
- [x] `ClaimDue` an toàn khi hai worker chạy cùng lúc. (`*_PlanAutoJobContract/workers_claiming_at_the_same_time_never_receive_the_same_job` (4 worker, 12 job, mỗi job đúng một lần) và test `ClaimDue` lease xanh)
- [x] Cài đặt dự án cũ đọc ra `plan_auto_generate=true`. (`*_PlanAutoGenerateDefaultsOnForOldRows` và `*_EngineSettingsContract` xanh)
- [x] Test hợp đồng xanh trên Postgres 16 và MySQL 8. (chạy toàn bộ `go test -tags=integration -p 1 ./internal/adapter/postgres/ ./internal/adapter/mysql/` xanh)

## Rủi ro và lưu ý

- Cột sinh MySQL chưa thử với biểu thức nhiều trạng thái: kiểm bằng test hợp đồng trước khi dựa vào.
- `ClaimDue` của MySQL đi qua transaction trên pool, không qua `ctx`: giống `ClaimExpired`, đừng gọi trong transaction của người gọi.
- Số `0101` có thể bị giữ; đổi số kéo theo đổi tên bốn file và cập nhật `migration_files.go` nếu nó liệt kê số.

## Ghi chú triển khai

**Hoàn thành 2026-10-20.** Đường dẫn dưới `backend-go/services/request-service/`.

- Migration `0101_plan_auto_generation.{up,down}.sql` cho `postgres` và `mysql` (số 0101 còn trống lúc làm). Postgres: bảng `request.plan_auto_jobs` với RLS `tenant_isolation`, cộng hai chính sách `relay_scan` (SELECT) và `relay_claim` (UPDATE) theo mẫu `analysis_runs`, vì `ClaimDue` đi qua `withRelayTx` và quét mọi tenant (spec không nhắc, nhưng không có chúng thì worker không thấy dòng nào của app role). Chỉ mục duy nhất `plan_auto_jobs_one_active` (partial theo 3 trạng thái hoạt động). MySQL: cột sinh `active_key VARCHAR(36)` = `IF(state IN (...), request_id, NULL)` với `UNIQUE (tenant_id, active_key)`; `origin` và `state` dùng `CHECK` (không dùng `ENUM`), cột `plan_auto_generate TINYINT(1) NOT NULL DEFAULT 1`. Hai `.down.sql` xoá cột và bảng.
- Mã: `internal/usecase/ports_plan_auto.go` (`PlanAutoJobRepository`; task 04 sẽ thêm `PlanAutoSwitch` vào cùng file), `internal/adapter/{postgres,mysql}/plan_auto_job_repository.go`, và cột `plan_auto_generate` trong `Get`/`Upsert` của `project_engine_settings_repository.go` (hai dialect). Postgres `Insert` dùng `ON CONFLICT DO NOTHING` không có target (phủ cả khoá chính lẫn chỉ mục hoạt động, không làm hỏng transaction của người gọi); MySQL `Insert` bắt lỗi 1062 thay vì `INSERT IGNORE` để không nuốt lỗi khoá ngoại. `ClaimDue` MySQL mở transaction riêng trên pool (không gọi trong transaction của người gọi).
- Test: `internal/adapter/contracttest/plan_auto_job_contract.go` (`RunPlanAutoJobContract`, 12 ca: đọc lại đủ trường, mặc định thời gian, trùng khoá chính, job hoạt động thứ hai ở cả 3 trạng thái, job cuối không chặn, `ClaimDue` bỏ qua chưa đến hạn/đã kết thúc/còn lease và giao lại lease hết hạn, batch qua nhiều tenant, 4 worker song song không nhận trùng, `Update` sai owner/sai tenant trả `false` và không ghi, `Update` đúng owner ghi và xoá lease, cách ly tenant), `expected_columns_plan_auto.go` (bảng `plan_auto_jobs` và cả bảng `project_engine_settings` với cột mới), `migration_split.go` (`SplitMigrationsAt`), ca mới "automatic Plan generation" trong `engine_settings_contract.go`, và `{postgres,mysql}/plan_auto_migration_integration_test.go` (nạp một dòng cài đặt trước 0101, chạy 0101, đọc ra `plan_auto_generate=true`). Đăng ký trong hai `request_repository_contract_test.go`.
- File ngoài danh sách của task nhưng bắt buộc để bộ test hiện có xanh: `internal/domain/erasable_columns.go` (khai báo các cột chữ của `plan_auto_jobs` là exempt: `origin`, `state`, `reason_code`, `lease_owner`, `anchor` ở mọi dialect; `tenant_id`, `request_id`, `project_id`, `run_id`, `requested_by`, `active_key` chỉ MySQL), `internal/adapter/postgres/rls_audit_integration_test.go` (fixture cách ly tenant cho `plan_auto_jobs`), `internal/adapter/mysql/tenant_scope_test.go` (`plan_auto_job_repository.go:ClaimDue` là relay), `internal/domain/engine_settings_test.go` (task 01).
- Lệnh đã chạy (DOCKER_HOST colima như brief): `go test -tags=integration -p 1 ./internal/adapter/postgres/ ./internal/adapter/mysql/ -count=1` toàn bộ xanh (Postgres 16-alpine 50s, MySQL 8.0 137s), gồm `TestPostgres_Migration_UpDownUp`/`TestMySQL_Migration_UpDownUp` (xuôi, ngược, xuôi lại), `*_SchemaContract_InformationSchema`, `*_EngineSettingsContract`, `*_PlanAutoJobContract`, `*_PlanAutoGenerateDefaultsOnForOldRows`, `TestTenantIsolationEveryTable`, `TestForgottenSetConfigReturnsNothing`, `TestTenantScope`, `*_EveryTextColumnDeclaredErasableOrExempt`. Lần chạy đầu bị 4+2 test kiểm toán toàn repo đỏ (fixture, erasable, tenant scope), đã sửa như trên. `go build ./...`, `go vet`, `go test` của domain/usecase/grpc/contracttest xanh.
- Blast radius: `ProjectEngineSettingsRepository.Get/Upsert` được dùng bởi `ManageProjectEngineSettings`, `engine_settings_defaults`, `cmd/server/store_wiring.go`; `Upsert` giờ ghi `s.PlanAutoGenerate` nên người gọi dựng struct bằng literal phải đặt `true` (xem ghi chú task 01, việc của task 04). `usecase.PlanAutoJobRepository` chưa được nối vào `store_wiring.go`/`wire_*` (cố ý, ngoài phạm vi: `cmd/server` thuộc task 06).
- Chưa kiểm: không có gì cần stack. Chưa nối `NewPlanAutoJobRepository` vào store wiring (task 05/06).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
