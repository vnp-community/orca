# TASK-BIGFILE-239 — Move: `useTaskPageLinearDraftState()` custom hook

**Loại:** Move (tách state thành custom hook — xem lưu ý "khác biệt với Move
cơ học" ở TASK-BIGFILE-235, áp dụng y hệt ở đây) · **Effort:** L
**Phụ thuộc:** TASK-BIGFILE-235 đã xong · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 2)
**Sinh ra từ:** TASK-BIGFILE-032 (Investigate)

## Input

- File nguồn: `frontend/src/renderer/src/components/TaskPage.tsx`
- **Xác nhận lại vị trí thật** bằng
  `grep -n "newLinearProjectOpen\|newLinearIssueOpen\|linearConnectOpen" TaskPage.tsx`
  trước khi làm.
- 26 state:
  - New-project draft (3533–3544, 12 state): `newLinearProjectOpen`,
    `newLinearProjectName`, `newLinearProjectDescription`,
    `newLinearProjectContent`, `newLinearProjectTeamId`,
    `newLinearProjectLeadId`, `newLinearProjectMemberIds`,
    `newLinearProjectLabelIds`, `newLinearProjectPriority`,
    `newLinearProjectStartDate`, `newLinearProjectTargetDate`,
    `newLinearProjectSubmitting`
  - New-issue draft (3568–3586, 13 state): `newLinearIssueOpen`,
    `newLinearIssueTitle`, `newLinearIssueBody`, `newLinearIssueTeamId`,
    `newLinearIssueSubmitting`, `newLinearIssueStateId`,
    `newLinearIssueAssigneeId`, `newLinearIssuePriority`,
    `newLinearIssueProjectId`, `newLinearIssueLabelIds`,
    `newLinearIssueProjects`, `newLinearIssueProjectsLoading`
  - Connect dialog (3668, 1 state): `linearConnectOpen`
- Effect liên quan: 3561–3565, 3588–3640 (3 effect — hydrate team/workspace
  cho draft), 3658–3666 (default state cho new-issue draft theo
  `newLinearStates.data`)
- **LƯU Ý QUAN TRỌNG** (giống TASK-BIGFILE-237): effect dòng 3726–3762 đọc
  `newLinearIssueOpen` để điều phối chéo với `newJiraIssueOpen` — **KHÔNG
  chuyển effect này vào hook**, để lại ở `TaskPage.tsx`. Hook chỉ export
  `newLinearIssueOpen`/`setNewLinearIssueOpen`.
- Tham số đọc cần nhận: `linearConnected`, `settings`,
  `linearTaskSourceContext`, `selectedLinearWorkspaceId`,
  `selectedLinearProject` (từ hook browse, TASK-BIGFILE-241 — nếu 241 CHƯA
  làm khi thực thi task này, giữ tham chiếu trực tiếp tới state cũ trong
  `TaskPage.tsx` và ghi chú lại phụ thuộc ngược tạm thời để dọn ở 241).

## Output

- File mới: `frontend/src/renderer/src/components/use-task-page-linear-draft-state.ts`
- `TaskPage.tsx`: thay 26 `useState` + 4 effect (KHÔNG gồm effect
  3726–3762) bằng 1 lời gọi hook.

## Các bước

Giống quy trình TASK-BIGFILE-235. Chú ý thứ tự thực thi: nếu làm TRƯỚC
TASK-BIGFILE-241 (Linear browse), hook này sẽ tạm thời còn phụ thuộc vào
state `selectedLinearProject`/`selectedLinearWorkspaceId` vẫn nằm ở
`TaskPage.tsx` — chấp nhận được (composition pattern, giống cách domain nhỏ
của `OrcaRuntimeService` inject field từ domain khác qua constructor), ghi
rõ trong PR description để 241 xử lý gọn lại khi tới lượt.

## Xác minh xong

- [ ] `pnpm exec tsc --noEmit -p frontend/tsconfig.json --composite false`
- [ ] `pnpm exec oxlint` trên 2 file đã đổi
- [ ] `pnpm exec vitest run --config config/vitest.config.ts
      src/renderer/src/components/feature-interaction-writer-boundaries.test.ts`
- [ ] `gitnexus detect_changes({scope: "all"})` hoặc grep thủ công
- [ ] `node scripts/find-frontend-bigfiles.mjs` — giảm thêm ~280–320 dòng
- [ ] Kiểm tra thủ công luồng tạo Linear project mới + issue mới + connect
      dialog.

## Rollback

```
git checkout -- frontend/src/renderer/src/components/TaskPage.tsx
rm frontend/src/renderer/src/components/use-task-page-linear-draft-state.ts
```
