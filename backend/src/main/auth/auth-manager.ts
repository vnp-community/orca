/**
 * AuthManager — Facade for all auth operations in server mode
 *
 * Owns SessionStore + UserStore + LocalHandler.
 * Provides cookie parsing and a periodic expired-session cleanup timer.
 * Injected into HTTP server, auth router, and admin router.
 *
 * @module main/auth/auth-manager
 */

import type { IDatabase } from '../db/types'
import { AuthSessionStore } from './auth-session-store'
import { AuthUserStore }    from './auth-user-store'
import { AuthLocalHandler } from './auth-local-handler'
import type { OrcaSession, LocalLoginInput, LocalLoginResult } from './auth-types'
// FIX TASK-AUTH-002: Import AuditLogger for login/logout audit trail
import type { AuditLogger } from './audit-logger'

/** How often to sweep expired sessions. 30 minutes. */
const CLEANUP_INTERVAL_MS = 30 * 60 * 1000

/** Cookie name used for all session tokens. */
export const SESSION_COOKIE_NAME = 'orca_session'

/** Regex that matches a valid 64-hex session token from a Cookie header. */
const SESSION_COOKIE_REGEX = /orca_session=([a-f0-9]{64})/

export class AuthManager {
  readonly sessionStore: AuthSessionStore
  readonly userStore:    AuthUserStore
  readonly localHandler: AuthLocalHandler

  private cleanupTimer: ReturnType<typeof setInterval> | null = null

  constructor(
    private readonly db: IDatabase,
    // FIX TASK-AUTH-002: Optional AuditLogger — undefined in tests/CLI mode
    private readonly auditLogger?: AuditLogger
  ) {
    this.sessionStore = new AuthSessionStore(db)
    this.userStore    = new AuthUserStore(db)
    this.localHandler = new AuthLocalHandler(this.userStore, this.sessionStore)

    // Periodic cleanup of expired sessions — don't block process exit
    this.cleanupTimer = setInterval(() => {
      void this.sessionStore.cleanupExpired().then((n) => {
        if (n > 0) {console.log(`[AuthManager] Cleaned ${n} expired session(s)`)}
      }).catch((err: unknown) => {
        console.warn('[AuthManager] Session cleanup error:', err)
      })
    }, CLEANUP_INTERVAL_MS)

    if (this.cleanupTimer.unref) {this.cleanupTimer.unref()}
  }

  /**
   * Parse `orca_session` cookie from Cookie header string, then validate.
   * Returns null for missing cookie, invalid format, or expired session.
   * Called by auth middleware on every request.
   */
  async validateRequest(cookieHeader: string | undefined): Promise<OrcaSession | null> {
    const sessionId = extractSessionCookie(cookieHeader)
    if (!sessionId) {return null}
    return this.sessionStore.validateSession(sessionId)
  }

  /** Attempt local email+password login. Returns sessionId on success. */
  async login(input: LocalLoginInput, ip: string, ua: string): Promise<LocalLoginResult> {
    const result = await this.localHandler.login(input, ip, ua)

    // FIX TASK-AUTH-002: Write audit log entry (fire-and-forget — never blocks login response).
    void this.auditLogger?.log({
      action:    result.success ? 'auth.login.success' : 'auth.login.failed',
      userId:    result.success ? result.user.id : 'unknown',
      userEmail: input.email,
      ip,
      userAgent: ua,
      details:   result.success
        ? { sessionId: result.sessionId }
        : { reason: 'invalid_credentials' },
    })

    return result
  }

  /** Revoke a session by ID (logout). */
  async logout(sessionId: string): Promise<void> {
    await this.sessionStore.revokeSession(sessionId)
  }

  /** Stop cleanup timer — call during server shutdown to avoid resource leaks. */
  destroy(): void {
    if (this.cleanupTimer) {
      clearInterval(this.cleanupTimer)
      this.cleanupTimer = null
    }
  }
}

export function extractSessionCookie(cookieHeader: string | undefined): string | null {
  if (!cookieHeader) {return null}
  const match = SESSION_COOKIE_REGEX.exec(cookieHeader)
  return match ? match[1]! : null
}
