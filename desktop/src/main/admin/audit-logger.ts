/**
 * AuditLogger — Write and query audit events in orca_audit_log
 *
 * Provides fire-and-forget `log()` (async but callers don't await)
 * and `query()` / `count()` for admin panel queries.
 *
 * Uses ISyncDatabase pattern (SQLite) — prepare() is synchronous.
 * For async adapters, use await log() at call sites.
 *
 * @module main/admin/audit-logger
 */

import type { ISyncDatabase } from '../db/types'
import type { AuditEvent, AuditLogInput, AuditQueryFilter } from './admin-types'

export class AuditLogger {
  constructor(private readonly db: ISyncDatabase) {}

  /**
   * Write an audit event.
   * Synchronous — does NOT require await at most call sites.
   * Errors are swallowed (audit should never crash the main request).
   */
  log(input: AuditLogInput): void {
    try {
      this.db.prepare(`
        INSERT INTO orca_audit_log
          (created_at, user_id, user_email, action, detail, ip_address)
        VALUES (?, ?, ?, ?, ?, ?)
      `).run(
        Date.now(),
        input.userId    ?? null,
        input.userEmail ?? null,
        input.action,
        input.detail ? JSON.stringify(input.detail) : null,
        input.ipAddress ?? null
      )
    } catch (err) {
      // Audit failures must never crash the calling request
      console.warn('[AuditLogger] Failed to write audit event:', err)
    }
  }

  /**
   * Query audit events with optional filters.
   * Returns at most min(limit, 1000) events, newest first.
   */
  query(filter: AuditQueryFilter): AuditEvent[] {
    const conditions: string[] = []
    const params: (string | number)[] = []

    if (filter.userId) { conditions.push('user_id = ?');    params.push(filter.userId) }
    if (filter.action) { conditions.push('action = ?');     params.push(filter.action) }
    if (filter.from)   { conditions.push('created_at >= ?'); params.push(filter.from) }
    if (filter.to)     { conditions.push('created_at <= ?'); params.push(filter.to) }

    const where  = conditions.length ? `WHERE ${conditions.join(' AND ')}` : ''
    const limit  = Math.min(filter.limit ?? 100, 1000)  // cap at 1000
    const offset = filter.offset ?? 0

    const rows = this.db.prepare(`
      SELECT id, created_at, user_id, user_email, action, detail, ip_address
      FROM orca_audit_log
      ${where}
      ORDER BY created_at DESC
      LIMIT ? OFFSET ?
    `).all(...params, limit, offset) as Record<string, unknown>[]

    return rows.map(row => ({
      id:        row['id']         as number,
      createdAt: row['created_at'] as number,
      userId:    (row['user_id']    ?? null) as string | null,
      userEmail: (row['user_email'] ?? null) as string | null,
      action:    row['action']     as string,
      detail:    row['detail']     ? JSON.parse(row['detail'] as string) as Record<string, unknown> : null,
      ipAddress: (row['ip_address'] ?? null) as string | null
    }))
  }

  /** Count audit events matching filter (no LIMIT) */
  count(filter: Pick<AuditQueryFilter, 'userId' | 'action'>): number {
    const conditions: string[] = []
    const params: (string | number)[] = []
    if (filter.userId) { conditions.push('user_id = ?'); params.push(filter.userId) }
    if (filter.action) { conditions.push('action = ?');  params.push(filter.action) }
    const where = conditions.length ? `WHERE ${conditions.join(' AND ')}` : ''
    const row = this.db.prepare(
      `SELECT COUNT(*) as n FROM orca_audit_log ${where}`
    ).get(...params) as Record<string, unknown> | undefined
    return (row?.['n'] as number) ?? 0
  }
}
