# TASK-BIGFILE-023 — Move: `source-control-banners.tsx`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md` (Giai đoạn 1)

## Input

- File nguồn: `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx`
- Đọc **đúng dòng 7,534–7,700**.
- Symbol cần chuyển: `ConflictSummaryCard`, `OperationBanner`,
  `TooManyChangesBanner`

## Output

- File mới: `frontend/src/renderer/src/components/right-sidebar/source-control-banners.tsx`
- File nguồn thay bằng `export { ... } from './source-control-banners'`

## Các bước

1. `gitnexus impact` cho 3 symbol — dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 7,534–7,700, copy nguyên văn + import cần thiết.
3. Tạo file mới, paste. Sửa `SourceControl.tsx` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `SourceControl.tsx` giảm
      ~165 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx
rm frontend/src/renderer/src/components/right-sidebar/source-control-banners.tsx
```
