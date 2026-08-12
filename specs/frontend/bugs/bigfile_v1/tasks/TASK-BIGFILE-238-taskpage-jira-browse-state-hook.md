# TASK-BIGFILE-238 — Move: `useTaskPageJiraBrowseState()` custom hook

**Loại:** Move (tách state thành custom hook — xem lưu ý "khác biệt với Move
cơ học" ở TASK-BIGFILE-235, áp dụng y hệt ở đây) · **Effort:** M
**Phụ thuộc:** TASK-BIGFILE-235, 237 đã xong · **Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 2)
**Sinh ra từ:** TASK-BIGFILE-032 (Investigate)

## Input

- File nguồn: `frontend/src/renderer/src/components/TaskPage.tsx`
- **Xác nhận lại vị trí thật** bằng
  `grep -n "selectedJiraIssueKey\|jiraIssues,\|availableJiraProjects" TaskPage.tsx`
  trước khi làm.
- 16 state:
  - Chọn/mở issue (2164–2165): `selectedJiraIssueKey`,
    `selectedJiraIssueFallback`
  - Browse (2420–2434): `jiraIssues`, `jiraLoading`, `jiraError`,
    `jiraErrorDetailsOpen`, `jiraSearchInput`, `appliedJiraSearch`,
    `activeJiraPreset`, `jiraRefreshNonce`, `jiraProjectStatusOrder`,
    `jiraOrderBy`, `jiraOrderDirection`, `jiraPrioritiesBySite`
  - Projects (2690–2691): `availableJiraProjects`, `jiraProjectsLoading`
- Effect liên quan (dep thật, TASK-BIGFILE-032):
  - 2206–2208 (deep-link mở Jira issue)
  - 2446–2472 (priorities fetch, dep gồm `jiraConnected`, `settings`,
    `taskSource` — đọc thêm, không sở hữu)
  - 2693–2724 (projects fetch)
  - 5693–5780 (3 effect — search debounce + fetch chính, effect chính
    5714–5780 dài, dep: `taskSource, jiraConnected, selectedJiraSiteId,
    appliedJiraSearch, activeJiraPreset, jiraRefreshNonce, taskResumeApplied,
    jiraTaskSourceContext, jiraTaskSourceScopeKey`)
  - 5792–5812 (đồng bộ `selectedJiraIssueFallback` theo danh sách hiển thị)
- Tham số đọc cần nhận: `taskSource`, `taskResumeApplied`, `jiraConnected`,
  `settings`, `selectedJiraSiteId`, `jiraTaskSourceContext`,
  `jiraTaskSourceScopeKey`, `pageData.openJiraIssue`.

## Output

- File mới: `frontend/src/renderer/src/components/use-task-page-jira-browse-state.ts`
- `TaskPage.tsx`: thay 16 `useState` + 6 effect bằng 1 lời gọi hook.

## Các bước

Giống quy trình TASK-BIGFILE-235.

## Xác minh xong

- [ ] `pnpm exec tsc --noEmit -p frontend/tsconfig.json --composite false`
- [ ] `pnpm exec oxlint` trên 2 file đã đổi
- [ ] `pnpm exec vitest run --config config/vitest.config.ts
      src/renderer/src/components/feature-interaction-writer-boundaries.test.ts`
- [ ] `gitnexus detect_changes({scope: "all"})` hoặc grep thủ công
- [ ] `node scripts/find-frontend-bigfiles.mjs` — giảm thêm ~180–220 dòng
- [ ] Kiểm tra thủ công luồng Jira tab: search, preset filter, chọn issue,
      sort/status order.

## Rollback

```
git checkout -- frontend/src/renderer/src/components/TaskPage.tsx
rm frontend/src/renderer/src/components/use-task-page-jira-browse-state.ts
```
