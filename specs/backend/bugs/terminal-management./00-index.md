# Bug Reports — Terminal (Backend Side)

**Module:** `src/main/session/` + `src/main/auth/` + `src/main/runtime/`
**Phát hiện:** 2026-07-31
**Phiên bản Orca:** b15.openledger.vn (production)
**Ngữ cảnh:** Phân tích code review luồng `terminal.create` trên project `test-repo`

---

## Danh Sách Bugs

| ID | Mức độ | Tiêu đề | Files liên quan | Status |
|----|--------|---------|-----------------|--------|
| [BUG-TRM-BE-001](./BUG-TRM-BE-001-auth-route-mismatch.md) | 🔴 Critical | Auth route `/auth/login` không tồn tại — phải là `/auth/local` | `http-server.ts`, `auth-router.ts` | 🔴 Open |
| [BUG-TRM-BE-002](./BUG-TRM-BE-002-ws-auth-close-4401.md) | 🔴 Critical | WsSessionRouter đóng WS 4401 khi chưa login — terminal không tạo được | `ws-session-router.ts`, `auth-manager.ts` | 🔴 Open |
| [BUG-TRM-BE-003](./BUG-TRM-BE-003-user-process-fork-fail.md) | 🔴 Critical | User process fork fail (ENOENT/timeout) → terminal.create không bao giờ chạy được | `session-manager.ts`, `user-process-entry.ts` | 🔴 Open |
| [BUG-TRM-BE-004](./BUG-TRM-BE-004-rbac-scoped-token-terminal.md) | 🟠 High | Scoped token thiếu `allowedServerIds` → `terminal.*` bị RBAC forbidden | `runtime-rpc.ts` | 🔴 Open |

---

## Phân Loại theo Priority

### 🔴 Critical — Chặn toàn bộ Terminal flow

- **BUG-TRM-BE-001**: Frontend gọi sai route → login 404 → không có session → mọi terminal fail
- **BUG-TRM-BE-002**: Hệ quả trực tiếp của BE-001 — WS đóng 4401 vì cookie thiếu
- **BUG-TRM-BE-003**: User process không khởi được → terminal.create không chạy được ở phía backend

### 🟠 High — Terminal bị chặn bởi permission

- **BUG-TRM-BE-004**: Token không được cấp quyền đúng → `terminal.create` forbidden

---

## Tham Khảo

- [Terminal Create Flow](../../../../docs/flows/terminal-create-flow.md)
- [Multi-User Session Flow](../../../../docs/flows/multi-user-session.md)
- [Authentication Flow](../../../../docs/flows/authentication.md)
