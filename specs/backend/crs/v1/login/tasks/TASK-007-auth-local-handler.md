# TASK-007: Tạo `auth-local-handler.ts` + test

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 1 — Auth Foundation
**Solution:** [SOL-LG-001](../solutions/SOL-LG-001-auth-session.md) §3.3, §4
**Depends on:** TASK-002, TASK-003, TASK-004
**Blocks:** TASK-008 (auth-manager)

---

## Mục tiêu

Tạo `AuthLocalHandler` — xử lý login email/password. Viết test với mock stores.

---

## File cần tạo: `src/main/auth/auth-local-handler.ts`

```typescript
// src/main/auth/auth-local-handler.ts
import type { AuthUserStore } from './auth-user-store'
import type { AuthSessionStore } from './auth-session-store'
import type { LocalLoginInput, LocalLoginResult } from './auth-types'

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export class AuthLocalHandler {
  constructor(
    private readonly userStore:    AuthUserStore,
    private readonly sessionStore: AuthSessionStore
  ) {}

  async login(input: LocalLoginInput, ip: string, ua: string): Promise<LocalLoginResult> {
    // 1. Validate input format trước khi query DB
    if (!input.email || !input.password) {
      return { success: false, error: 'validation_error', detail: 'email and password required' }
    }
    if (!EMAIL_REGEX.test(input.email)) {
      return { success: false, error: 'validation_error', detail: 'invalid email format' }
    }

    // 2. Verify credentials
    const user = await this.userStore.verifyPassword(input.email, input.password)
    if (!user) {
      return { success: false, error: 'invalid_credentials' }
    }

    // 3. Create session
    const session = await this.sessionStore.createSession({
      userId: user.id, userEmail: user.email, role: user.role,
      ipAddress: ip, userAgent: ua
    })

    return { success: true, sessionId: session.sessionId, user }
  }
}
```

---

## File cần tạo: `src/main/auth/__tests__/auth-local-handler.test.ts`

```typescript
// src/main/auth/__tests__/auth-local-handler.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { AuthLocalHandler } from '../auth-local-handler'
import type { AuthUserStore } from '../auth-user-store'
import type { AuthSessionStore } from '../auth-session-store'

describe('AuthLocalHandler', () => {
  let userStore: AuthUserStore
  let sessionStore: AuthSessionStore
  let handler: AuthLocalHandler

  const mockUser = {
    id: 'u1', email: 'a@test.com', name: 'A', role: 'developer' as const, provider: 'none' as const
  }
  const mockSession = {
    sessionId: 'sid-abc123', userId: 'u1', userEmail: 'a@test.com', role: 'developer' as const,
    createdAt: Date.now(), expiresAt: Date.now() + 28800000,
    lastSeenAt: null, ipAddress: '127.0.0.1', userAgent: 'ua'
  }

  beforeEach(() => {
    userStore    = { verifyPassword: vi.fn() } as any
    sessionStore = { createSession: vi.fn() }  as any
    handler = new AuthLocalHandler(userStore, sessionStore)
  })

  describe('login — success', () => {
    it('returns sessionId and user on valid credentials', async () => {
      vi.mocked(userStore.verifyPassword).mockResolvedValue(mockUser)
      vi.mocked(sessionStore.createSession).mockResolvedValue(mockSession)

      const result = await handler.login({ email: 'a@test.com', password: 'pw' }, '127.0.0.1', 'ua')
      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.sessionId).toBe('sid-abc123')
        expect(result.user.id).toBe('u1')
      }
    })

    it('calls verifyPassword with correct email', async () => {
      vi.mocked(userStore.verifyPassword).mockResolvedValue(mockUser)
      vi.mocked(sessionStore.createSession).mockResolvedValue(mockSession)
      await handler.login({ email: 'a@test.com', password: 'correct' }, '1.2.3.4', 'ua')
      expect(userStore.verifyPassword).toHaveBeenCalledWith('a@test.com', 'correct')
    })
  })

  describe('login — failure', () => {
    it('returns invalid_credentials when verifyPassword returns null', async () => {
      vi.mocked(userStore.verifyPassword).mockResolvedValue(null)
      const result = await handler.login({ email: 'a@test.com', password: 'wrong' }, '1.2.3.4', 'ua')
      expect(result.success).toBe(false)
      if (!result.success) expect(result.error).toBe('invalid_credentials')
    })

    it('returns validation_error for invalid email format', async () => {
      const result = await handler.login({ email: 'not-an-email', password: 'pw' }, '1.2.3.4', 'ua')
      expect(result.success).toBe(false)
      if (!result.success) expect(result.error).toBe('validation_error')
      // Should NOT query the database
      expect(userStore.verifyPassword).not.toHaveBeenCalled()
    })

    it('returns validation_error for empty email', async () => {
      const result = await handler.login({ email: '', password: 'pw' }, '1.2.3.4', 'ua')
      expect(result.success).toBe(false)
      if (!result.success) expect(result.error).toBe('validation_error')
    })

    it('does not create session on failed login', async () => {
      vi.mocked(userStore.verifyPassword).mockResolvedValue(null)
      await handler.login({ email: 'a@test.com', password: 'wrong' }, '1.2.3.4', 'ua')
      expect(sessionStore.createSession).not.toHaveBeenCalled()
    })
  })
})
```

---

## Cách chạy test

```bash
pnpm test src/main/auth/__tests__/auth-local-handler.test.ts
```

---

## Acceptance Criteria

- [x] `auth-local-handler.ts` tồn tại, TypeScript compile sạch
- [x] Email validation không query DB trước khi validate format
- [x] Test: success path trả về `sessionId` và `user`
- [x] Test: sai password → `invalid_credentials`
- [x] Test: email format sai → `validation_error` và không gọi `verifyPassword`
- [x] Test: `createSession` không được gọi khi login fail
- [x] Tất cả 6 test cases pass
