# BE-CV-TASK-082-04: Repository Postgres cho run và finding

**From Solution:** BE-CV-SOL-082
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/quality_run_ports.go`, `internal/adapter/postgres/quality_run_repository.go`, `quality_finding_repository.go` (mới)
**Depends on:** BE-CV-TASK-082-02, 082-03, BE-CV-SOL-010 (`TxRunner`, `OutboxWriter`)
**Status:** [ ] TODO

## Context
Cổng ở SOL-082 mục 2.E. `InTx` tham gia giao dịch của ctx; `set_config('app.tenant_id', $1, true)` mỗi giao dịch.

## Việc cần làm
1. Cài `Create` (vi phạm UNIQUE `active_key`, mã `23505` → `ErrRunActive`), `Get`, `GetByAgentRunID`, `ListByBinding` (keyset), `ListByRepoCommit`, `Touch`, `Finish` (CAS `version`, `active_key=NULL`, ghi outbox cùng giao dịch), `MarkOrphans`, `PurgeExpired`.
2. Finding: `InsertBatch` (`ON CONFLICT (tenant_id, run_id, ordinal) DO NOTHING`, lô ≤ 500), `ListByRun`, `CountByRun`, `ListByFingerprint`, `PurgeExpired`.
3. Mọi câu có `WHERE tenant_id = $n`; dọn `DELETE … IN (SELECT … LIMIT n)`.

## Kiểm thử
- Chạy bộ hợp đồng của 082-06 trên Postgres (role không superuser).

## Tiêu chí hoàn thành
- [ ] Không câu SQL thiếu `tenant_id` (test AST/grep).

## Rủi ro
Sự kiện outbox payload theo C-DM §5; sai khoá làm consumer SOL-085 hỏng.
