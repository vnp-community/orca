/**
 * Migration 0020 — Orchestration Schema (ADR-021)
 *
 * Mirrors the 5 tables in the standalone `orchestration.db` SQLite file
 * (`runtime/orchestration/db.ts`, SCHEMA_VERSION=6) into the Postgres
 * `orchestration` schema created by migration 0019. Column-for-column
 * transcription of `OrchestrationDb.createTables()` — verified against that
 * method's literal DDL (not just `types.ts`'s row types, which an earlier
 * version of this migration was transcribed from and got 6 columns wrong:
 * timestamps are TEXT here — matching SQLite's `datetime('now')` /
 * `new Date().toISOString()` string values the row types (`created_at:
 * string` etc.) actually carry — not BIGINT epoch-ms like every other
 * ADR-021 migration; `dispatch_contexts`/`decision_gates` were missing their
 * own `created_at` column; `messages` was missing its `UNIQUE(id)` and
 * `(to_handle, read, delivered_at, sequence)` indexes).
 *
 * Backing store for `PgOrchestrationDb` (runtime/orchestration/pg-db.ts) —
 * see that file's module doc comment for the sync→async conversion this
 * schema supports, and for why the runtime (`coordinator.ts` + 9 dependent
 * files) does NOT read/write through this schema yet.
 *
 * @module db/migrations/0020_orchestration_schema
 */

import type { Migration } from './types'
import { autoIncrementPrimaryKeySql, serviceQualifiedTable } from './sql-dialect'

export const migration0020OrchestrationSchema: Migration = {
  version: 20,
  name: 'orchestration_schema',

  async up(db) {
    const t = (name: string): string => serviceQualifiedTable(db.capabilities.dialect, 'orchestration', name)
    const idx = (table: string, suffix: string): string => `idx_${t(table).replace('.', '_')}_${suffix}`
    const autoIncrementPk = autoIncrementPrimaryKeySql(db.capabilities.dialect)

    // ── messages — inter-agent mailbox ────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS ${t('messages')} (
        sequence          ${autoIncrementPk},
        id                TEXT    NOT NULL,
        tenant_id         TEXT,
        from_handle       TEXT    NOT NULL,
        to_handle         TEXT    NOT NULL,
        subject           TEXT    NOT NULL,
        body              TEXT    NOT NULL DEFAULT '',
        type              TEXT    NOT NULL DEFAULT 'status'
          CHECK(type IN ('status', 'dispatch', 'worker_done', 'merge_ready',
                          'escalation', 'handoff', 'decision_gate', 'heartbeat')),
        priority          TEXT    NOT NULL DEFAULT 'normal'
          CHECK(priority IN ('normal', 'high', 'urgent')),
        thread_id         TEXT,
        payload           TEXT,
        read              INTEGER NOT NULL DEFAULT 0,
        created_at        TEXT    NOT NULL,
        delivered_at      TEXT,
        sender_pane_key   TEXT
      )
    `)
    await db.exec(`CREATE UNIQUE INDEX IF NOT EXISTS ${idx('messages', 'id')} ON ${t('messages')}(id)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('messages', 'inbox')} ON ${t('messages')}(to_handle, read)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('messages', 'undelivered_inbox')} ON ${t('messages')}(to_handle, read, delivered_at, sequence)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('messages', 'thread')} ON ${t('messages')}(thread_id)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('messages', 'tenant')} ON ${t('messages')}(tenant_id)`)

    // ── tasks (TaskRow) — coordinator DAG node ────────────────────────────────
    // Note: intentionally NOT named the same as task-service's `orca_tasks` —
    // this is the "complex path" sub-task DAG, a different id space entirely
    // (see specs/backend/models/04-orchestration-db.md §2).
    await db.exec(`
      CREATE TABLE IF NOT EXISTS ${t('tasks')} (
        id                          TEXT    PRIMARY KEY,
        tenant_id                   TEXT,
        parent_id                   TEXT,
        created_by_terminal_handle  TEXT,
        task_title                  TEXT,
        display_name                TEXT,
        spec                        TEXT    NOT NULL,
        status                      TEXT    NOT NULL DEFAULT 'pending'
          CHECK(status IN ('pending', 'ready', 'dispatched', 'completed', 'failed', 'blocked')),
        deps                        TEXT    NOT NULL DEFAULT '[]',
        result                      TEXT,
        created_at                  TEXT    NOT NULL,
        completed_at                TEXT
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('tasks', 'parent')} ON ${t('tasks')}(parent_id)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('tasks', 'status')} ON ${t('tasks')}(status)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('tasks', 'tenant')} ON ${t('tasks')}(tenant_id)`)

    // ── dispatch_contexts — worker assignment + liveness tracking ─────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS ${t('dispatch_contexts')} (
        id                  TEXT    PRIMARY KEY,
        tenant_id           TEXT,
        task_id             TEXT    NOT NULL,
        assignee_handle     TEXT,
        assignee_pane_key   TEXT,
        status              TEXT    NOT NULL DEFAULT 'pending'
          CHECK(status IN ('pending', 'dispatched', 'completed', 'failed', 'circuit_broken')),
        failure_count       INTEGER NOT NULL DEFAULT 0,
        last_failure        TEXT,
        dispatched_at       TEXT,
        completed_at        TEXT,
        created_at          TEXT    NOT NULL,
        last_heartbeat_at   TEXT
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('dispatch_contexts', 'task')} ON ${t('dispatch_contexts')}(task_id)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('dispatch_contexts', 'status')} ON ${t('dispatch_contexts')}(status)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('dispatch_contexts', 'tenant')} ON ${t('dispatch_contexts')}(tenant_id)`)

    // ── decision_gates — human/agent decision checkpoints ─────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS ${t('decision_gates')} (
        id            TEXT    PRIMARY KEY,
        tenant_id     TEXT,
        task_id       TEXT    NOT NULL,
        question      TEXT    NOT NULL,
        options       TEXT    NOT NULL DEFAULT '[]',
        status        TEXT    NOT NULL DEFAULT 'pending'
          CHECK(status IN ('pending', 'resolved', 'timeout')),
        resolution    TEXT,
        created_at    TEXT    NOT NULL,
        resolved_at   TEXT
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('decision_gates', 'task')} ON ${t('decision_gates')}(task_id)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('decision_gates', 'status')} ON ${t('decision_gates')}(status)`)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('decision_gates', 'tenant')} ON ${t('decision_gates')}(tenant_id)`)

    // ── coordinator_runs — 1 row per top-level coordinator session ────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS ${t('coordinator_runs')} (
        id                    TEXT    PRIMARY KEY,
        tenant_id             TEXT,
        spec                  TEXT    NOT NULL,
        status                TEXT    NOT NULL DEFAULT 'idle'
          CHECK(status IN ('idle', 'running', 'completed', 'failed')),
        coordinator_handle    TEXT    NOT NULL,
        poll_interval_ms      INTEGER NOT NULL DEFAULT 2000,
        created_at            TEXT    NOT NULL,
        completed_at          TEXT
      )
    `)
    await db.exec(`CREATE INDEX IF NOT EXISTS ${idx('coordinator_runs', 'tenant')} ON ${t('coordinator_runs')}(tenant_id)`)
  },

  async down(db) {
    const t = (name: string): string => serviceQualifiedTable(db.capabilities.dialect, 'orchestration', name)
    await db.exec(`DROP TABLE IF EXISTS ${t('coordinator_runs')}`)
    await db.exec(`DROP TABLE IF EXISTS ${t('decision_gates')}`)
    await db.exec(`DROP TABLE IF EXISTS ${t('dispatch_contexts')}`)
    await db.exec(`DROP TABLE IF EXISTS ${t('tasks')}`)
    await db.exec(`DROP TABLE IF EXISTS ${t('messages')}`)
  }
}
