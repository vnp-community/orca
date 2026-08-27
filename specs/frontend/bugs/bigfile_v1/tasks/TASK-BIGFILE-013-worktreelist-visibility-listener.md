# TASK-BIGFILE-013 — Move: `worktree-list-visibility-listener.ts`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-008-worktreelist.md`

## Input

- File nguồn: `frontend/src/renderer/src/components/sidebar/WorktreeList.tsx`
- Đọc **quanh dòng 5,150** (hàm ngắn, ~20 dòng — xác nhận điểm kết thúc chính
  xác khi đọc, ước tính ban đầu 5,150–5,170).
- Symbol cần chuyển: `installWorktreeVisibleRefreshVisibilityListener`

## Output

- File mới: `frontend/src/renderer/src/components/sidebar/worktree-list-visibility-listener.ts`
- File nguồn thay bằng:
  ```ts
  export { installWorktreeVisibleRefreshVisibilityListener } from './worktree-list-visibility-listener'
  ```

## Các bước

1. `gitnexus impact({target: "installWorktreeVisibleRefreshVisibilityListener", direction: "upstream"})`
   — dừng nếu risk HIGH/CRITICAL.
2. Đọc quanh dòng 5,150, xác nhận điểm bắt đầu/kết thúc chính xác, copy
   nguyên văn + import cần thiết.
3. Tạo file mới, paste. Sửa `WorktreeList.tsx` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `WorktreeList.tsx` giảm
      ~20 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/sidebar/WorktreeList.tsx
rm frontend/src/renderer/src/components/sidebar/worktree-list-visibility-listener.ts
```
