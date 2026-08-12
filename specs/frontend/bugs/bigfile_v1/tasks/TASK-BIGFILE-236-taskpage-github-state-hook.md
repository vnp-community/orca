# TASK-BIGFILE-236 — Move: `useTaskPageGitHubState()` custom hook

**Loại:** Move (tách state thành custom hook — xem lưu ý "khác biệt với Move
cơ học" ở TASK-BIGFILE-235, áp dụng y hệt ở đây) · **Effort:** L
**Phụ thuộc:** TASK-BIGFILE-235 đã xong (xác nhận pattern hoạt động trên
domain nhỏ trước) · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 2)
**Sinh ra từ:** TASK-BIGFILE-032 (Investigate)

## Input

- File nguồn: `frontend/src/renderer/src/components/TaskPage.tsx`
- **Đọc lại đúng vị trí thật trước khi làm** (số dòng dưới xác nhận ở thời
  điểm viết task, sau 027–031; xác nhận lại bằng
  `grep -n "githubMode\|newIssueOpen\|const \[pages," TaskPage.tsx`).
- 24 state:
  - `githubMode` (dòng 1548)
  - Search/filter/pagination chung của GitHub tab: `taskSearchInput`,
    `appliedTaskSearch`, `activeTaskPreset`, `tasksLoading`,
    `tasksRefreshing`, `tasksFiltering`, `tasksError`, `failedCount`,
    `taskRefreshNonce` (dòng 1604–1618)
  - Pagination: `pages`, `currentPage`, `paginationLoading`,
    `loadingTargetPage`, `totalItemCount` (dòng 1636–1662)
  - `dialogInitialTab` (1678), `retryingSourceKeys` (1906)
  - New-issue draft: `newIssueOpen`, `newIssueTitle`, `newIssueBody`,
    `newIssueLabels`, `newIssueAssignees`, `newIssueSubmitting`,
    `newIssueRepoId` (dòng 1933–1939)
- Effect liên quan (dep thật, trích ở TASK-BIGFILE-032):
  - 1666–1670 (`selectedRepos, appliedTaskSearch,
    workItemsInvalidationNonce`)
  - 1758–1765 (deep-link mở GitHub work item)
  - 1855–1896 (2 effect — cache selection theo `githubMode`/`taskSource`)
  - 2001–2040 (2 effect — new-issue draft persist/clear)
  - 4052–4060 (PR checks eager-load)
  - 4153–4185 (2 effect — task-resume liên quan preset/search)
  - **4187–4383** (effect chính, ~196 dòng — fetch GitHub work items, dep:
    `selectedRepos, appliedTaskSearch, taskRefreshNonce, taskSource,
    githubMode, workItemsInvalidationNonce, taskResumeApplied`) — đọc kỹ
    trước khi tách, đây là effect lớn nhất trong domain GitHub.
- Tham số đọc cần nhận: `taskSource`, `taskResumeApplied`, `selectedRepos`,
  `pageData.openGitHubInitialTab`, `pageData.openGitHubWorkItem`,
  `setDialogWorkItem` (setter từ dialog state — xác nhận dialog state có
  thuộc domain GitHub hay là UI-chung trước khi quyết định truyền vào hay
  giữ ngoài).

## Output

- File mới: `frontend/src/renderer/src/components/use-task-page-github-state.ts`
- `TaskPage.tsx`: thay 24 `useState` + ~10 effect bằng 1 lời gọi hook, sửa
  mọi điểm tham chiếu còn lại.

## Các bước

Giống hệt quy trình TASK-BIGFILE-235 bước 1–6 (gitnexus impact trước, đọc
xác nhận ranh giới thật, grep toàn file cho từng tên trước khi tách, tsc
`--composite false` sau khi sửa). Điểm khác: effect 4187–4383 dài — đọc kỹ
toàn bộ thân effect trước khi copy, xác nhận không có state/setter ngoài
domain GitHub bị đọc/ghi bên trong (nếu có, ghi chú lại và giữ effect đó ở
`TaskPage.tsx`, không ép chuyển).

## Xác minh xong

- [ ] `pnpm exec tsc --noEmit -p frontend/tsconfig.json --composite false`
- [ ] `pnpm exec oxlint` trên 2 file đã đổi
- [ ] `pnpm exec vitest run --config config/vitest.config.ts
      src/renderer/src/components/feature-interaction-writer-boundaries.test.ts`
- [ ] `gitnexus detect_changes({scope: "all"})` hoặc grep thủ công
- [ ] `node scripts/find-frontend-bigfiles.mjs` — giảm thêm ~350–420 dòng
- [ ] Kiểm tra thủ công luồng GitHub tab: search/filter, phân trang, mở
      new-issue dialog, PR checks eager-load — không chỉ dựa tsc xanh.

## Rollback

```
git checkout -- frontend/src/renderer/src/components/TaskPage.tsx
rm frontend/src/renderer/src/components/use-task-page-github-state.ts
```
