# TASK-BIGFILE-024 — Move: `source-control-tree-rows.tsx`

**Loại:** Move (cơ học) · **Effort:** M · **Phụ thuộc:** — · **Status:** ✅ Done
(2026-08-12 — đúng 2 symbol module-private nêu trong Input, KHÔNG chuyển
`DiffLineCounts`/`SubmodulePlaceholderRow`/`UncommittedEntryRow` nằm cùng
khoảng dòng gốc (ngoài scope). File mới import `ActionButton` NGƯỢC từ
`./SourceControl` (circular, chưa tách ở lúc này — TASK-BIGFILE-025 chạy
sau) — an toàn vì chỉ dùng trong JSX render, không ở module-eval time; xác
minh bằng chạy toàn bộ 1257 test trong thư mục. Giảm 134 dòng, không đạt
~595 vì lý do trên.)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md` (Giai đoạn 1)

## Input

- File nguồn: `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx`
- Đọc **đúng dòng 7,701–8,296**.
- Symbol cần chuyển: `SourceControlTreeDirectoryRow`,
  `SourceControlBranchTreeDirectoryRow` — **lưu ý: 2 component này KHÔNG
  export ở file gốc** (module-private, chỉ dùng nội bộ `SourceControlInner`).

## Output

- File mới: `frontend/src/renderer/src/components/right-sidebar/source-control-tree-rows.tsx`
  — `export` 2 component này (cần export để `SourceControl.tsx` import được,
  dù chúng không phải public API của module `SourceControl.tsx` ra bên
  ngoài).
- File nguồn: xoá 2 định nghĩa gốc, thêm
  `import { SourceControlTreeDirectoryRow, SourceControlBranchTreeDirectoryRow } from './source-control-tree-rows'`

## Các bước

1. `gitnexus impact` cho 2 symbol — vì chúng module-private, impact chỉ nên
   thấy caller trong CHÍNH `SourceControl.tsx` (`SourceControlInner`) — nếu
   thấy caller ở nơi khác, dừng lại và xác nhận lại (bất thường so với kỳ
   vọng).
2. Đọc dòng 7,701–8,296, copy nguyên văn + import cần thiết.
3. Tạo file mới, `export` 2 component. Sửa `SourceControl.tsx` dùng `import`
   thường (không phải barrel re-export, vì đây không phải public API).

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `SourceControl.tsx` giảm
      ~595 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx
rm frontend/src/renderer/src/components/right-sidebar/source-control-tree-rows.tsx
```
