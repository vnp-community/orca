# BE-CV-TASK-083-08: Ước lượng coverage từ `ChangeOverlay`

**From Solution:** BE-CV-SOL-083
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/estimate_coverage.go`, `estimate_coverage_test.go` (mới); cổng `ChangeOverlayReader` trong `coverage_ports.go`
**Depends on:** BE-CV-SOL-036-change-overlay-pipeline, BE-CV-TASK-083-03
**Status:** [x] DONE

## Context
K5/H7: `estimated` không phần trăm, không cộng với `measured`; `estimatedNote` là khoá i18n. Chỉ lưu khi có run liên quan.

## Việc cần làm
1. Từ `uncoveredSymbols`/`tested` dựng `totals.changedSymbolsTested/Untested/Unknown`; `diff=nil`.
2. Lưu `estimated` chỉ khi có `quality_run_id`; ngược lại tính nóng.
3. Overlay không có → `report=nil`, `reason=overlay_unavailable`.

## Kiểm thử
- Bảng ca; phản chiếu: không trường pct/stmts/covered.

## Tiêu chí hoàn thành
- [x] Cổng (SOL-085) nhận `source=estimated` phân biệt được.

## Rủi ro
Báo nhầm kế thừa CR-036.
