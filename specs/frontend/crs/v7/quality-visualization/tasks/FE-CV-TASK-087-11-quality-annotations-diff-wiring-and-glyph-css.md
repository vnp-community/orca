# FE-CV-TASK-087-11: Gắn hook vào `DiffViewer`, `DiffSectionItem`; CSS glyph; công tắc chú thích

**From Solution:** [FE-CV-SOL-087-quality-diff-annotations](../solutions/FE-CV-SOL-087-quality-diff-annotations.md) mục 2.1, 2.3
**Priority:** P0
**Area:** frontend / editor
**File:** `frontend/src/renderer/src/components/editor/DiffViewer.tsx`, `DiffSectionItem.tsx` (sửa: mỗi file một lời gọi hook), `frontend/src/renderer/src/assets/main.css` (lớp `.orca-quality-glyph-*`), công tắc "Chú thích kiểm tra" (nơi đặt: thanh công cụ diff hoặc lens; chốt khi đọc code)
**Depends on:** 087-10, FE-CV-TASK-088-01
**Status:** [ ] TODO

## Context

- `DiffViewer.tsx`: `useDiffCommentDecorator` :110, `DiffEditor` options :444-478 (không `glyphMargin`). `DiffSectionItem.tsx`:176. `DiffViewer` nhận `worktreeId`, `relativePath`; không nhận `diffSource` hiện nay → cần truyền từ `EditorContent.tsx` (:964) / `ChangesModeView.tsx` (:87) hoặc suy ra từ store (đọc code, chọn cách ít chạm nhất).
- `ExternalFileChangeCompareDialog.tsx` không `worktreeId` → không chú thích.
- Trước khi sửa symbol có sẵn: GitNexus `impact` (`DiffViewer`, `DiffSectionItem`; chưa chạy khi soạn). Không thêm `max-lines` disable; không sửa `useDiffCommentDecorator`.

## Việc cần làm

1. Thêm prop tuỳ chọn `diffSource?` (và `compareHeadOid?` nếu cần) cho `DiffViewer`; truyền từ các nơi gọi có `worktreeId`.
2. Một lời gọi `useQualityFindingMarkers` trong `DiffViewer` và `DiffSectionItem` (section có `path`); hiển thị ghi chú gọn dưới tiêu đề khi `quality-annotation-notice` có nội dung.
3. CSS: `.orca-quality-glyph-error|warning|info` bằng `clip-path` (bát giác/tam giác/tròn), nền `var(--quality-*)`, kích thước theo `lineHeight`; không hex.
4. Công tắc `annotationsOn` (state 087-02) với nhãn `translate()`, mặc định bật khi có phát hiện.

## Kiểm thử

Test `DiffViewer`/`DiffSectionItem` hiện có phải xanh; test mới: hook được gọi với đúng đối số; không chú thích khi không `worktreeId`; công tắc tắt thì dọn. Kiểm tay sáng/tối, glyph hình dạng. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/editor/DiffViewer src/renderer/src/components/editor/DiffSectionItem`.

## Tiêu chí hoàn thành

- [ ] Mỗi file sửa chỉ thêm một lời gọi hook + prop; diff nhỏ.
- [ ] Glyph khác hình theo mức; không hex.

## Rủi ro

- Truyền `diffSource` qua nhiều tầng có thể chạm `EditorContent` (file lớn); ưu tiên selector từ store nếu có.
