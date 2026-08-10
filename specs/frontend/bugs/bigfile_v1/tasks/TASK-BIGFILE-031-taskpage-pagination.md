# TASK-BIGFILE-031 — Move: `task-page-pagination.tsx`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 1)

## Input

- File nguồn: `frontend/src/renderer/src/components/TaskPage.tsx`
- Đọc **đúng dòng 2,951–3,055** (khối cuối cùng trước component chính, dòng
  3,056).
- Symbol cần chuyển: `PaginationBar`

## Output

- File mới: `frontend/src/renderer/src/components/task-page-pagination.tsx`
- File nguồn thay bằng `export { PaginationBar } from './task-page-pagination'`

## Các bước

1. `gitnexus impact({target: "PaginationBar", direction: "upstream"})` — LƯU
   Ý tên khá chung chung, xác nhận đúng symbol trong `TaskPage.tsx` (dùng
   `file_path` param nếu cần disambiguate — có thể có `PaginationBar` khác ở
   nơi khác trong repo).
2. Đọc dòng 2,951–3,055, copy nguyên văn + import cần thiết.
3. Tạo file mới, paste. Sửa `TaskPage.tsx` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `TaskPage.tsx` giảm
      ~105 dòng, tổng cộng sau TASK 027–031: 12,833 → ~9,777 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/TaskPage.tsx
rm frontend/src/renderer/src/components/task-page-pagination.tsx
```
