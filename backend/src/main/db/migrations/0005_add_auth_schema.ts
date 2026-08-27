/**
 * Migration 0005 — Auth Schema (Users, Sessions, Audit Log, Access Policies)
 *
 * Adds multi-user auth tables for server mode:
 * - orca_users: User accounts (local + SSO)
 * - orca_sessions: HTTP session tokens
 * - orca_audit_log: Immutable audit trail
 * - orca_access_policies: RBAC policy definitions
 *
 * @module db/migrations/0005_add_auth_schema
 */

import type { Migration } from './types'
import { autoIncrementPrimaryKeySql } from './sql-dialect'

export const migration0005AddAuthSchema: Migration = {
  version: 5,
  name: 'add_auth_schema',

  async up(db) {
    // BUG-BE-RPC-003: AUTOINCREMENT is SQLite-only — see sql-dialect.ts.
    const autoIncrementPk = autoIncrementPrimaryKeySql(db.capabilities.dialect)
    // ── orca_users ───────────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_users (
        id               TEXT    PRIMARY KEY,
        email            TEXT    UNIQUE NOT NULL,
        name             TEXT    NOT NULL,
        password_hash    TEXT,
        role             TEXT    NOT NULL DEFAULT 'developer',
        provider         TEXT    NOT NULL DEFAULT 'none',
        provider_user_id TEXT,
        avatar_url       TEXT,
        teams            TEXT    NOT NULL DEFAULT '[]',
        projects         TEXT    NOT NULL DEFAULT '[]',
        created_at       BIGINT NOT NULL,
        last_login_at    BIGINT,
        is_active        INTEGER NOT NULL DEFAULT 1
      )
    `)

    // ── orca_sessions ────────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_sessions (
        session_id    TEXT    PRIMARY KEY,
        user_id       TEXT    NOT NULL REFERENCES orca_users(id) ON DELETE CASCADE,
        created_at    BIGINT NOT NULL,
        expires_at    BIGINT NOT NULL,
        last_seen_at  BIGINT,
        ip_address    TEXT,
        user_agent    TEXT
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_sessions_user
        ON orca_sessions(user_id)
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_sessions_expires
        ON orca_sessions(expires_at)
    `)

    // ── orca_audit_log ───────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_audit_log (
        id          ${autoIncrementPk},
        created_at  BIGINT NOT NULL,
        user_id     TEXT,
        user_email  TEXT,
        action      TEXT    NOT NULL,
        detail      TEXT,
        ip_address  TEXT
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_audit_user
        ON orca_audit_log(user_id, created_at DESC)
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_audit_action
        ON orca_audit_log(action, created_at DESC)
    `)

    // ── orca_access_policies ─────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_access_policies (
        id                     TEXT    PRIMARY KEY,
        name                   TEXT    NOT NULL,
        teams                  TEXT    NOT NULL DEFAULT '[]',
        roles                  TEXT    NOT NULL DEFAULT '[]',
        users                  TEXT    NOT NULL DEFAULT '[]',
        allowed_servers        TEXT    NOT NULL DEFAULT '"*"',
        allowed_projects       TEXT    NOT NULL DEFAULT '"*"',
        agent_trust            TEXT    NOT NULL DEFAULT 'standard',
        can_create_worktrees   INTEGER NOT NULL DEFAULT 1,
        can_delete_worktrees   INTEGER NOT NULL DEFAULT 1,
        can_access_production  INTEGER NOT NULL DEFAULT 0,
        created_at             BIGINT NOT NULL,
        updated_at             BIGINT NOT NULL
      )
    `)
  },

  async down(db) {
    await db.exec('DROP TABLE IF EXISTS orca_access_policies')
    await db.exec('DROP INDEX IF EXISTS idx_orca_audit_action')
    await db.exec('DROP INDEX IF EXISTS idx_orca_audit_user')
    await db.exec('DROP TABLE IF EXISTS orca_audit_log')
    await db.exec('DROP INDEX IF EXISTS idx_orca_sessions_expires')
    await db.exec('DROP INDEX IF EXISTS idx_orca_sessions_user')
    await db.exec('DROP TABLE IF EXISTS orca_sessions')
    await db.exec('DROP TABLE IF EXISTS orca_users')
  }
}
