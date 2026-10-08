# FE-CV-TASK-087-10: Hook `useQualityFindingMarkers` và `useQualityFindingsForFile`

**From Solution:** [FE-CV-SOL-087-quality-diff-annotations](../solutions/FE-CV-SOL-087-quality-diff-annotations.md) mục 2.3
**Priority:** P0
**Area:** frontend / editor hooks
**File:** `frontend/src/renderer/src/components/editor/quality-annotations/useQualityFindingMarkers.ts`, `quality-annotation-notice.ts` (mới); `frontend/src/renderer/src/hooks/useQualityFindingsForFile.ts` (mới) và `*.test.ts(x)`
**Depends on:** 087-09, 087-02, 087-03
**Status:** [x] DONE (verified 2026-10-07: useQualityFindingMarkers.test.tsx + useQualityFindingsForFile.test.tsx + quality-glyph-decorations.test.ts, 18/18 pass; oxlint + tsc sạch)

## Context

- `setModelMarkers` chưa được dùng ở đâu trong `frontend/src` (grep rỗng): cơ chế mới; mẫu decoration: `MonacoEditor.tsx:735-760` (`createDecorationsCollection`, `.set`, `.clear`).
- `DiffViewer` giữ `modifiedEditor` trong state (:82) và `modelKey`; model giữ lại (`keepCurrentModifiedModel`).
- Marker không tự dời theo chỉnh sửa → phải xoá khi nội dung đổi.

## Việc cần làm

1. `useQualityFindingsForFile`: dữ liệu đã nạp nếu đầy đủ, ngược lại `quality.findings {file, runId, limit:500}`; cache ≤ 16 tệp/worktree trong `codeIntelQualityByWorktree`; mảng ổn định; không gọi khi không `enabled`.
2. `useQualityFindingMarkers`: điều kiện vẽ (support, `annotationsOn`, eligible, có dữ liệu); `setModelMarkers(model,'orca-quality',...)` + decorations; `updateOptions({glyphMargin:true})` chỉ khi có phát hiện; xoá ngay ở `onDidChangeModelContent` và đặt ghi chú; dọn khi gỡ/đổi model/tắt; `onMouseDown` glyph → `setQualityUi({selectedFingerprint})`.
3. Trạng thái ghi chú (đã ẩn vì nội dung đổi; có thể lệch vì `dirty`) trả ra để `DiffViewer` hiển thị gọn (không toast).

## Kiểm thử

Editor/model giả (`setModelMarkers`, `createDecorationsCollection`, `onDidChangeModelContent`, `onMouseDown`): đặt, thay khi dữ liệu đổi, xoá khi nội dung đổi, dọn khi unmount và đổi model, không chạm marker owner khác, không gọi khi chưa có run; hook tệp: đủ/thiếu trang, cache 16. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/editor/quality-annotations/useQualityFindingMarkers`.

## Tiêu chí hoàn thành

- [ ] Owner cố định `'orca-quality'`; không rò rỉ listener.
- [ ] Không gọi mạng khi `disabled`.

## Rủi ro

- Tương tác với view zone của `useDiffCommentDecorator` (đẩy dòng): marker theo dòng model nên không ảnh hưởng, chưa kiểm chứng bằng chạy thật.

## Ghi chú triển khai (2026-10-07)

- Owner marker `orca-quality`; xoá khi nội dung đổi, dọn khi gỡ/đổi model. Glyph tách ra `quality-glyph-decorations.ts`. Chưa kiểm chứng trên Monaco thật: `glyphMargin` mặc định, F8, `onMouseDown` glyph trong `DiffEditor` (chỉ test bằng editor giả).
