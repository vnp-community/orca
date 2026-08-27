/**
 * Migration 0014 — Workflow Pause State
 *
 * FIX BUG-BE-HLD-009: user-triggered pause/resume needs a timestamp for
 * "paused since" in the Workflow Execution UI and for audit — `status='paused'`
 * itself needs no schema change (orca_workflow_executions.status is an
 * unconstrained TEXT column, see 0009_workflows.ts).
 *
 * @module db/migrations/0014_workflow_pause_state
 */

import type { Migration } from './types'

export const migration0014WorkflowPauseState: Migration = {
  version: 14,
  name: 'workflow_pause_state',

  async up(db) {
    // Why: nullable — most rows never pause. Cleared back to NULL by
    // WorkflowOrchestrator.resumeFromPause() so it always reflects "currently paused since".
    await db.exec(`ALTER TABLE orca_workflow_executions ADD COLUMN paused_at BIGINT`)
  },

  async down(_db) {
    // SQLite không hỗ trợ DROP COLUMN trực tiếp trước 3.35 — no-op an toàn,
    // theo đúng pattern 0013_workflow_trace_correlation.ts.
  },
}
