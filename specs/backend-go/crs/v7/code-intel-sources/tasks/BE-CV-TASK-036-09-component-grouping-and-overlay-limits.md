# BE-CV-TASK-036-09: Gom component (C4 hoặc đường dẫn) và `ApplyLimits` (đếm trước khi cắt)

**From Solution:** BE-CV-SOL-036-reading-order-and-risk
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/changeoverlay/component_grouping.go`, `overlay_limits.go` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-036-07, BE-CV-TASK-036-08
**Status:** [x] DONE

---

## Context

Solution reading-order §2.D; hợp đồng PQ-14 (≤ 2 MiB). Mở khoá TASK-036-05 (pipeline).

## Việc cần làm

1. `component_grouping.go`: `GroupComponents(files []ChangedFile, steps []ReadingStep, reasons []RiskReason, idx ComponentLookup) []ComponentGroup`: `ComponentLookup` là hàm thuần `path → (componentId, containerId, label, ok)` (hiện thực ở usecase từ C4 snapshot; ở đây chỉ nhận hàm). Khi `ok=false` dùng nhóm đường dẫn: `backend-go/services/<svc>/internal/<domain|usecase|adapter/<tên>>`, `agent/src/<relay|main|shared>`, `frontend/src/renderer/src/<cấp 1>`, còn lại thư mục cấp 1; `componentId:"path:<prefix>"`, `label` đánh dấu suy luận qua trường `Inferred=true` (UI dịch). `riskPoints` = Σ điểm các reason có `evidence` thuộc nhóm; không cộng vào tổng. Sắp nhóm theo `riskPoints` giảm dần rồi `componentId`.
2. `overlay_limits.go`: `ApplyLimits(o Overlay) Overlay`: ghi `totalCounts` (files, symbols, flows, steps) **trước**, rồi cắt: files ≤ 2 000 (giữ theo `|added|+|removed|`), symbols ≤ 2 000, flows ≤ 100, steps ≤ 300; đặt `truncated.{files,symbols,flows,steps}`; không đụng `risk`.
3. `ShrinkToSize(o Overlay, sizeOf func(Overlay) int, max int) (Overlay, error)`: vòng co theo thứ tự cố định (bỏ `uncoveredSymbols` > 500 → `changedSymbols` > 1 000 → `changedFiles` > 1 000), mỗi bước đặt `truncated`; vẫn vượt ⇒ `ErrOverlayTooLarge` (ánh xạ `CODEINTEL_RESPONSE_TOO_LARGE` ở handler).

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/changeoverlay/ -run 'Group|Limits|Shrink'`: C4 có/không; nhóm dự phòng cho từng tiền tố; 3 000 tệp ⇒ `truncated.files`, `totalCounts.files=3000`, thứ tự giữ theo kích thước; `ShrinkToSize` với `sizeOf` giả; `riskPoints` không đổi sau cắt; xác định 100 lần.

## Tiêu chí hoàn thành

- [x] Tiêu chí "giới hạn" và "nhóm component" của §9 đạt.
- [x] `risk` bất biến qua `ApplyLimits`.

## Rủi ro và lưu ý

- Khớp tiền tố dài nhất phải theo ranh giới thư mục (`a/b` không khớp `a/bc`).
