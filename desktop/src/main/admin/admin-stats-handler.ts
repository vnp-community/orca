/**
 * Admin Stats Handler — Dashboard statistics from DB
 *
 * Queries orca_users and orca_sessions tables directly for counts.
 * Requires ISyncDatabase (SQLite).
 *
 * @module main/admin/admin-stats-handler
 */

import type { Request, Response } from 'express'
import type { ISyncDatabase } from '../db/types'
import type { AdminStats } from './admin-types'

export class AdminStatsHandler {
  constructor(private readonly db: ISyncDatabase) {}

  getStats = (_req: Request, res: Response): void => {
    const now = Date.now()

    const stats: AdminStats = {
      totalUsers:     this.countQuery('SELECT COUNT(*) FROM orca_users'),
      activeUsers:    this.countQuery('SELECT COUNT(*) FROM orca_users WHERE is_active = 1'),
      activeSessions: this.countQuery(`SELECT COUNT(*) FROM orca_sessions WHERE expires_at > ${now}`),
      pairedDevices:  0   // Stub — DeviceRegistry not available here
    }

    res.json(stats)
  }

  private countQuery(sql: string): number {
    const row = this.db.prepare(sql).get() as Record<string, number> | undefined
    if (!row) return 0
    return Object.values(row)[0] ?? 0
  }
}
