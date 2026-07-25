# TASK-008: Tạo `src/main/auth/auth-manager.ts`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 1 — Auth Foundation
**Solution:** [SOL-LG-001](../solutions/SOL-LG-001-auth-session.md) §4.4
**Depends on:** TASK-003, TASK-004, TASK-007
**Blocks:** TASK-009, TASK-010, TASK-011, TASK-012

---

## Mục tiêu

Tạo `AuthManager` — facade thống nhất cho toàn bộ auth operations. Được inject vào HTTP server và admin router.

---

## File cần tạo

**Path:** `src/main/auth/auth-manager.ts`

---

## Nội dung

```typescript
// src/main/auth/auth-manager.ts
import type { IDatabase } from '../db/types'
import { AuthSessionStore } from './auth-session-store'
import { AuthUserStore }    from './auth-user-store'
import { AuthLocalHandler } from './auth-local-handler'
import type { OrcaSession, LocalLoginInput, LocalLoginResult } from './auth-types'

const CLEANUP_INTERVAL_MS = 30 * 60 * 1000  // 30 phút

export class AuthManager {
  readonly sessionStore: AuthSessionStore
  readonly userStore:    AuthUserStore
  readonly localHandler: AuthLocalHandler

  private cleanupTimer: ReturnType<typeof setInterval> | null = null

  constructor(db: IDatabase) {
    this.sessionStore = new AuthSessionStore(db)
    this.userStore    = new AuthUserStore(db)
    this.localHandler = new AuthLocalHandler(this.userStore, this.sessionStore)

    // Cleanup expired sessions định kỳ
    this.cleanupTimer = setInterval(() => {
      const removed = this.sessionStore.cleanupExpired()
      if (removed > 0) console.log(`[AuthManager] Cleaned up ${removed} expired sessions`)
    }, CLEANUP_INTERVAL_MS)

    // Không block process exit
    if (this.cleanupTimer.unref) this.cleanupTimer.unref()
  }

  /**
   * Validate session từ HTTP cookie header.
   * Dùng trong middleware hoặc admin handlers.
   */
  validateRequest(cookieHeader: string | undefined): OrcaSession | null {
    const sessionId = extractSessionIdFromCookie(cookieHeader)
    if (!sessionId) return null
    return this.sessionStore.validateSession(sessionId)
  }

  async login(input: LocalLoginInput, ip: string, ua: string): Promise<LocalLoginResult> {
    return this.localHandler.login(input, ip, ua)
  }

  logout(sessionId: string): void {
    this.sessionStore.revokeSession(sessionId)
  }

  destroy(): void {
    if (this.cleanupTimer) {
      clearInterval(this.cleanupTimer)
      this.cleanupTimer = null
    }
  }
}

function extractSessionIdFromCookie(cookieHeader: string | undefined): string | null {
  if (!cookieHeader) return null
  const match = cookieHeader.match(/orca_session=([a-f0-9]{64})/)
  return match ? match[1]! : null
}
```

---

## Acceptance Criteria

- [x] File tồn tại, TypeScript compile sạch
- [x] `AuthManager` constructor khởi tạo đủ 3 sub-stores
- [x] `validateRequest()` parse cookie `orca_session=<64-hex>` → `OrcaSession | null`
- [x] `validateRequest()` trả về `null` khi cookie không có hoặc session expired
- [x] Cleanup timer dùng `.unref()` để không giữ process sống
- [x] `destroy()` clear cleanup timer (tránh memory leak trong tests)
