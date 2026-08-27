# TASK-BIGFILE-020 — Move: `source-control-helpers.ts`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ✅ Done
(2026-08-12 — chỉ chuyển 12 export nêu trong Input, KHÔNG chuyển toàn bộ
dòng 354–802: khoảng đó còn chứa nhiều const/type/hook module-private dùng
chéo bởi `SourceControlInner` và các component khác trong file — xem
`source-control-helpers.ts` để biết ranh giới thật. Giảm 209 dòng
(8,370→8,161), không đạt ~450 vì lý do trên.)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md` (Giai đoạn 1)

## Input

- File nguồn: `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx`
- Đọc **đúng dòng 354–802** (trước `function SourceControlInner()` dòng 803).
- Symbol cần chuyển (12 export, không JSX):
  `resolveSourceControlBaseRef`, `resolveSourceControlCompareBaseRef`,
  `shouldClearBranchCompareForMissingBase`, `resolveSourceControlPickerBaseRef`,
  `BRANCH_REFRESH_INTERVAL_MS`, `normalizeSourceControlViewMode`,
  `readCommitDraftForWorktree`, `writeCommitDraftForWorktree`,
  `shouldRenderCommitArea`, `pickDefaultSourceControlAgent`,
  `refreshSourceControlAfterRemoteAction`,
  `clearRemoteActionErrorsForCompletedConflictOperations`

## Output

- File mới: `frontend/src/renderer/src/components/right-sidebar/source-control-helpers.ts`
- File nguồn thay 12 định nghĩa bằng
  `export { ... } from './source-control-helpers'`

## Các bước

1. `gitnexus impact` cho 12 symbol — dừng nếu bất kỳ risk HIGH/CRITICAL.
2. Đọc dòng 354–802, copy nguyên văn + import cần thiết.
3. Tạo file mới, paste. Sửa `SourceControl.tsx` thành barrel export.
4. Bổ sung `-- Why:` cho `/* eslint-disable max-lines */` ở dòng 1 của
   `SourceControl.tsx` (hiện KHÔNG có giải thích — yêu cầu riêng từ
   `BUG-FE-HLD-006`, làm ngay trong task này vì đang sửa file này).

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `SourceControl.tsx` giảm
      ~450 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx
rm frontend/src/renderer/src/components/right-sidebar/source-control-helpers.ts
```
