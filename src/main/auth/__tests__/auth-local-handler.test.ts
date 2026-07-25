/**
 * AuthLocalHandler Unit Tests
 *
 * Uses mocked userStore and sessionStore — no real DB needed.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { AuthLocalHandler } from '../auth-local-handler'
import type { AuthUserStore } from '../auth-user-store'
import type { AuthSessionStore } from '../auth-session-store'

describe('AuthLocalHandler', () => {
  let userStore:    AuthUserStore
  let sessionStore: AuthSessionStore
  let handler:      AuthLocalHandler

  const mockUser = {
    id: 'u1', email: 'alice@test.com', name: 'Alice',
    role: 'developer' as const, provider: 'none' as const
  }
  const mockSession = {
    sessionId:   'sid-deadbeef'.padEnd(64, '0'),
    userId:      'u1',
    userEmail:   'alice@test.com',
    role:        'developer' as const,
    createdAt:   Date.now(),
    expiresAt:   Date.now() + 28800000,
    lastSeenAt:  null,
    ipAddress:   '127.0.0.1',
    userAgent:   'vitest'
  }

  beforeEach(() => {
    userStore    = { verifyPassword: vi.fn() } as unknown as AuthUserStore
    sessionStore = { createSession:  vi.fn() } as unknown as AuthSessionStore
    handler = new AuthLocalHandler(userStore, sessionStore)
  })

  // ── success path ───────────────────────────────────────────────────────────

  describe('login — success', () => {
    it('returns sessionId and user on valid credentials', async () => {
      vi.mocked(userStore.verifyPassword).mockResolvedValue(mockUser)
      vi.mocked(sessionStore.createSession).mockResolvedValue(mockSession)

      const result = await handler.login({ email: 'alice@test.com', password: 'correct' }, '127.0.0.1', 'ua')
      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.sessionId).toBe(mockSession.sessionId)
        expect(result.user.id).toBe('u1')
      }
    })

    it('calls verifyPassword with the correct email and password', async () => {
      vi.mocked(userStore.verifyPassword).mockResolvedValue(mockUser)
      vi.mocked(sessionStore.createSession).mockResolvedValue(mockSession)

      await handler.login({ email: 'alice@test.com', password: 'secret' }, '1.2.3.4', 'ua')
      expect(userStore.verifyPassword).toHaveBeenCalledWith('alice@test.com', 'secret')
    })

    it('passes ipAddress and userAgent to createSession', async () => {
      vi.mocked(userStore.verifyPassword).mockResolvedValue(mockUser)
      vi.mocked(sessionStore.createSession).mockResolvedValue(mockSession)

      await handler.login({ email: 'alice@test.com', password: 'pw' }, '10.0.0.1', 'Firefox/100')
      expect(sessionStore.createSession).toHaveBeenCalledWith(expect.objectContaining({
        ipAddress: '10.0.0.1', userAgent: 'Firefox/100'
      }))
    })
  })

  // ── failure paths ──────────────────────────────────────────────────────────

  describe('login — failure', () => {
    it('returns invalid_credentials when verifyPassword returns null', async () => {
      vi.mocked(userStore.verifyPassword).mockResolvedValue(null)

      const result = await handler.login({ email: 'alice@test.com', password: 'wrong' }, '127.0.0.1', 'ua')
      expect(result.success).toBe(false)
      if (!result.success) expect(result.error).toBe('invalid_credentials')
    })

    it('returns validation_error for invalid email format', async () => {
      const result = await handler.login({ email: 'not-an-email', password: 'pw' }, '127.0.0.1', 'ua')
      expect(result.success).toBe(false)
      if (!result.success) expect(result.error).toBe('validation_error')
      // Must NOT query the database for invalid format
      expect(userStore.verifyPassword).not.toHaveBeenCalled()
    })

    it('returns validation_error for empty email', async () => {
      const result = await handler.login({ email: '', password: 'pw' }, '127.0.0.1', 'ua')
      expect(result.success).toBe(false)
      if (!result.success) expect(result.error).toBe('validation_error')
      expect(userStore.verifyPassword).not.toHaveBeenCalled()
    })

    it('returns validation_error for empty password', async () => {
      const result = await handler.login({ email: 'a@test.com', password: '' }, '127.0.0.1', 'ua')
      expect(result.success).toBe(false)
      if (!result.success) expect(result.error).toBe('validation_error')
      expect(userStore.verifyPassword).not.toHaveBeenCalled()
    })

    it('does NOT create session on failed login', async () => {
      vi.mocked(userStore.verifyPassword).mockResolvedValue(null)
      await handler.login({ email: 'a@test.com', password: 'wrong' }, '127.0.0.1', 'ua')
      expect(sessionStore.createSession).not.toHaveBeenCalled()
    })

    it('does NOT create session when email format is invalid', async () => {
      await handler.login({ email: 'bad-email', password: 'pw' }, '127.0.0.1', 'ua')
      expect(sessionStore.createSession).not.toHaveBeenCalled()
    })
  })
})
