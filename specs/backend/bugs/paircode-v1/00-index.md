# Bug Reports — PairCode v1 (Web Mode Browser Connection)

**Module:** `src/main/session/` + `src/renderer/src/web/web-preload-api.ts` + `src/server/index.ts`  
**Phát hiện:** 2026-07-27  
**Phiên bản Orca:** 1.4.138  
**Ngữ cảnh:** Phân tích sau khi deploy agent direct-websocket thành công nhưng UI Settings → Dev Servers không hiển thị gì. Orca server chạy ở web/server mode với `ORCA_MULTI_USER=1`, `ORCA_AUTH_MODE=local`.

---

## Bối Cảnh

Sau khi fix xong bugs `dev-server-v1`, agent đã kết nối thành công:
```log
[DevServerManager] Daemon agent connected: id=dev-local platform=linux node=unknown
```

Tuy nhiên UI tại `https://b15.openledger.vn` → **Settings → Dev Servers** không hiển thị bất kỳ dev server nào. Phân tích source code và test trực tiếp phát hiện 3 bugs liên quan đến cơ chế kết nối browser ↔ RPC server trong web mode.

---

## Kiến Trúc Thực Tế (Phát Hiện Qua Phân Tích)

```
                     ORCA SERVER (172.20.2.39)
                     ┌──────────────────────────────────┐
                     │                                  │
Agent (172.20.2.31)  │  Port 6769/agent  ✅ HOẠT ĐỘNG   │
 node agent.js ─────►│  AgentWebSocketServer             │
                     │  DevServerManager (in-memory)     │
                     │                                  │
Browser              │  Port 6768  ❌ CHƯA KẾT NỐI      │
 https://b15...  ───►│  OrcaRuntimeRpcServer             │
                     │  (yêu cầu Pair Code / E2EE)       │
                     │                                  │
                     └──────────────────────────────────┘
```

**Hai kênh hoàn toàn độc lập.** Agent connected ≠ Browser có thể đọc dữ liệu.

---

## Danh Sách Bugs

| ID | Mức độ | Tiêu đề | Files liên quan | Status |
|----|--------|---------|-----------------|--------|
| [BUG-PC-001](./BUG-PC-001-browser-requires-paircode.md) | 🔴 Critical | Browser yêu cầu Pair Code dù đã login email/password | `web-preload-api.ts`, `web-runtime-environment.ts` | 🔴 Open |
| [BUG-PC-002](./BUG-PC-002-ws-session-router-not-wired.md) | 🔴 Critical | `WsSessionRouter` được tạo nhưng không được wired vào WS server | `server/index.ts` | 🔴 Open |
| [BUG-PC-003](./BUG-PC-003-devserver-list-silent-fail.md) | 🟠 High | `devServer.list` fail silently — UI hiển thị empty không có lỗi | `web-preload-api.ts`, `useDevServersSync.ts` | 🔴 Open |

---

## Phân Loại theo Priority

### 🔴 Critical — Browser không thể kết nối RPC server
- **BUG-PC-001**: `requireActiveEnvironment()` throw khi không có pair code → mọi RPC call đều fail
- **BUG-PC-002**: Login email/password không tạo WS session vì `WsSessionRouter` chưa attached

### 🟠 High — Silent failure gây khó debug
- **BUG-PC-003**: `catch(() => [])` nuốt lỗi RPC → UI trống, không có error message

---

## Tác Động

- **UI Settings → Dev Servers**: luôn hiển thị "No dev servers configured" dù agent đã kết nối
- **Toàn bộ RPC calls** từ browser (`devServer.list`, `preflight.check`, `git.*`, v.v.) đều fail silently
- **Login email/password** hoàn toàn không tạo ra kênh RPC cho browser (WsSessionRouter chưa wired)
- User bắt buộc phải dùng Pair Code URL mỗi lần muốn dùng web UI

---

## Tham Khảo

- [Agent Connection Modes Flow](../../../../docs/flows/agent-connection-modes.md)
- [Dev Server v1 Bugs](../dev-server-v1/00-index.md)
- Source files:
  - [`src/server/index.ts`](../../../../../src/server/index.ts) — L129 `void wsRouter`
  - [`src/renderer/src/web/web-preload-api.ts`](../../../../../src/renderer/src/web/web-preload-api.ts) — L3067 `requireActiveEnvironment()`
  - [`src/renderer/src/hooks/useDevServersSync.ts`](../../../../../src/renderer/src/hooks/useDevServersSync.ts)
