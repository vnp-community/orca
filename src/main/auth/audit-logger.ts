/**
 * AuditLogger — Immutable audit trail service (TASK-AUTH-002)
 *
 * Writes login, logout, and other security-relevant events to orca_audit_log.
 * Non-throwing: write failures are logged to console but NEVER crash the caller.
 *
 * Table: orca_audit_log (migration 0005_add_auth_schema)
 *
 * @module main/auth/audit-logger
 */

import type { IConnectionPool } from '../db/types'

// ── Types ─────────────────────────────────────────────────────────────────────

export interface AuditEntry {
  /** Dot-separated action code, e.g. 'auth.login.success', 'auth.login.failed' */
  action:     string
  /** User ID for successful auth; 'unknown' for failed/unauthenticated attempts */
  userId:     string
  /** Email address (raw input, may be invalid on failure) */
  userEmail:  string
  /** IP address of the request */
  ip:         string
  /** User-Agent header value */
  userAgent?: string
  /** Optional structured context for the audit entry */
  details?:   Record<string, unknown>
}

// ── AuditLogger ───────────────────────────────────────────────────────────────

export class AuditLogger {
  constructor(private readonly pool: IConnectionPool) {}

  /**
   * Write an audit entry to orca_audit_log.
   * Non-throwing — errors are logged to console, never rethrown.
   * Use fire-and-forget (void) for non-blocking callers.
   */
  async log(entry: AuditEntry): Promise<void> {
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_audit_log
           (action, user_id, user_email, ip, user_agent, details_json, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        [
          entry.action,
          entry.userId,
          entry.userEmail,
          entry.ip,
          entry.userAgent ?? '',
          JSON.stringify(entry.details ?? {}),
          now,
        ]
      )
    ).catch((err: unknown) => {
      // FIX TASK-AUTH-002: Audit log write failure MUST NOT crash the request.
      // This is intentional — audit is best-effort, not transactional.
      console.error('[AuditLogger] Write failed (non-fatal):', err)
    })
  }
}
