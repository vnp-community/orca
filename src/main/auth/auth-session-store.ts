/**
 * Auth Session Store
 *
 * Manages CRUD operations for HTTP sessions stored in `orca_sessions` table.
 * Sessions are identified by 64-hex tokens stored as HttpOnly cookies.
 *
 * @module main/auth/auth-session-store
 */

import { randomBytes } from 'node:crypto'
import type { IDatabase } from '../db/types'
import type { OrcaSession, CreateSessionInput } from './auth-types'
import { SESSION_TTL_MS } from './auth-types'

export class AuthSessionStore {
  constructor(private readonly db: IDatabase) {}

  /** Create a new session and persist to DB. Returns the full OrcaSession. */
  async createSession(input: CreateSessionInput): Promise<OrcaSession> {
    const sessionId = randomBytes(32).toString('hex')
    const now       = Date.now()
    const expiresAt = now + SESSION_TTL_MS

    const stmt = await this.db.prepare(`
      INSERT INTO orca_sessions
        (session_id, user_id, created_at, expires_at, last_seen_at, ip_address, user_agent)
      VALUES (?, ?, ?, ?, NULL, ?, ?)
    `)
    await stmt.run(sessionId, input.userId, now, expiresAt, input.ipAddress ?? null, input.userAgent ?? null)

    return {
      sessionId,
      userId:      input.userId,
      userEmail:   input.userEmail,
      role:        input.role,
      createdAt:   now,
      expiresAt,
      lastSeenAt:  null,
      ipAddress:   input.ipAddress ?? null,
      userAgent:   input.userAgent ?? null
    }
  }

  /** Retrieve session by ID (joins orca_users for email + role). No expiry check. */
  async getSession(sessionId: string): Promise<OrcaSession | null> {
    const stmt = await this.db.prepare(`
      SELECT
        s.session_id, s.user_id, s.created_at, s.expires_at, s.last_seen_at,
        s.ip_address, s.user_agent, u.email AS user_email, u.role
      FROM orca_sessions s
      JOIN orca_users u ON u.id = s.user_id
      WHERE s.session_id = ?
    `)
    const row = await stmt.get(sessionId)
    return row ? this.rowToSession(row) : null
  }

  /**
   * Validate session: check exists + not expired, update last_seen_at.
   * Returns null for missing or expired sessions (also deletes expired ones).
   */
  async validateSession(sessionId: string): Promise<OrcaSession | null> {
    const session = await this.getSession(sessionId)
    if (!session) return null

    if (session.expiresAt < Date.now()) {
      await this.revokeSession(sessionId)
      return null
    }

    // Touch last_seen_at
    const updateStmt = await this.db.prepare(
      `UPDATE orca_sessions SET last_seen_at = ? WHERE session_id = ?`
    )
    await updateStmt.run(Date.now(), sessionId)

    return session
  }

  /** Delete a single session by ID. Idempotent (no-op if not found). */
  async revokeSession(sessionId: string): Promise<void> {
    const stmt = await this.db.prepare(`DELETE FROM orca_sessions WHERE session_id = ?`)
    await stmt.run(sessionId)
  }

  /** Delete all sessions for a user. Returns count of deleted sessions. */
  async revokeAllUserSessions(userId: string): Promise<number> {
    const stmt = await this.db.prepare(`DELETE FROM orca_sessions WHERE user_id = ?`)
    const result = await stmt.run(userId)
    return (result as { changes: number }).changes ?? 0
  }

  /** List active (non-expired) sessions for a user. */
  async listUserSessions(userId: string): Promise<OrcaSession[]> {
    const stmt = await this.db.prepare(`
      SELECT
        s.session_id, s.user_id, s.created_at, s.expires_at, s.last_seen_at,
        s.ip_address, s.user_agent, u.email AS user_email, u.role
      FROM orca_sessions s
      JOIN orca_users u ON u.id = s.user_id
      WHERE s.user_id = ? AND s.expires_at > ?
      ORDER BY s.created_at DESC
    `)
    const rows = await stmt.all(userId, Date.now())
    return rows.map((r) => this.rowToSession(r))
  }

  /** Delete all sessions with expires_at < now(). Returns count of deleted rows. */
  async cleanupExpired(): Promise<number> {
    const stmt = await this.db.prepare(`DELETE FROM orca_sessions WHERE expires_at < ?`)
    const result = await stmt.run(Date.now())
    return (result as { changes: number }).changes ?? 0
  }

  private rowToSession(row: Record<string, unknown>): OrcaSession {
    return {
      sessionId:  row['session_id']   as string,
      userId:     row['user_id']      as string,
      userEmail:  (row['user_email']  ?? row['email']) as string,
      role:       row['role']         as OrcaSession['role'],
      createdAt:  row['created_at']   as number,
      expiresAt:  row['expires_at']   as number,
      lastSeenAt: (row['last_seen_at'] as number | null) ?? null,
      ipAddress:  (row['ip_address']   as string | null) ?? null,
      userAgent:  (row['user_agent']   as string | null) ?? null
    }
  }
}
