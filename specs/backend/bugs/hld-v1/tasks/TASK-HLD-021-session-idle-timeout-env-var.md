# TASK-HLD-021: Đọc SESSION_IDLE_TIMEOUT_MS từ env var ở cả 2 entry point

**Priority:** 🟡 MEDIUM — vận hành production không thể tinh chỉnh idle-timeout mà không sửa code
**Effort:** ~1 giờ (code + 2 test case)
**Status:** ✅ DONE — 2026-08-09 (`resolveIdleTimeoutMsFromEnv()` export mới trong `session-manager.ts` (gộp chung đợt sửa với TASK-HLD-020 do cùng vùng code); wire vào cả `server-bootstrap.ts` và `backend/src/server/index.ts`. Tương thích ngược 100% khi không set env var — `tsc --noEmit` sạch hoàn toàn cho cả 3 file.)
**Bug refs:** BUG-BE-HLD-011 (phần 2 — idle timeout config qua env var)
**Solution ref:** [SOLUTION-session-manager-exact.md](../solutions/SOLUTION-session-manager-exact.md)
**Depends on:** Không — độc lập, có thể làm song song với TASK-HLD-020

---

## Mục tiêu

`SessionManager` hỗ trợ override `idleTimeoutMs` qua constructor (`config.idleTimeoutMs`), nhưng cả 2 call site khởi tạo `SessionManager` (`server-bootstrap.ts:311-316` và `backend/src/server/index.ts:137-142`) đều **không truyền giá trị này**, nên luôn rơi về hằng số cứng `DEFAULT_IDLE_TIMEOUT_MS = 4h`. Grep toàn `backend/src/` cho `SESSION_IDLE_TIMEOUT_MS` cho 0 kết quả. Cần thêm hàm parse env var an toàn và gọi nó ở cả 2 entry point.

## File cần sửa/tạo

```
backend/src/main/session/session-manager.ts   (sửa — thêm hàm export resolveIdleTimeoutMsFromEnv)
backend/src/main/server-bootstrap.ts           (sửa — truyền idleTimeoutMs)
backend/src/server/index.ts                    (sửa — truyền idleTimeoutMs)
backend/src/main/session/session-manager.test.ts  (thêm test)
```

Không tạo file `utils`/`helpers` mới — theo quy tắc đặt tên trong AGENTS.md, hàm parse env được đặt ngay trong `session-manager.ts` (cùng module định nghĩa `idleTimeoutMs`).

## Thay đổi cụ thể

### 1. `session-manager.ts` — thêm hàm export `resolveIdleTimeoutMsFromEnv` (cạnh các hằng số hiện có)

```typescript
/**
 * Parse SESSION_IDLE_TIMEOUT_MS from the environment. Returns undefined (so the
 * SessionManager constructor's DEFAULT_IDLE_TIMEOUT_MS fallback applies) when the
 * var is unset, blank, non-numeric, or <= 0 — an invalid value must never silently
 * disable idle cleanup.
 */
export function resolveIdleTimeoutMsFromEnv(env: NodeJS.ProcessEnv = process.env): number | undefined {
  const raw = env['SESSION_IDLE_TIMEOUT_MS']
  if (raw === undefined || raw.trim() === '') return undefined

  const parsed = Number(raw)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    console.warn(
      `[SessionManager] Ignoring invalid SESSION_IDLE_TIMEOUT_MS="${raw}" — must be a positive number (ms). Using default.`
    )
    return undefined
  }
  return parsed
}
```

Quy tắc parse:
- `undefined`/chuỗi rỗng → trả `undefined` (constructor tự fallback `DEFAULT_IDLE_TIMEOUT_MS`, hành vi hiện tại giữ nguyên 100% khi không set env).
- `NaN` hoặc `<= 0` → log cảnh báo + trả `undefined` (không throw, không phá vỡ server boot).
- Số hợp lệ (`> 0`) → trả về số đó (ms).

### 2. `server-bootstrap.ts` — Lines 306-318 (thay thế)

```typescript
  if (process.env['ORCA_MULTI_USER'] === '1') {
    const { SessionManager, resolveIdleTimeoutMsFromEnv } = await import('./session/session-manager')
    const userProcessEntry = pathJoin(
      platform.app.getAppPath(), 'out', 'main', 'user-process-entry.js'
    )
    sessionManager = new SessionManager({
      baseDataPath: userDataPath,
      userProcessEntry,
      serverSecret: process.env['ORCA_SERVER_SECRET'],
      // FIX BUG-BE-HLD-011: allow ops to override idle timeout without a code change.
      idleTimeoutMs: resolveIdleTimeoutMsFromEnv(process.env),
      devServerManager
    })
    console.log('[ServerBootstrap] ✅ SessionManager initialized (multi-user mode, serverSecret present:', !!process.env['ORCA_SERVER_SECRET'], ')')
  }
```

### 3. `backend/src/server/index.ts` — Lines 128-142 (thay thế)

```typescript
    const { SessionManager, resolveIdleTimeoutMsFromEnv } = await import('../main/session/session-manager')
    const { WsSessionRouter }  = await import('../main/session/ws-session-router')
    const { WebSocketServer }  = await import('ws')
    const { resolve: resolvePath } = await import('node:path')
    const { AGENT_WS_PATH }    = await import('../shared/agent-wire-protocol')

    const baseDataPath      = adapter.app.getPath('userData')
    const userProcessEntry  = resolvePath(__dirname, 'user-process-entry.js')

    const sessionManager = new SessionManager({ 
      baseDataPath, 
      userProcessEntry,
      serverSecret: process.env['ORCA_SERVER_SECRET'],
      // FIX BUG-BE-HLD-011: allow ops to override idle timeout without a code change.
      idleTimeoutMs: resolveIdleTimeoutMsFromEnv(process.env),
      devServerManager
    })
```

### Lưu ý tương thích ngược

Khi `SESSION_IDLE_TIMEOUT_MS` không được set (trường hợp hiện tại của mọi deployment), `resolveIdleTimeoutMsFromEnv()` trả `undefined` và constructor fallback về `DEFAULT_IDLE_TIMEOUT_MS = 4h` y hệt hành vi cũ — **không có breaking change** cho các cấu hình đang chạy.

## Verification

```bash
cd /opt/repos/orca
pnpm --filter backend tsc --noEmit
pnpm --filter backend test session-manager

# Xác nhận cả 2 entry point đã import + dùng hàm mới
grep -n "resolveIdleTimeoutMsFromEnv" backend/src/main/server-bootstrap.ts backend/src/server/index.ts backend/src/main/session/session-manager.ts

# Test thủ công: set env var và xác nhận sweepIdleProcesses dùng timeout mới
SESSION_IDLE_TIMEOUT_MS=60000 ORCA_MULTI_USER=1 node backend/out/server/index.js
```

Test case cần thêm:

1. `resolveIdleTimeoutMsFromEnv`: unset → `undefined`; `"7200000"` → `7200000`; `"0"`, `"-100"`, `"abc"`, `"  "` → `undefined` + có log cảnh báo (spy `console.warn`).
2. Constructor honor override: tạo `SessionManager` với `idleTimeoutMs` từ `resolveIdleTimeoutMsFromEnv({ SESSION_IDLE_TIMEOUT_MS: '60000' })` → `sweepIdleProcesses()` kill process sau 60s idle thay vì 4h (dùng fake timers).
