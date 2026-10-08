# FE-CV-TASK-060-05: Lưu notes/sentBatches/turnMarkers qua `reviewState.save`

**From Solution:** [FE-CV-SOL-060](../solutions/FE-CV-SOL-060-review-notes-and-turn-compare.md) mục 2.4
**Priority:** P1
**Area:** frontend / renderer hooks
**File:** `frontend/src/renderer/src/hooks/useReviewNotesPersistence.ts` (mới) + test; `store/slices/code-intel.ts` (khoá `reviewNotesState`, `reviewTurnsState`)
**Depends on:** FE-CV-SOL-052-reading-order-and-progress (bộ ghi `reviewState.save`); FE-CV-SOL-050-store-and-query-hooks; FE-CV-TASK-060-04; fake backend G4 (073-02)
**Status:** [x] DONE (verified 2026-10-07: useReviewNotesPersistence.test.tsx 8/8 PASS, tsc/oxlint sạch)

## Context

- Hợp đồng §3.1/§4.6: `save` nhận `baseCommit, headCommit, readingProgress, notes?, turnMarkers?, status?, expectedVersion`; `turnMarkers` chỉ ở dòng `('','')`; `CODEINTEL_VERSION_CONFLICT` có `currentVersion`; `CODEINTEL_PAYLOAD_TOO_LARGE` có `limit`.
- Nếu SOL-052 chưa phơi API patch, thống nhất giao diện với chủ SOL-052 trước (không tạo bộ ghi thứ hai).

## Việc cần làm

1. `useReviewNotesPersistence(worktreeId)`: `get` hai dòng (worktree-level và `(base,head)`); `saveNotes(patch)`, `saveTurnMarkers(markers)` qua bộ ghi 052 (tuần tự).
2. Gộp khi xung đột: anchors theo commentId, sentBatches theo `batchId`, markers theo `turnId` (giữ 5 mới nhất); thử lại một lần; thất bại ⇒ trạng thái `error` để UI hiện banner.
3. `PAYLOAD_TOO_LARGE` ⇒ `trimSentBatches` mạnh hơn rồi thử lại một lần.
4. Offline ⇒ hoãn, đánh dấu cũ; không toast.

## Kiểm thử

- Hook với mock `codeIntelClient.call`: gộp xung đột, trần, offline, hai dòng độc lập; rò rỉ khi xoá worktree.
- `pnpm --filter orca-frontend test -- src/renderer/src/hooks/useReviewNotesPersistence src/renderer/src/store/slices`.

## Tiêu chí hoàn thành

- [ ] Chỉ một đường ghi `reviewState.save` cho mỗi dòng.
- [ ] Không gọi kênh khi cờ tắt.

## Rủi ro

- `SetReadLimit` 320 KiB (O-2) chưa chốt; `save` > 32 KiB có thể bị cắt kết nối.

## Ghi chú triển khai (2026-10-07)

- Dùng writer của 052 (`patchReviewState` của slice `review-progress`, một hàng chờ) cho dòng (base,head); `turnMarkers` đi qua `turns/review-turn-marker-row.ts` (dòng `('','')`, tuần tự theo worktree, gộp 1 lần khi xung đột). Slice 052 khi xung đột ghi đè `serverState` bằng bản từ xa nên hook gộp lại (hợp, local thắng) sau khi hàng ở trạng thái `saved`.
- Sai lệch: KHÔNG thêm `reviewNotesState/reviewTurnsState` vào slice `code-intel` (state nằm trong slice review-progress và state cục bộ hook ⇒ không thêm khoá cần dọn).
- Sửa nhỏ `review-shell-data.ts`: ánh xạ `CODEINTEL_PAYLOAD_TOO_LARGE` → `too-large` để hook thu nhỏ (250 mục) và thử lại một lần.
- Neo `ReviewNoteAnchor` không có trường `at` (hợp đồng) nên hợp nhất neo theo commentId không so thời gian: local thắng.
