/**
 * Migration 0025 — OpenCode Usage State Blob (ADR-021, "chỉ dùng 1 database")
 *
 * Same whole-state-blob pattern as migration 0023
 * (`usage.{claude,codex}_usage_state_blob`), added separately here (rather
 * than amending 0023) because that migration may already be applied in
 * existing deployments — migrations are append-only. Backs
 * `OpenCodeUsageStore` (backend/src/main/opencode-usage/store.ts), the
 * server-mode port of desktop's OpenCode usage analytics store, via
 * `PgUsageStatePersistence(pool, 'opencode', tenantId)`.
 *
 * @module db/migrations/0025_opencode_usage_state_blob
 */

import type { Migration } from './types'
import { serviceQualifiedTable } from './sql-dialect'

export const migration0025OpenCodeUsageStateBlob: Migration = {
  version: 25,
  name: 'opencode_usage_state_blob',

  async up(db) {
    const table = serviceQualifiedTable(db.capabilities.dialect, 'usage', 'opencode_usage_state_blob')
    await db.exec(`
      CREATE TABLE IF NOT EXISTS ${table} (
        tenant_id    TEXT    NOT NULL DEFAULT '',
        user_id      TEXT    NOT NULL DEFAULT '',
        state_json   TEXT    NOT NULL,
        updated_at   BIGINT  NOT NULL,
        PRIMARY KEY (tenant_id, user_id)
      )
    `)
  },

  async down(db) {
    const table = serviceQualifiedTable(db.capabilities.dialect, 'usage', 'opencode_usage_state_blob')
    await db.exec(`DROP TABLE IF EXISTS ${table}`)
  }
}
