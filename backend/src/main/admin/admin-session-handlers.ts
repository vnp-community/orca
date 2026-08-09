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

  /** List all active (non-expired) sessions across all users. Supports ?limit=&offset= pagination. */
  listAllSessions = async (req: Request, res: Response): Promise<void> => {
    const q = req.query as Record<string, string>
    // FIX BUG-BE-HLD-006: pagination pattern copied from admin-audit-handlers.ts (cap limit at 1000)
    const limit  = q['limit']  ? Math.min(Number(q['limit']), 1000) : 100
    const offset = q['offset'] ? Number(q['offset']) : 0

    const { sessions, total } = await this.deps.sessionStore.listAllActiveSessions(limit, offset)
    res.json({ sessions, total })
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
