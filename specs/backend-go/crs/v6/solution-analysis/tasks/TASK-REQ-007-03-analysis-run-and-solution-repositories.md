# TASK-REQ-007-03: Repository `analysis_runs` và `solutions` (hai dialect), lease và phục hồi

**From Solution:** [BE-REQ-SOL-007](../solutions/BE-REQ-SOL-007-solution-generation-options-and-selection.md) mục E (repository), C
**Priority:** P0
**Service/Area:** `request-service` / adapter
**File:** `internal/adapter/{postgres,mysql}/analysis_run_repository.go` (mới), `solution_repository.go` (mới; hoặc mở rộng nếu CR-REQ-002 đã tạo), `internal/usecase/ports.go` (sửa), và `_integration_test.go`
**Depends on:** TASK-REQ-007-01, TASK-REQ-007-02
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test -tags integration ./internal/adapter/postgres/... ./internal/adapter/mysql/... -run "SolutionContract|AnalysisRuns"` trên Postgres 16 và MySQL 8.0 thật)

## Context

- Mẫu hai dialect cho lease và `SKIP LOCKED`: `task-service/internal/adapter/postgres/execution_leases.go` (dòng 53, 109, 174-186) và `adapter/mysql/execution_leases.go` (dòng 41: `SELECT ... FOR UPDATE SKIP LOCKED`, MySQL >= 8.0.1).
- Giờ DB cho lease (không dùng đồng hồ ứng dụng), như CR-TG-008.
- `solutions` có từ CR-REQ-002; kiểm `ls internal/adapter/postgres/solution_repository.go` trước khi tạo.

## Việc cần làm

1. Port `AnalysisRunRepository`: `InsertRunWithSolution(ctx, tx, run, sol) (existingRun *AnalysisRun, err)` (trả run `running` hoặc cùng `idempotency_key` thay vì tạo mới khi trùng chỉ mục), `RenewLease(ctx, runID, owner string, ttl time.Duration) (bool, error)`, `Complete(ctx, tx, run)`, `Fail(ctx, tx, run)`, `ClaimExpired(ctx, owner string, batch int) ([]ExpiredRun, error)`, `ListRecent(ctx, tenant, requestID, limit) ([]AnalysisRun, error)`, `CountRunning(ctx, tx, tenant, projectID, mode) (int, error)` (cho SOL-008).
2. Port `SolutionRepository`: `GetByID`, `ListByRequest(filter)`, `UpdateOptions(tx, id, options, status)`, `Choose(tx, id, idx, expectedVersion) (bool, error)`, `SupersedeOpen(tx, requestID, kind, exceptID)`, `DeleteDraft(tx, id)`.
3. Postgres `ClaimExpired`: `UPDATE ... SET lease_owner=$1, lease_expires_at=now()+ttl WHERE id IN (SELECT id FROM analysis_runs WHERE status='running' AND lease_expires_at < now() ORDER BY lease_expires_at LIMIT $2 FOR UPDATE SKIP LOCKED) RETURNING ...`. MySQL: transaction `SELECT ... FOR UPDATE SKIP LOCKED` rồi `UPDATE`.
4. `InsertRunWithSolution`: bắt vi phạm chỉ mục "một run `running`" và `idempotency` (Postgres `23505`, MySQL `1062`), đọc lại run đang chạy và trả về; không để lỗi trùng lọt ra RPC.
5. `Choose`: `UPDATE ... SET chosen_option=?, version=version+1 WHERE id=? AND tenant_id=? AND status='proposed' AND version=?`.
6. Mọi truy vấn lọc `tenant_id`; ghi `options` bằng tham số JSON hợp lệ (`::jsonb` Postgres, `CAST(? AS JSON)` MySQL), đọc lại nguyên Unicode.

## Kiểm thử

- Integration hai DB: chỉ mục duy nhất của run; hai worker `ClaimExpired` song song không trùng; `RenewLease` sai chủ sở hữu trả `false`; `Choose` sai version trả `false`; `DeleteDraft`; digest của `options` sau vòng ghi/đọc bằng với trước (dùng `DigestOptions`); Unicode tiếng Việt; chéo tenant.
- Lệnh: `go test ./internal/adapter/postgres/... ./internal/adapter/mysql/... -run "AnalysisRun|Solution"`.

## Tiêu chí hoàn thành

- [x] Một bộ test chạy với cả hai adapter.
- [x] `SKIP LOCKED` hai worker không chiếm trùng ở cả hai DB.
- [x] Digest ổn định qua JSONB và JSON.
- [x] Không câu SQL nào thiếu `tenant_id`.

## Rủi ro và lưu ý

- MySQL < 8.0.1 không chạy được `SKIP LOCKED`: kiểm bằng `dbcapability` lúc khởi động và fail-fast.
- `CountRunning` trên MySQL phải trong transaction có khoá (SOL-008 dùng).

## Ghi chú triển khai (2026-10-08)

- Cổng nằm ở `internal/usecase/ports_solution.go` (`AnalysisRunStore`, `SolutionStore`), không ở `ports.go`; hai interface giả cũ `AnalysisRunRepository`/`SolutionRepository` trong `ports.go` (không ai dùng) đã xoá. `SolutionStore` nhúng `SolutionCoreRepository` của R1a nên một struct duy nhất `SolutionRecordRepository` đáp cả hai.
- Lệch so với task: `InsertRunWithSolution` đổi tên `StartRun` và nhận `StartRunOptions{LeaseTTL, MaxAgentRuns}`; thêm `EnsureProjectGate`; `Complete`/`Fail` gộp thành `FinishOwned(run, owner)` (chỉ ghi khi còn giữ lease, trả `false` khi mất lease). Mọi truy vấn lọc `tenant_id`; ngày giờ lease dùng đồng hồ DB.
- Hai lỗi chỉ lộ khi chạy MySQL thật và đã sửa: (1) đếm run đồng thời bằng `COUNT(*)` thường dùng snapshot cũ nên cho 8 run qua cổng giới hạn 2, đổi sang `FOR SHARE` (current read); (2) tra cứu run thắng sau khi thua cuộc đua chèn cũng phải là đọc khoá. Tạo hàng khoá `analysis_project_gates` ngoài giao dịch (autocommit) vì hai giao dịch cùng chèn một khoá dễ deadlock gap lock.
- Cổng đồng thời dùng hàng `analysis_project_gates` + `SELECT ... FOR UPDATE` (cả hai dialect) thay vì khoá tư vấn: cùng một cách chạy trên Postgres và MySQL, kiểm bằng 8 goroutine, đúng 2 qua cổng.
- MySQL < 8.0.1 không có `SKIP LOCKED`: chưa thêm kiểm `dbcapability` lúc khởi động riêng cho điểm này (kiểm chung của service vẫn như cũ).
