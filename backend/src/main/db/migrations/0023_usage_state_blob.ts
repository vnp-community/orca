/**
 * Migration 0023 — Usage State Blob (ADR-021, "chỉ dùng 1 database")
 *
 * `ClaudeUsageStore`/`CodexUsageStore` (backend/src/main/{claude,codex}-usage/store.ts)
 * persist their ENTIRE `ClaudeUsagePersistedState`/`CodexUsagePersistedState` as
 * one atomically-rewritten JSON file (`orca-claude-usage.json`) — load-whole,
 * mutate-in-memory, save-whole, exactly like `persistence.ts`'s `Store` itself.
 * Migration 0022's `usage.{claude,codex}_usage_sessions`/`_daily`/
 * `_processed_files` tables assumed a granular per-row upsert model that would
 * require rewriting ~30 internal methods across two ~900-line classes to
 * populate correctly — real work, deliberately NOT done in this pass (see
 * `usage/pg-usage-store.ts`'s module doc comment).
 *
 * This migration adds the pragmatic alternative actually wired up: one JSON
 * blob column per (tenant, user), same shape as `orca_global_settings.value`/
 * `orca_projects.data` elsewhere in this schema family — safe, minimal-risk
 * swap of the persistence *boundary* (load()/writeToDisk()) with ZERO changes
 * to either class's internal scan/aggregate/dedupe logic, which keeps
 * operating on the in-memory state object exactly as before.
 *
 * The granular tables from migration 0022 are NOT dropped — they remain
 * available for a future normalized implementation that queries usage
 * per-session/per-day directly instead of round-tripping the whole blob.
 *
 * @module db/migrations/0023_usage_state_blob
 */

import type { Migration } from './types'
import { serviceQualifiedTable } from './sql-dialect'

export const migration0023UsageStateBlob: Migration = {
  version: 23,
  name: 'usage_state_blob',

  async up(db) {
    for (const provider of ['claude', 'codex'] as const) {
      const table = serviceQualifiedTable(db.capabilities.dialect, 'usage', `${provider}_usage_state_blob`)
      await db.exec(`
        CREATE TABLE IF NOT EXISTS ${table} (
          tenant_id    TEXT    NOT NULL DEFAULT '',
          user_id      TEXT    NOT NULL DEFAULT '',
          state_json   TEXT    NOT NULL,
          updated_at   BIGINT  NOT NULL,
          PRIMARY KEY (tenant_id, user_id)
        )
      `)
    }
  },

  async down(db) {
    for (const provider of ['claude', 'codex'] as const) {
      const table = serviceQualifiedTable(db.capabilities.dialect, 'usage', `${provider}_usage_state_blob`)
      await db.exec(`DROP TABLE IF EXISTS ${table}`)
    }
  }
}
