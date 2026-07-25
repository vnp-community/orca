/**
 * Admin Router — Mount all /admin/api/* routes
 *
 * All routes are guarded by requireAdmin middleware.
 * Mount this router at '/admin/api' in the Express app.
 *
 * @module main/admin/admin-router
 */

import { Router } from 'express'
import { requireAdmin } from './admin-middleware'
import type { AdminUserHandlers }    from './admin-user-handlers'
import type { AdminSessionHandlers } from './admin-session-handlers'
import type { AdminStatsHandler }    from './admin-stats-handler'
import type { AdminAuditHandlers }   from './admin-audit-handlers'

export function createAdminRouter(deps: {
  userHandlers:    AdminUserHandlers
  sessionHandlers: AdminSessionHandlers
  statsHandler:    AdminStatsHandler
  auditHandlers:   AdminAuditHandlers
}): Router {
  const router = Router()

  // Guard ALL admin routes — must come before any route handlers
  router.use(requireAdmin)

  // ── Stats Dashboard ──────────────────────────────────────────────
  router.get('/stats', deps.statsHandler.getStats)

  // ── Users ────────────────────────────────────────────────────────
  router.get('/users',        deps.userHandlers.listUsers)
  router.post('/users',       deps.userHandlers.createUser)
  router.delete('/users/:id', deps.userHandlers.deactivateUser)

  // ── Sessions ─────────────────────────────────────────────────────
  router.get('/sessions',                       deps.sessionHandlers.listAllSessions)
  router.delete('/sessions/:sessionId',         deps.sessionHandlers.killSession)
  router.delete('/users/:userId/sessions',      deps.sessionHandlers.killAllUserSessions)

  // ── Audit Log ────────────────────────────────────────────────────
  router.get('/audit', deps.auditHandlers.queryAuditLog)

  return router
}
