/**
 * Migration 0004 — Orca Server-mode Application Tables
 *
 * Creates `orca_*` tables for SqlStateRepository (server mode).
 * These are separate from the core `projects`, `repos` tables (migration 0001)
 * to maintain clear separation between server-mode state and system state.
 *
 * @module db/migrations/0004_orca_app_tables
 */

import type { Migration } from './types'

export const migration0004OrcaAppTables: Migration = {
  version: 4,
  name: 'orca_app_tables',

  async up(db) {
    // ── orca_projects ────────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_projects (
        id          TEXT PRIMARY KEY,
        name        TEXT NOT NULL,
        tab_order   INTEGER NOT NULL DEFAULT 0,
        data        TEXT NOT NULL DEFAULT '{}',
        created_at  TEXT NOT NULL DEFAULT (datetime('now'))
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_projects_tab_order ON orca_projects(tab_order)
    `)

    // ── orca_repos ───────────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_repos (
        id          TEXT PRIMARY KEY,
        project_id  TEXT,
        data        TEXT NOT NULL DEFAULT '{}',
        created_at  TEXT NOT NULL DEFAULT (datetime('now'))
      )
    `)

    // ── orca_ssh_targets ─────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_ssh_targets (
        id          TEXT PRIMARY KEY,
        label       TEXT NOT NULL,
        host        TEXT NOT NULL,
        port        INTEGER NOT NULL DEFAULT 22,
        username    TEXT NOT NULL,
        data        TEXT NOT NULL DEFAULT '{}',
        created_at  TEXT NOT NULL DEFAULT (datetime('now'))
      )
    `)

    // ── orca_global_settings ─────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_global_settings (
        key         TEXT PRIMARY KEY,
        value       TEXT NOT NULL,
        updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
      )
    `)
  },

  async down(db) {
    await db.exec('DROP TABLE IF EXISTS orca_global_settings')
    await db.exec('DROP TABLE IF EXISTS orca_ssh_targets')
    await db.exec('DROP TABLE IF EXISTS orca_repos')
    await db.exec('DROP INDEX IF EXISTS idx_orca_projects_tab_order')
    await db.exec('DROP TABLE IF EXISTS orca_projects')
  }
}
