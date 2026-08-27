# TASK-BIGFILE-025 — Move: `source-control-action-button.tsx`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ✅ Done
(2026-08-12 — không trùng tên component nào khác trong repo, xác nhận qua
grep toàn repo. Đây cũng là bước khiến import ngược từ
`source-control-tree-rows.tsx` (TASK-024) hết circular thật sự — barrel
re-export ở `SourceControl.tsx` giữ nguyên đường import đó hợp lệ. Giảm 51
dòng — thấp hơn ~75 vì tổng SourceControl.tsx qua 6 task 020–025 chỉ giảm
1,284 dòng, không phải ~2,670 như ước tính gốc, do các task trước không
chuyển toàn bộ dải dòng gốc mà chỉ đúng symbol nêu trong scope — xem ghi
chú TASK-020/021/022/024.)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md` (Giai đoạn 1)

## Input

- File nguồn: `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx`
- Đọc **đúng dòng 8,297–8,370** (khối cuối file).
- Symbol cần chuyển: `ActionButton`

## Output

- File mới: `frontend/src/renderer/src/components/right-sidebar/source-control-action-button.tsx`
- File nguồn thay bằng `export { ActionButton } from './source-control-action-button'`

## Các bước

1. `gitnexus impact({target: "ActionButton", direction: "upstream"})` — LƯU
   Ý tên `ActionButton` khá chung chung, có thể trùng tên với component khác
   trong repo — xác nhận đúng symbol trong `SourceControl.tsx` (dùng
   `file_path` param của gitnexus impact nếu cần disambiguate).
2. Đọc dòng 8,297–8,370, copy nguyên văn + import cần thiết.
3. Tạo file mới, paste. Sửa `SourceControl.tsx` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `SourceControl.tsx` giảm
      ~75 dòng, tổng cộng sau TASK 020–025: 8,370 → ~5,700 dòng (còn lại là
      `SourceControlInner`, xem TASK-BIGFILE-026)
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx
rm frontend/src/renderer/src/components/right-sidebar/source-control-action-button.tsx
```
