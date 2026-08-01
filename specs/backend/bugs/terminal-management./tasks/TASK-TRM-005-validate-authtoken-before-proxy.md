# TASK-TRM-005: Thêm authToken validation trước khi proxy trong WsSessionRouter

**Priority:** 🟠 MEDIUM — silent fail nếu authToken empty  
**Effort:** ~5 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-TM-003  
**Solution ref:** [SOLUTION-TRM-BE-exact.md](../solutions/SOLUTION-TRM-BE-exact.md)

---

## Mục tiêu

Thêm validation check cho `authToken` trước khi bắt đầu proxy WebSocket ↔ Unix socket. Nếu `proc.authToken` là empty (trường hợp spawn lỗi), đóng WS ngay thay vì inject empty token gây silent fail.

## File cần sửa

```
src/main/session/ws-session-router.ts
```

## Thay đổi cụ thể

Sau lines 75–82 (sau `if (!socketPath)` check), thêm authToken validation:

**HIỆN TẠI (lines 75–82):**
```typescript
const socketPath = proc.socketPath
const authToken  = proc.authToken

if (!socketPath) {
  span.fail('no socket path', { userId })
  ws.close(1011, 'Internal error: user session socket unavailable')
  return
}
```

**SAU — thêm authToken check:**
```typescript
const socketPath = proc.socketPath
const authToken  = proc.authToken

if (!socketPath) {
  span.fail('no socket path', { userId })
  ws.close(1011, 'Internal error: user session socket unavailable')
  return
}

// FIX BE-TM-003: Validate authToken exists before proxying
if (!authToken) {
  span.fail('no auth token', { userId })
  ws.close(1011, 'Internal error: auth token unavailable for user session')
  return
}
```

## Lý do

`proc.authToken` được set khi user process gửi IPC 'ready' message. Nếu process spawn fail sau forking (race condition), `authToken` có thể là empty string. Nếu inject empty string vào RPC request, user process sẽ reject với "Invalid auth token" — silent fail mà không có meaningful error cho user.

## Verification

```bash
pnpm tsc --noEmit

# Test: spawn một user session bình thường → không bị ảnh hưởng
# Test: simulate missing authToken → WS đóng với code 1011 (không phải silent fail)
```
