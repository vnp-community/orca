# SOLUTION: Auth Domain — Fix tất cả Bugs

**Domain:** auth  
**TDD Reference:** TDD-04 (RPC Server Auth), TDD-11 (Web Server Mode), CR-LOGIN-001~004  
**Files cần thay đổi:** `src/main/auth/auth-local-handler.ts`, `src/server/http-server.ts`, `src/main/session/session-manager.ts`  
**Tổng số bugs:** 6 (AUTH-001~003, BE-AUTH-001~003)

---

## Tổng quan phụ thuộc

```
BUG-BE-AUTH-003 (per-user process isolation) — foundational, phải implement trước
    └── BUG-BE-AUTH-001 (login missing audit log)
    └── BUG-BE-AUTH-002 (cookie SameSite Lax)

BUG-AUTH-001 (login no audit log) — giống BE-AUTH-001
BUG-AUTH-002 (cookie SameSite Lax) — giống BE-AUTH-002
BUG-AUTH-003 (session no idle timeout) — liên quan BE-AUTH-003
```

**Thứ tự fix:** `BE-AUTH-003 → AUTH-003 → BE-AUTH-001/AUTH-001 → BE-AUTH-002/AUTH-002`

---

## BUG-BE-AUTH-003 & BUG-AUTH-003 — Fix per-user process isolation

**Mức độ:** 🔴 CRITICAL  
**Root cause:** Tất cả users share cùng process → user A có thể truy cập data của user B.

### Fix — Implement SessionManager per-user process isolation

Theo TDD v5 Addendum E `CR-LOGIN-002`:

```typescript
// src/main/session/session-manager.ts

import { fork, ChildProcess } from 'node:child_process'
import { join } from 'node:path'

export const ORCA_MULTI_USER = process.env.ORCA_MULTI_USER === '1'

export interface UserProcess {
  userId:     string
  pid:        number
  socketPath: string
  process:    ChildProcess
  lastActive: number
}

export interface SessionManagerConfig {
  userDataPath: string
  idleTimeoutMs: number  // default: 4 * 60 * 60 * 1000 (4 giờ)
  spawnTimeoutMs: number // default: 30_000 (30s)
}

export class SessionManager {
  private processes = new Map<string, UserProcess>()
  private cleanupTimer?: NodeJS.Timeout

  constructor(private readonly config: SessionManagerConfig) {
    // Cleanup idle processes mỗi 10 phút
    this.cleanupTimer = setInterval(() => this.cleanupIdleProcesses(), 10 * 60 * 1000)
  }

  /**
   * Get hoặc spawn user process.
   * Mỗi userId có một process riêng với Unix socket.
   */
  async getOrSpawnUserProcess(userId: string): Promise<UserProcess> {
    const existing = this.processes.get(userId)
    if (existing && existing.process.exitCode === null) {
      existing.lastActive = Date.now()
      return existing
    }

    return await this.spawnUserProcess(userId)
  }

  private async spawnUserProcess(userId: string): Promise<UserProcess> {
    const socketPath = join(this.config.userDataPath, 'users', userId, 'orca.sock')
    
    const child = fork(
      join(__dirname, 'user-process-entry'),
      [],
      {
        env: {
          ...process.env,
          ORCA_USER_ID:     userId,
          ORCA_SOCKET_PATH: socketPath,
          NODE_OPTIONS:     '--max-old-space-size=512',
        },
        stdio: ['ignore', 'inherit', 'inherit', 'ipc'],
      }
    )

    const userProcess: UserProcess = {
      userId,
      pid: child.pid!,
      socketPath,
      process: child,
      lastActive: Date.now(),
    }

    this.processes.set(userId, userProcess)

    // Wait for process ready signal
    await new Promise<void>((resolve, reject) => {
      const timeout = setTimeout(() => reject(new Error('User process spawn timeout')), this.config.spawnTimeoutMs)
      child.once('message', (msg: any) => {
        if (msg?.type === 'ready') {
          clearTimeout(timeout)
          resolve()
        }
      })
      child.once('exit', (code) => {
        clearTimeout(timeout)
        reject(new Error(`User process exited with code ${code}`))
      })
    })

    return userProcess
  }

  /**
   * FIX AUTH-003: Session idle timeout.
   * Terminate process sau khi không có activity trong idleTimeoutMs.
   */
  private cleanupIdleProcesses(): void {
    const now = Date.now()
    for (const [userId, proc] of this.processes.entries()) {
      if (now - proc.lastActive > this.config.idleTimeoutMs) {
        proc.process.kill('SIGTERM')
        this.processes.delete(userId)
        this.log.info(`[SessionManager] Idle process terminated: userId=${userId}`)
      }
    }
  }

  destroy(): void {
    clearInterval(this.cleanupTimer)
    for (const [, proc] of this.processes) {
      proc.process.kill('SIGTERM')
    }
    this.processes.clear()
  }
}
```

---

## BUG-BE-AUTH-001 & BUG-AUTH-001 — Fix login thiếu audit log

**Mức độ:** 🟠 HIGH  
**Root cause:** Login thành công/thất bại không được ghi vào audit log → không thể detect brute force.

### Fix — Thêm audit log trong auth-local-handler.ts

Theo TDD v5 Addendum E `CR-LOGIN-004` (AuditLogger đã implement):

```typescript
// src/main/auth/auth-local-handler.ts

export class AuthLocalHandler {
  constructor(
    private readonly userStore: AuthUserStore,
    private readonly sessionStore: AuthSessionStore,
    private readonly auditLogger: AuditLogger,  // ← inject
  ) {}

  async login(
    email: string,
    password: string,
    ip: string,
    userAgent?: string,
  ): Promise<{ session: OrcaSession } | { error: string }> {
    const user = await this.userStore.findByEmail(email)

    // FIX AUTH-001: Log tất cả login attempts
    if (!user || !(await this.userStore.verifyPassword(user, password))) {
      // Log failed login
      await this.auditLogger.logAction(
        user?.id ?? 'unknown',
        email,
        'auth.login.failed',
        { reason: user ? 'wrong_password' : 'user_not_found', ip, userAgent },
        ip,
      )
      return { error: 'Invalid credentials' }
    }

    if (!user.active) {
      await this.auditLogger.logAction(user.id, email, 'auth.login.blocked', { reason: 'account_deactivated', ip }, ip)
      return { error: 'Account deactivated' }
    }

    const session = await this.sessionStore.create(user.id, user.email, user.role)
    
    // Log successful login
    await this.auditLogger.logAction(
      user.id,
      email,
      'auth.login.success',
      { sessionId: session.id, ip, userAgent },
      ip,
    )

    return { session }
  }

  async logout(sessionId: string, userId: string, ip: string): Promise<void> {
    await this.sessionStore.delete(sessionId)
    await this.auditLogger.logAction(userId, '', 'auth.logout', { sessionId, ip }, ip)
  }
}
```

---

## BUG-BE-AUTH-002 & BUG-AUTH-002 — Fix cookie SameSite Lax → Strict

**Mức độ:** 🟠 HIGH (Security — CSRF)  
**Root cause:** `SameSite: Lax` cho phép cookie được gửi trong cross-site GET requests → CSRF vulnerability.

### Fix — Đổi sang SameSite: Strict

```typescript
// src/server/http-server.ts hoặc src/main/auth/auth-router.ts

// TRƯỚC:
res.cookie('orca-session', session.id, {
  httpOnly: true,
  secure:   process.env.NODE_ENV === 'production',
  sameSite: 'lax',  // BUG
  maxAge:   8 * 60 * 60 * 1000,  // 8h
})

// SAU:
res.cookie('orca-session', session.id, {
  httpOnly: true,
  secure:   process.env.NODE_ENV === 'production',
  sameSite: 'strict',  // FIX: Strict ngăn CSRF hoàn toàn
  maxAge:   8 * 60 * 60 * 1000,  // 8h (= SESSION_TTL_MS từ auth-types.ts)
  path:     '/',
})

// Note: SameSite: Strict có thể break OAuth redirect flows.
// Nếu cần OAuth, dùng separate short-lived token, không dùng session cookie.
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/session/session-manager.ts` | Implement per-user process fork | BE-AUTH-003 |
| `src/main/session/ws-session-router.ts` | Route WS theo userId → user socket | BE-AUTH-003 |
| `src/main/session/user-process-entry.ts` | NEW — fork entry point | BE-AUTH-003 |
| `src/main/auth/auth-local-handler.ts` | Add AuditLogger.logAction on login | AUTH-001, BE-AUTH-001 |
| `src/server/http-server.ts` | Cookie sameSite: 'lax' → 'strict' | AUTH-002, BE-AUTH-002 |
| `src/main/session/session-manager.ts` | Add idle timeout cleanup | AUTH-003 |

---

## Verification Plan

```bash
# Security tests:
# 1. Login with wrong password → verify audit log entry created
# 2. Login success → verify audit log entry + session cookie with SameSite=Strict
# 3. Cross-site request with Lax cookie → verify rejected (Strict mode)
# 4. Idle session after 4h → verify process terminated

# Multi-user isolation test:
# 1. Login as user A → make request → verify only user A's data returned
# 2. Login as user B → make same request → verify different data (isolated)
# 3. User A's process crash → verify user B still works

# Unit tests:
pnpm vitest run src/main/auth/__tests__/
pnpm vitest run src/main/session/__tests__/
```
