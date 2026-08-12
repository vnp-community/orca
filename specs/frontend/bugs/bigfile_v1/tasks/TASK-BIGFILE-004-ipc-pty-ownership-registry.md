# TASK-BIGFILE-004 — Move: `pty-ownership-registry.ts`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ⛔ Superseded — xem TASK-BIGFILE-250
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md`

## ⛔ Blocked (2026-08-10) — không thực thi

`hasPtyProviderForInspection` và các hàm ownership-registry lân cận (dòng
1050–1251) nằm trong cùng vùng state pervasive được nhóm TASK-001/002
xác nhận dùng chéo khắp `registerPtyHandlers`. Chặn theo nhóm — xem
`TASKS-INDEX.md` § "Phát hiện khi thực thi". Nội dung bên dưới giữ làm
tài liệu tham khảo, KHÔNG dùng để thực thi trực tiếp.

## Input

- File nguồn: `frontend/src/main/ipc/pty.ts`
- Đọc **đúng dòng 1050–1251**.
- Symbol cần chuyển (8 export function):
  `registerRemotePtyProvider`, `unregisterRemotePtyProvider`,
  `getRemotePtyProvider`, `getLocalPtyProvider`, `setLocalPtyProvider`,
  `getPtyIdsForConnection`, `clearPtyOwnershipForConnection`,
  `clearProviderPtyState`, `deletePtyOwnership`, `setPtyOwnership`,
  `rebindLocalProviderListeners` (11 hàm — đếm lại số thực tế khi đọc, bảng
  export gốc liệt kê 11, không phải 8; xác nhận số chính xác khi đọc).

## Output

- File mới: `frontend/src/main/ipc/pty-ownership-registry.ts`
- File nguồn thay bằng `export { ... } from './pty-ownership-registry'` liệt
  kê đủ toàn bộ symbol đã xác nhận ở bước đọc.

## Các bước

1. `gitnexus impact` cho từng symbol — dừng nếu bất kỳ symbol nào risk
   HIGH/CRITICAL.
2. Đọc dòng 1050–1251 **toàn bộ** (đừng chỉ lấy theo tên hàm đã liệt kê ở
   trên — có thể có biến module-level (`Map`/`Set`) định nghĩa xen giữa các
   hàm, PHẢI chuyển theo cùng, đây là lý do nhóm 11 hàm này đi CHUNG 1 file
   thay vì tách riêng từng hàm — chúng nhiều khả năng chia sẻ state).
3. Tạo file mới, copy nguyên văn toàn bộ khối (bao gồm biến module-level nếu
   có) + import cần thiết.
4. Sửa `ipc/pty.ts` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `ipc/pty.ts` giảm ~200 dòng
- [ ] Test liên quan (nếu có) pass — đặc biệt test liên quan PTY ownership
      /reconnect vì nhóm hàm này quản lý mapping connectionId↔ptyId

## Rollback

```
git checkout -- frontend/src/main/ipc/pty.ts
rm frontend/src/main/ipc/pty-ownership-registry.ts
```
