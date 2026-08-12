# TASK-BIGFILE-240 — Move: `useTaskPageLinearTeamsState()` custom hook

**Loại:** Move (tách state thành custom hook — xem lưu ý "khác biệt với Move
cơ học" ở TASK-BIGFILE-235, áp dụng y hệt ở đây) · **Effort:** S
**Phụ thuộc:** TASK-BIGFILE-235 đã xong · **Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 2)
**Sinh ra từ:** TASK-BIGFILE-032 (Investigate)

## Input

- File nguồn: `frontend/src/renderer/src/components/TaskPage.tsx`
- **Xác nhận lại vị trí thật** bằng
  `grep -n "availableTeams\|linearTeamSelection\|linearAttributeFilter" TaskPage.tsx`
  trước khi làm.
- 3 state: `availableTeams`, `linearTeamRefreshNonce` (2645–2646),
  `linearTeamSelection` (2903)
- Effect liên quan:
  - 2648–2679: fetch teams (dep: `taskSource, linearConnected,
    selectedLinearWorkspaceId, linearTeamRefreshNonce, taskResumeApplied,
    getCachedLinearTeams, listLinearTeams, linearTaskSourceContext`)
  - 3045–3052: default team selection theo `linearTeamOptions`
  - 3072–3099: 2 effect reconcile `linearAttributeFilter` theo team đổi —
    **LƯU Ý**: `linearAttributeFilter` bản thân là state thuộc nhóm Linear
    browse (TASK-BIGFILE-241), KHÔNG thuộc domain teams. Hook này chỉ sở
    hữu `availableTeams`/`linearTeamRefreshNonce`/`linearTeamSelection`;
    2 effect reconcile filter cần `linearAttributeFilter`/
    `setLinearAttributeFilter` làm tham số (đọc+ghi) truyền vào từ hook 241
    hoặc từ `TaskPage.tsx` nếu 241 chưa làm.
- Tham số đọc cần nhận: `taskSource`, `taskResumeApplied`, `linearConnected`,
  `selectedLinearWorkspaceId`, `linearTaskSourceContext`,
  `linearAttributeFilter` + `setLinearAttributeFilter` (đọc-ghi, xem lưu ý
  trên).

## Output

- File mới: `frontend/src/renderer/src/components/use-task-page-linear-teams-state.ts`
- `TaskPage.tsx`: thay 3 `useState` + 3 effect bằng 1 lời gọi hook.

## Các bước

Giống quy trình TASK-BIGFILE-235. Domain nhỏ (3 state) — effort S, nhưng
LƯU Ý phụ thuộc 2 chiều với `linearAttributeFilter` (thuộc domain browse,
241) cần xử lý bằng tham số đọc-ghi qua lại, không tự ý gộp state đó vào
hook teams.

## Xác minh xong

- [ ] `pnpm exec tsc --noEmit -p frontend/tsconfig.json --composite false`
- [ ] `pnpm exec oxlint` trên 2 file đã đổi
- [ ] `pnpm exec vitest run --config config/vitest.config.ts
      src/renderer/src/components/feature-interaction-writer-boundaries.test.ts`
- [ ] `gitnexus detect_changes({scope: "all"})` hoặc grep thủ công
- [ ] `node scripts/find-frontend-bigfiles.mjs` — giảm thêm ~60–80 dòng
- [ ] Kiểm tra thủ công: đổi team filter trong Linear tab, xác nhận
      `linearAttributeFilter` reconcile đúng.

## Rollback

```
git checkout -- frontend/src/renderer/src/components/TaskPage.tsx
rm frontend/src/renderer/src/components/use-task-page-linear-teams-state.ts
```
