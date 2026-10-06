# FE-CV-TASK-050-17: `TabGroupPanel`, `TabBar`, kéo-thả, `ReviewTabHost`

**From Solution:** [FE-CV-SOL-050-review-tab-wiring](../solutions/FE-CV-SOL-050-review-tab-wiring.md) mục 4.2, 4.4
**Priority:** P0
**Area:** frontend / components
**File:** `components/tab-group/TabGroupPanel.tsx`, `useTabGroupWorkspaceModel.ts`, `useTabDragSplit.ts`, `tab-drag-preview-activation.ts`, `components/tab-bar/TabBar.tsx`, `group-tab-order.ts`, `reconcile-order.ts` (đọc), `components/review-map/ReviewTabHost.tsx`, `ReviewTabUnavailableNotice.tsx` (mới), tests
**Depends on:** FE-CV-TASK-050-16, FE-CV-TASK-050-12, FE-CV-TASK-050-20
**Status:** [ ] TODO

## Context

- `TabGroupPanel.tsx:354-358` vẽ `EditorPanel` cho mọi loại trừ terminal/browser/simulator; simulator vẽ ở lớp worktree. Review cần loại khỏi nhánh này **và** có nhánh vẽ riêng (lazy + Suspense, như EditorPanel).
- Icon `ScanSearch` (có trong lucide 0.577). `TabBar.tsx` 16 chỗ `'simulator'`.

## Việc cần làm

1. `ReviewTabHost({worktreeId, tabId})`: `unknown` ⇒ Skeleton; `disabled`/`unsupported` ⇒ `ReviewTabUnavailableNotice` inline (nút "Đóng tab", không toast, không gọi kênh); `enabled` ⇒ thân tạm (SOL-051 thay bằng `ReviewWorkspace`).
2. `TabGroupPanel`: loại `review` khỏi nhánh EditorPanel; thêm nhánh vẽ `ReviewTabHost`; `activeTabType='review'`.
3. `useTabGroupWorkspaceModel`, `useTabDragSplit`, `tab-drag-preview-activation`, `TabBar`, `group-tab-order`: kiểu và nhánh `review`, icon, thứ tự, trạng thái chọn.
4. Chạy `tsc`/`lint:switch-exhaustiveness`; liệt kê chỗ còn thiếu.

## Kiểm thử

- Test `TabGroupPanel`/`TabBar` hiện có mở rộng; `ReviewTabHost.test.tsx` (`// @vitest-environment happy-dom`): 3 trạng thái support.

## Tiêu chí hoàn thành

- [ ] Tab Review hiển thị, chọn, kéo, chia nhóm; không `EditorPanel` cho review; không `max-lines` disable mới.

## Rủi ro

- `TabBar.tsx` lớn; sửa tối thiểu, `impact` trước.
