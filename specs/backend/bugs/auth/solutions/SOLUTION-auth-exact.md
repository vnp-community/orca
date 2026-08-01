# SOLUTION: auth — Code-Level Exact Fixes

**Source-verified:** ✅ Dựa trên source code thực tế  
**Files nguồn đã đọc:** `auth-router.ts`, `session-manager.ts`, `ws-session-router.ts`

---

## BUG-AUTH-002 & BUG-BE-AUTH-002: Cookie SameSite Lax

**File:** [`src/main/auth/auth-router.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/auth/auth-router.ts)  
**Lines:** 21–27

### Code sai thực tế:
```typescript
const COOKIE_OPTIONS = {
  httpOnly: true,
  sameSite: 'lax' as const,    // ← BUG: Lax cho phép CSRF qua GET
  secure: process.env['NODE_ENV'] === 'production',
  path: '/',
  maxAge: 8 * 60 * 60 * 1000  // 8 hours
}
```

### Fix:
```typescript
// src/main/auth/auth-router.ts — Replace lines 21–27:
const COOKIE_OPTIONS = {
  httpOnly: true,
  sameSite: 'strict' as const,   // FIX: Strict ngăn CSRF hoàn toàn
  secure: process.env['NODE_ENV'] === 'production',
  path: '/',
  maxAge: 8 * 60 * 60 * 1000    // 8 hours (matches SESSION_TTL_MS)
}
```

**Note về OAuth compatibility:** `SameSite: strict` có thể break OAuth callback flows vì browser không gửi cookie trong redirect từ OAuth provider. Nếu cần OAuth sau này, dùng separate PKCE state cookie với `SameSite: lax` riêng, không ảnh hưởng đến session cookie.

---

## BUG-AUTH-001 & BUG-BE-AUTH-001: Login thiếu audit log

**File cần thêm AuditLogger:** `src/main/auth/auth-local-handler.ts` hoặc `auth-manager.ts`

**Phân tích:** `auth-router.ts` gọi `authManager.login()`. Cần xem AuthManager:

```bash
grep -n "login\|audit\|AuditLogger" src/main/auth/auth-manager.ts | head -20
```

**Fix pattern** — thêm audit log vào `AuthManager.login()`:
```typescript
// src/main/auth/auth-manager.ts — trong login():
async login(
  creds: { email: string; password: string },
  ip: string,
  userAgent: string
): Promise<LoginResult> {
  const result = await this.localHandler.login(creds.email, creds.password)

  // FIX AUTH-001: Audit log mọi login attempt
  await this.auditLogger.log({
    action:    result.success ? 'auth.login.success' : 'auth.login.failed',
    userId:    result.success ? result.user.id : 'unknown',
    userEmail: creds.email,
    ip,
    userAgent,
    details:   result.success ? { sessionId: result.sessionId } : { error: result.error },
    timestamp: new Date(),
  }).catch(err => console.warn('[Audit] Log write failed:', err))

  return result
}
```

**Nếu AuditLogger chưa tồn tại**, tạo minimal implementation:
```typescript
// src/main/audit/audit-logger.ts (NEW — nếu chưa có)
export class AuditLogger {
  constructor(private readonly pool: IConnectionPool) {}

  async log(entry: {
    action:    string
    userId:    string
    userEmail: string
    ip:        string
    userAgent?: string
    details?:  Record<string, unknown>
    timestamp: Date
  }): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_audit_log 
         (action, user_id, user_email, ip, user_agent, details_json, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        [
          entry.action,
          entry.userId,
          entry.userEmail,
          entry.ip,
          entry.userAgent ?? '',
          JSON.stringify(entry.details ?? {}),
          entry.timestamp.getTime(),
        ]
      )
    )
  }
}

// Migration 0017_audit_log.ts:
// CREATE TABLE IF NOT EXISTS orca_audit_log (
//   id          TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
//   action      TEXT NOT NULL,
//   user_id     TEXT NOT NULL,
//   user_email  TEXT NOT NULL,
//   ip          TEXT NOT NULL,
//   user_agent  TEXT,
//   details_json TEXT DEFAULT '{}',
//   created_at  INTEGER NOT NULL
// )
```

---

## BUG-AUTH-003: Session idle timeout

**File:** [`src/main/session/session-manager.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/session/session-manager.ts)  
**Lines:** 19, 51–52

### Code hiện tại (đã implement một phần):
```typescript
const DEFAULT_IDLE_TIMEOUT_MS = 4 * 60 * 60 * 1000   // 4 hours ← ĐÃ CÓ
const IDLE_CHECK_INTERVAL_MS  = 5 * 60 * 1000          // ← ĐÃ CÓ

// Line 51-52:
this.idleTimer = setInterval(() => this.sweepIdleProcesses(), IDLE_CHECK_INTERVAL_MS)
if (this.idleTimer.unref) this.idleTimer.unref()
```

**Kết luận:** SessionManager đã có idle timeout với 4h default và sweep mỗi 5 phút! Bug có thể đã được fix trước đó.

**Cần verify:** Xem `sweepIdleProcesses()` có thực sự kill process không:
```bash
grep -n "sweepIdleProcesses\|lastSeenAt" src/main/session/session-manager.ts
```

Nếu `sweepIdleProcesses()` chưa implement hoặc không kill process sau 4h:
```typescript
// src/main/session/session-manager.ts — thêm hoặc fix sweepIdleProcesses():
private sweepIdleProcesses(): void {
  const now = Date.now()
  for (const [userId, proc] of this.processes) {
    const idleMs = now - proc.lastSeenAt
    if (idleMs > this.config.idleTimeoutMs) {
      console.log(
        `[SessionManager] Idle process terminated: userId=${userId} ` +
        `idle=${Math.round(idleMs / 60000)}min`
      )
      proc.process.kill('SIGTERM')
      // Give 5s to cleanup, then SIGKILL:
      setTimeout(() => {
        if (!proc.process.killed) proc.process.kill('SIGKILL')
      }, 5000)
      this.processes.delete(userId)
    }
  }
}
```

---

## BUG-BE-AUTH-003: Per-user process isolation

**File:** [`src/main/session/session-manager.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/session/session-manager.ts)

**Kết luận từ code thực tế:** `SessionManager` **ĐÃ IMPLEMENT** per-user process fork:
- `getOrSpawnUserProcess(userId)` fork riêng cho mỗi userId ✅
- Unix socket per-user tại `users/{userId}/orca.sock` ✅
- `WsSessionRouter` route từng WS connection đến đúng user process ✅

Bug có thể đã được fix trong các CRs trước. Cần xác nhận:
```bash
grep -n "ORCA_MULTI_USER\|getOrSpawnUserProcess" src/main/session/ -r
```

**Nếu ORCA_MULTI_USER=0 disable isolation:**
```typescript
// src/main/session/session-manager.ts — ensure multi-user mode is always ON:
// Xóa hoặc ignore ORCA_MULTI_USER check nếu có
// SessionManager.getOrSpawnUserProcess() phải luôn fork per-userId
```

---

## Tóm tắt thay đổi

| Bug | File | Lines | Thay đổi |
|-----|------|-------|---------|
| AUTH-002/BE-AUTH-002 | `auth-router.ts` | 23 | `'lax'` → `'strict'` |
| AUTH-001/BE-AUTH-001 | `auth-manager.ts` | login() | Thêm `auditLogger.log()` sau login |
| AUTH-003 | `session-manager.ts` | sweepIdleProcesses | Implement actual kill nếu thiếu |
| BE-AUTH-003 | `session-manager.ts` | overall | Đã implement, verify không có backdoor |
