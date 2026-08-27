/**
 * Migration 0022 — Notification + Usage Schemas (Phase 0 of ADR-021)
 *
 * `notification` schema: replaces `PersistedState.webPushSubscriptions`/
 * `vapidKeys` (JSON, read/written by `WebPushManager` via `Store` even in
 * server mode) — see specs/backend/models/02-sql-schema-catalog.md Nhóm K.
 * The VAPID `privateKey` value itself is NOT stored here — only public
 * key/status metadata — per ADR-021 §4: secret material stays out of the
 * unified Postgres, tracked instead via the `credential` schema's
 * metadata-only pattern (see 05-credential-secret-stores.md).
 *
 * `usage` schema: replaces `orca-claude-usage.json`/`orca-codex-usage.json`
 * (see specs/backend/models/07-usage-tracking-stores.md) for server mode.
 * Column shapes mirror `ClaudeUsageSession`/`ClaudeUsageDailyAggregate`
 * (backend/src/main/claude-usage/types.ts) and the Codex equivalents —
 * re-verify against those files before Phase 1 wiring. `locationBreakdown`
 * (per-session location rollup) and `processedFiles`/`scanState`
 * (incremental-scan bookkeeping) are kept as JSON columns rather than further
 * normalized tables — they're written wholesale by the scanner, never queried
 * piecemeal, so normalizing them would add join cost with no query benefit.
 *
 * DDL-only, Phase 0 — see 0021_automation_schema.ts's module doc comment for
 * why these tables have zero consumers until ADR-021 Phase 1.
 *
 * @module db/migrations/0022_notification_usage_schema
 */

import type { Migration } from './types'
import { serviceQualifiedTable } from './sql-dialect'

function usageTables(db: Parameters<Migration['up']>[0], provider: 'claude' | 'codex'): { sessions: string; daily: string; files: string } {
  const t = (name: string): string => serviceQualifiedTable(db.capabilities.dialect, 'usage', name)
  return {
    sessions: t(`${provider}_usage_sessions`),
    daily: t(`${provider}_usage_daily`),
    files: t(`${provider}_usage_processed_files`)
  }
}

export const migration0022NotificationUsageSchema: Migration = {
  version: 22,
  name: 'notification_usage_schema',

  async up(db) {
    const nt = (name: string): string => serviceQualifiedTable(db.capabilities.dialect, 'notification', name)

    // ── notification.push_subscriptions ────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS ${nt('push_subscriptions')} (
        id           TEXT    PRIMARY KEY,
        tenant_id    TEXT,
        user_id      TEXT    NOT NULL,
        endpoint     TEXT    NOT NULL UNIQUE,
        auth         TEXT    NOT NULL,
        p256dh       TEXT    NOT NULL,
        user_agent   TEXT,
        added_at     BIGINT  NOT NULL
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_${nt('push_subscriptions').replace('.', '_')}_user ON ${nt('push_subscriptions')}(user_id)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_${nt('push_subscriptions').replace('.', '_')}_tenant ON ${nt('push_subscriptions')}(tenant_id)`)

    // ── notification.vapid_key_metadata — NO private_key column, see module doc ──
    await db.exec(`
      CREATE TABLE IF NOT EXISTS ${nt('vapid_key_metadata')} (
        key_id       TEXT    PRIMARY KEY,
        tenant_id    TEXT,
        public_key   TEXT    NOT NULL,
        status       TEXT    NOT NULL DEFAULT 'active',
        created_at   BIGINT  NOT NULL
      )
    `)

    // ── usage.{claude,codex}_usage_sessions / _daily / _processed_files ─────────
    for (const provider of ['claude', 'codex'] as const) {
      const tables = usageTables(db, provider)

      await db.exec(`
        CREATE TABLE IF NOT EXISTS ${tables.sessions} (
          session_id                TEXT    PRIMARY KEY,
          tenant_id                 TEXT,
          user_id                   TEXT,
          first_timestamp           TEXT    NOT NULL,
          last_timestamp            TEXT    NOT NULL,
          model                     TEXT,
          last_cwd                  TEXT,
          last_git_branch           TEXT,
          primary_worktree_id       TEXT,
          primary_repo_id           TEXT,
          turn_count                INTEGER NOT NULL DEFAULT 0,
          total_input_tokens        INTEGER NOT NULL DEFAULT 0,
          total_output_tokens       INTEGER NOT NULL DEFAULT 0,
          total_cache_read_tokens   INTEGER NOT NULL DEFAULT 0,
          total_cache_write_tokens  INTEGER NOT NULL DEFAULT 0,
          location_breakdown_json   TEXT    NOT NULL DEFAULT '[]'
        )
      `)
      await db.exec(`CREATE INDEX IF NOT EXISTS idx_${tables.sessions.replace('.', '_')}_tenant ON ${tables.sessions}(tenant_id, user_id)`)

      await db.exec(`
        CREATE TABLE IF NOT EXISTS ${tables.daily} (
          id                           TEXT    PRIMARY KEY,
          tenant_id                   TEXT,
          user_id                     TEXT,
          day                         TEXT    NOT NULL,
          model                       TEXT,
          project_key                 TEXT    NOT NULL,
          project_label                TEXT    NOT NULL,
          repo_id                      TEXT,
          worktree_id                  TEXT,
          turn_count                   INTEGER NOT NULL DEFAULT 0,
          zero_cache_read_turn_count   INTEGER NOT NULL DEFAULT 0,
          input_tokens                 INTEGER NOT NULL DEFAULT 0,
          output_tokens                INTEGER NOT NULL DEFAULT 0,
          cache_read_tokens             INTEGER NOT NULL DEFAULT 0,
          cache_write_tokens            INTEGER NOT NULL DEFAULT 0
        )
      `)
      await db.exec(`CREATE INDEX IF NOT EXISTS idx_${tables.daily.replace('.', '_')}_lookup ON ${tables.daily}(tenant_id, user_id, day DESC)`)
      // Upsert key for the daily-rollup writer (mirrors orca_provider_usage's
      // UNIQUE(account_id,date) pattern from 0008_ai_providers.ts) — 1 row per
      // user+day+model+project+repo+worktree combination.
      // ⚠️ Phase 1 caveat: SQL NULL is never "equal" to itself in a UNIQUE
      // index (Postgres and SQLite both), so rows with NULL repo_id/worktree_id
      // won't collide/upsert as intended — Phase 1's writer must coalesce those
      // to a non-null sentinel (e.g. '') before insert, not rely on this index
      // alone to dedupe NULL-bearing rows.
      await db.exec(`
        CREATE UNIQUE INDEX IF NOT EXISTS idx_${tables.daily.replace('.', '_')}_upsert
          ON ${tables.daily}(tenant_id, user_id, day, model, project_key, repo_id, worktree_id)
      `)

      // Incremental-scan bookkeeping (ownedDedupeKeys/hasDeferredClaims — see
      // specs/backend/models/07-usage-tracking-stores.md for why this can't be
      // dropped: it's what makes forked/resumed transcript re-scans idempotent).
      await db.exec(`
        CREATE TABLE IF NOT EXISTS ${tables.files} (
          path                    TEXT    PRIMARY KEY,
          tenant_id               TEXT,
          user_id                 TEXT,
          mtime_ms                 BIGINT  NOT NULL,
          size                      BIGINT  NOT NULL,
          line_count                INTEGER NOT NULL DEFAULT 0,
          owned_dedupe_keys_json    TEXT    NOT NULL DEFAULT '[]',
          has_deferred_claims       INTEGER NOT NULL DEFAULT 0,
          sessions_json             TEXT    NOT NULL DEFAULT '[]',
          daily_aggregates_json     TEXT    NOT NULL DEFAULT '[]'
        )
      `)
    }
  },

  async down(db) {
    for (const provider of ['claude', 'codex'] as const) {
      const tables = usageTables(db, provider)
      await db.exec(`DROP TABLE IF EXISTS ${tables.files}`)
      await db.exec(`DROP TABLE IF EXISTS ${tables.daily}`)
      await db.exec(`DROP TABLE IF EXISTS ${tables.sessions}`)
    }
    const nt = (name: string): string => serviceQualifiedTable(db.capabilities.dialect, 'notification', name)
    await db.exec(`DROP TABLE IF EXISTS ${nt('vapid_key_metadata')}`)
    await db.exec(`DROP TABLE IF EXISTS ${nt('push_subscriptions')}`)
  }
}
