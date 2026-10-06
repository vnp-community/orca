# FE-CV-TASK-061-06: Tab Review ở right sidebar và `ReviewSummaryPanel`

**From Solution:** [FE-CV-SOL-061](../solutions/FE-CV-SOL-061-review-entry-points.md) mục 2.6
**Priority:** P1
**Area:** frontend / renderer components + hooks + shared types
**File:** `frontend/src/shared/types.ts` (`RightSidebarTab`), `store/right-sidebar-route.ts` (+test), `components/right-sidebar/{index.tsx,activity-bar-buttons.tsx,right-sidebar-panel-content.tsx,right-sidebar-activity-visibility.ts(+test),activity-bar-overflow.ts}` (sửa); `components/right-sidebar/ReviewSummaryPanel.tsx`, `components/review-map/entry/useCodeIntelReviewSummary.ts` (mới) + test
**Depends on:** FE-CV-TASK-061-01, 061-02; FE-CV-SOL-051-review-workspace-shell (`IndexFreshnessChip`, trạng thái chuẩn); FE-CV-SOL-052 (tiến độ); fake backend G4
**Status:** [ ] TODO

## Context

- Đã xác minh `RightSidebarTab` ở `shared/types.ts:3337`. **Chạy `gitnexus_impact` trên `normalizeRightSidebarRoute`, `ActivityBarItem`, `getVisibleRightSidebarActivityItems` và quét `switch` theo union trước khi sửa.**
- Hợp đồng: `changeOverlay {detail:'summary'}` chỉ có `scope, limits.totalCounts, risk, components, indexFreshness`; khoá `totalCounts` chưa chốt.

## Việc cần làm

1. Thêm `'review'` vào union + `normalizeRightSidebarRoute`; `codeIntelOnly` + `isCodeIntelEnabled` trong bộ lọc hiển thị.
2. `ReviewSummaryPanel` (lazy) và `useCodeIntelReviewSummary`: chỉ yêu cầu khi tab hiển thị; ưu tiên cache của khung, nếu không `detail:'summary'` + `findings{limit:50}` + `reviewState.get`; làm mới theo `changed`/resync/completion mới; không polling.
3. Chip rủi ro (`risk.level` + lý do, `incomplete` ⇒ "chưa đủ dữ liệu"), số liệu (chip thiếu khoá ⇒ ẩn), phát hiện `error|warning|info` (`N+` khi còn trang), tiến độ, lượt gần nhất + nút Review; chip mở Review kèm bộ lọc.
4. `statusIndicator` chấm "chưa review" (nhãn chữ), xử lý ở menu tràn.

## Kiểm thử

- `normalizeRightSidebarRoute('review')` + tab lạ; `codeIntelOnly` ẩn khi cờ tắt/không git; panel rỗng/lỗi/chưa index; không gọi khi tab ẩn; chấm biến mất sau khi mở Review.
- `pnpm --filter orca-frontend test -- src/renderer/src/store/right-sidebar-route src/renderer/src/components/right-sidebar`.

## Tiêu chí hoàn thành

- [ ] Tab cũ không đổi; không polling; không yêu cầu trùng khung.
- [ ] Panel không có điểm số đơn.

## Rủi ro

- Khoá `totalCounts` chưa chốt; lan union sang lưu phiên.
