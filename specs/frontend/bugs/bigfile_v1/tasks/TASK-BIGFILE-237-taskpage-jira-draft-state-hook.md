# TASK-BIGFILE-237 — Move: `useTaskPageJiraDraftState()` custom hook

**Loại:** Move (tách state thành custom hook — xem lưu ý "khác biệt với Move
cơ học" ở TASK-BIGFILE-235, áp dụng y hệt ở đây) · **Effort:** M
**Phụ thuộc:** TASK-BIGFILE-235 đã xong · **Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 2)
**Sinh ra từ:** TASK-BIGFILE-032 (Investigate)

## Input

- File nguồn: `frontend/src/renderer/src/components/TaskPage.tsx`
- **Xác nhận lại vị trí thật** bằng
  `grep -n "newJiraIssueOpen\|jiraConnectOpen" TaskPage.tsx` trước khi làm.
- 20 state:
  - New-issue draft (dòng 3699–3714): `newJiraIssueOpen`,
    `newJiraIssueTitle`, `newJiraIssueBody`, `newJiraIssueProjectId`,
    `newJiraIssueProjectComboboxOpen`, `newJiraIssueProjectQuery`,
    `newJiraIssueProjectCommandValue`, `newJiraIssueTypeId`,
    `newJiraIssueSubmitting`, `availableJiraIssueTypes`,
    `jiraIssueTypesLoading`, `jiraCreateFields`, `jiraCreateFieldsLoading`,
    `jiraCreateFieldsError`, `newJiraIssueCustomFieldValues`
  - Connect dialog (dòng 3717–3722): `jiraConnectOpen`, `jiraSiteUrlDraft`,
    `jiraEmailDraft`, `jiraApiTokenDraft`, `jiraConnectState`,
    `jiraConnectError`
- Effect liên quan: 3815–3829 (project combobox), 3875–3957 (2 effect —
  hydrate create fields theo project/type)
- **LƯU Ý QUAN TRỌNG**: effect dòng 3726–3762 có dep
  `newJiraIssueOpen, newLinearIssueOpen, providerRuntimeContextKey` — đây là
  effect CHÉO DOMAIN (đóng draft Linear khi mở draft Jira và ngược lại, xem
  TASK-BIGFILE-032 mục "4 effect cross-domain"). **KHÔNG chuyển effect này
  vào hook** — để lại ở `TaskPage.tsx`, hook chỉ export
  `newJiraIssueOpen`/`setNewJiraIssueOpen` để `TaskPage.tsx` tự dùng trong
  effect điều phối đó.
- Tham số đọc cần nhận: `settings`, `jiraConnected`, `jiraTaskSourceContext`.

## Output

- File mới: `frontend/src/renderer/src/components/use-task-page-jira-draft-state.ts`
- `TaskPage.tsx`: thay 20 `useState` + 3 effect (KHÔNG gồm effect
  3726–3762) bằng 1 lời gọi hook.

## Các bước

Giống quy trình TASK-BIGFILE-235. Bước riêng: xác nhận effect 3726–3762 ở
lại `TaskPage.tsx` — hook chỉ trả `newJiraIssueOpen`/`setNewJiraIssueOpen`
để component chính tiếp tục dùng trong effect điều phối chéo-domain đó.

## Xác minh xong

- [ ] `pnpm exec tsc --noEmit -p frontend/tsconfig.json --composite false`
- [ ] `pnpm exec oxlint` trên 2 file đã đổi
- [ ] `pnpm exec vitest run --config config/vitest.config.ts
      src/renderer/src/components/feature-interaction-writer-boundaries.test.ts`
- [ ] `gitnexus detect_changes({scope: "all"})` hoặc grep thủ công
- [ ] `node scripts/find-frontend-bigfiles.mjs` — giảm thêm ~200–240 dòng
- [ ] Kiểm tra thủ công luồng tạo Jira issue mới + connect dialog.

## Rollback

```
git checkout -- frontend/src/renderer/src/components/TaskPage.tsx
rm frontend/src/renderer/src/components/use-task-page-jira-draft-state.ts
```
