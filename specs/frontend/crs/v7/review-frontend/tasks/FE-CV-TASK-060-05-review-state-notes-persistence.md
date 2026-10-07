# FE-CV-TASK-060-05: Lưu notes/sentBatches/turnMarkers qua `reviewState.save`

**From Solution:** [FE-CV-SOL-060](../solutions/FE-CV-SOL-060-review-notes-and-turn-compare.md) mục 2.4
**Priority:** P1
**Area:** frontend / renderer hooks
**File:** `frontend/src/renderer/src/hooks/useReviewNotesPersistence.ts` (mới) + test; `store/slices/code-intel.ts` (khoá `reviewNotesState`, `reviewTurnsState`)
**Depends on:** FE-CV-SOL-052-reading-order-and-progress (bộ ghi `reviewState.save`); FE-CV-SOL-050-store-and-query-hooks; FE-CV-TASK-060-04; fake backend G4 (073-02)
**Status:** [x] DONE

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
