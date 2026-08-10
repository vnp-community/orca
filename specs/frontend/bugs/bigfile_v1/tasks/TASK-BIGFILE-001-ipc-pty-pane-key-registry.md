# TASK-BIGFILE-001 — Move: `pty-pane-key-registry.ts`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ⛔ Blocked
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md`

## ⛔ Blocked (2026-08-10) — không thực thi

Kiểm tra thực tế dòng 210–464 cho thấy `getPtyIdForPaneKey`,
`registerPaneKeyTeardownListener`, `hasPendingRendererSerializerForPaneKey`
chia sẻ state module-private (`paneKeyPtyId`, `ptyPaneKey`,
`pendingByPaneKey`, `paneSpawnReservationsByPaneKey`,
`ptyPendingGenByPtyId`, `rendererSerializerByPtyId`,
`paneKeyTeardownListeners`) được đọc/ghi tại 40+ điểm rải khắp
`registerPtyHandlers` (dòng 1140–5145) — KHÔNG tách rời cơ học như mô tả.
Cần thiết kế lại thành task Investigate trước; xem ghi chú tổng hợp trong
`TASKS-INDEX.md` § "Phát hiện khi thực thi". Nội dung template Move gốc bên
dưới được giữ nguyên làm tài liệu tham khảo, KHÔNG dùng để thực thi trực
tiếp.

## Input

- File nguồn: `frontend/src/main/ipc/pty.ts`
- Đọc **đúng dòng 219–460** (không cần đọc phần khác của file — 4,725 dòng
  còn lại không liên quan tới task này).
- Symbol cần chuyển (3 export function):
  - `getPtyIdForPaneKey`
  - `registerPaneKeyTeardownListener`
  - `hasPendingRendererSerializerForPaneKey`

## Output

- File mới: `frontend/src/main/ipc/pty-pane-key-registry.ts`
- File nguồn (`ipc/pty.ts`) thay 3 định nghĩa trên bằng:
  ```ts
  export {
    getPtyIdForPaneKey,
    registerPaneKeyTeardownListener,
    hasPendingRendererSerializerForPaneKey
  } from './pty-pane-key-registry'
  ```

## Các bước

1. `gitnexus impact({target: "getPtyIdForPaneKey", direction: "upstream"})` —
   lặp lại cho 2 symbol còn lại. Dừng nếu risk = HIGH/CRITICAL, báo cáo lại.
2. Đọc dòng 219–460 của `ipc/pty.ts` để lấy nguyên văn 3 hàm + xác định
   import cần thiết (module nào chúng dùng — chỉ import những gì thực sự
   dùng trong đúng đoạn này).
3. Tạo `pty-pane-key-registry.ts`: paste nguyên văn 3 hàm + import.
4. Sửa `ipc/pty.ts`: xoá 3 định nghĩa gốc (dòng 219–460), thêm dòng
   `export { ... } from './pty-pane-key-registry'` tại đúng vị trí đó.
5. Kiểm tra 3 hàm này có dùng chung 1 biến/`Map` module-level nào không — nếu
   có, biến đó PHẢI chuyển theo cùng file mới (không để lại ở `ipc/pty.ts`).

## Xác minh xong

- [ ] `pnpm run typecheck` (3 target: node/cli/web) pass
- [ ] `pnpm run lint` pass
- [ ] `gitnexus detect_changes({scope: "all"})` — risk = low, chỉ đúng 3
      symbol này bị đổi vị trí (không có "surprise")
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `ipc/pty.ts` giảm ~240 dòng
- [ ] Test liên quan (nếu có `pty.test.ts` cùng thư mục) pass

## Rollback

```
git checkout -- frontend/src/main/ipc/pty.ts
rm frontend/src/main/ipc/pty-pane-key-registry.ts
```
