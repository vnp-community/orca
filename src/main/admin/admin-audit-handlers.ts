/**
 * Admin Audit Handlers — Query audit log via REST
 *
 * @module main/admin/admin-audit-handlers
 */

import type { Request, Response } from 'express'
import type { AuditLogger } from './audit-logger'

export class AdminAuditHandlers {
  constructor(private readonly auditLogger: AuditLogger) {}

  /**
   * Query audit events with optional filters via query string.
   * Supported params: userId, action, from (ms), to (ms), limit (max 1000), offset
   */
  queryAuditLog = (req: Request, res: Response): void => {
    const q = req.query as Record<string, string>

    const events = this.auditLogger.query({
      userId:  q['userId']  || undefined,
      action:  q['action']  || undefined,
      from:    q['from']    ? Number(q['from'])   : undefined,
      to:      q['to']      ? Number(q['to'])     : undefined,
      limit:   q['limit']   ? Math.min(Number(q['limit']),  1000) : 100,
      offset:  q['offset']  ? Number(q['offset']) : 0
    })

    res.json({ events, total: events.length })
  }
}
