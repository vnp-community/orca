# FE-CV-TASK-087-11: Gắn hook vào `DiffViewer`, `DiffSectionItem`; CSS glyph; công tắc chú thích

**From Solution:** [FE-CV-SOL-087-quality-diff-annotations](../solutions/FE-CV-SOL-087-quality-diff-annotations.md) mục 2.1, 2.3
**Priority:** P0
**Area:** frontend / editor
**File:** `frontend/src/renderer/src/components/editor/DiffViewer.tsx`, `DiffSectionItem.tsx` (sửa: mỗi file một lời gọi hook), `frontend/src/renderer/src/assets/main.css` (lớp `.orca-quality-glyph-*`), công tắc "Chú thích kiểm tra" (nơi đặt: thanh công cụ diff hoặc lens; chốt khi đọc code)
**Depends on:** 087-10, FE-CV-TASK-088-01
**Status:** [x] DONE (verified 2026-10-08: quality-annotations/* + MonacoEditor.* + DiffViewer/DiffSectionItem/ChangesModeView 9 file / 42 test PASS; Monaco 0.55.1 thật trong Chromium headless: glyph 3 hình đúng clip-path ở glyph margin cả editor thường lẫn DiffEditor, F8 mở marker widget, click glyph trả GUTTER_GLYPH_MARGIN đúng dòng; tsc frontend 117 lỗi = baseline, oxlint sạch)

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

## Ghi chú triển khai (2026-10-07)

- Mỗi file chỉ thêm một lời gọi hook + prop `diffSource`/`compareHeadOid` + `QualityAnnotationStrip` (công tắc `annotationsOn` và ghi chú ẩn/lệch). Để `DiffSectionItem.tsx` không vượt 400 dòng, tách logic dọn model Monaco có sẵn sang `useDiffSectionModelDisposal.ts` (nguyên văn, không đổi hành vi).
- CSS `.orca-quality-glyph-*` trong `main.css`. GitNexus impact: DiffViewer/DiffSectionItem/MonacoEditor đều LOW.
- Chưa làm: gắn `MonacoEditor` (plain editor); kiểm tay sáng/tối.

## Ghi chú triển khai (2026-10-08)

- `MonacoEditor` (soạn thảo thường) gắn qua `quality-annotations/useEditorQualityAnnotations.ts`: coi là phía `worktree` chỉ khi tab mở lúc không có bản nháp/`isDirty` (chốt một lần lúc mount; sửa sau đó do hook marker dọn + ghi chú "nội dung đã đổi"), bỏ qua tab `readOnly` và tệp không `worktreeId`. Sai lệch có chủ đích so với "không gắn MonacoEditor ở MVP" (sai lệch #5 của solution).
- `QualityAnnotationStrip` thêm prop `className`; ở editor thường strip nổi góc dưới-phải (không lấy chiều cao của Monaco). MonacoEditor chỉ thêm 1 lời gọi hook + 1 strip (+19 dòng; file đã có trong baseline max-lines từ trước). GitNexus impact `MonacoEditor`: LOW.
- Test mới: `useEditorQualityAnnotations.test.tsx` (5), `MonacoEditor.quality-annotations.test.tsx` (3).
- Kiểm Monaco thật: harness tạm (esbuild bundle `monaco-editor` + `quality-marker-model` + `quality-glyph-decorations` thật, CSS glyph trích từ `main.css`) chạy Playwright/Chromium headless, ngoài repo. Ảnh chụp token sáng/tối: 3 hình (bát giác/tam giác/vòng) phân biệt rõ.
