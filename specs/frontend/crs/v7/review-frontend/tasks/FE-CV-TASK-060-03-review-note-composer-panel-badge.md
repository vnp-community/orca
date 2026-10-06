# FE-CV-TASK-060-03: Composer, huy hiệu và `ReviewNotesPanel`

**From Solution:** [FE-CV-SOL-060](../solutions/FE-CV-SOL-060-review-notes-and-turn-compare.md) mục 2.2, 2.6
**Priority:** P1
**Area:** frontend / renderer components
**File:** `frontend/src/renderer/src/components/review-map/notes/{ReviewNoteButton,ReviewNoteComposerPopover,ReviewNodeNoteBadge,ReviewNotesPanel}.tsx` (mới) + test
**Depends on:** FE-CV-TASK-060-01; FE-CV-SOL-051-review-workspace-shell; FE-CV-SOL-053-impact-lens-and-symbol-detail (panel chi tiết, nhảy tới neo)
**Status:** [ ] TODO

## Context

- Ghi chú lưu bằng `addDiffComment` (chờ lưu xong mới đóng popover, rollback khi lỗi, `null` giữ nháp). `ui/popover`, `ui/textarea`; `isScreenSubmitShortcut` + `ShortcutKeyCombo`.

## Việc cần làm

1. `ReviewNoteButton` (icon `MessageSquarePlus`) cắm vào panel chi tiết của symbol/ERD/hợp đồng/phát hiện; khoá + lý do khi `resolveGraphNodeCommentTarget`=null.
2. `ReviewNoteComposerPopover`: tiêu đề hiện `tệp:dòng` thật sự sẽ gắn; `Mod+Enter` lưu; khoá nút ngay, trạng thái lưu sau ~200 ms.
3. `ReviewNodeNoteBadge`: số ghi chú trên nút xyflow qua selector theo `nodeKey`.
4. `ReviewNotesPanel`: nhóm theo lens/neo, sửa/xoá, nhảy tới neo (`setReviewLens` + chọn nút + mở diff đúng dòng khi SOL-053 hỗ trợ); sửa ghi chú đã gửi đưa về hàng chờ (hành vi `updateDiffComment` có sẵn).
5. Phím `n` mở soạn ghi chú nút đang chọn (registry SOL-052; bỏ qua ở ô nhập).

## Kiểm thử

- Testing Library: lưu, giữ nháp khi lỗi, `Mod+Enter` theo nền tảng, nút khoá khi không có tệp, nhóm theo lens, sửa đưa `sentAt` về trống, nhảy tới neo.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/notes`.

## Tiêu chí hoàn thành

- [ ] Tiêu chí 1–3 của SOL-060 mục 5.
- [ ] Không hex; chuỗi `translate()`.

## Rủi ro

- Nhảy đúng dòng phụ thuộc SOL-053.
