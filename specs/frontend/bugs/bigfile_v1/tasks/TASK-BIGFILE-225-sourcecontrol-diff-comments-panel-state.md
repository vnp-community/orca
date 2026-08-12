# TASK-BIGFILE-225 — Move: `useSourceControlDiffCommentsPanelState` hook

**Loại:** Move — custom hook extraction · **Effort:** M · **Phụ thuộc:**
TASK-BIGFILE-020..025 đã xong · **Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md` (Giai đoạn 2)

## Bối cảnh

Sinh ra từ TASK-BIGFILE-026 (Investigate `SourceControlInner`). Cụm state
"diff comments panel" (expand/collapse, copy-to-clipboard, clear
confirmation) trong `SourceControlInner` được xác nhận **an toàn** để tách
qua grep toàn bộ tên biến — không bị đọc/ghi từ cụm logic nào khác ngoài 1
effect reset dùng chung khi đổi worktree.

**QUAN TRỌNG — đọc lại dòng thật trước khi tách** (đúng nguyên tắc chung của
nhóm): số dòng dưới đây đo tại thời điểm viết task này (sau TASK 020–025,
`SourceControl.tsx` = 7,086 dòng). Nếu nhóm khác đã sửa file trước khi task
này chạy, PHẢI grep lại theo TÊN BIẾN, không tin số dòng literal.

## Input

- File nguồn: `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx`
- `SourceControlInner` bắt đầu dòng 576.
- Khối chính cần đọc: dòng **721–826** (khai báo state + 2 `useCallback` +
  2 `useMemo`).
- 3 điểm reset dùng chung nằm trong effect "đổi worktree" ở dòng **1,847–1,869**
  (chỉ lấy 2 dòng gọi setter của cụm này, KHÔNG di chuyển cả effect — effect
  này reset nhiều cụm state khác, xem "Các bước" bên dưới):
  ```
  setPendingDiffCommentsClear(null)
  setIsClearingDiffComments(false)
  ```
- State/hàm cần chuyển vào hook mới (7 symbol, tên giữ nguyên):
  `diffCommentsExpanded` (+ `setDiffCommentsExpanded`), `diffCommentsCopied`
  (qua `useCopyFeedbackState` — hàm này ĐANG ở lại `SourceControl.tsx`,
  import lại), `pendingDiffCommentsClear` (+ setter), `isClearingDiffComments`
  (+ setter), `handleCopyDiffComments`, `pendingDiffCommentsClearCount`,
  `resolvedPendingDiffCommentsClear`, `pendingDiffCommentsClearDescription`,
  `handleConfirmDiffCommentsClear`.
- Input hook cần nhận từ component cha (props, KHÔNG import lại từ
  `SourceControlInner`): `activeWorktreeId: string | null`,
  `diffCommentsForActive: DiffComment[]`, `diffCommentsPrompt: string`,
  `clearDiffComments: (worktreeId: string) => Promise<boolean>`,
  `clearDiffCommentsForFile: (worktreeId: string, filePath: string) => Promise<boolean>`.
- Trước khi sửa: `gitnexus impact` cho `handleCopyDiffComments` và
  `handleConfirmDiffCommentsClear` (2 hàm được gọi từ JSX, đại diện cụm) —
  dừng nếu risk HIGH/CRITICAL. Kỳ vọng: risk thấp, chỉ 2 điểm gọi trong JSX
  của chính `SourceControlInner` (dòng ~5,382–5,493 và ~6,109–6,138 tại thời
  điểm viết task — xác nhận lại bằng grep).

## Output

- File mới: `frontend/src/renderer/src/components/right-sidebar/use-source-control-diff-comments-panel.ts`
  — export 1 hook `useSourceControlDiffCommentsPanelState(input): {...}`
  trả về đúng 9 field/hàm liệt kê ở trên (đổi tên nếu cần cho rõ nghĩa
  nhưng PHẢI cập nhật mọi điểm gọi trong `SourceControlInner`).
  - Hook PHẢI expose thêm 1 hàm `resetDiffCommentsPanel(): void` (gọi cả 2
    setter `setPendingDiffCommentsClear(null)` +
    `setIsClearingDiffComments(false)`) để effect reset dùng chung ở
    `SourceControlInner` gọi thay vì set trực tiếp.
- `SourceControl.tsx`: xoá khối 721–826, gọi
  `const diffCommentsPanel = useSourceControlDiffCommentsPanelState({...})`
  và destructure; sửa effect reset (1,847–1,869) gọi
  `diffCommentsPanel.resetDiffCommentsPanel()` thay vì 2 dòng setter cũ; sửa
  2 điểm JSX dùng đúng field từ `diffCommentsPanel`.

## Các bước

1. `gitnexus impact` theo Input ở trên.
2. Đọc lại đúng dòng 721–826 và 1,847–1,869 (KHÔNG tin số dòng ở trên nếu
   file đã đổi) — copy nguyên văn logic, không refactor nội dung bên trong
   mỗi hàm.
3. Tạo file hook mới, paste, đổi tham số state cục bộ thành input của hook,
   thêm `resetDiffCommentsPanel`.
4. Sửa `SourceControl.tsx`: gọi hook, destructure, sửa effect reset dùng
   chung, sửa 2 điểm JSX.
5. `pnpm exec tsc --noEmit` + `pnpm exec oxlint` trên 2 file đã đổi.
6. Chạy toàn bộ test trong `frontend/src/renderer/src/components/right-sidebar/`
   (không chỉ test riêng — nhiều test file cùng import `SourceControl.tsx`).

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `SourceControl.tsx` giảm
      ~105 dòng (không nhiều — mục tiêu chính là tách concern, không phải
      giảm dòng)
- [ ] Test liên quan pass — đặc biệt hành vi: mở/đóng panel diff comments,
      copy-to-clipboard, xác nhận xoá 1 file / xoá tất cả, và việc panel bị
      reset đúng khi đổi worktree (test effect reset dùng chung vẫn hoạt
      động sau khi sửa)

## Rollback

```
git checkout -- frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx
rm frontend/src/renderer/src/components/right-sidebar/use-source-control-diff-comments-panel.ts
```
