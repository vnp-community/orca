/**
 * AdminSessionHandlers Unit Tests
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { AdminSessionHandlers } from '../admin-session-handlers'
import type { AuthSessionStore } from '../../auth/auth-session-store'
import type { AuditLogger } from '../audit-logger'
import type { Request, Response } from 'express'

function mockRes(): Response {
  const res: Record<string, unknown> = {}
  res['status'] = vi.fn().mockReturnValue(res)
  res['json']   = vi.fn().mockReturnValue(res)
  return res as unknown as Response
}

const adminSession = { userId: 'admin-1', userEmail: 'admin@test.com', role: 'admin' as const }

describe('AdminSessionHandlers', () => {
  let sessionStore: AuthSessionStore
  let auditLogger:  AuditLogger
  let handlers:     AdminSessionHandlers

  beforeEach(() => {
    sessionStore = {
      revokeSession:         vi.fn(),
      revokeAllUserSessions: vi.fn().mockReturnValue(3)
    } as unknown as AuthSessionStore
    auditLogger = { log: vi.fn() } as unknown as AuditLogger
    handlers = new AdminSessionHandlers({ sessionStore, auditLogger })
  })

  // ── listAllSessions ────────────────────────────────────────────────────────

  describe('listAllSessions', () => {
    it('returns stub response with sessions: [], total: 0', () => {
      const req = { orcaSession: adminSession } as unknown as Request
      const res = mockRes()
      handlers.listAllSessions(req, res)
      expect((res as any).json).toHaveBeenCalledWith(
        expect.objectContaining({ sessions: [], total: 0 })
      )
    })
  })

  // ── killSession ────────────────────────────────────────────────────────────

  describe('killSession', () => {
    it('revokes the specified session', () => {
      const req = { orcaSession: adminSession, params: { sessionId: 'sid-abc' }, ip: '127.0.0.1' } as unknown as Request
      handlers.killSession(req, mockRes())
      expect(sessionStore.revokeSession).toHaveBeenCalledWith('sid-abc')
    })

    it('logs audit event with targetSessionId', () => {
      const req = { orcaSession: adminSession, params: { sessionId: 'sid-xyz' }, ip: '1.2.3.4' } as unknown as Request
      handlers.killSession(req, mockRes())
      expect(auditLogger.log).toHaveBeenCalledWith(expect.objectContaining({
        action: 'session.kill',
        detail: expect.objectContaining({ targetSessionId: 'sid-xyz' })
      }))
    })

    it('returns { ok: true }', () => {
      const req = { orcaSession: adminSession, params: { sessionId: 'sid-1' }, ip: '127.0.0.1' } as unknown as Request
      const res = mockRes()
      handlers.killSession(req, res)
      expect((res as any).json).toHaveBeenCalledWith({ ok: true })
    })
  })

  // ── killAllUserSessions ────────────────────────────────────────────────────

  describe('killAllUserSessions', () => {
    it('revokes all sessions for target user', () => {
      const req = { orcaSession: adminSession, params: { userId: 'user-target' }, ip: '127.0.0.1' } as unknown as Request
      handlers.killAllUserSessions(req, mockRes())
      expect(sessionStore.revokeAllUserSessions).toHaveBeenCalledWith('user-target')
    })

    it('logs audit event with revokedCount', () => {
      const req = { orcaSession: adminSession, params: { userId: 'user-t' }, ip: '127.0.0.1' } as unknown as Request
      handlers.killAllUserSessions(req, mockRes())
      expect(auditLogger.log).toHaveBeenCalledWith(expect.objectContaining({
        action: 'session.kill_all',
        detail: expect.objectContaining({ revokedCount: 3 })
      }))
    })

    it('returns { ok: true, revokedCount }', () => {
      const req = { orcaSession: adminSession, params: { userId: 'u1' }, ip: '127.0.0.1' } as unknown as Request
      const res = mockRes()
      handlers.killAllUserSessions(req, res)
      expect((res as any).json).toHaveBeenCalledWith({ ok: true, revokedCount: 3 })
    })
  })
})
