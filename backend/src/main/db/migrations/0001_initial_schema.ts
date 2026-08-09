/**
 * Migration 0001 — Initial Schema
 *
 * Creates the core Orca state tables:
 *   - settings (global key-value store)
 *   - projects
 *   - repos
 *   - ssh_targets
 *
 * @module db/migrations/0001_initial_schema
 */

import type { Migration } from './types'

export const migration0001InitialSchema: Migration = {
  version: 1,
  name: 'initial_schema',

  async up(db) {
    // ── settings ────────────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS settings (
        key         TEXT PRIMARY KEY,
        value       TEXT NOT NULL,
        updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
      )
    `)

    // ── projects ─────────────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS projects (
        id          TEXT PRIMARY KEY,
        name        TEXT NOT NULL,
        path        TEXT NOT NULL,
        created_at  TEXT NOT NULL DEFAULT (datetime('now')),
        updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
      )
    `)

    // ── repos ────────────────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS repos (
        id          TEXT PRIMARY KEY,
        project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
        name        TEXT NOT NULL,
        remote_url  TEXT,
        created_at  TEXT NOT NULL DEFAULT (datetime('now')),
        updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_repos_project_id ON repos(project_id)
    `)

    // ── ssh_targets ───────────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS ssh_targets (
        id          TEXT PRIMARY KEY,
        alias       TEXT NOT NULL UNIQUE,
        host        TEXT NOT NULL,
        port        INTEGER NOT NULL DEFAULT 22,
        username    TEXT NOT NULL,
        key_path    TEXT,
        created_at  TEXT NOT NULL DEFAULT (datetime('now')),
        updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
      )
    `)
  },

  async down(db) {
    await db.exec('DROP TABLE IF EXISTS ssh_targets')
    await db.exec('DROP INDEX IF EXISTS idx_repos_project_id')
    await db.exec('DROP TABLE IF EXISTS repos')
    await db.exec('DROP TABLE IF EXISTS projects')
    await db.exec('DROP TABLE IF EXISTS settings')
  }
}
