# TASK-FE-ANNOTATE-003: Fix or remove `annotation-panel.tsx` (calls `annotation.create`/`list` with wrong shape, non-functional)

**Solution:** [SOL-FE-ANNOTATE-001](../solutions/SOL-FE-ANNOTATE-001-persist-diff-comments-to-annotation-service.md) §3 | **CR:** CR-ANNOTATE-001
**Depends on:** Không (độc lập với TASK-FE-ANNOTATE-001/002 — file khác, vấn đề khác)
**Status:** ✅ DONE (2026-09-11)

---

## Mục tiêu

`frontend/src/renderer/src/components/code-review/annotation-panel.tsx`
gọi `annotation.create`/`annotation.list` với shape (`projectId`/`reviewId`/
`lineNumber`) không khớp proto thật của backend-go (`repo_id`/`worktree_id`/
`file_path`/`line`) — không hoạt động ở cả desktop (0 IPC handler) lẫn web
mode (field sai tên, bị domain validation reject). Quyết định: sửa đúng
shape, hoặc xoá nếu `DiffCommentCard`/`useDiffCommentDecorator` (đã hoạt
động đúng sau TASK-FE-ANNOTATE-001/002) đã đủ thay thế UX.

## Files cần sửa (tuỳ quyết định ở Bước 1)

1. `frontend/src/renderer/src/components/code-review/annotation-panel.tsx` (MODIFY hoặc DELETE)
2. `frontend/src/renderer/src/components/code-review/annotation-panel.test.tsx` (MODIFY hoặc DELETE)
3. 5 file import `AnnotationPanel` — CHỈ sửa nếu quyết định xoá: `DiffViewer.tsx`, `DiffSectionBody.tsx`, `MonacoEditor.tsx`, `RichMarkdownAnnotationOverlay.tsx`, `code-review-panel.tsx`

## Bước 1 — Xác nhận danh sách caller thật + có ai dùng `reviewId` không

**Bắt buộc chạy `context({name: "AnnotationPanel"})` trước khi quyết định**
(SOL-FE-ANNOTATE-001 §3 đã nêu rõ, không suy đoán từ 1 lần grep trong lúc
viết CR/solution). Với mỗi trong 5 caller đã biết
(`DiffViewer.tsx`/`DiffSectionBody.tsx`/`MonacoEditor.tsx`/
`RichMarkdownAnnotationOverlay.tsx`/`code-review-panel.tsx`), kiểm tra
prop `reviewId` có được truyền giá trị thật (không phải `undefined`/không
truyền) hay không.

- **Nếu không caller nào truyền `reviewId` thật** → đi Bước 2A (xoá).
- **Nếu có ít nhất 1 caller truyền `reviewId` thật** (1 khái niệm
  review-session tách biệt khỏi worktree mà sản phẩm thật sự cần) → đi
  Bước 2B (sửa shape, giữ file).

## Bước 2A — Xoá (nếu Bước 1 xác nhận không ai dùng `reviewId` thật)

- Xoá `annotation-panel.tsx` + `annotation-panel.test.tsx`.
- Ở mỗi trong 5 caller: xoá import `AnnotationPanel` + JSX render nó, thay
  bằng đường dẫn hiện có tới `DiffCommentCard`/`useDiffCommentDecorator`
  cho đúng chỗ click-to-comment tương đương (đọc lại từng caller để biết
  chính xác nó đang render `AnnotationPanel` ở đâu/khi nào trước khi xoá —
  không xoá mù).
- Chạy `detect_changes({scope: "compare", base_ref: "main"})` sau khi xoá —
  xác nhận không có execution flow nào khác bị ảnh hưởng ngoài dự kiến.

## Bước 2B — Sửa shape (nếu Bước 1 xác nhận có nhu cầu thật cho `reviewId`)

Đổi `callRuntimeRpc` calls trong `annotation-panel.tsx`:

```typescript
// Trước:
callRuntimeRpc<Annotation[]>(target, 'annotation.list', {
  projectId: project.id, reviewId, filePath, lineNumber
})
// Sau (field khớp proto thật — xác nhận lại bằng context() trước khi code):
callRuntimeRpc<AnnotationListResponse>(target, 'annotation.list', {
  repoId: project.repoId, // hoặc field đúng ánh xạ project → repo, xác nhận lại
  filePath, line: lineNumber
})
```

Cân nhắc: nếu `reviewId` là khái niệm thật cần giữ, backend-go's
`annotation-service` hiện KHÔNG có field `review_id` (đã bị bỏ khi thiết kế
rẽ hướng sang `RepoID`+`WorktreeID`, xem SOL-BE-ANNOTATE-001 §2) — sửa
shape ở bước này có thể phát hiện cần 1 task backend-go bổ sung
`review_id` trở lại. Đây là phát hiện có thể xảy ra khi implement, không
phải quyết định trước — nếu gặp, dừng lại và báo cáo, không tự ý thêm field
backend-go trong 1 task frontend.

## Test plan

- Bước 2A: `tsc --noEmit` sạch sau khi xoá (không còn import treo); 5 caller
  vẫn render đúng UI (Monaco/DiffViewer/DiffSectionBody/RichMarkdownAnnotationOverlay/
  code-review-panel) qua đường thay thế.
- Bước 2B: `annotation.create`/`annotation.list` gọi với field đúng, có
  test giả lập response thật từ backend-go's shape (không phải response cũ
  của mock hiện có trong `annotation-panel.test.tsx`, vốn khớp shape sai).

## Verify

```bash
cd frontend && npx vitest run src/renderer/src/components/code-review/
npx tsc --noEmit
```

## gitnexus

`context({name: "AnnotationPanel"})` **bắt buộc trước khi làm bất cứ gì** —
đây là task duy nhất trong nhóm mà quyết định (sửa vs xoá) phụ thuộc hoàn
toàn vào kết quả công cụ, không phải thiết kế đã chốt sẵn trong solution.
Nếu xoá, thêm `impact({target: "AnnotationPanel", direction: "upstream"})`
trước khi xoá file, và `detect_changes()` sau khi xoá.


---

## ✅ Kết quả thực tế (2026-09-11)

**Phát hiện quan trọng, khác với dự kiến ban đầu của CR/solution**: GitNexus
`context({name: "AnnotationPanel"})` (chạy sau khi re-index — index lúc viết
CR/solution lỗi thời 155 commit) cho thấy **chỉ 1 caller thật**:
`CodeReviewPanel` (`code-review-panel.tsx`) — không phải 5 file như CR-ANNOTATE-001/
SOL-FE-ANNOTATE-001 đã ghi (`DiffViewer.tsx`, `DiffSectionBody.tsx`,
`MonacoEditor.tsx`, `RichMarkdownAnnotationOverlay.tsx` — grep xác nhận lại
trực tiếp: **0 kết quả** cho "AnnotationPanel" trong cả 4 file đó). Đây là
sai sót cần đính chính trong tài liệu trước đó, không phải thay đổi thật của
codebase.

**Đi xa hơn 1 bước**: `context({name: "CodeReviewPanel"})` cho `incoming: {}`
— chính `CodeReviewPanel` cũng **không có caller thật nào**. Grep xác nhận
độc lập (không có JSX `<CodeReviewPanel`/import nào ngoài các sub-component
nó tự import và 2 file test). Kết luận: toàn bộ `components/code-review/`
là 1 nhánh code chết, có từ trước, bị thay thế bởi `components/diff-comments/`
(`useDiffCommentDecorator`) mà không được dọn dẹp.

**Quyết định thực tế (Bước 2A — xoá)**: xoá `annotation-panel.tsx` +
`annotation-panel.test.tsx`; sửa `code-review-panel.tsx` gỡ import +
JSX render `AnnotationPanel` + 2 field không còn dùng
(`handleLineClick`/`closeAnnotation`) khỏi destructuring. **Không** đầu tư
thay thế bằng `DiffCommentCard` như dự kiến ban đầu — vì `CodeReviewPanel`
tự nó đã là dead code, đầu tư tích hợp thật vào đó không mang lại giá trị
cho người dùng. Để lại `code-review-panel.tsx` (và toàn bộ thư mục
`code-review/`) nguyên trạng — flagged bằng comment tại đầu file — làm ứng
viên cho 1 CR dọn dẹp riêng, ngoài phạm vi task này.

**Phát hiện phụ (không sửa, ngoài phạm vi)**: baseline `tsc --noEmit` (kiểm
tra TRƯỚC khi sửa gì, qua `git stash`) đã có sẵn 5 lỗi type trong chính thư
mục `code-review/`, không liên quan tới `annotation-panel.tsx`:
`commit-message-generator.tsx` (`worktreePath`/`rootPath` không tồn tại
trên type), `pr-create-dialog.tsx` (prop shape sai). Xác nhận thêm bằng
chứng "toàn bộ thư mục là dead code, chưa từng được ai sửa/build sạch từ
lâu" — không sửa các lỗi này (ngoài phạm vi task, thuộc về CR dọn dẹp riêng
nêu trên). 2 lỗi TS cụ thể của `annotation-panel.tsx` (module `@/components/ui/avatar`
và `date-fns` not found) biến mất cùng với việc xoá file — xác nhận file đó
còn hỏng cả ở tầng type, không chỉ sai RPC shape như solution đã nêu.

**Sự cố ngoài ý muốn trong lúc đo baseline (tự phát hiện, tự khắc phục,
không mất dữ liệu)**: dùng `git stash`/`git stash pop` để so sánh trạng thái
trước/sau — `git stash pop` bị chặn bởi xung đột ở `frontend/tsconfig.tsbuildinfo`
(file cache TypeScript, bị `tsc --noEmit` ghi đè trong lúc stash đang áp
dụng). Xử lý: `git checkout -- frontend/tsconfig.tsbuildinfo` (an toàn — chỉ
là cache, không phải source) rồi `git stash pop` lại thành công, xác nhận
lại bằng `git stash list` rằng 1 stash có sẵn từ trước (không phải của phiên
này, thuộc nhánh `feature/project-delete-ui` khác) không hề bị đụng tới.

**Verify thật đã chạy**: `tsc --noEmit` không còn báo lỗi nào cho
`code-review-panel.tsx` (2 lỗi cũ do `annotation-panel.tsx` biến mất, 1 lỗi
`handleLineClick` unused tự hết do xoá khỏi destructuring, dọn thêm 2 lỗi
unused-import có sẵn — `useCallback`/`ChangedFile` — vì đang sửa cùng file);
`vitest run src/renderer/src/components/code-review/` — 2/2 test PASS
(`changed-files-tree.test.tsx`, không còn file test nào tham chiếu
`annotation-panel`).

**Files đã sửa:**
- `frontend/src/renderer/src/components/code-review/annotation-panel.tsx` (DELETE)
- `frontend/src/renderer/src/components/code-review/annotation-panel.test.tsx` (DELETE)
- `frontend/src/renderer/src/components/code-review/code-review-panel.tsx` (MODIFY)
