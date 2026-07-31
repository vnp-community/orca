/**
 * Migration 0006 — Company, Department, UserProfile tables
 *
 * Adds multi-tenant organization structure for Profile Hierarchy (TDD-14):
 * - orca_companies: Top-level company entities
 * - orca_departments: Departmental sub-units under a company
 * - orca_user_profiles: Per-user profile overrides (JSON)
 * - department_id column added to orca_users
 *
 * @module db/migrations/0006_company_dept
 */

import type { Migration } from './types'

export const migration0006CompanyDept: Migration = {
  version: 6,
  name: 'company_dept',

  async up(db) {
    // ── orca_companies ────────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_companies (
        id            TEXT    PRIMARY KEY,
        name          TEXT    NOT NULL,
        profile_json  TEXT    NOT NULL DEFAULT '{}',
        admin_user_id TEXT,
        created_at    INTEGER NOT NULL,
        updated_at    INTEGER NOT NULL,
        updated_by    TEXT
      )
    `)

    // ── orca_departments ──────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_departments (
        id             TEXT    PRIMARY KEY,
        company_id     TEXT    NOT NULL REFERENCES orca_companies(id) ON DELETE CASCADE,
        name           TEXT    NOT NULL,
        parent_dept_id TEXT    REFERENCES orca_departments(id) ON DELETE SET NULL,
        profile_json   TEXT    NOT NULL DEFAULT '{}',
        created_at     INTEGER NOT NULL,
        updated_at     INTEGER NOT NULL,
        updated_by     TEXT
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_departments_company
        ON orca_departments(company_id)
    `)

    // ── orca_user_profiles ────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_user_profiles (
        user_id      TEXT    PRIMARY KEY REFERENCES orca_users(id) ON DELETE CASCADE,
        profile_json TEXT    NOT NULL DEFAULT '{}',
        updated_at   INTEGER NOT NULL
      )
    `)

    // ── Add department_id to orca_users (idempotent) ──────────────────────────
    try {
      await db.exec(
        `ALTER TABLE orca_users ADD COLUMN department_id TEXT REFERENCES orca_departments(id) ON DELETE SET NULL`
      )
    } catch {
      // Column may already exist — safe to ignore
    }
  },

  async down(db) {
    await db.exec('DROP TABLE IF EXISTS orca_user_profiles')
    await db.exec('DROP INDEX IF EXISTS idx_orca_departments_company')
    await db.exec('DROP TABLE IF EXISTS orca_departments')
    await db.exec('DROP TABLE IF EXISTS orca_companies')
    // Note: department_id column on orca_users cannot be dropped in SQLite
    // (SQLite does not support DROP COLUMN on older versions)
  }
}
