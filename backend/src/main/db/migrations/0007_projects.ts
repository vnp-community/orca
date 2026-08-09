/**
 * Migration 0007 — v5 Project Management Tables
 *
 * Adds project management tables for TDD-15 (Project-Dev Server Binding).
 * Note: uses orca_v5_* prefix to avoid collision with legacy orca_projects
 * table created in migration 0004 (which stores tab/state data).
 *
 * - orca_v5_projects: Full project entities linked to a dev server
 * - orca_v5_project_members: Project membership + role table
 *
 * @module db/migrations/0007_projects
 */

import type { Migration } from './types'

export const migration0007Projects: Migration = {
  version: 7,
  name: 'projects',

  async up(db) {
    // ── orca_v5_projects ──────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_v5_projects (
        id             TEXT    PRIMARY KEY,
        name           TEXT    NOT NULL,
        description    TEXT,
        dev_server_id  TEXT    NOT NULL,
        repo_path      TEXT    NOT NULL,
        default_branch TEXT    NOT NULL DEFAULT 'main',
        visibility     TEXT    NOT NULL DEFAULT 'team',
        created_by     TEXT    NOT NULL,
        created_at     INTEGER NOT NULL,
        updated_at     INTEGER NOT NULL
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_v5_projects_server
        ON orca_v5_projects(dev_server_id)
    `)

    // ── orca_v5_project_members ───────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_v5_project_members (
        project_id TEXT    NOT NULL REFERENCES orca_v5_projects(id) ON DELETE CASCADE,
        user_id    TEXT    NOT NULL REFERENCES orca_users(id) ON DELETE CASCADE,
        role       TEXT    NOT NULL DEFAULT 'member',
        added_at   INTEGER NOT NULL,
        PRIMARY KEY (project_id, user_id)
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_v5_project_members_user
        ON orca_v5_project_members(user_id)
    `)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_orca_v5_project_members_user')
    await db.exec('DROP TABLE IF EXISTS orca_v5_project_members')
    await db.exec('DROP INDEX IF EXISTS idx_orca_v5_projects_server')
    await db.exec('DROP TABLE IF EXISTS orca_v5_projects')
  }
}
