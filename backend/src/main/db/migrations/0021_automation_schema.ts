/**
 * Migration 0021 — Automation Schema (Phase 0 of ADR-021)
 *
 * Proper SQL home for `Automation`/`AutomationRun`
 * (backend/src/shared/automations-types.ts), which in server mode currently
 * live only inside `PersistedState.automations`/`automationRuns` — the
 * Electron-desktop JSON store (`persistence.ts`'s `Store`), read/written by
 * `AutomationService` (backend/src/main/automations/service.ts) even when
 * running headless server mode, since that service takes a `Store`, not a SQL
 * pool. See specs/backend/models/02-sql-schema-catalog.md Nhóm B for why this
 * is a NEW design, not a revival of the dormant `automations` table from
 * migration 0002 (that one never matched the real `Automation` shape).
 *
 * DDL-only, Phase 0: `AutomationService` still reads/writes `Store` exclusively
 * today — wiring it onto these tables (server mode only; Electron desktop mode
 * keeps `Store`) is ADR-021 Phase 1, not this migration.
 *
 * @module db/migrations/0021_automation_schema
 */

import type { Migration } from './types'
import { serviceQualifiedTable } from './sql-dialect'

export const migration0021AutomationSchema: Migration = {
  version: 21,
  name: 'automation_schema',

  async up(db) {
    const t = (name: string): string => serviceQualifiedTable(db.capabilities.dialect, 'automation', name)

    // ── automations — scheduled agent-prompt definition ───────────────────────
    // Field shapes mirror backend/src/shared/automations-types.ts `Automation`
    // (types.ts:90) — re-verify against that file before Phase 1 wiring.
    await db.exec(`
      CREATE TABLE IF NOT EXISTS ${t('automations')} (
        id                        TEXT    PRIMARY KEY,
        tenant_id                 TEXT,
        name                      TEXT    NOT NULL,
        prompt                    TEXT    NOT NULL,
        precheck_json             TEXT,
        agent_id                  TEXT    NOT NULL,
        run_context_json          TEXT,
        source_context_json       TEXT,
        project_id                TEXT,
        execution_target_type     TEXT    NOT NULL,
        execution_target_id       TEXT    NOT NULL,
        scheduler_owner           TEXT    NOT NULL,
        workspace_mode            TEXT    NOT NULL DEFAULT 'existing',
        workspace_id              TEXT,
        base_branch                TEXT,
        setup_decision_json        TEXT,
        reuse_session               INTEGER NOT NULL DEFAULT 0,
        timezone                    TEXT    NOT NULL,
        rrule                       TEXT    NOT NULL,
        dtstart                     TEXT    NOT NULL,
        enabled                     INTEGER NOT NULL DEFAULT 1,
        next_run_at                 BIGINT,
        last_run_at                 BIGINT,
        missed_run_policy           TEXT    NOT NULL DEFAULT 'run_once_within_grace',
        missed_run_grace_minutes    INTEGER NOT NULL DEFAULT 0,
        created_at                  BIGINT  NOT NULL,
        updated_at                  BIGINT  NOT NULL
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_${t('automations').replace('.', '_')}_tenant ON ${t('automations')}(tenant_id, enabled)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_${t('automations').replace('.', '_')}_next_run ON ${t('automations')}(enabled, next_run_at)`)

    // ── automation_runs — 1 row per dispatched execution ──────────────────────
    // Field shapes mirror `AutomationRun` (automations-types.ts:128).
    await db.exec(`
      CREATE TABLE IF NOT EXISTS ${t('automation_runs')} (
        id                       TEXT    PRIMARY KEY,
        tenant_id                TEXT,
        automation_id            TEXT    NOT NULL,
        run_context_json         TEXT,
        source_context_json      TEXT,
        title                    TEXT,
        scheduled_for            BIGINT,
        status                   TEXT    NOT NULL DEFAULT 'pending',
        trigger                  TEXT    NOT NULL DEFAULT 'scheduled',
        workspace_id             TEXT,
        workspace_display_name   TEXT,
        session_kind             TEXT    NOT NULL DEFAULT 'terminal',
        chat_session_id          TEXT,
        terminal_session_id      TEXT,
        terminal_pane_key        TEXT,
        terminal_pty_id          TEXT,
        output_snapshot_json     TEXT,
        precheck_result_json     TEXT,
        usage_json               TEXT,
        error                    TEXT,
        run_number               INTEGER,
        started_at               BIGINT,
        dispatched_at            BIGINT,
        created_at               BIGINT  NOT NULL
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_${t('automation_runs').replace('.', '_')}_automation ON ${t('automation_runs')}(automation_id, created_at DESC)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS idx_${t('automation_runs').replace('.', '_')}_tenant ON ${t('automation_runs')}(tenant_id, created_at DESC)`)
  },

  async down(db) {
    const t = (name: string): string => serviceQualifiedTable(db.capabilities.dialect, 'automation', name)
    await db.exec(`DROP TABLE IF EXISTS ${t('automation_runs')}`)
    await db.exec(`DROP TABLE IF EXISTS ${t('automations')}`)
  }
}
