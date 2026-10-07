# FE-CV-TASK-051-05: Bố cục ba cột, registry lens, drawer và `ReviewWorkspace`

**From Solution:** [FE-CV-SOL-051-review-workspace-shell](../solutions/FE-CV-SOL-051-review-workspace-shell.md) mục 4.1, 4.2
**Priority:** P0
**Area:** frontend / review-map
**File:** `ReviewWorkspace.tsx`, `ReviewHeaderBar.tsx`, `ReviewScopePicker.tsx`, `ReviewLensTabs.tsx`, `ReviewDetailDrawer.tsx`, `review-lens-registry.ts`, `review-layout-storage.ts`, `ReviewTabHost.tsx` (sửa, nối `ReviewWorkspace`), tests
**Depends on:** FE-CV-TASK-051-02, 051-03, 051-04, FE-CV-TASK-050-17
**Status:** [x] DONE

## Context

- `components/ui/resizable.tsx`; `WorkspaceLayout.tsx:88-160` (mẫu render có điều kiện); `ui/tabs`, `ui/sheet`; `lib/editable-target.ts`; `ShortcutKeyCombo`.
- `ReviewLensProps {worktreeId, scope, overlay, selectedSymbolKey, chipFilter, onSelectSymbol, onOpenDiff}`; `REVIEW_LENS_DEFINITIONS` tải lười (`React.lazy` + `Suspense` + Skeleton).

## Việc cần làm

1. Bố cục theo `ResizeObserver` (≥1100 / 720-1099 / <720); phím `[`, `]`, `Esc`; chip phím chỉ cho phím đã cài.
2. `ReviewScopePicker` (Popover + ToggleGroup: Nhánh gốc, Khoảng commit, Review đã gửi).
3. Registry: chỉ lens đã đăng ký có tab; thứ tự cố định Ảnh hưởng, Kiến trúc, Luồng, ERD, Lưu trữ, Cấu trúc, Hợp đồng; `ReviewDrawerContent` context.
4. `review-layout-storage.ts`: khoá `orca.review.layout.v1`, `try/catch`; tiêu điểm mặc định chỉ khi mở có chủ đích; `motion-reduce:transition-none`.
5. Nối `ReviewTabHost` (050-17) sang `ReviewWorkspace` lazy; ghép slice `review-ui`, chip, banner (051-06).

## Kiểm thử

- `ReviewWorkspace.test.tsx` (mock `ResizeObserver`), `ReviewLensTabs.test.tsx` (đăng ký thêm lens không sửa file), `review-layout-storage.test.ts` (storage chặn).

## Tiêu chí hoàn thành

- [ ] Ba bố cục; hình học giữ; `Esc` đóng drawer trả tiêu điểm.

## Rủi ro

- Ngưỡng px ước lượng; hộp panel cần kích thước thật khi test.
