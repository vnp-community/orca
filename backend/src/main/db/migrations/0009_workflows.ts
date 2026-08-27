/**
 * Migration 0009 — Workflow Templates, Executions & Step Executions
 *
 * Adds DAG-based workflow orchestration tables for TDD-17:
 * - orca_workflow_templates: Reusable workflow definitions with inheritance
 * - orca_workflow_executions: Runtime workflow execution state
 * - orca_workflow_step_executions: Per-step execution results
 *
 * @module db/migrations/0009_workflows
 */

import type { Migration } from './types'

export const migration0009Workflows: Migration = {
  version: 9,
  name: 'workflows',

  async up(db) {
    // ── orca_workflow_templates ───────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_workflow_templates (
        id                  TEXT    PRIMARY KEY,
        name                TEXT    NOT NULL,
        version             INTEGER NOT NULL DEFAULT 1,
        parent_template_id  TEXT    REFERENCES orca_workflow_templates(id) ON DELETE SET NULL,
        description         TEXT,
        definition_json     TEXT    NOT NULL DEFAULT '{"steps":[]}',
        owner_id            TEXT,
        scope               TEXT    NOT NULL DEFAULT 'user',
        created_at          BIGINT NOT NULL,
        updated_at          BIGINT NOT NULL
      )
    `)

    // ── orca_workflow_executions ──────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_workflow_executions (
        id                  TEXT    PRIMARY KEY,
        definition_snapshot TEXT    NOT NULL,
        status              TEXT    NOT NULL DEFAULT 'pending',
        inputs_json         TEXT    NOT NULL DEFAULT '{}',
        current_wave        INTEGER NOT NULL DEFAULT 0,
        triggered_by        TEXT    NOT NULL,
        project_id          TEXT,
        started_at          BIGINT,
        completed_at        BIGINT,
        error_message       TEXT,
        created_at          BIGINT NOT NULL
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_workflow_exec_status
        ON orca_workflow_executions(status, created_at DESC)
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_workflow_exec_project
        ON orca_workflow_executions(project_id, status)
    `)

    // ── orca_workflow_step_executions ─────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_workflow_step_executions (
        id            TEXT    PRIMARY KEY,
        execution_id  TEXT    NOT NULL REFERENCES orca_workflow_executions(id) ON DELETE CASCADE,
        step_id       TEXT    NOT NULL,
        status        TEXT    NOT NULL DEFAULT 'pending',
        started_at    BIGINT,
        completed_at  BIGINT,
        output_json   TEXT,
        error_message TEXT
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_step_exec_execution
        ON orca_workflow_step_executions(execution_id, step_id)
    `)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_orca_step_exec_execution')
    await db.exec('DROP TABLE IF EXISTS orca_workflow_step_executions')
    await db.exec('DROP INDEX IF EXISTS idx_orca_workflow_exec_project')
    await db.exec('DROP INDEX IF EXISTS idx_orca_workflow_exec_status')
    await db.exec('DROP TABLE IF EXISTS orca_workflow_executions')
    await db.exec('DROP TABLE IF EXISTS orca_workflow_templates')
  }
}
