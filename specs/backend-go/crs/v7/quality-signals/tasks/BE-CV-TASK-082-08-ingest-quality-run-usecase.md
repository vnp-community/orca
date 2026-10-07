# BE-CV-TASK-082-08: Use case `IngestQualityRun` (progress, finished, reconcile)

**From Solution:** BE-CV-SOL-082
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/ingest_quality_run.go`, `ingest_quality_run_test.go` (mới)
**Depends on:** BE-CV-TASK-082-04, 082-05, 082-07, BE-CV-SOL-080 (`IndexBasisReader` qua SOL-012), BE-CV-SOL-024-event-distribution
**Status:** [x] DONE

## Context
SOL-082 mục 2.F. Nạp findings **trước**, `Finish` **sau** (CAS + outbox `orca.codeintel.quality.run_finished` cùng giao dịch). Mọi replica có thể nạp: UNIQUE+CAS.

## Việc cần làm
1. `OnProgress`: `queued→running`, `Touch` ≤ 1 lần/2 s.
2. `OnFinished`/`Reconcile`: jitter 0-500 ms, đọc lại run, `runStatus` → `results` phân trang → sanitize/validate/`InsertBatch` → `Finish` (số đếm trước cắt, `index_commit`, `index_basis`, `steps`, `error_code`).
3. `RUN_NOT_FOUND` ⇒ `failed` giữ phần đã nạp.
4. Lỗi đọc `IndexBasis` không làm run thất bại.

## Kiểm thử
- Cổng giả: `finished` hai lần, ngắt giữa trang, trang trùng, 6000 dòng, `interrupted`, đồng hồ giả throttle.

## Tiêu chí hoàn thành
- [x] Run `succeeded` luôn có đủ phát hiện đã nhận; không trùng `ordinal`.

## Rủi ro
N replica cùng gọi agent (chưa đo).
