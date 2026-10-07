# BE-CV-TASK-080-07: Re-plan sau `reindex.finished` và metric

**From Solution:** BE-CV-SOL-080
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/auto_refresh_index.go` (thêm `OnReindexFinished`), `internal/adapter/eventbus/reindex_finished_consumer.go` (mới)
**Depends on:** BE-CV-TASK-080-05, BE-CV-SOL-024-event-distribution, BE-CV-SOL-071-metrics-tracing-and-budgets
**Status:** [x] DONE

## Context
SOL-080 2.D: không có cờ `pendingRefresh`; suy lại từ `codeintel.status`. Metric theo CR-080 2.6: `orca_codeintel_auto_refresh_total{outcome}`, `orca_codeintel_auto_refresh_seconds{tier}`, `orca_codeintel_index_basis_total{scope}`; không nhãn đường dẫn/repo. Stream `CODEINTEL` bắt `orca.codeintel.>`, phải lọc subject (C-DM §5).

## Việc cần làm
1. Consumer `orca.codeintel.reindex.finished`, chỉ xử lý `trigger='agent_done'`.
2. Nếu `freshness=stale` và `dedupe_key` khác `trigger_event_id` job gần nhất (5 phút) ⇒ `UpsertSlot` mới.
3. Đăng ký ba metric.

## Kiểm thử
- Unit: lệch → một slot mới; không lệch → không; cùng dedupe_key → không.

## Tiêu chí hoàn thành
- [x] Không vòng lặp vô hạn (test: hai lượt liên tiếp cùng head/dirty chỉ một slot).

## Rủi ro
Nếu agent vẫn `running`, re-plan bị huỷ bởi sự kiện `running`; chấp nhận.
