# BE-CV-TASK-082-09: Bảo trì, run mồ côi và wiring

**From Solution:** BE-CV-SOL-082
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/quality_run_maintenance.go`, `cmd/server/main.go` (sửa), `internal/config/quality_run.go` (mới)
**Depends on:** BE-CV-TASK-082-08, BE-CV-SOL-011-repositories-and-maintenance
**Status:** [x] DONE

## Context
C-DM §4.3: findings 30 ngày, runs 180 ngày, mồ côi 60 phút (`CODEINTEL_QUALITY_RUN_STALE_AFTER`) → `CODEINTEL_QUALITY_RUN_ORPHANED`, `active_key=NULL`. Biến `CODEINTEL_QUALITY_FINDINGS_RETENTION`.

## Việc cần làm
1. Đăng ký ba việc vào job `withMaintenanceTx` (chu kỳ 10 phút, lô 500).
2. Cấu hình env, giá trị sai bị bỏ + cảnh báo.
3. Wiring consumer sự kiện `quality_progress|finished` tới `IngestQualityRun`.

## Kiểm thử
- Integration hai dialect: run `running` quá hạn → `failed`; xoá theo lô; không xoá run còn `queued|running`.

## Tiêu chí hoàn thành
- [x] Mọi truy vấn bảo trì dùng đồng hồ DB.

## Rủi ro
Hạn dữ liệu chưa đo dung lượng.
