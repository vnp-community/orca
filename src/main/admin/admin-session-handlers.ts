/**
 * Admin Session Handlers — Kill individual or all user sessions
 *
 * @module main/admin/admin-session-handlers
 */

import type { Request, Response } from 'express'
import type { AuthSessionStore } from '../auth/auth-session-store'
import type { AuditLogger } from './audit-logger'
import { AUDIT_ACTIONS } from './admin-types'

export class AdminSessionHandlers {
  constructor(private readonly deps: {
    sessionStore: AuthSessionStore
    auditLogger:  AuditLogger
  }) {}

  /** List all active sessions — stub (full listing requires additional store method) */
  listAllSessions = (_req: Request, res: Response): void => {
    // TODO: implement AuthSessionStore.listAllActiveSessions() in a future iteration
    res.json({ sessions: [], total: 0, note: 'Full listing not yet implemented' })
  }

  /** Revoke a specific session by sessionId */
  killSession = (req: Request, res: Response): void => {
    const { sessionId } = req.params

    this.deps.sessionStore.revokeSession(sessionId!)
    this.deps.auditLogger.log({
      userId:    req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action:    AUDIT_ACTIONS.SESSION_KILL,
      ipAddress: req.ip,
      detail:    { targetSessionId: sessionId }
    })

    res.json({ ok: true })
  }

  /** Revoke all sessions for a specific userId */
  killAllUserSessions = (req: Request, res: Response): void => {
    const { userId } = req.params

    const revokedCount = this.deps.sessionStore.revokeAllUserSessions(userId!)
    this.deps.auditLogger.log({
      userId:    req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action:    AUDIT_ACTIONS.SESSION_KILL_ALL,
      ipAddress: req.ip,
      detail:    { targetUserId: userId, revokedCount }
    })

    res.json({ ok: true, revokedCount })
  }
}
