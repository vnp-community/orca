/**
 * Auth Middleware — Express middleware for authenticating requests
 *
 * Injects `req.orcaSession` for all routes that need authentication.
 * Supports skip patterns for public routes (login, health, etc.)
 *
 * @module main/auth/auth-middleware
 */

import type { Request, Response, NextFunction } from 'express'
import type { AuthManager } from './auth-manager'
import type { OrcaSession } from './auth-types'
import { SESSION_COOKIE_NAME } from './auth-manager'

// Augment Express Request with session info
declare global {
  namespace Express {
    interface Request {
      orcaSession?: OrcaSession
    }
  }
}

/** Route prefixes/patterns that bypass auth validation. */
const PUBLIC_PATHS = [
  '/auth/local',   // login endpoint — must be public
  '/auth/logout',  // logout — no session required (revoke by cookie even if expired)
  '/health',       // health check
  '/push',         // web push subscription (public)
]

/**
 * Create Express auth middleware.
 * Attaches `req.orcaSession` if a valid session cookie is present.
 * Does NOT reject requests without a session (use `requireAuth` for that).
 */
export function createAuthMiddleware(authManager: AuthManager) {
  return async function authMiddleware(req: Request, _res: Response, next: NextFunction): Promise<void> {
    // Always run validation — even for public paths — so downstream handlers can see req.orcaSession
    const cookieHeader = req.headers.cookie
    const session = await authManager.validateRequest(cookieHeader)
    if (session) {
      req.orcaSession = session
    }
    next()
  }
}

/**
 * Guard middleware: Require a valid session cookie.
 * Return 401 if not authenticated. Place AFTER createAuthMiddleware().
 * Used for routes that require authentication.
 */
export function requireAuth(req: Request, res: Response, next: NextFunction): void {
  if (!req.orcaSession) {
    res.status(401).json({
      error:   'unauthenticated',
      message: 'Session required. Please log in first.'
    })
    return
  }
  next()
}

/**
 * Utility: extract raw session token from HttpOnly cookie.
 * Used by logout handler to revoke by session ID.
 */
export function extractSessionToken(req: Request): string | null {
  const cookie = req.headers.cookie ?? ''
  const match  = /orca_session=([a-f0-9]{64})/.exec(cookie)
  return match?.[1] ?? null
}

export type { OrcaSession }
export { SESSION_COOKIE_NAME }
