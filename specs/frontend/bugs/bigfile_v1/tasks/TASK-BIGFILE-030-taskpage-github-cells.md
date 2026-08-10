# TASK-BIGFILE-030 — Move: `task-page-github-cells.tsx`

**Loại:** Move (cơ học) · **Effort:** M · **Phụ thuộc:** — · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 1)

## Input

- File nguồn: `frontend/src/renderer/src/components/TaskPage.tsx`
- Đọc **đúng dòng 1,069–2,950** (khối lớn nhất trong Giai đoạn 1, ~1,880
  dòng, 9 component).
- Symbol cần chuyển: `GHStatusCell`, `ReviewChipAvatar`,
  `GitHubAssigneeAvatar`, `GitHubIssueLabelSelector`,
  `GitHubIssueAssigneeSelector`, `GHAssigneesCell`, `PRReviewCell`,
  `PRChecksCell`, `PRMergeCell`

## Output

- File mới: `frontend/src/renderer/src/components/task-page-github-cells.tsx`
- File nguồn thay 9 định nghĩa bằng
  `export { ... } from './task-page-github-cells'`

**Nếu file mới sau khi tách vẫn còn > 1,000 dòng:** cân nhắc tách tiếp thành
2 file (`task-page-github-review-cells.tsx` cho `ReviewChipAvatar`/
`PRReviewCell`/`PRChecksCell`/`PRMergeCell`, `task-page-github-assignee-cells.tsx`
cho phần còn lại) — quyết định khi đọc thực tế, không bắt buộc nếu <1,000
dòng.

## Các bước

1. `gitnexus impact` cho 9 symbol — dừng nếu bất kỳ risk HIGH/CRITICAL.
2. Đọc dòng 1,069–2,950, copy nguyên văn + import cần thiết.
3. Tạo file mới (hoặc 2 file theo lưu ý trên nếu cần), paste. Sửa
   `TaskPage.tsx` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `TaskPage.tsx` giảm
      ~1,880 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/TaskPage.tsx
rm frontend/src/renderer/src/components/task-page-github-cells.tsx
```
