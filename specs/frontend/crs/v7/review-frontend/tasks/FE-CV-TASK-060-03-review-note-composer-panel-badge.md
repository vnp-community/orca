# FE-CV-TASK-060-03: Composer, huy hiệu và `ReviewNotesPanel`

**From Solution:** [FE-CV-SOL-060](../solutions/FE-CV-SOL-060-review-notes-and-turn-compare.md) mục 2.2, 2.6
**Priority:** P1
**Area:** frontend / renderer components
**File:** `frontend/src/renderer/src/components/review-map/notes/{ReviewNoteButton,ReviewNoteComposerPopover,ReviewNodeNoteBadge,ReviewNotesPanel}.tsx` (mới) + test
**Depends on:** FE-CV-TASK-060-01; FE-CV-SOL-051-review-workspace-shell; FE-CV-SOL-053-impact-lens-and-symbol-detail (panel chi tiết, nhảy tới neo)
**Status:** [x] DONE (verified 2026-10-08: ReviewNodeNoteBadge.nodes.test 4/4, notes/ + impact/ + erd/ 129/129 PASS)

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

## Ghi chú triển khai (2026-10-07)

- Ghi chú đã gửi bị `clearDeliveredDiffComments` xoá khỏi store nên "sửa ghi chú đã gửi đưa về hàng chờ" chỉ áp dụng cho ghi chú còn trong store (legacy `sentAt`).
- Điều hướng neo: `review-note-navigation.ts` (erd → `selectErdTable`, storage → `selectStorageNode`, dataflow → `setReviewDataFlowId`, impact/architecture/structure → `selectReviewSymbol`, rồi mở diff tại dòng).
- Thêm `review-note-selectors.ts` (`noteCountByNodeKey`, `groupNotesByLens`) cho badge/panel.

## Ghi chú tích hợp (W6, 2026-10-07)

`ReviewNoteButton` gắn vào `impact/SymbolDetailPanel.tsx` (anchor graph-node lens `impact`, `hotkey`) và `erd/ErdTableDetail.tsx` (prop mới `worktreeId`, anchor lens `erd`, file = `lastMigration || firstMigration`; không có file thì nút bị khoá kèm lý do). `ReviewSendMenu` ghi quyết định `send_to_agent` (095-05). Test cũ `SymbolDetailPanel.test`/`ErdLens.test` mock `@/store` một phần nên phải mock `../notes/ReviewNoteButton` (giả định cũ "panel không dùng store-bound child" không còn đúng).

## Ghi chú hoàn thiện (2026-10-08, P4)

- `ReviewNodeNoteBadge` gắn lên nút xyflow: `impact/ImpactSymbolNode.tsx` (góc phải trên) và `erd/ErdTableNode.tsx` (header bảng); số đếm lấy từ `notes/use-review-node-note-counts.ts` (chỉ đọc store: `getDiffComments` + `serverState.notes.anchors`, không mount hook persistence), cấp qua `ImpactGraphCanvas`/`ErdCanvas` (prop `worktreeId`).
- Phím `n`: SOL-052 không có registry phím dùng chung (đã tìm, không tồn tại), nên giữ cài cục bộ qua prop `hotkey` (bỏ qua ở ô nhập) — đúng nhánh "nếu có" của spec.
- `ErdLens.test` mock thêm `../notes/use-review-node-note-counts` (store giả của test không có `getDiffComments`).
