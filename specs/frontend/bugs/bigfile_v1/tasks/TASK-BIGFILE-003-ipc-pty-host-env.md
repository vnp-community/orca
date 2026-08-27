# TASK-BIGFILE-003 — Move: `pty-host-env.ts`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ⛔ Superseded — xem TASK-BIGFILE-250
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md`

## ⛔ Blocked (2026-08-10) — không thực thi

Chặn theo nhóm cùng với TASK-001/002/004–007: `stripRemotePaneEnvWhenHooksDisabled`
và các helper liên quan nằm trong cùng vùng state/provider-resolution dùng
chéo pervasive trong `registerPtyHandlers` (xem `TASKS-INDEX.md` § "Phát
hiện khi thực thi"). Chưa xác minh riêng ranh giới dòng 545–833 có thực sự
tách rời được độc lập hay không — cần Investigate lại toàn nhóm trước khi mở
lại task này. Nội dung bên dưới giữ làm tài liệu tham khảo, KHÔNG dùng để
thực thi trực tiếp.

## Input

- File nguồn: `frontend/src/main/ipc/pty.ts`
- Đọc **đúng dòng 545–833**.
- Symbol cần chuyển: `type BuildPtyHostEnvOptions`, `buildPtyHostEnv`

## Output

- File mới: `frontend/src/main/ipc/pty-host-env.ts`
- File nguồn thay bằng:
  ```ts
  export { buildPtyHostEnv, type BuildPtyHostEnvOptions } from './pty-host-env'
  ```

## Các bước

1. `gitnexus impact({target: "buildPtyHostEnv", direction: "upstream"})` —
   dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 545–833, copy nguyên văn + import cần thiết.
3. Tạo file mới, paste. Sửa `ipc/pty.ts` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `ipc/pty.ts` giảm ~290 dòng
- [ ] Test liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/main/ipc/pty.ts
rm frontend/src/main/ipc/pty-host-env.ts
```
