# FE-CV-TASK-061-04: Điểm vào ở Source Control

**From Solution:** [FE-CV-SOL-061](../solutions/FE-CV-SOL-061-review-entry-points.md) mục 2.4
**Priority:** P0
**Area:** frontend / renderer components
**File:** `frontend/src/renderer/src/components/right-sidebar/source-control-review-entry.tsx` (mới) + test; `source-control-header-toolbar.tsx`, `source-control-header-overflow-menu.tsx`, `source-control-branch-context-row.tsx`, `SourceControl.tsx` (sửa nhỏ)
**Depends on:** FE-CV-TASK-061-01, 061-02
**Status:** [x] DONE (verified 2026-10-07: source-control-review-entry.test.tsx PASS; right-sidebar suite chỉ còn lỗi sẵn có FileExplorer.test; oxlint/tsc sạch)

## Context

- `SourceControl.tsx` rất lớn (baseline max-lines): chỉ thêm một hook + prop; **không** thêm `max-lines` disable. Chạy `gitnexus_impact` cho các symbol sửa.

## Việc cần làm

1. `useSourceControlReviewEntry({worktreeId}) → {visible, open, hasUnreviewed}`.
2. Mục "Review changes" (`ScanSearch`) trong menu tràn trước cụm Notes; `SourceControlHeaderIconButton` ở hàng ngữ cảnh nhánh khi `changedFiles>0`.
3. Mở với phạm vi O7; không truyền bộ lọc tên tệp; `visible=false` ⇒ không render.

## Kiểm thử

- Test hook (hiện/ẩn theo cờ + số thay đổi); hồi quy `source-control-*` test hiện có.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/right-sidebar`.

## Tiêu chí hoàn thành

- [ ] `SourceControl.tsx` chỉ thêm vài dòng.
- [ ] Không render khi cờ tắt.

## Rủi ro

- Thanh công cụ có thể chật; kiểm mắt.

## Ghi chú triển khai (2026-10-07)

- Lệch spec: hook `useSourceControlReviewEntry` được gọi trong `SourceControlHeaderToolbar` (prop mới `reviewWorktreeId`) thay vì trong `SourceControl.tsx` (hook muộn trong component lớn có nguy cơ sau early return); `SourceControl.tsx` chỉ thêm 1 dòng prop. Nút ở hàng nhánh chỉ khi `summary.changedFiles>0`; mục menu tràn luôn hiện khi cờ bật.
