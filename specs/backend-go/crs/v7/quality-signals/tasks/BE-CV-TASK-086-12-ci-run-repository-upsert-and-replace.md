# BE-CV-TASK-086-12: Repository run CI: upsert, thay finding, tỉa (hai dialect)

**From Solution:** BE-CV-SOL-086-ci-run-merge-and-comparison
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/postgres/ci_run_repository.go`, `backend-go/services/code-intel-service/internal/adapter/mysql/ci_run_repository.go`, cổng trong `ci_ports.go` (mới)
**Depends on:** BE-CV-TASK-082-04, 082-05, 086-09
**Status:** [x] DONE

## Context
Dùng bảng của SOL-082 (không migration). `UpsertCiRun` cạnh tranh: CAS `version`. Không `active_key`.

## Việc cần làm
1. `UpsertCiRun`, `ReplaceFindings(runID, fs)` (xoá + chèn trong cùng giao dịch), `LatestCiRun(binding, provider, pr)`, `PruneCiRuns(binding, keep=20)`.
2. Phát outbox `run_finished` chỉ khi **lần đầu** chuyển sang `succeeded|failed`.
3. Mọi câu `WHERE tenant_id`; Postgres `set_config` mỗi giao dịch.

## Kiểm thử
- Contract hai dialect: upsert cùng SHA không nhân đôi, replace thay hoàn toàn, tỉa, cô lập tenant, event một lần.

## Tiêu chí hoàn thành
- [x] Hai dialect xanh.

## Rủi ro
Xoá+chèn finding lớn (≤ 500) trong một giao dịch: chưa đo.
