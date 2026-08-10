# TASK-BIGFILE-027 — Move: `task-page-types.ts`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 1)

## Input

- File nguồn: `frontend/src/renderer/src/components/TaskPage.tsx`
- Đọc **đúng dòng 600–650**.
- Symbol cần chuyển: `type LinearProjectTab`, `type LinearGroupSection`,
  `type LinearIssueListRow`

## Output

- File mới: `frontend/src/renderer/src/components/task-page-types.ts`
- File nguồn thay bằng `export type { ... } from './task-page-types'`

## Các bước

1. `gitnexus impact` cho 3 type — dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 600–650, copy nguyên văn + import cần thiết.
3. Tạo file mới, paste. Sửa `TaskPage.tsx` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `TaskPage.tsx` giảm ~50 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/TaskPage.tsx
rm frontend/src/renderer/src/components/task-page-types.ts
```
