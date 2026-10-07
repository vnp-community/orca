# BE-CV-TASK-085-12: `GetQualityTrend` và bảo trì (điểm, waiver)

**From Solution:** BE-CV-SOL-085-waivers-and-trend
**Priority:** P1
**Service:** `code-intel-service`
**File:** `internal/usecase/get_quality_trend.go`, `quality_trend_maintenance.go` (mới); handler trong `quality_gate_waiver_trend_server.go`
**Depends on:** BE-CV-TASK-085-10; BE-CV-SOL-011-repositories-and-maintenance (`withMaintenanceTx`)
**Status:** [x] DONE

## Việc cần làm
1. `GetQualityTrend`: `limit ≤ 200` (ngoài khoảng ⇒ `CODEINTEL_INVALID_PARAMS`), `from/to`, `group_by (commit|turn)`; `points[]`, `truncated`, `total_count`; điểm `unknown` giữ nguyên, không nội suy; khoá vắng ≠ 0 (PQ-33).
2. Bảo trì mỗi `CODEINTEL_MAINTENANCE_INTERVAL` (lô 500): xoá điểm > 90 ngày hoặc vượt 200/binding (giữ mới nhất); xoá waiver quá 365 ngày kể từ `expires_at`/`revoked_at`.
3. Delta không có trong proto (D6); ghi chú Q1 cho người duyệt.

## Kiểm thử
- Integration hai dialect: truncation, `group_by`, lô xoá, chạy lặp không đổi; cách ly tenant.

## Tiêu chí hoàn thành
- [x] `limit>200` bị từ chối; [ ] giữ-200 đúng; [ ] idempotent.

## Rủi ro
- Hạn mức giữ là giá trị khởi điểm chưa hiệu chỉnh.
