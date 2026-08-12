# TASK-BIGFILE-012 — Investigate+Move: `worktree-list-helpers.ts`

**Loại:** Investigate+Move (xác nhận độc lập trước khi cắt) · **Effort:** M
**Phụ thuộc:** — · **Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-008-worktreelist.md`

## Input

- File nguồn: `frontend/src/renderer/src/components/sidebar/WorktreeList.tsx`
- Đọc **đúng dòng 324–1,223** (7 hàm rải rác trong khoảng này — KHÔNG liên
  tục, khác các task Move khác trong đợt này).
- Symbol cần chuyển (7 export function, vị trí rời rạc):
  `countRecordKeysByReference` (324), `shouldAdjustWorktreeSidebarMeasuredRowScroll`
  (334), `resolvePendingSidebarReveal` (342), `renderRowContainsWorktree`
  (1,050), `getRenderRowKey` (1,166), `getWorktreeDragGroups` (1,191),
  `canKeepImportedWorktreesHidden` (1,223)

## ⚠️ Xác nhận độc lập TRƯỚC khi cắt

Vì các hàm rải rác (không liên tục), giữa chúng có khoảng trống lớn (vd
342→1,050, ~700 dòng) chứa nội dung KHÁC (type definition hoặc đầu component
chính — chưa xác định). **Bắt buộc đọc để xác nhận:**
1. 7 hàm này có đọc/ghi biến module-level chung với phần nội dung XEN GIỮA
   chúng hay không (nếu có, KHÔNG tách — chúng không thực sự độc lập).
2. Khoảng trống giữa các hàm chứa gì (type dùng chung cần tách theo, hay
   phần đầu component chính cần giữ nguyên tại chỗ).

Nếu phát hiện các hàm này KHÔNG độc lập (phụ thuộc closure/biến chung với
phần component), **dừng lại**, ghi phát hiện vào
`../BUG-FE-BIGFILE-008-worktreelist.md`, KHÔNG thực hiện phần Move của task
này.

## Output (nếu xác nhận độc lập)

- File mới: `frontend/src/renderer/src/components/sidebar/worktree-list-helpers.ts`
- File nguồn thay 7 định nghĩa bằng `export { ... } from './worktree-list-helpers'`

## Các bước

1. Đọc dòng 324–1,223, xác nhận độc lập theo mục "⚠️" ở trên.
2. `gitnexus impact` cho 7 symbol — dừng nếu risk HIGH/CRITICAL.
3. Nếu độc lập: copy nguyên văn 7 hàm (bỏ qua phần xen giữa không liên
   quan) + import cần thiết.
4. Tạo file mới, paste. Sửa `WorktreeList.tsx` thành barrel export cho 7
   symbol (giữ nguyên phần xen giữa tại chỗ trong file gốc).

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `WorktreeList.tsx` giảm
      ~250 dòng (chỉ phần hàm, không tính khoảng trống)
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/sidebar/WorktreeList.tsx
rm frontend/src/renderer/src/components/sidebar/worktree-list-helpers.ts
```
