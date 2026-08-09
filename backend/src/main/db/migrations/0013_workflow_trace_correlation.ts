/**
 * Migration 0013 — Workflow Trace Correlation
 *
 * FIX CR-TRACE-017 (SOL-BE-TRACE-017): Persist the workflow execution's root
 * trace span id so `resumeRunningExecutions()` can re-create the same parent
 * span after an Orca Server restart, keeping TracePanel able to group
 * pre-restart and post-restart step spans under the same execution.
 *
 * @module db/migrations/0013_workflow_trace_correlation
 */

import type { Migration } from './types'

export const migration0013WorkflowTraceCorrelation: Migration = {
  version: 13,
  name: 'workflow_trace_correlation',

  async up(db) {
    // Why: rootTraceId phải sống sót qua Orca Server restart để resumeRunningExecutions()
    // tái tạo đúng span cha (CR-TRACE-000 §3.1 resume) — nếu không, TracePanel mất khả năng
    // nhóm step cũ (trước restart) với step mới (sau restart) dưới cùng 1 execution.
    await db.exec(`ALTER TABLE orca_workflow_executions ADD COLUMN root_trace_id TEXT`)
  },

  async down(_db) {
    // SQLite không hỗ trợ DROP COLUMN trực tiếp trước 3.35 — no-op an toàn,
    // cột thừa không ảnh hưởng hành vi nếu rollback (theo pattern các migration khác trong repo).
  },
}
