/**
 * Migration 0011 — Terminal Session Persistence
 *
 * Adds terminal session snapshot storage for TDD-FE §TM-003:
 * - orca_terminal_sessions: Persist terminal scrollback + metadata
 *   across disconnects so sessions can be restored on reconnect.
 *
 * @module db/migrations/0011_terminal_sessions
 */

import type { Migration } from './types'

export const migration0011TerminalSessions: Migration = {
  version: 11,
  name: 'terminal_sessions',

  async up(db) {
    // ── orca_terminal_sessions ─────────────────────────────────────────────────
    // Stores snapshots of active/recent terminal sessions.
    // snapshot_data: base64-encoded xterm SerializeAddon output (scrollback).
    // snapshot_cols / snapshot_rows: viewport at snapshot time.
    // session_key: unique identifier — worktreeId + tabId + leafId composite.
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_terminal_sessions (
        id               TEXT    PRIMARY KEY,
        -- Composite natural key for lookup
        worktree_id      TEXT    NOT NULL,
        tab_id           TEXT    NOT NULL,
        leaf_id          TEXT    NOT NULL DEFAULT '',
        runtime_env_id   TEXT    NOT NULL DEFAULT '',
        -- Serialized scrollback buffer from xterm SerializeAddon
        snapshot_data    TEXT,
        snapshot_cols    INTEGER NOT NULL DEFAULT 80,
        snapshot_rows    INTEGER NOT NULL DEFAULT 24,
        -- Remote terminal handle (for re-attach)
        remote_handle    TEXT,
        -- State
        status           TEXT    NOT NULL DEFAULT 'active',
        last_active_at   INTEGER NOT NULL,
        created_at       INTEGER NOT NULL,
        updated_at       INTEGER NOT NULL
      )
    `)

    await db.exec(`
      CREATE UNIQUE INDEX IF NOT EXISTS idx_orca_terminal_sessions_key
        ON orca_terminal_sessions(worktree_id, tab_id, leaf_id, runtime_env_id)
    `)

    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_terminal_sessions_active
        ON orca_terminal_sessions(status, last_active_at DESC)
    `)

    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_terminal_sessions_worktree
        ON orca_terminal_sessions(worktree_id, status)
    `)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_orca_terminal_sessions_worktree')
    await db.exec('DROP INDEX IF EXISTS idx_orca_terminal_sessions_active')
    await db.exec('DROP INDEX IF EXISTS idx_orca_terminal_sessions_key')
    await db.exec('DROP TABLE IF EXISTS orca_terminal_sessions')
  }
}
