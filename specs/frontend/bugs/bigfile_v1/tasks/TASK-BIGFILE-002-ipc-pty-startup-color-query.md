# TASK-BIGFILE-002 — Move: `pty-startup-color-query.ts`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ⛔ Superseded — xem TASK-BIGFILE-250
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md`

## ⛔ Blocked (2026-08-10) — không thực thi

`answerStartupTerminalColorQueriesForPty` cần `getProvider`/
`getProviderForPty`/`tryGetProviderForPty`/`getAppPtyId`/`getRelayPtyId`/
`getProviderForStartupTerminalColorReply` — các helper resolve-provider
này được dùng tại 40+ điểm trong `registerPtyHandlers` (dòng 1486–5092),
KHÔNG tách rời cơ học như mô tả. Cần thiết kế lại thành task Investigate
trước; xem `TASKS-INDEX.md` § "Phát hiện khi thực thi". Nội dung bên dưới
giữ làm tài liệu tham khảo, KHÔNG dùng để thực thi trực tiếp.

## Input

- File nguồn: `frontend/src/main/ipc/pty.ts`
- Đọc **đúng dòng 461–544**.
- Symbol cần chuyển: `answerStartupTerminalColorQueriesForPty`

## Output

- File mới: `frontend/src/main/ipc/pty-startup-color-query.ts`
- File nguồn thay định nghĩa bằng:
  ```ts
  export { answerStartupTerminalColorQueriesForPty } from './pty-startup-color-query'
  ```

## Các bước

1. `gitnexus impact({target: "answerStartupTerminalColorQueriesForPty", direction: "upstream"})`
   — dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 461–544, copy nguyên văn + import cần thiết.
3. Tạo file mới, paste. Sửa `ipc/pty.ts` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `ipc/pty.ts` giảm ~85 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/main/ipc/pty.ts
rm frontend/src/main/ipc/pty-startup-color-query.ts
```
