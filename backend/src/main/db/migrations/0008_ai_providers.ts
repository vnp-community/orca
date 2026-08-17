/**
 * Migration 0008 — AI Provider Accounts & Usage Tracking
 *
 * Adds AI provider management tables for TDD-16:
 * - orca_ai_provider_accounts: Provider account registry (credential stored on Dev Server)
 * - orca_provider_usage: Daily token/cost usage tracking per account
 *
 * @module db/migrations/0008_ai_providers
 */

import type { Migration } from './types'
import { autoIncrementPrimaryKeySql } from './sql-dialect'

export const migration0008AiProviders: Migration = {
  version: 8,
  name: 'ai_providers',

  async up(db) {
    // BUG-BE-RPC-003: AUTOINCREMENT is SQLite-only — see sql-dialect.ts.
    const autoIncrementPk = autoIncrementPrimaryKeySql(db.capabilities.dialect)

    // ── orca_ai_provider_accounts ─────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_ai_provider_accounts (
        id                TEXT    PRIMARY KEY,
        dev_server_id     TEXT    NOT NULL,
        provider          TEXT    NOT NULL,
        scope             TEXT    NOT NULL DEFAULT 'server',
        scope_ref_id      TEXT,
        label             TEXT    NOT NULL,
        model             TEXT,
        base_url          TEXT,
        status            TEXT    NOT NULL DEFAULT 'pending',
        last_health_check BIGINT,
        quota_limit_day   INTEGER NOT NULL DEFAULT 0,
        created_by        TEXT    NOT NULL,
        created_at        BIGINT NOT NULL,
        updated_at        BIGINT NOT NULL
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_ai_providers_server
        ON orca_ai_provider_accounts(dev_server_id, status)
    `)

    // ── orca_provider_usage ───────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_provider_usage (
        id          ${autoIncrementPk},
        account_id  TEXT    NOT NULL REFERENCES orca_ai_provider_accounts(id) ON DELETE CASCADE,
        date        TEXT    NOT NULL,
        tokens_used INTEGER NOT NULL DEFAULT 0,
        requests    INTEGER NOT NULL DEFAULT 0,
        cost_usd    REAL    NOT NULL DEFAULT 0,
        UNIQUE(account_id, date)
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_provider_usage_date
        ON orca_provider_usage(account_id, date DESC)
    `)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_orca_provider_usage_date')
    await db.exec('DROP TABLE IF EXISTS orca_provider_usage')
    await db.exec('DROP INDEX IF EXISTS idx_orca_ai_providers_server')
    await db.exec('DROP TABLE IF EXISTS orca_ai_provider_accounts')
  }
}
