# TASK-TRM-007: Fix WS 4401 — Frontend xử lý redirect về login

**Priority:** 🟠 HIGH — Browser không xử lý đúng khi session hết hạn  
**Effort:** ~20 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-TRM-BE-001, BUG-TRM-BE-002  
**Solution ref:** [SOLUTION-TRM-BE-exact.md](../solutions/SOLUTION-TRM-BE-exact.md)

---

## Mục tiêu

Khi WsSessionRouter đóng WS với code 4401 (session hết hạn hoặc chưa login), renderer phải hiển thị trang login thay vì lỗi generic.

## Bước 1 — Tìm WS close handler trong renderer

```bash
grep -rn "ws.on\('close'\|addEventListener.*close\|onclose\|4401" \
  src/renderer/src/ --include="*.ts" --include="*.tsx" | head -20
```

## Bước 2 — File cần sửa

Tìm file khởi tạo WebSocket kết nối tới backend (thường là `useRuntimeRpc.ts` hoặc `ws-client.ts`):

```bash
grep -rn "new WebSocket\|createWebSocket" src/renderer/src/ | head -10
```

## Bước 3 — Thêm 4401 handler

Trong WebSocket close handler của renderer:

```typescript
// Tìm đoạn ws.addEventListener('close', ...) hoặc ws.onclose = ...
// Thêm 4401 handler:

ws.addEventListener('close', (event) => {
  if (event.code === 4401) {
    // Session expired or not logged in — redirect to login
    console.warn('[WS] Session expired (4401) — redirecting to login')
    const returnPath = window.location.pathname + window.location.search
    window.location.href = `/login?redirect=${encodeURIComponent(returnPath)}`
    return
  }

  // ... existing close handling ...
})
```

## Bước 4 (Optional — chuẩn hơn) — HTTP 401 trước WS upgrade

Thay vì xử lý ở frontend, validate session trước khi accept WS upgrade trong `http-server.ts`:

```typescript
// src/server/http-server.ts hoặc src/main/server-bootstrap.ts
// Tìm: httpServer.on('upgrade', ...)
// Thêm auth check:

httpServer.on('upgrade', async (req, socket, head) => {
  // Kiểm tra session cookie trước khi upgrade
  const session = await authManager.validateRequest(req.headers.cookie)
  if (!session) {
    socket.write('HTTP/1.1 401 Unauthorized\r\nContent-Length: 0\r\n\r\n')
    socket.destroy()
    return
  }
  wss.handleUpgrade(req, socket, head, (ws) => {
    wsSessionRouter.handleConnection(ws, req)
  })
})
```

## Verification

```bash
pnpm tsc --noEmit

# Test: logout → terminal pane → page redirects to /login
# Test: login redirect preserves return URL
```
