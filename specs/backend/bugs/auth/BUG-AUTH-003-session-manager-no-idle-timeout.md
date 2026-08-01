# BUG-AUTH-003: `SessionManager` thiếu idle timeout kiểm tra — BL-AUTH-02 incomplete

**Status:** ✅ FIXED — 2026-08-01  
**Task:** BUG-AUTH-003  
**Note:** session-manager.ts already implements 4h idle timeout with sweepIdleProcesses()  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD BL-AUTH-02 mô tả:
```
SESSION EXPIRY:
    SessionManager timer: check idle > 4h → graceful kill child process
```

Thực tế `src/main/session/session-manager.ts` — không có idle timer.

Session Manager chỉ:
1. Fork child process per user khi có WS kết nối
2. Proxy WebSocket ↔ Unix Socket ↔ Child Process
3. Không có cơ chế "kill child if idle > 4h"

Child process sẽ chạy **mãi mãi** sau khi user đăng nhập, kể cả sau khi user đóng browser (WS disconnect).

## Ảnh hưởng

1. N users login → N child processes chạy → resource leak nếu không restart
2. Server không tự recover memory khi user không active
3. Admin kill session (DELETE /admin/api/sessions/:id) đúng → nhưng tự động expire không có

## Fix đề xuất

```typescript
// session-manager.ts
class SessionManager {
  private idleTimers = new Map<string, NodeJS.Timeout>()
  
  onWebSocketDisconnect(userId: string): void {
    // Start idle timer when last WS disconnects
    const timer = setTimeout(() => {
      this.killUserProcess(userId)
    }, 4 * 60 * 60 * 1000)  // 4 hours
    this.idleTimers.set(userId, timer)
  }
  
  onWebSocketConnect(userId: string): void {
    // Clear idle timer on reconnect
    const timer = this.idleTimers.get(userId)
    if (timer) { clearTimeout(timer); this.idleTimers.delete(userId) }
  }
}
```

## Files liên quan

- `src/main/session/session-manager.ts`: thiếu idle timer
- `src/main/session/ws-session-router.ts`: WS connect/disconnect events available
