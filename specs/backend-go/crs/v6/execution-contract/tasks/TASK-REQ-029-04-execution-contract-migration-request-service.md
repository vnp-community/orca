# TASK-REQ-029-04: Migration `execution_contract` ở `request-service` và repository `execution_packets`, `task_readiness_reports`

**From Solution:** [BE-REQ-SOL-029](../solutions/BE-REQ-SOL-029-execution-contract-and-readiness-gate.md) mục 2.G
**Priority:** P0
**Service/Area:** `request-service` (mới) / migration hai dialect, domain, adapter postgres và mysql
**File:** `backend-go/services/request-service/migrations/postgres/NNNN_execution_contract.{up,down}.sql` (mới), `migrations/mysql/NNNN_execution_contract.{up,down}.sql` (mới), `internal/domain/task_readiness_report.go` (mới), `internal/domain/execution_packet_record.go` (mới), `internal/usecase/execution_contract_ports.go` (mới), `internal/adapter/postgres/execution_packet_repository.go`, `readiness_report_repository.go` (mới), bản `mysql` tương ứng, và các `_test.go`
**Depends on:** TASK-REQ-001-04 (`TxRunner`, executor trong ctx), TASK-REQ-013-03 (migration `phase_starts`, `task_run_outcomes`), TASK-REQ-014-01 (`request_checks`)
**Status:** [x] DONE

---

## Context

- `ls backend-go/services/request-service` báo không tồn tại ngày 2026-10-06. Số migration của `request-service` **chồng nhau** giữa các CR 001, 002, 004, 006 (feature request-service-foundation: `0001_init`, `0002_request_core`; CR khác cũng nhắc `0002_processed_events`, `0003_request_source_hints`, `0005_outbox`, `NNNN_phase_execution`, `NNNN_request_checks`). Vì vậy `NNNN` ở đây **bắt buộc đọc thư mục `migrations/` lúc làm** và lấy số lớn nhất cộng một; thứ tự chạy phải sau `phase_execution` (cần `task_run_outcomes`) và `request_checks`.
- `task_run_outcomes` được tạo ở SOL-013 (`TASK-REQ-013-03-phase-execution-migration-and-repositories.md`): `(id, tenant_id, request_id, task_id, container_id NULL, outcome CHECK, cause, execution_link_id NULL, error_message, event_id, occurred_at)`; solution này **thêm cột** bằng `ALTER TABLE`, không sửa file của 013-03.
- `request_checks` (CR-REQ-014 mục 2.1, `TASK-REQ-014-01`): có cột `source`; CR-REQ-029 yêu cầu thêm giá trị `orca_verified`. Nếu cột là `CHECK (source IN (...))` thì phải bỏ và tạo lại constraint; nếu là `TEXT` tự do thì chỉ cần cập nhật tài liệu. **Đọc `request_checks` migration thật để biết dạng.**
- Quy ước chung (README v6 mục 8 điều 3): mọi bảng có `tenant_id`; không FK sang `task-service`; `outbox_events`, `processed_events` đã có từ CR-REQ-001. Postgres schema `request`, RLS thật theo mẫu `mcp-service` (`FORCE`, `NULLIF(current_setting('app.tenant_id', true), '')::uuid`, `set_config` mỗi giao dịch) như SOL-001 mục 2.D; MySQL lọc `tenant_id` ở mọi `WHERE`.
- JSON: Postgres `JSONB`, MySQL `JSON` không DEFAULT literal. `body` của packet cắt 128 KB: `TEXT` Postgres, `MEDIUMTEXT` MySQL.

## Việc cần làm

1. Chạy `ls migrations/postgres migrations/mysql` và `cat` migration `phase_execution`, `request_checks`; chốt `NNNN`.
2. Bảng `request.execution_packets`: `id UUID PK`, `tenant_id UUID NOT NULL`, `request_id UUID NOT NULL REFERENCES request.requests(id)`, `task_id UUID NOT NULL` (không FK), `attempt INT NOT NULL`, `spec_digest CHAR(64) NOT NULL`, `template_version VARCHAR(16) NOT NULL`, `digest CHAR(64) NOT NULL`, `input_digest CHAR(64) NOT NULL`, `nonce_hash CHAR(64) NOT NULL`, `body TEXT NOT NULL`, `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`:
   - `UNIQUE (tenant_id, task_id, attempt)`
   - chỉ mục `(tenant_id, request_id, created_at DESC)`.
3. Bảng `request.task_readiness_reports`: `id`, `tenant_id`, `request_id` (FK nội bộ), `task_id`, `attempt INT`, `worktree_id UUID NULL`, `outcome TEXT CHECK (outcome IN ('ready','needs_info','spec_defect','env_defect'))`, `tier TEXT CHECK (tier IN ('structure','semantic','environment'))`, `findings JSONB NOT NULL`, `base_sha CHAR(40) NULL`, `head_sha CHAR(40) NULL`, `spec_digest CHAR(64) NOT NULL`, `baseline JSONB NULL`, `duration_ms INT NOT NULL`, `dry_run BOOLEAN NOT NULL DEFAULT FALSE`, `created_at`; chỉ mục `(tenant_id, request_id, task_id, created_at DESC)` và `(tenant_id, worktree_id, head_sha)`. Append-only (không có `UPDATE` trong repository).
4. `ALTER TABLE request.task_run_outcomes ADD COLUMN failure_class TEXT NULL CHECK (failure_class IN ('retryable','needs_info','spec_defect','env_defect','agent_defect')), ADD COLUMN verdict JSONB NULL, ADD COLUMN execution_record_id UUID NULL;` (MySQL: `VARCHAR(16)`, `JSON`, `CHAR(36)`, `ALTER ... ADD CONSTRAINT ... CHECK`).
5. `request_checks.source`: thêm `orca_verified` vào CHECK (Postgres: `DROP CONSTRAINT ... ADD CONSTRAINT ...` đặt tên rõ; MySQL: `ALTER TABLE ... DROP CHECK ..., ADD CONSTRAINT ...`; nếu không có CHECK thì bỏ bước).
6. `down`: bỏ ba cột của `task_run_outcomes`, trả CHECK của `request_checks` về cũ, `DROP TABLE` hai bảng mới (theo thứ tự ngược). Test `up -> down -> up` sạch.
7. Domain: `TaskReadinessReport{ID, TenantID, RequestID, TaskID string; Attempt int; WorktreeID string; Outcome ReadinessOutcome; Tier ReadinessTier; Findings []ReadinessFinding; BaseSHA, HeadSHA, SpecDigest string; Baseline []BaselineResult; DurationMS int; DryRun bool; CreatedAt time.Time}`, `ReadinessFinding{Code, Tier, Path, Message string}` (mã ổn định, **không** chứa giá trị biến môi trường: `Validate()` từ chối `Message` quá 500 ký tự), `BaselineResult{CheckID string; Exit int; TimedOut bool; TailSHA string}`. `ExecutionPacketRecord{...}` tương ứng cột.
8. Port `execution_contract_ports.go`: `ExecutionPacketRepository.Insert(ctx, rec) error` (nuốt vi phạm UNIQUE `(tenant_id, task_id, attempt)` thành `ErrAlreadyExists` để `AdvanceExecution` lặp không tạo hai packet), `Get(ctx, tenantID, taskID string, attempt int)`; `ReadinessReportRepository.Insert(ctx, rep) error`, `Latest(ctx, tenantID, taskID string) (TaskReadinessReport, bool, error)`, `ListByTasks(ctx, tenantID string, taskIDs []string) ([]TaskReadinessReport, error)` (báo cáo mới nhất mỗi task; usecase lấy `task_ids` của Phase từ `task-service` rồi gọi, không join chéo service), `LatestBaseline(ctx, tenantID, worktreeID, headSHA string, notBefore time.Time) ([]BaselineResult, bool, error)`.
9. Cài hai adapter; mọi truy vấn qua executor trong ctx (`exec(ctx)`) và `WHERE tenant_id = ...`.

## Kiểm thử

- Integration hai dialect: `TestExecutionPacketRepository_InsertGet`, `_DuplicateAttemptReturnsAlreadyExists`, `_BodyUnicode128KB`, `TestReadinessReportRepository_InsertLatest`, `_LatestIsNewest`, `_LatestBaselineByWorktreeAndSha`, `_BaselineNotBeforeFilter`, `_FindingsJSONRoundTrip`, `_TenantIsolation`, `TestTaskRunOutcomes_NewColumnsNullable`, `TestRequestChecks_AcceptsOrcaVerified`, `TestMigrationNNNN_UpDownUp`, và Postgres: `TestRLS_ExecutionContractTables` với role không phải superuser.
- Unit: `TestTaskReadinessReport_Validate_RejectsLongMessage`, `_RejectsEnvValueLikeMessage` (heuristic: `Message` chứa `=` kèm tên biến chữ hoa trong `requires.env_names` thì lỗi, để bắt thói quen nhét giá trị).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/...` rồi `go test -tags=integration ./services/request-service/internal/adapter/...`.

## Tiêu chí hoàn thành

- [x] `up`, `down`, `up` sạch trên Postgres 14+ và MySQL 8.0.16+ sau các migration của SOL-013 và SOL-014.
- [x] Hai `Insert` packet cùng `(task_id, attempt)`: một thành công, một `ErrAlreadyExists`.
- [x] Báo cáo sẵn sàng append-only; `LatestBaseline` trả đúng bản mới nhất theo `(worktree_id, head_sha)` và theo `notBefore`.
- [x] Không FK sang `task-service`; mọi bảng có `tenant_id`.
- [x] Số `NNNN` ghi trong mô tả PR kèm danh sách migration tại thời điểm làm.

## Thứ tự thực hiện gợi ý

1. Commit 1: chỉ migration `up`/`down` hai dialect kèm test `UpDownUp`; chưa có code Go dùng bảng. Dễ review và dễ revert nếu số `NNNN` va chạm.
2. Commit 2: domain (`TaskReadinessReport`, `ExecutionPacketRecord`) và port; chưa có adapter.
3. Commit 3: adapter Postgres kèm integration test; commit 4: adapter MySQL cùng bộ test bảng.
4. Commit 5: `ALTER TABLE task_run_outcomes` và `request_checks.source` **chỉ** khi đã đọc xong migration thật của TASK-REQ-013-03 và 014-01; nếu hai tệp đó chưa merge thì tách phần `ALTER` sang migration riêng chạy sau và ghi rõ ở PR.

## Kiểm tra thủ công sau khi xong

- `psql`: `\d request.task_readiness_reports` kiểm hai chỉ mục và CHECK; `SELECT * FROM pg_policies WHERE tablename IN ('execution_packets','task_readiness_reports')` kiểm RLS.
- MySQL: `SHOW CREATE TABLE task_readiness_reports` kiểm `CHECK` và kiểu `JSON` không DEFAULT.
- Chạy `migrate ... down 1` rồi `up` trên DB đã có dữ liệu mẫu (vài dòng packet và báo cáo) để chắc `down` không làm hỏng bảng khác.

## Điểm cần người review soi kỹ

- Mọi `CREATE TABLE` có `tenant_id NOT NULL` và mọi chỉ mục bắt đầu bằng `tenant_id` (README v6 mục 8 điều 3); không FK sang `task-service` (kiểm bằng `grep REFERENCES` chỉ thấy bảng nội bộ).
- `down` không `DROP` thứ không thuộc migration này (đặc biệt `task_run_outcomes`, `request_checks`: chỉ bỏ cột/constraint đã thêm).
- Không có DEFAULT literal cho cột JSON ở MySQL; mọi `CHECK` đặt tên rõ để `DROP CONSTRAINT` ở `down` không phải đoán tên.

## Rủi ro và lưu ý

- Chồng số migration là rủi ro điều phối: hai PR cùng lấy `NNNN`. Đồng bộ qua người điều phối; không merge hai migration cùng số.
- `request_checks` CHECK không xác định được tên (Postgres sinh `request_checks_source_check` nếu inline; MySQL tên do mình đặt ở 014-01): đọc file thật trước khi viết `DROP`.
- `body` 128 KB × mọi lần thử: chưa có chính sách dọn; ghi vào câu hỏi mở của SOL-029 (Q5).
- Cột `dry_run` ngoài CR (CR chỉ nói `CheckReadiness` "ghi báo cáo"); thêm để lọc khi đo tỉ lệ qua cổng lần đầu (chạy khô không được tính vào chỉ số).
