/**
 * Migration 0002 — Add Automations Table
 *
 * @module db/migrations/0002_add_automations
 */

import type { Migration } from './types'

export const migration0002AddAutomations: Migration = {
  version: 2,
  name: 'add_automations',

  async up(db) {
    await db.exec(`
      CREATE TABLE IF NOT EXISTS automations (
        id          TEXT PRIMARY KEY,
        project_id  TEXT REFERENCES projects(id) ON DELETE CASCADE,
        name        TEXT NOT NULL,
        trigger     TEXT NOT NULL,
        config      TEXT NOT NULL DEFAULT '{}',
        enabled     INTEGER NOT NULL DEFAULT 1,
        created_at  TEXT NOT NULL DEFAULT (datetime('now')),
        updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_automations_project_id ON automations(project_id)
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_automations_enabled ON automations(enabled)
    `)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_automations_enabled')
    await db.exec('DROP INDEX IF EXISTS idx_automations_project_id')
    await db.exec('DROP TABLE IF EXISTS automations')
  }
}
