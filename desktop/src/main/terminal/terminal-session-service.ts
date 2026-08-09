/**
 * Terminal Session Service
 *
 * Manages CRUD + snapshot operations for `orca_terminal_sessions`.
 * Called by the renderer via IPC to persist and restore terminal scrollback
 * snapshots across disconnects (TDD-FE §TM-003).
 *
 * @module main/terminal/terminal-session-service
 */

import { randomUUID } from 'node:crypto'
import type { IDatabase } from '../db/types'

// ─── Types ────────────────────────────────────────────────────────────────────

export type TerminalSessionKey = {
  worktreeId:     string
  tabId:          string
  leafId?:        string
  runtimeEnvId?:  string
}

export type TerminalSessionSnapshot = {
  id:            string
  worktreeId:    string
  tabId:         string
  leafId:        string
  runtimeEnvId:  string
  snapshotData:  string | null  // base64-encoded xterm SerializeAddon output
  snapshotCols:  number
  snapshotRows:  number
  remoteHandle:  string | null  // for re-attach on same runtime
  status:        'active' | 'archived'
  lastActiveAt:  number
  createdAt:     number
  updatedAt:     number
}

export type UpsertSnapshotInput = TerminalSessionKey & {
  snapshotData: string
  snapshotCols: number
  snapshotRows: number
  remoteHandle?: string
}

// ─── Service ──────────────────────────────────────────────────────────────────

export class TerminalSessionService {
  constructor(private readonly db: IDatabase) {}

  // ── Save / upsert ────────────────────────────────────────────────────────────

  /**
   * Upsert a terminal snapshot. Inserts if new, updates snapshot_data,
   * cols, rows, remote_handle, and last_active_at if already exists.
   */
  async saveSnapshot(input: UpsertSnapshotInput): Promise<TerminalSessionSnapshot> {
    const {
      worktreeId,
      tabId,
      leafId       = '',
      runtimeEnvId = '',
      snapshotData,
      snapshotCols,
      snapshotRows,
      remoteHandle = null,
    } = input

    const now = Date.now()

    // Try update first (most common path for existing sessions)
    const upd = await this.db.prepare(`
      UPDATE orca_terminal_sessions
         SET snapshot_data  = ?,
             snapshot_cols  = ?,
             snapshot_rows  = ?,
             remote_handle  = COALESCE(?, remote_handle),
             last_active_at = ?,
             updated_at     = ?
       WHERE worktree_id = ? AND tab_id = ? AND leaf_id = ? AND runtime_env_id = ?
    `)
    const result = await upd.run(
      snapshotData, snapshotCols, snapshotRows, remoteHandle,
      now, now,
      worktreeId, tabId, leafId, runtimeEnvId
    )

    if ((result.changes as number) > 0) {
      return this.getSnapshot({ worktreeId, tabId, leafId, runtimeEnvId }) as Promise<TerminalSessionSnapshot>
    }

    // Insert new record
    const id = randomUUID()
    const ins = await this.db.prepare(`
      INSERT INTO orca_terminal_sessions
        (id, worktree_id, tab_id, leaf_id, runtime_env_id,
         snapshot_data, snapshot_cols, snapshot_rows, remote_handle,
         status, last_active_at, created_at, updated_at)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?)
    `)
    await ins.run(
      id, worktreeId, tabId, leafId, runtimeEnvId,
      snapshotData, snapshotCols, snapshotRows, remoteHandle,
      now, now, now
    )

    return {
      id, worktreeId, tabId, leafId, runtimeEnvId,
      snapshotData, snapshotCols, snapshotRows,
      remoteHandle: remoteHandle ?? null,
      status: 'active',
      lastActiveAt: now, createdAt: now, updatedAt: now,
    }
  }

  // ── Retrieve ─────────────────────────────────────────────────────────────────

  /** Get snapshot for a specific session key. Returns null if not found. */
  async getSnapshot(key: TerminalSessionKey): Promise<TerminalSessionSnapshot | null> {
    const stmt = await this.db.prepare(`
      SELECT * FROM orca_terminal_sessions
       WHERE worktree_id = ? AND tab_id = ? AND leaf_id = ? AND runtime_env_id = ?
         AND status = 'active'
       LIMIT 1
    `)
    const row = await stmt.get(
      key.worktreeId,
      key.tabId,
      key.leafId     ?? '',
      key.runtimeEnvId ?? ''
    )
    return row ? this.rowToSnapshot(row) : null
  }

  /** List all active snapshots for a worktree, ordered by most recently active. */
  async listByWorktree(worktreeId: string): Promise<TerminalSessionSnapshot[]> {
    const stmt = await this.db.prepare(`
      SELECT * FROM orca_terminal_sessions
       WHERE worktree_id = ? AND status = 'active'
       ORDER BY last_active_at DESC
    `)
    const rows = await stmt.all(worktreeId) as Record<string, unknown>[]
    return rows.map(r => this.rowToSnapshot(r))
  }

  // ── Archive / Cleanup ─────────────────────────────────────────────────────────

  /** Soft-delete a session by marking it archived. */
  async archiveSnapshot(key: TerminalSessionKey): Promise<void> {
    const stmt = await this.db.prepare(`
      UPDATE orca_terminal_sessions
         SET status = 'archived', updated_at = ?
       WHERE worktree_id = ? AND tab_id = ? AND leaf_id = ? AND runtime_env_id = ?
    `)
    await stmt.run(
      Date.now(),
      key.worktreeId, key.tabId, key.leafId ?? '', key.runtimeEnvId ?? ''
    )
  }

  /**
   * Purge archived snapshots older than `maxAgeMs` (default 7 days).
   * Called periodically by the runtime cleanup job.
   */
  async purgeOldArchived(maxAgeMs = 7 * 24 * 60 * 60_000): Promise<number> {
    const cutoff = Date.now() - maxAgeMs
    const stmt   = await this.db.prepare(`
      DELETE FROM orca_terminal_sessions
       WHERE status = 'archived' AND updated_at < ?
    `)
    const result = await stmt.run(cutoff)
    return result.changes as number
  }

  // ── Row mapper ────────────────────────────────────────────────────────────────

  private rowToSnapshot(row: Record<string, unknown>): TerminalSessionSnapshot {
    return {
      id:            row['id']              as string,
      worktreeId:    row['worktree_id']     as string,
      tabId:         row['tab_id']          as string,
      leafId:        row['leaf_id']         as string,
      runtimeEnvId:  row['runtime_env_id']  as string,
      snapshotData:  row['snapshot_data']   as string | null,
      snapshotCols:  row['snapshot_cols']   as number,
      snapshotRows:  row['snapshot_rows']   as number,
      remoteHandle:  row['remote_handle']   as string | null,
      status:        row['status']          as 'active' | 'archived',
      lastActiveAt:  row['last_active_at']  as number,
      createdAt:     row['created_at']      as number,
      updatedAt:     row['updated_at']      as number,
    }
  }
}
