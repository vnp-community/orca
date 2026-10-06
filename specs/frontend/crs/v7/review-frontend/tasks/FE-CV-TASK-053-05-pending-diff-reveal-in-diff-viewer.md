# FE-CV-TASK-053-05: `pendingDiffReveal` và cuộn dòng ở `DiffViewer`

**From Solution:** [FE-CV-SOL-053-impact-lens-and-symbol-detail](../solutions/FE-CV-SOL-053-impact-lens-and-symbol-detail.md) mục 4.5
**Priority:** P0
**Area:** frontend / editor
**File:** `store/slices/editor.ts` (:740-741 mẫu), `components/editor/diff-viewer-props.ts`, `EditorContent.tsx` (:963-981 theo CR), `DiffViewer.tsx` (:166-245, :318-383), `components/editor/use-diff-line-reveal.ts` (mới), tests
**Depends on:** không
**Status:** [ ] TODO

## Context

- `DiffViewer` 488 dòng: chỉ có cuộn-tới-ghi-chú và tự cuộn tới thay đổi đầu tiên. `pendingEditorReveal` chỉ cho chế độ sửa. Nhiều nơi dùng chung ⇒ chạy `impact` GitNexus trước; chạy toàn bộ test diff hiện có.

## Việc cần làm

1. `pendingDiffReveal: {fileId, line, side:'original'|'modified', nonce} | null` + `setPendingDiffReveal` (không thuộc `OpenFile`, không lưu phiên).
2. `DiffViewerProps` thêm `revealLine?`, `revealSide?`, `revealNonce?`; `EditorContent` truyền khi `fileId` khớp rồi xoá sau khi áp dụng.
3. `use-diff-line-reveal.ts`: khi `revealNonce` đổi và editor đã gắn ⇒ `revealLineInCenter` + `setPosition` trên editor của `side`; chưa gắn thì giữ tới khi gắn; nhường/ưu tiên so với tự cuộn lần đầu.

## Kiểm thử

- Hook với editor giả; hoãn tới khi gắn; `nonce` mới mới áp dụng lại; test `DiffViewer`/`EditorContent` hiện có xanh.

## Tiêu chí hoàn thành

- [ ] Cuộn đúng dòng kể cả Monaco mount sau; không hồi quy.

## Rủi ro

- `diffViewStateCache`, `didAutoScrollFirstDiffRef` tương tác với reveal.
