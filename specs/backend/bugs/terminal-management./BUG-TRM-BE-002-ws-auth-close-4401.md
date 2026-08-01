# BUG-TRM-BE-002 — WsSessionRouter Đóng WS 4401 Khi Chưa Auth

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-TRM-007  
**Note:** web-session-client.ts: 4401 auth redirect  

**ID:** BUG-TRM-BE-002
**Mức độ:** 🔴 Critical
**Module:** `WsSessionRouter` — WebSocket auth guard
**Phát hiện:** 2026-07-31
**Status:** 🔴 Open

---

## Mô Tả

Khi browser mở WebSocket tới `wss://b15.openledger.vn` mà không có session cookie hợp lệ (do chưa login hoặc session hết hạn), `WsSessionRouter` đóng kết nối ngay lập tức với code **4401**. Terminal pane nhận được lỗi disconnect và không thể gửi `terminal.create` RPC.

Đây là hệ quả trực tiếp của BUG-TRM-BE-001 (login fail → không có cookie → WS 4401).

---

## Root Cause

**[`ws-session-router.ts:47-55`](../../../../src/main/session/ws-session-router.ts):**

```typescript
async handleConnection(ws: WebSocket, req: IncomingMessage): Promise<void> {
  const span = wsRouter.start()
  const userId = await this.resolveUserFromRequest(req)

  if (!userId) {
    span.fail('auth required', { cookie: req.headers.cookie ? 'present' : 'absent' })
    ws.close(4401, 'Authentication required. Please log in first.')
    return
  }
  // ...
}
```

`resolveUserFromRequest(req)` gọi `AuthManager.getSession(cookie)`. Nếu cookie vắng mặt hoặc invalid → trả về `null` → WS đóng ngay.

**Không có cơ chế:**
- Retry sau khi login
- Thông báo rõ ràng đến UI rằng cần login
- Redirect tự động đến trang login

---

## Tái Hiện

```bash
# Test WS upgrade không có cookie
wscat -c wss://b15.openledger.vn
# → connection closed with code 4401 'Authentication required'

# Hoặc qua curl
curl -v --include \
  -H "Upgrade: websocket" \
  -H "Connection: Upgrade" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  https://b15.openledger.vn/
# → HTTP 101 rồi immediate close 4401
```

**Trace log:**
```
[TRACE] wsSession:route → FAIL 'auth required' { cookie: 'absent' }
```

---

## Hậu Quả

- Browser WebSocket đóng ngay → `remote-runtime-pty-transport.ts` nhận disconnect
- `callRuntime('terminal.create', ...)` throw `WebSocket connection closed`
- Terminal pane hiện lỗi ngay khi mở
- Mọi RPC call (git, filesystem, agent) đều không thể thực hiện

---

## Fix Đề Xuất

### Phương án A — UI xử lý WS close 4401 bằng cách redirect login

```typescript
// renderer: ws client error handler
ws.on('close', (code, reason) => {
  if (code === 4401) {
    // Redirect đến trang login thay vì hiện lỗi generic
    navigate('/login?redirect=' + encodeURIComponent(window.location.pathname))
  }
})
```

### Phương án B — Trả về HTTP 401 trước khi upgrade (chuẩn hơn)

Validate session trước khi accept WS upgrade trong HTTP layer:

```typescript
// http-server.ts — upgrade handler
server.on('upgrade', (req, socket, head) => {
  const session = await authManager.getSession(req)
  if (!session) {
    socket.write('HTTP/1.1 401 Unauthorized\r\n\r\n')
    socket.destroy()
    return
  }
  wss.handleUpgrade(req, socket, head, (ws) => {
    wsSessionRouter.handleConnection(ws, req)
  })
})
```

---

## Files Liên Quan

| File | Vị trí | Vai trò |
|------|--------|---------|
| [`main/session/ws-session-router.ts`](../../../../src/main/session/ws-session-router.ts) | `handleConnection()` L47-55 | Bug location — close 4401 |
| [`main/auth/auth-manager.ts`](../../../../src/main/auth/auth-manager.ts) | `getSession()` | Session validation |
| [`server/http-server.ts`](../../../../src/server/http-server.ts) | WS upgrade path | Thiếu auth check trước upgrade |
