/**
 * Migration 0024 — Orca Data State Blob (ADR-021, "chỉ dùng 1 database")
 *
 * Backing table for `persistence.ts`'s `Store` (the ~3900-line class behind
 * `orca-data.json` — Project/Repo/SshTarget/Worktree/Tab/UI state/Automation/
 * WebPush/Usage-adjacent fields, ~30 top-level `PersistedState` keys). Same
 * whole-state-blob pattern as migration 0023 (`usage.*_usage_state_blob`) —
 * one JSON blob per (tenant, user), not normalized into per-entity tables.
 *
 * Why blob, not normalized tables, for something this central: `Store`'s
 * ~100+ mutator methods all read/write `this.state` in memory and call one
 * shared `scheduleSave()` — none of them know or care that persistence is
 * file-based. Normalizing into per-entity Postgres tables would mean
 * rewriting every one of those methods individually (a full data-model
 * redesign, not a persistence-backend swap) — explicitly out of scope for
 * this pass, same reasoning as PgUsageStore's module doc comment. This blob
 * table is what lets `Store` keep every existing method's behavior
 * byte-for-byte identical while only its load()/write() *boundary* moves to
 * Postgres — see `main/orca-data-state-persistence.ts`.
 *
 * NOT included here: `orca-github-cache.json` (explicitly excluded from
 * `Store`'s own durable state already — see `getDurableState()`'s
 * `githubCache` omission comment) and the mobile-pairing/usage-token sidecar
 * files (`orca-devices.json`, `orca-e2ee-keypair.json`,
 * `openai-speech-token.enc`, integration credential files) — those are
 * either ephemeral caches or already covered by
 * specs/backend/models/05-credential-secret-stores.md's separate,
 * intentionally-not-Postgres secret stores (ADR-021 §4).
 *
 * @module db/migrations/0024_orca_data_state_blob
 */

import type { Migration } from './types'
import { serviceQualifiedTable } from './sql-dialect'

export const migration0024OrcaDataStateBlob: Migration = {
  version: 24,
  name: 'orca_data_state_blob',

  async up(db) {
    // Why its own schema ('core'), not reusing 'usage' or any single
    // existing ADR-021 service schema: this blob backs the union of nearly
    // every domain (Project/Repo/Worktree/Tab/UI/Automation/WebPush/...),
    // not one bounded-context service — see this migration's module doc
    // comment on why it isn't normalized into those services' own schemas.
    if (db.capabilities.dialect === 'postgresql') {
      await db.exec(`CREATE SCHEMA IF NOT EXISTS core`)
    }
    const table = serviceQualifiedTable(db.capabilities.dialect, 'core', 'orca_data_state_blob')
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
    const table = serviceQualifiedTable(db.capabilities.dialect, 'core', 'orca_data_state_blob')
    await db.exec(`DROP TABLE IF EXISTS ${table}`)
  }
}
