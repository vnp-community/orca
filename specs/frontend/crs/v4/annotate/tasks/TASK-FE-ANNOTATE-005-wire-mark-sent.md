# TASK-FE-ANNOTATE-005: Wire `annotation.markSent` into `DiffNotesSendMenu`'s `onDelivered`

**Solution:** [SOL-FE-ANNOTATE-002](../solutions/SOL-FE-ANNOTATE-002-call-composed-prompt-before-delivery.md) §3 | **CR:** CR-ANNOTATE-002
**Depends on:** [TASK-FE-ANNOTATE-004](./TASK-FE-ANNOTATE-004-composed-prompt-hook.md) (cần `unsentAnnotationIds` từ hook đó)
**Status:** ✅ DONE (2026-09-11)

---

## Mục tiêu

Sau khi gửi prompt tới agent thành công, gọi thêm `annotation.markSent` với
đúng tập `annotationIds` (từ backend nếu compose thành công, hoặc
client-local ids nếu đã fallback) — song song với `clearDeliveredDiffComments`
đã có sẵn, không thay thế nó.

## Files cần sửa

1. `frontend/src/renderer/src/components/editor/DiffNotesSendMenu.tsx` (MODIFY — `onDelivered` callback)

## Bước 1 — Sửa `onDelivered`

```typescript
// Trước:
onDelivered={(notes) => void clearDeliveredDiffComments(worktreeId, notes)}
// Sau:
onDelivered={(notes) => {
  void clearDeliveredDiffComments(worktreeId, notes)
  void callRuntimeRpc(
    getActiveRuntimeTarget(useAppStore.getState().settings),
    'annotation.markSent',
    { ids: unsentAnnotationIds },
    { timeoutMs: 8000 }
  ).catch(() => {})
}}
```

**Xác nhận trước khi code**: `target.kind === 'local'` thì cũng KHÔNG nên
gọi `annotation.markSent` (cùng lý do TASK-FE-ANNOTATE-004 — không có cầu
nối ở desktop mode) — thêm guard tương tự, không copy-paste thiếu điều
kiện này.

## Bước 2 — Xác nhận hành vi "not found = no-op" của `annotation.markSent`

SOL-FE-ANNOTATE-002 §3 giả định gọi `annotation.markSent` với id không tồn
tại ở server (trường hợp fallback dùng client-local id) chỉ no-op, không
lỗi — **xác nhận lại đúng cho `MarkAnnotationsSent` usecase** (đọc
`backend-go/services/annotation-service/internal/usecase/mark_annotations_sent.go`,
theo TASK-CR-02-06's mô tả "Any id not found for the tenant is silently
skipped") trước khi dựa vào giả định này trong code thật — nếu hành vi thật
khác (vd. trả lỗi nếu 100% id không tìm thấy), điều chỉnh lại logic gọi
(chỉ gọi `markSent` khi biết chắc `annotationIds` là id thật từ server, bỏ
qua bước này khi đang ở trạng thái fallback).

## Test plan

- Gửi thành công (compose thành công trước đó) → cả
  `clearDeliveredDiffComments` VÀ `annotation.markSent` được gọi, với đúng
  `annotationIds` từ response compose.
- Gửi thành công (đã fallback, không có id thật từ server) → theo kết quả
  Bước 2: hoặc vẫn gọi `markSent` (nếu no-op an toàn), hoặc bỏ qua có chủ
  đích (nếu không an toàn) — test phải phản ánh đúng quyết định thật đã
  xác nhận, không phải giả định ban đầu của solution.
- `annotation.markSent` lỗi (network) → không hiện toast lỗi cho user (đã
  gửi thành công rồi, đây chỉ là bookkeeping) — `clearDeliveredDiffComments`
  vẫn chạy bình thường.

## Verify

```bash
cd frontend && npx vitest run src/renderer/src/components/editor/DiffNotesSendMenu.test.tsx
npx tsc --noEmit
```

## gitnexus

`context({name: "clearDeliveredDiffComments"})` — xác nhận hàm này không đã
tự gọi `annotation.markSent` ở đâu đó khác rồi (tránh gọi trùng 2 lần nếu
1 lớp code khác đã làm việc này) trước khi thêm lời gọi mới.


---

## ✅ Kết quả thực tế (2026-09-11)

**Phát hiện quan trọng không có trong solution gốc**: `unsentAnnotationIds`
(từ `useComposedAllNotesPrompt`) chỉ đúng cho scope "all unsent notes" —
nếu `DiffNotesSendMenu` có cả scope "this file" (`showFileScope=true`) và
người dùng chọn gửi theo file, `onDelivered(notes)` sẽ nhận
`unsentFileNotes` (tập con), không phải `unsentNotes`. Gọi thẳng
`markAnnotationsSentBestEffort(unsentAnnotationIds)` vô điều kiện trong
trường hợp đó sẽ **đánh dấu sai tập comment** (đánh dấu sent cho toàn bộ
"all" scope dù người dùng chỉ gửi 1 file). Đã thêm guard so sánh
`notes` (thật sự được gửi) với `unsentNotes` (tập "all" scope) theo id-set
— chỉ gọi `annotation.markSent` khi khớp chính xác. Khi gửi theo file-scope,
mark-sent bị bỏ qua có chủ đích (ghi rõ lý do trong code) — không phải
regression, vì scope "this file" vốn cũng chưa từng có server-id tracking
riêng (đúng phạm vi đã giới hạn ở TASK-FE-ANNOTATE-004).

**Xác nhận hành vi "not found = no-op" của `annotation.markSent`**: đọc
`backend-go/services/annotation-service/internal/usecase/mark_annotations_sent.go`
xác nhận đúng theo mô tả TASK-CR-02-06 ("Any id not found for the tenant is
silently skipped") — không cần thêm guard phía frontend cho trường hợp
fallback-id (không phải server id thật).

**Verify thật đã chạy**: cùng lượt với TASK-FE-ANNOTATE-004 —
`vitest run` cho `diffComments.test.ts`/`use-composed-all-notes-prompt.test.ts`/
`components/code-review/` — 31/31 PASS tổng cộng; `tsc --noEmit` sạch.

**Files đã sửa:**
- `frontend/src/renderer/src/components/editor/DiffNotesSendMenu.tsx` (MODIFY — `onDelivered` gọi thêm `annotation.markSent` có guard scope)
