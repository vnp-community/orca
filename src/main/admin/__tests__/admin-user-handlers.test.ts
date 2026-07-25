/**
 * AdminUserHandlers Unit Tests
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { AdminUserHandlers } from '../admin-user-handlers'
import type { AuthUserStore } from '../../auth/auth-user-store'
import type { AuthSessionStore } from '../../auth/auth-session-store'
import type { AuditLogger } from '../audit-logger'
import type { Request, Response } from 'express'

function mockRes(): Response {
  const res: Record<string, unknown> = {}
  res['status'] = vi.fn().mockReturnValue(res)
  res['json']   = vi.fn().mockReturnValue(res)
  return res as unknown as Response
}

function mockReq(overrides: Partial<Record<string, unknown>> = {}): Request {
  return {
    orcaSession: { userId: 'admin-1', userEmail: 'admin@test.com', role: 'admin' },
    body: {}, params: {}, query: {}, ip: '127.0.0.1',
    ...overrides
  } as unknown as Request
}

const mockUser = {
  id: 'u1', email: 'a@test.com', name: 'Alice',
  role: 'developer' as const, provider: 'local' as const,
  isActive: true, createdAt: Date.now()
}

describe('AdminUserHandlers', () => {
  let userStore:    AuthUserStore
  let sessionStore: AuthSessionStore
  let auditLogger:  AuditLogger
  let handlers:     AdminUserHandlers

  beforeEach(() => {
    userStore    = {
      listUsers:        vi.fn().mockReturnValue([mockUser]),
      createLocalUser:  vi.fn().mockResolvedValue(mockUser),
      deactivateUser:   vi.fn()
    } as unknown as AuthUserStore
    sessionStore = { revokeAllUserSessions: vi.fn().mockReturnValue(2) } as unknown as AuthSessionStore
    auditLogger  = { log: vi.fn() } as unknown as AuditLogger
    handlers = new AdminUserHandlers({ userStore, sessionStore, auditLogger })
  })

  // ── listUsers ──────────────────────────────────────────────────────────────

  describe('listUsers', () => {
    it('returns user list with total', async () => {
      const res = mockRes()
      await handlers.listUsers(mockReq(), res)
      expect((res as any).json).toHaveBeenCalledWith(
        expect.objectContaining({ users: [mockUser], total: 1 })
      )
    })

    it('calls userStore.listUsers()', async () => {
      await handlers.listUsers(mockReq(), mockRes())
      expect(userStore.listUsers).toHaveBeenCalled()
    })
  })

  // ── createUser ─────────────────────────────────────────────────────────────

  describe('createUser', () => {
    it('creates user and returns 201', async () => {
      const req = mockReq({ body: { email: 'new@test.com', name: 'New', password: 'pw123', role: 'developer' } })
      const res = mockRes()
      await handlers.createUser(req, res)
      expect(userStore.createLocalUser).toHaveBeenCalled()
      expect((res as any).status).toHaveBeenCalledWith(201)
    })

    it('logs audit event on create', async () => {
      const req = mockReq({ body: { email: 'new2@test.com', name: 'N', password: 'pw', role: 'developer' } })
      await handlers.createUser(req, mockRes())
      expect(auditLogger.log).toHaveBeenCalledWith(
        expect.objectContaining({ action: 'user.create' })
      )
    })

    it('returns 400 for missing fields', async () => {
      const req = mockReq({ body: { name: 'No Email' } })
      const res = mockRes()
      await handlers.createUser(req, res)
      expect((res as any).status).toHaveBeenCalledWith(400)
      expect(userStore.createLocalUser).not.toHaveBeenCalled()
    })

    it('returns 400 for invalid role', async () => {
      const req = mockReq({ body: { email: 'x@test.com', name: 'X', password: 'pw', role: 'superuser' } })
      const res = mockRes()
      await handlers.createUser(req, res)
      expect((res as any).status).toHaveBeenCalledWith(400)
    })

    it('returns 409 on duplicate email', async () => {
      vi.mocked(userStore.createLocalUser).mockRejectedValue(new Error('UNIQUE constraint failed'))
      const req = mockReq({ body: { email: 'dup@test.com', name: 'Dup', password: 'pw', role: 'developer' } })
      const res = mockRes()
      await handlers.createUser(req, res)
      expect((res as any).status).toHaveBeenCalledWith(409)
    })
  })

  // ── deactivateUser ─────────────────────────────────────────────────────────

  describe('deactivateUser', () => {
    it('deactivates user and revokes all sessions', async () => {
      const req = mockReq({ params: { id: 'u-target' } })
      const res = mockRes()
      await handlers.deactivateUser(req, res)
      expect(userStore.deactivateUser).toHaveBeenCalledWith('u-target')
      expect(sessionStore.revokeAllUserSessions).toHaveBeenCalledWith('u-target')
      expect((res as any).json).toHaveBeenCalledWith(expect.objectContaining({ ok: true }))
    })

    it('logs audit event on deactivate', async () => {
      const req = mockReq({ params: { id: 'u-target' } })
      await handlers.deactivateUser(req, mockRes())
      expect(auditLogger.log).toHaveBeenCalledWith(
        expect.objectContaining({ action: 'user.deactivate' })
      )
    })

    it('returns 400 when admin tries to deactivate themselves', async () => {
      const req = mockReq({ params: { id: 'admin-1' } })  // same as orcaSession.userId
      const res = mockRes()
      await handlers.deactivateUser(req, res)
      expect((res as any).status).toHaveBeenCalledWith(400)
      expect(userStore.deactivateUser).not.toHaveBeenCalled()
    })

    it('includes revokedSessions count in response', async () => {
      const req = mockReq({ params: { id: 'u-target' } })
      const res = mockRes()
      await handlers.deactivateUser(req, res)
      expect((res as any).json).toHaveBeenCalledWith(
        expect.objectContaining({ revokedSessions: 2 })
      )
    })
  })
})
