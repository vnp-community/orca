/**
 * Admin Middleware — requireAdmin Express guard
 *
 * Protects /admin/api/* routes — only allows users with role = 'admin'.
 * Must be placed AFTER createAuthMiddleware() (requires req.orcaSession).
 *
 * @module main/admin/admin-middleware
 */

import type { Request, Response, NextFunction } from 'express'

/**
 * Guard: allow requests only from users with role = 'admin'.
 * Place AFTER createAuthMiddleware() (which populates req.orcaSession).
 *
 * Returns:
 * - 401 if no session (not logged in)
 * - 403 if session exists but role !== 'admin'
 * - calls next() if role === 'admin'
 */
export function requireAdmin(req: Request, res: Response, next: NextFunction): void {
  const session = req.orcaSession  // populated by createAuthMiddleware()

  if (!session) {
    res.status(401).json({
      error:   'unauthenticated',
      message: 'Login required'
    })
    return
  }

  if (session.role !== 'admin') {
    res.status(403).json({
      error:         'forbidden',
      message:       'Admin role required',
      required_role: 'admin',
      your_role:     session.role
    })
    return
  }

  next()
}
