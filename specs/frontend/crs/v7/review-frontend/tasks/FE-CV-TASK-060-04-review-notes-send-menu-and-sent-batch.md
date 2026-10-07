# FE-CV-TASK-060-04: `ReviewNotesSendMenu`, bản ghi lô đã gửi, xem trước lời nhắc

**From Solution:** [FE-CV-SOL-060](../solutions/FE-CV-SOL-060-review-notes-and-turn-compare.md) mục 2.3
**Priority:** P1
**Area:** frontend / renderer components + hàm thuần
**File:** `frontend/src/renderer/src/components/review-map/notes/{ReviewNotesSendMenu,ReviewSendPreviewDialog,ReviewSentBatchList}.tsx`, `review-sent-batch.ts` (mới) + test
**Depends on:** FE-CV-TASK-060-01, 060-02, 060-03
**Status:** [x] DONE

## Context

- `NotesSendMenu<TNote>` generic và `NotesSendMenuScope {id,label,notes,prompt}` đã tồn tại; `useComposedAllNotesPrompt` cho scope `all`.
- Hợp đồng §4.6 `ReviewSentBatch`; giới hạn `notes` ≤ 500 mục/256 KiB.

## Việc cần làm

1. `ReviewNotesSendMenu` bọc `NotesSendMenu` với scope `all` (`useComposedAllNotesPrompt`), `lens`, `selection` (`formatDiffComments`); `source='diff-notes'`.
2. `onDelivered`: `recordReviewSentBatch` → `clearDeliveredDiffComments` → `markAnnotationsSentBestEffort` chỉ scope `all`.
3. `review-sent-batch.ts`: `buildSentBatch` (body đã mask, cắt 2000; `fileIdentityAtSend`), `trimSentBatches` (tổng mục ≤ 500, dung lượng ≤ ngân sách, bỏ batch cũ nhất).
4. `ReviewSendPreviewDialog` (`ui/dialog`): lời nhắc chỉ đọc + cảnh báo > 50 ghi chú; `ReviewSentBatchList` ("Đã gửi ở lượt trước") + nhãn "tệp đã đổi từ khi gửi" (gợi ý, ước lượng).

## Kiểm thử

- Thứ tự gọi (record trước clear) bằng giả lập; phạm vi `selection`/`lens` không gọi `markSent`; trim; nhãn gợi ý không viết "đã xử lý".
- `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/notes`.

## Tiêu chí hoàn thành

- [ ] Tiêu chí 4–5 và 8 của SOL-060 mục 5.
- [ ] Lỗi lưu lô chỉ báo inline, không chặn gửi.

## Rủi ro

- Nếu lưu lô thất bại mà ghi chú đã bị xoá thì mất lịch sử (chấp nhận, báo inline).
