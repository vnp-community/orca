# TASK-BIGFILE-021 — Move: `source-control-commit-area.tsx`

**Loại:** Move (cơ học) · **Effort:** M · **Phụ thuộc:** — · **Status:** ✅ Done
(2026-08-12 — cũng chuyển `PRIMARY_ICONS` và type `CreatePrIntentNotice`/
`CreatePrIntentTone` theo `CommitArea` vì chỉ dùng độc quyền bởi component
này; file mới 546 dòng vượt ngân sách max-lines .tsx (400) do là verbatim
extraction của phần vốn đã to — thêm `eslint-disable max-lines -- Why:`,
đề xuất baseline riêng. Giảm 574 dòng, khớp ước tính ~500.)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md` (Giai đoạn 1)

## Input

- File nguồn: `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx`
- Đọc **đúng dòng 6,556–7,056**.
- Symbol cần chuyển: `CommitArea`

## Output

- File mới: `frontend/src/renderer/src/components/right-sidebar/source-control-commit-area.tsx`
- File nguồn thay bằng `export { CommitArea } from './source-control-commit-area'`

## Các bước

1. `gitnexus impact({target: "CommitArea", direction: "upstream"})` — dừng
   nếu risk HIGH/CRITICAL.
2. Đọc dòng 6,556–7,056, copy nguyên văn + import cần thiết (props type nếu
   định nghĩa riêng ngay trước component, copy cùng).
3. Tạo file mới, paste. Sửa `SourceControl.tsx` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `SourceControl.tsx` giảm
      ~500 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx
rm frontend/src/renderer/src/components/right-sidebar/source-control-commit-area.tsx
```
