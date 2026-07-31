# Tasks — PairCode v1 Bug Fixes

**Nguồn:** [solutions/](../solutions/)  
**Mục tiêu:** Chia nhỏ mỗi giải pháp thành các tác vụ độc lập, AI có thể thực thi từng cái mà không cần context từ cái khác.

---

## Danh Sách Tasks

| ID | Solution | Tiêu đề | File mục tiêu | Phụ thuộc | Est. |
|----|----------|---------|----------------|-----------|------|
| [TASK-PC-001](./TASK-PC-001-ws-session-router-wire.md) | SOL-PC-001 | Wire WsSessionRouter vào HTTP server upgrade event | `src/server/index.ts` | — | ✅ DONE |
| [TASK-PC-002](./TASK-PC-002-session-env-factory.md) | SOL-PC-002 | Thêm `createSessionWebRuntimeEnvironment()` | `src/renderer/src/web/web-runtime-environment.ts` | — | ✅ DONE |
| [TASK-PC-003](./TASK-PC-003-bootstrap-session-env.md) | SOL-PC-002 | Set session env trong bootstrap khi đã login | `src/renderer/src/web/main-web-bootstrap.tsx` | TASK-PC-002 | ✅ DONE |
| [TASK-PC-004](./TASK-PC-004-preload-session-client.md) | SOL-PC-002 | Handle session connectionType trong `getClientForEnvironment` | `src/renderer/src/web/web-preload-api.ts` | TASK-PC-002 | ✅ DONE |
| [TASK-PC-005](./TASK-PC-005-devserver-error-state.md) | SOL-PC-002 | Error state + logging cho `devServer.list` fail | `web-preload-api.ts`, `useDevServersSync.ts` | — | ✅ DONE |

---

## Thứ Tự Thực Hiện

```
Sprint 1 — Server-side (deploy trước):
  TASK-PC-001  Wire WsSessionRouter   ← Độc lập, chạy đầu tiên
  TASK-PC-002  Session env factory    ← Độc lập, chạy song song

Sprint 2 — Client-side (sau Sprint 1 và TASK-PC-002):
  TASK-PC-003  (sau TASK-PC-002) Bootstrap set session env
  TASK-PC-004  (sau TASK-PC-002) Preload session client
  TASK-PC-005  (độc lập) DevServer error state
```

**Deploy sequence:**
1. Implement + build TASK-PC-001 → deploy server → verify WsSessionRouter active
2. Implement TASK-PC-002, 003, 004, 005 → build frontend → verify e2e

---

## Bug Coverage

| Bug | Tasks fix |
|-----|-----------|
| BUG-PC-001 (Browser cần Pair Code) | TASK-PC-001 + TASK-PC-003 + TASK-PC-004 |
| BUG-PC-002 (WsSessionRouter not wired) | TASK-PC-001 |
| BUG-PC-003 (Silent fail `devServer.list`) | TASK-PC-005 |

---

## Format Mỗi Task File

Mỗi TASK file có cấu trúc chuẩn:
1. **Mục tiêu** — một câu ngắn
2. **Context** — files cần đọc trước
3. **Exact change** — đoạn code cần tìm + code thay thế (copy-paste ready)
4. **Verify** — lệnh kiểm tra kết quả
5. **Definition of Done** — checklist rõ ràng
