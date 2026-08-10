# TASK-BIGFILE-029 — Move: `task-page-jira-banner.tsx`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 1)

## Input

- File nguồn: `frontend/src/renderer/src/components/TaskPage.tsx`
- Đọc **đúng dòng 884–1,068**.
- Symbol cần chuyển: `TaskPageJiraErrorBanner`

## Output

- File mới: `frontend/src/renderer/src/components/task-page-jira-banner.tsx`
- File nguồn thay bằng `export { TaskPageJiraErrorBanner } from './task-page-jira-banner'`

## Các bước

1. `gitnexus impact({target: "TaskPageJiraErrorBanner", direction: "upstream"})`
   — dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 884–1,068, copy nguyên văn + import cần thiết.
3. Tạo file mới, paste. Sửa `TaskPage.tsx` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `TaskPage.tsx` giảm
      ~185 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/TaskPage.tsx
rm frontend/src/renderer/src/components/task-page-jira-banner.tsx
```
