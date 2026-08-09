// src/main/audit/audit-log.ts
// NDJSON (newline-delimited JSON) audit log for fleet and agent operations.
// Respects ORCA_DATA_DIR env var for path override (container/headless support).
import * as fs from 'node:fs/promises'
import * as path from 'node:path'
import { app } from 'electron'

export type AuditEventType =
  | 'connect'
  | 'disconnect'
  | 'create-worktree'
  | 'delete-worktree'
  | 'access-denied'
  | 'fleet-import'
  | 'bootstrap-start'
  | 'bootstrap-complete'
  | 'token-issued'
  | 'token-revoked'

export type AuditEvent = {
  timestamp: number
  eventType: AuditEventType
  userId?: string
  userEmail?: string
  targetId?: string
  targetLabel?: string
  ip?: string
  success?: boolean
  details?: Record<string, unknown>
}

export class AuditLog {
  private logPath: string

  constructor(dataDir?: string) {
    // Prefer explicit dataDir, then ORCA_DATA_DIR env var, then app.getPath('userData')
    const dir = dataDir ?? process.env.ORCA_DATA_DIR ?? app.getPath('userData')
    this.logPath = path.join(dir, 'audit.log')
  }

  /** Append a single audit event as a JSON line. */
  async record(event: AuditEvent): Promise<void> {
    const line = JSON.stringify({ ...event, timestamp: event.timestamp ?? Date.now() })
    await fs.appendFile(this.logPath, `${line  }\n`, 'utf-8')
  }

  /**
   * Query recent audit events matching optional filter criteria.
   * Returns up to `limit` events in reverse chronological order (newest first).
   */
  async query(
    filter: Partial<Pick<AuditEvent, 'eventType' | 'userId' | 'targetId'>>,
    limit = 100
  ): Promise<AuditEvent[]> {
    let content: string
    try {
      content = await fs.readFile(this.logPath, 'utf-8')
    } catch {
      return []
    }

    const lines = content.trim().split('\n').filter(Boolean)
    const events: AuditEvent[] = []

    // Read from end to return most-recent first
    for (let i = lines.length - 1; i >= 0 && events.length < limit; i--) {
      try {
        const event: AuditEvent = JSON.parse(lines[i])
        if (filter.eventType && event.eventType !== filter.eventType) {continue}
        if (filter.userId && event.userId !== filter.userId) {continue}
        if (filter.targetId && event.targetId !== filter.targetId) {continue}
        events.push(event)
      } catch {
        // Skip malformed lines
      }
    }

    return events
  }

  /** Absolute path to the log file. */
  getLogPath(): string {
    return this.logPath
  }
}

// Process-lifetime singleton — respects ORCA_DATA_DIR at construction time
export const auditLog = new AuditLog()
