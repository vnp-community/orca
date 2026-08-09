/**
 * Auth Router — Express routes for login/logout/me
 *
 * Mounts at /auth. Requires createAuthMiddleware() applied at app level.
 *
 * Routes:
 *   POST /auth/local  — local email+password login
 *   POST /auth/logout — revoke session, clear cookie
 *   GET  /auth/me     — return current session user
 *
 * @module main/auth/auth-router
 */

import { Router } from 'express'
import type { Request, Response } from 'express'
import type { AuthManager } from './auth-manager'
import { SESSION_COOKIE_NAME, extractSessionCookie } from './auth-manager'
import { requireAuth, extractSessionToken } from './auth-middleware'

/** Cookie options for HttpOnly session cookie */
const COOKIE_OPTIONS = {
  httpOnly: true,
  sameSite: 'strict' as const,   // FIX TASK-AUTH-001: Strict prevents CSRF via cross-site requests
  secure: process.env['NODE_ENV'] === 'production',
  path: '/',
  maxAge: 8 * 60 * 60 * 1000  // 8 hours in ms (matches SESSION_TTL_MS)
}

export function createAuthRouter(authManager: AuthManager): Router {
  const router = Router()

  // ── POST /auth/local ─────────────────────────────────────────────────────
  // Local email + password login. Returns 200 with session cookie on success.
  router.post('/local', async (req: Request, res: Response): Promise<void> => {
    const { email, password } = (req.body ?? {}) as Record<string, unknown>

    const result = await authManager.login(
      { email: String(email ?? ''), password: String(password ?? '') },
      req.ip ?? req.socket?.remoteAddress ?? '0.0.0.0',
      req.headers['user-agent'] ?? 'unknown'
    )

    if (!result.success) {
      // Return 401 for invalid_credentials (not 403 — no active session exists yet)
      const status = result.error === 'validation_error' ? 400 : 401
      res.status(status).json({ error: result.error, detail: result.detail })
      return
    }

    // Set HttpOnly session cookie
    res.cookie(SESSION_COOKIE_NAME, result.sessionId, COOKIE_OPTIONS)
    res.status(200).json({
      ok:   true,
      user: result.user
    })
  })

  // ── POST /auth/logout ───────────────────────────────────────────────────
  // Revoke current session and clear cookie. Idempotent — no-op if no session.
  router.post('/logout', async (req: Request, res: Response): Promise<void> => {
    const sessionId = extractSessionToken(req)
    if (sessionId) {
      await authManager.logout(sessionId)
    }
    res.clearCookie(SESSION_COOKIE_NAME)
    res.json({ ok: true })
  })

  // ── GET /auth/me ────────────────────────────────────────────────────────
  // Return current session user. 401 if not authenticated.
  router.get('/me', requireAuth, (req: Request, res: Response): void => {
    const session = req.orcaSession!
    res.json({
      id:        session.userId,
      email:     session.userEmail,
      role:      session.role,
      sessionId: session.sessionId,
      expiresAt: session.expiresAt
    })
  })

  // ── GET /auth/sso/:provider ─────────────────────────────────────────────
  // SSO login stub — returns 501 Not Implemented (SSO is a future feature).
  // Does NOT crash; gives a clear actionable response.
  router.get('/sso/:provider', (_req: Request, res: Response): void => {
    res.status(501).json({
      error:   'not_implemented',
      message: 'SSO login is not yet implemented. Use local email/password login.',
    })
  })

  // ── GET /auth/config ────────────────────────────────────────────────────
  // Returns auth configuration (enabled providers, local login state).
  router.get('/config', (_req: Request, res: Response): void => {
    res.json({
      providers: [],
      localEnabled: true
    })
  })

  return router
}
