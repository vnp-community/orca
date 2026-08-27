# TASK-BIGFILE-028 — Move: `task-page-linear-cells.tsx`

**Loại:** Move (cơ học) · **Effort:** S
**Phụ thuộc:** TASK-BIGFILE-027 (dùng `task-page-types.ts` nếu
`LinearStateCell` tham chiếu 3 type đã tách)
**Status:** ✅ Done (thêm `import` song song `export` — LinearStateCell dùng
trong JSX của TaskPage chính, `export ... from` không tạo local binding)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 1)

## Input

- File nguồn: `frontend/src/renderer/src/components/TaskPage.tsx`
- Đọc **đúng dòng 651–883**.
- Symbol cần chuyển: `LinearStateCell`

## Output

- File mới: `frontend/src/renderer/src/components/task-page-linear-cells.tsx`
- File nguồn thay bằng `export { LinearStateCell } from './task-page-linear-cells'`

## Các bước

1. `gitnexus impact({target: "LinearStateCell", direction: "upstream"})` —
   dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 651–883, copy nguyên văn + import cần thiết (nếu dùng
   `LinearProjectTab`/`LinearGroupSection`/`LinearIssueListRow`, import từ
   `./task-page-types` thay vì định nghĩa lại).
3. Tạo file mới, paste. Sửa `TaskPage.tsx` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `TaskPage.tsx` giảm
      ~230 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/TaskPage.tsx
rm frontend/src/renderer/src/components/task-page-linear-cells.tsx
```
