# FE-CV-TASK-053-08: Đăng ký lens Ảnh hưởng, i18n, e2e

**From Solution:** [FE-CV-SOL-053-impact-lens-and-symbol-detail](../solutions/FE-CV-SOL-053-impact-lens-and-symbol-detail.md) mục 8
**Priority:** P1
**Area:** frontend / review-map + i18n
**File:** `review-lens-registry.ts` (thêm mục `impact`, mặc định), `ReviewDetailDrawer` (mặc định `SymbolDetailPanel`), `i18n/locales/*.json`, `code-intel-locale-coverage.test.ts`, `tests/e2e/review-impact.spec.ts` (mới)
**Depends on:** FE-CV-TASK-053-03, 053-04, 053-06
**Status:** [~] PARTIAL — lens `impact` registered (lazy), drawer = SymbolDetailPanel (impact/symbol-detail-drawer-registration), 94 i18n keys x 5 locales, i18n/impact-structure-locale-coverage.test pass, ReviewWorkspace.test updated; NOT done: e2e `tests/e2e/review-impact.spec.ts` (not written/run)

## Context

- Registry: chỉ lens đã đăng ký có tab; lens Ảnh hưởng là mặc định.

## Việc cần làm

1. Đăng ký `impact` (icon `lucide-react`, `labelKey`, `load` lười).
2. Thêm khoá i18n của 053 (mã hoá lớp phủ, panel, nhãn `sourceOmitted`, thông báo không cạnh) đủ 5 locale; copy "Chưa tìm thấy ..." không overclaim.
3. E2E kịch bản: chọn symbol ⇒ lens + panel ⇒ "Xem diff" cuộn đúng dòng (fake backend).

## Kiểm thử

- `pnpm --dir frontend test -- src/renderer/src/i18n/code-intel-locale-coverage`; e2e chưa chạy.

## Tiêu chí hoàn thành

- [ ] Lens hiển thị là tab đầu; test phủ khoá xanh.

## Rủi ro

- E2E phụ thuộc cách tiêm backend giả vào web build (chưa kiểm).

## Ghi chú triển khai (2026-10-07, W3-A)

**Sai lệch so với spec**
- File lens đặt trong thư mục con `components/review-map/impact/` (không phẳng): `ImpactLens`, `ImpactToolbar`, `ImpactGraphCanvas` (lazy, default export), `ImpactSymbolNode`, `ImpactNodeCard`, `ImpactColumnsList`, `impact-column-layout.ts`, `impact-lens-model.ts`, `use-symbol-detail.ts`, `SymbolDetailPanel` + `SymbolDetailSection/RelationList/CoveringTests/RelatedFlows/SourcePreview`, `symbol-detail-actions-slot.tsx`, `symbol-detail-drawer-registration.tsx`, `use-diff-cursor-symbol-sync.ts`. `review-overlay-model.ts`, `ReviewOverlayLegend.tsx`, `symbol-line-index.ts` giữ ở `review-map/`.
- `useCodeIntelQuery` (không phải Paged) cho `impact`/`symbol`; mã nguồn symbol đọc bằng `defaultCodeIntelCall` + state cục bộ để không nằm trong cache LRU.
- Drawer: `symbol-detail-drawer-registration.tsx` được `ReviewWorkspace` import (side effect) để tránh vòng import với registry.
- `ReviewTabHost.onOpenDiff` đổi từ `openDiff` sang `openReviewDiffAtSymbol` (nhận `line`); lỗi hiện qua toast (đường dẫn thoát worktree / scope không hỗ trợ).
- `DiffViewer`: prop `reviewReveal` (một object `{line, side, nonce, onApplied}`) thay vì ba prop rời; hook gói `useDiffReviewLinks` (reveal + emitter con trỏ). Để không vượt cap `max-lines` của `DiffViewer.tsx`, effect theo dõi popover được tách sang `useDiffPopoverTracking.ts` (di chuyển nguyên văn).
- `review-ui` slice thêm `impactFocusKey`, `selectedSymbolSource`, `setReviewImpactFocus`, `selectReviewSymbolFromDiff`; `selectReviewSymbol` đặt `selectedSymbolSource:'user'`.
- Diff→đồ thị chỉ so khớp với `changedSymbols` (không có nút `impact`), vì chỉ mục dựng ở `ReviewWorkspace`.
- Tooltip ở legend/hàng/nút có `TooltipProvider` cục bộ để chạy được ngoài vỏ ứng dụng.
- Chưa làm: dòng "Số dòng theo index tại {commit}" (053-04), e2e (053-08), kiểm tay `var()` trong SVG xyflow.
