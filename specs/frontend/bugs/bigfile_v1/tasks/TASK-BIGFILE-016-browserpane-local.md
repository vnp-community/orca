# TASK-BIGFILE-016 — Move: `browser-pane-local.tsx`

**Loại:** Move (cơ học) · **Effort:** M
**Phụ thuộc:** TASK-BIGFILE-015 (làm sau, để tránh 2 task cùng sửa
`BrowserPane.tsx` song song và conflict dòng)
**Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-010-browserpane.md`

## Input

- File nguồn: `frontend/src/renderer/src/components/browser-pane/BrowserPane.tsx`
- Đọc **đúng dòng 2,675–5,841** (đây là phần LỚN NHẤT của file, ~3,166 dòng
  — nhưng ranh giới đã xác định rõ, chỉ cần đọc để copy, không cần thiết kế
  lại).
- Symbol cần chuyển: `BrowserPagePane`

## Output

- File mới: `frontend/src/renderer/src/components/browser-pane/browser-pane-local.tsx`
- File nguồn import `BrowserPagePane` từ file mới.

## Các bước

1. `gitnexus impact({target: "BrowserPagePane", direction: "upstream"})` —
   dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 2,675–5,841, copy nguyên văn + import cần thiết.
3. Tạo file mới, `export function BrowserPagePane(...)`.
4. Sửa `BrowserPane.tsx`: xoá định nghĩa gốc, thêm import từ file mới.
5. Sau bước này, `BrowserPane.tsx` chỉ còn phần wrapper (dòng 783–900,
   `export default function BrowserPane(...)`) + 2 dòng import — xác nhận
   file gốc còn ~120 dòng.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `BrowserPane.tsx` giảm
      xuống ~120 dòng (đã ra khỏi danh sách bigfile hoàn toàn)
- [ ] Test liên quan pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/browser-pane/BrowserPane.tsx
rm frontend/src/renderer/src/components/browser-pane/browser-pane-local.tsx
```
