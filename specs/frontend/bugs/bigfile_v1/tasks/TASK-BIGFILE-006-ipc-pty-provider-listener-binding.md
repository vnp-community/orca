# TASK-BIGFILE-006 — Move: `pty-provider-listener-binding.ts`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ⛔ Blocked
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md`

## ⛔ Blocked (2026-08-10) — không thực thi

`unbindLocalProviderListeners` thao tác trực tiếp trên provider-listener
state dùng chéo với nhóm TASK-002/004 (provider resolution). Chặn theo
nhóm — xem `TASKS-INDEX.md` § "Phát hiện khi thực thi". Nội dung bên dưới
giữ làm tài liệu tham khảo, KHÔNG dùng để thực thi trực tiếp.

## Input

- File nguồn: `frontend/src/main/ipc/pty.ts`
- Đọc **đúng dòng 1448–1458** (khối rất nhỏ, ~10 dòng).
- Symbol cần chuyển: `unbindLocalProviderListeners`

## Output

- File mới: `frontend/src/main/ipc/pty-provider-listener-binding.ts`
- File nguồn thay bằng:
  ```ts
  export { unbindLocalProviderListeners } from './pty-provider-listener-binding'
  ```

## Các bước

1. `gitnexus impact({target: "unbindLocalProviderListeners", direction: "upstream"})`
   — dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 1448–1458, copy nguyên văn + import cần thiết.
3. **Lưu ý:** hàm này rất có thể tham chiếu ngược tới state trong
   `pty-ownership-registry.ts` (từ TASK-BIGFILE-004, tên gợi ý "listener
   binding" liên quan "ownership") — nếu đúng, import từ file đó thay vì
   duplicate logic. Chạy TASK-BIGFILE-004 TRƯỚC task này nếu chưa làm.
4. Tạo file mới, paste. Sửa `ipc/pty.ts` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `ipc/pty.ts` giảm ~10 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/main/ipc/pty.ts
rm frontend/src/main/ipc/pty-provider-listener-binding.ts
```
