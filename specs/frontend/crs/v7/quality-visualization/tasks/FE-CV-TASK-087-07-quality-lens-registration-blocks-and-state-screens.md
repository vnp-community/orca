# FE-CV-TASK-087-07: Lens `quality`: đăng ký, khung khối, màn trạng thái

**From Solution:** [FE-CV-SOL-087-quality-scorecard-and-state](../solutions/FE-CV-SOL-087-quality-scorecard-and-state.md) mục 2.7
**Priority:** P0
**Area:** frontend / review shell
**File:** `frontend/src/renderer/src/components/review-map/quality/QualityLens.tsx`, `QualityStateScreen.tsx`, `quality-lens-blocks.ts` (mới); sửa nhỏ ở file CR-051: `ReviewLensId`, `REVIEW_LENS_DEFINITIONS`, khe `trailing` của `ReviewHeaderBar`
**Depends on:** 087-03..087-06; FE-CV-SOL-051-review-workspace-shell
**Status:** [x] DONE (verified 2026-10-07: QualityLens.test.tsx 8/8, review-lens-registry.test.ts + shell tests pass; oxlint + tsc sạch)

## Context

- Registry lens và `ReviewHeaderBar` do CR-051 sở hữu (chưa có code); `React.lazy` để `quality-charts` không vào chunk khởi động.
- Solution 3 đăng ký các khối (Phủ test, Xu hướng, Hotspot, Phụ thuộc) qua `QUALITY_LENS_BLOCKS`; khối không có nguồn hiển thị "Chưa có dữ liệu" kèm lý do (không ẩn).
- Không dùng `components/code-review/*`.

## Việc cần làm

1. `quality-lens-blocks.ts`: `{id, titleKey, phase, load: () => import(...)}`; `QualityLens` render `Collapsible` chỉ nạp khi mở.
2. Đăng ký tab sau "Hợp đồng" chỉ khi `useQualitySupport()==='enabled'`; thêm `'quality'` vào `ReviewLensId`; chip vào khe `trailing`.
3. `QualityStateScreen` theo bảng 2.7 (kind → copy + hành động), persistent inline.
4. Liên kết "Xem {n} phát hiện" mở dock nguồn "Kiểm tra" (task 087-13).

## Kiểm thử

Tab ẩn khi `disabled|unsupported`; lens lazy (module không import ở khởi động: test đọc nguồn `App.tsx`/registry); mỗi `QualityViewState` hiển thị đúng; khối chỉ nạp khi mở. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/quality/QualityLens`.

## Tiêu chí hoàn thành

- [ ] Không tab chết; không toast cho trạng thái.
- [ ] Hai sửa nhỏ ở CR-051 không đổi hành vi lens khác.

## Rủi ro

- Phụ thuộc tên/registry của CR-051 chưa tồn tại.

## Ghi chú triển khai (2026-10-07)

- Đăng ký `qualityLensDefinition` (order 80, `requiresQuality`, lazy) trong `review-lens-registry.ts`; thêm prop `trailing` ở `ReviewHeaderBar` và một dòng truyền `QualityGateChip` trong `ReviewWorkspace` (file chung, sửa tối thiểu).
- Danh sách phát hiện nằm ở dock qua `registerReviewDockPanel` (không còn ToggleGroup nguồn), xem 087-13.
- Bốn khối nạp lazy khi mở (`QualityLensBlockHost`), trạng thái mở lưu ở `ui.openBlocks`.
