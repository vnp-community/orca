# TASK-REQ-007-01: Migration `analysis_runs` (Postgres và MySQL) và domain `AnalysisRun`

**From Solution:** [BE-REQ-SOL-007](../solutions/BE-REQ-SOL-007-solution-generation-options-and-selection.md) mục B, C
**Priority:** P0
**Service/Area:** `request-service` / migrations, domain
**File:** `backend-go/services/request-service/migrations/{postgres,mysql}/NNNN_analysis_runs.{up,down}.sql` (mới), `internal/domain/analysis_run.go` (mới), `internal/domain/analysis_run_test.go` (mới)
**Depends on:** CR-REQ-002 (`requests`, `solutions` đã có)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08, kiểm lại sau đánh số lại R1a: `go test -tags integration ./internal/adapter/postgres/... ./internal/adapter/mysql/... -run "Migration|SolutionContract"` và `go test ./internal/domain/... -run "AnalysisRun|TruncateRaw"`)

## Context

- `request-service` chưa tồn tại lúc soạn; đọc `migrations/postgres` và `migrations/mysql` của nó để lấy số kế tiếp (sau `NNNN_approvals` nếu SOL-009 đã vào). Hai dialect cùng số.
- Mẫu lease: `task-service/migrations/*/0013_execution_leases.up.sql`. Mẫu cột sinh MySQL: `0012_task_sources.up.sql`.
- Không sửa bảng `solutions` (README 3.5, CR-REQ-002).

## Việc cần làm

1. Đọc số migration kế tiếp; ghi vào PR.
2. Postgres up: `request.analysis_runs` đủ cột theo solution mục C (`kind` CHECK 4 giá trị, `mode` CHECK `complete|agent_readonly`, `status` CHECK `running|succeeded|failed`, `attempt INT DEFAULT 1`, `lease_owner`, `lease_expires_at`, `error_code`, `error_message`, `raw_output`, `started_at`, `finished_at`); chỉ mục duy nhất một run `running` cho `(tenant_id, request_id, kind)`; duy nhất `(tenant_id, request_id, idempotency_key)` khi khác NULL; chỉ mục quét lease `(status, lease_expires_at)`; RLS `tenant_isolation`.
3. MySQL up: `active_key VARCHAR(80) GENERATED ALWAYS AS (IF(status='running', CONCAT(request_id,':',kind), NULL)) STORED` + `UNIQUE KEY (tenant_id, active_key)`; `UNIQUE KEY (tenant_id, request_id, idempotency_key)`; `raw_output MEDIUMTEXT`; thời gian `TIMESTAMP(6)`.
4. Down hai dialect: `DROP TABLE analysis_runs`.
5. `domain/analysis_run.go`: `RunStatus`, `AnalysisMode`, struct `AnalysisRun`, `func (r *AnalysisRun) Succeed(now)`, `Fail(code, msg string, now)`, `RecordAttempt()` (tối đa 2, vượt thì `ErrTooManyAttempts`), `TruncateRaw(s string) string` (cắt ở 256 KB không làm hỏng UTF-8).

## Kiểm thử

- Migration up/down/up hai DB; chèn hai run `running` cùng `(request, kind)` thất bại, run `running` thứ hai khác `kind` thành công; sau khi run đầu `failed` có thể chèn run `running` mới; `idempotency_key` NULL lặp không va chạm.
- `analysis_run_test.go`: chuyển trạng thái hợp lệ/không; `TruncateRaw` với chuỗi tiếng Việt cắt ở ranh giới rune.
- Lệnh: `go test ./internal/domain/... -run AnalysisRun` và bộ test migration của service.

## Tiêu chí hoàn thành

- [x] Chỉ mục "một run `running`" từ chối bản ghi thứ hai ở cả hai DB.
- [x] up/down/up sạch; cùng số hai dialect.
- [x] `TruncateRaw` không tạo UTF-8 hỏng.

## Rủi ro và lưu ý

- MySQL thiếu partial index: hành vi NULL của khoá duy nhất là chỗ phải test thật, không chỉ đọc tài liệu.
- `attempt` tối đa 2 là quy ước của solution (gọi lại một lần khi JSON sai).

## Ghi chú triển khai (2026-10-08)

- Bảng `analysis_runs` nằm ở `0006_analysis_runs` (đã đánh số lại ở R1a). Migration `0030_solution_analysis` (cả hai dialect) thêm các cột còn thiếu: `solutions.kind/status/content_ref/generation_run_id`, `analysis_runs.solution_id/project_id/actor_id/feedback/enforcement/repo_check`, bảng khoá `analysis_project_gates` (RLS), chính sách `relay_scan/relay_claim` cho quét lease xuyên tenant.
- Chỉ mục "một run `running`" được kiểm bằng SQL thật ở cả Postgres 16 và MySQL 8.0 (`RunUniqueIndexes`): hai run `running` cùng `(request, kind)` bị từ chối, khác `kind` được nhận, sau khi run kết thúc được chèn run mới, `idempotency_key` NULL lặp không va chạm, khoá trùng bị từ chối.
- Up/down/up kiểm bằng `TestPostgres_Migration_UpDownUp` và `TestMySQL_Migration_UpDownUp` (đã gồm `0030`).
