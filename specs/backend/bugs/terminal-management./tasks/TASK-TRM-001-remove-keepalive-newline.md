# TASK-TRM-001: Xóa keepalive `\n` vào Unix socket trong WsSessionRouter

**Priority:** 🔴 HIGH — session corruption mỗi 15 giây  
**Effort:** ~5 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-TM-002  
**Solution ref:** [SOLUTION-TRM-BE-exact.md](../solutions/SOLUTION-TRM-BE-exact.md)

---

## Mục tiêu

Xóa `setInterval` keepalive đang gửi byte `\n` vào Unix socket JSON-RPC stream mỗi 15 giây, gây parse error ở user process.

## File cần sửa

```
src/main/session/ws-session-router.ts
```

## Thay đổi cụ thể

### Xóa lines 98–102:
```typescript
// XÓA ĐOẠN NÀY:
const keepaliveTimer = setInterval(() => {
  if (upstream.writable) {
    upstream.write('\n')
  }
}, 15000)
```

### Cập nhật ws.on('close') — xóa clearInterval (line 140):
```diff
     ws.on('close', () => {
-      clearInterval(keepaliveTimer)
       upstream.end()
       this.sessionManager.touch(userId)
     })
```

### Cập nhật upstream.on('close') — xóa clearInterval (line 147):
```diff
     upstream.on('close', () => {
-      clearInterval(keepaliveTimer)
       const wsAny = ws as unknown as { ... }
       if (wsAny.readyState === wsAny.OPEN) wsAny.close(1011, 'User session ended')
     })
```

## Lý do

Unix domain sockets là local IPC — không có TCP NAT timeout, không cần keepalive. Bare `\n` không phải JSON-RPC payload hợp lệ → user process nhận được và có thể fail parse, gây session corruption sau mỗi 15 giây idle.

## Verification

```bash
# Sau fix, idle terminal sessions không có corruption sau 15 giây
# Check: không còn parse error logs từ user-process-entry

# Run existing tests:
pnpm vitest run src/main/session/__tests__/
```

## Context

- `ws-session-router.ts` là proxy layer giữa Browser WebSocket ↔ Unix socket (user process)
- Unix socket dùng JSON-RPC 2.0 newline-delimited protocol
- WebSocket-level keepalive (ping/pong) hoạt động riêng ở tầng WS, không cần can thiệp
