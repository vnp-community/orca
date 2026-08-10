# TASK-BIGFILE-022 — Move: `source-control-compare-summary.tsx`

**Loại:** Move (cơ học) · **Effort:** M · **Phụ thuộc:** — · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md` (Giai đoạn 1)

## Input

- File nguồn: `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx`
- Đọc **đúng dòng 7,057–7,533**.
- Symbol cần chuyển: `shouldRefreshBranchCompareForStatusHead`,
  `shouldRefreshBranchCompareForRemoteStatus`, `shouldShowCompareSummary`,
  `CompareSummary`, `CompareSummaryToolbarButton`

## Output

- File mới: `frontend/src/renderer/src/components/right-sidebar/source-control-compare-summary.tsx`
- File nguồn thay bằng `export { ... } from './source-control-compare-summary'`

## Các bước

1. `gitnexus impact` cho 5 symbol — dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 7,057–7,533, copy nguyên văn + import cần thiết.
3. Tạo file mới, paste. Sửa `SourceControl.tsx` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `SourceControl.tsx` giảm
      ~475 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx
rm frontend/src/renderer/src/components/right-sidebar/source-control-compare-summary.tsx
```
