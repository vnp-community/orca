/**
 * Migration 0003 — Add Workspace Sessions Table
 *
 * @module db/migrations/0003_add_workspace_sessions
 */

import type { Migration } from './types'
import { nowTextDefaultSql } from './sql-dialect'

export const migration0003AddWorkspaceSessions: Migration = {
  version: 3,
  name: 'add_workspace_sessions',

  async up(db) {
    // BUG-BE-RPC-003: datetime('now') is SQLite-only — see sql-dialect.ts.
    const now = nowTextDefaultSql(db.capabilities.dialect)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS workspace_sessions (
        id           TEXT PRIMARY KEY,
        project_id   TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
        repo_id      TEXT REFERENCES repos(id) ON DELETE SET NULL,
        agent        TEXT NOT NULL,
        status       TEXT NOT NULL DEFAULT 'active',
        started_at   TEXT NOT NULL DEFAULT ${now},
        ended_at     TEXT,
        metadata     TEXT NOT NULL DEFAULT '{}'
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_ws_sessions_project_id ON workspace_sessions(project_id)
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_ws_sessions_status ON workspace_sessions(status)
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_ws_sessions_agent ON workspace_sessions(agent)
    `)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_ws_sessions_agent')
    await db.exec('DROP INDEX IF EXISTS idx_ws_sessions_status')
    await db.exec('DROP INDEX IF EXISTS idx_ws_sessions_project_id')
    await db.exec('DROP TABLE IF EXISTS workspace_sessions')
  }
}
