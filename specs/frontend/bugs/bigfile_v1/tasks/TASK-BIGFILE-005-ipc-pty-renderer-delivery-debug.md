# TASK-BIGFILE-005 — Move: `pty-renderer-delivery-debug.ts`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ⛔ Blocked
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md`

## ⛔ Blocked (2026-08-10) — không thực thi

Chặn theo nhóm cùng TASK-001–004/006/007 — xem `TASKS-INDEX.md` §
"Phát hiện khi thực thi". Chưa xác minh riêng ranh giới dòng 1252–1357;
cần Investigate lại toàn nhóm `ipc/pty.ts` trước. Nội dung bên dưới giữ
làm tài liệu tham khảo, KHÔNG dùng để thực thi trực tiếp.

## Input

- File nguồn: `frontend/src/main/ipc/pty.ts`
- Đọc **đúng dòng 1252–1357**.
- Symbol cần chuyển: `type PtyRendererDeliveryDebugSnapshot`,
  `getPtyRendererDeliveryDebugSnapshot`, `resetPtyRendererDeliveryDebug`

## Output

- File mới: `frontend/src/main/ipc/pty-renderer-delivery-debug.ts`
- File nguồn thay bằng:
  ```ts
  export {
    getPtyRendererDeliveryDebugSnapshot,
    resetPtyRendererDeliveryDebug,
    type PtyRendererDeliveryDebugSnapshot
  } from './pty-renderer-delivery-debug'
  ```

## Các bước

1. `gitnexus impact` cho 2 hàm — dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 1252–1357, copy nguyên văn + import cần thiết (kiểm tra có biến
   module-level lưu debug counter/state hay không — nếu có, chuyển theo).
3. Tạo file mới, paste. Sửa `ipc/pty.ts` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `ipc/pty.ts` giảm ~110 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/main/ipc/pty.ts
rm frontend/src/main/ipc/pty-renderer-delivery-debug.ts
```
