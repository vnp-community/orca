/**
 * `createAutomationRun`/`updateAutomationRun` bodies for `PgAutomationStore`,
 * split out purely to stay under the repo's max-lines lint rule — same
 * queries/behavior as before, just moved out of the class. `listAutomationRuns`
 * is passed in (bound to the caller's `this`) rather than duplicated here, so
 * there is exactly one query implementation for "list runs for an automation".
 *
 * @module main/automations/pg-automation-store-run-mutations
 */

import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import { serviceQualifiedTable } from '../db/migrations/sql-dialect'
import type { Automation, AutomationDispatchResult, AutomationRun, AutomationRunTrigger } from '../../shared/automations-types'
import { pruneAutomationRuns, nextAutomationRunNumber } from '../../shared/automation-run-retention'
import type { AutomationRunRow } from './pg-automation-store-rows'
import { AUTOMATION_RUN_COLUMNS, rowToAutomationRun } from './pg-automation-store-rows'

function automationsTable(dialect: Parameters<typeof serviceQualifiedTable>[0]): string {
  return serviceQualifiedTable(dialect, 'automation', 'automations')
}

function automationRunsTable(dialect: Parameters<typeof serviceQualifiedTable>[0]): string {
  return serviceQualifiedTable(dialect, 'automation', 'automation_runs')
}

export async function createAutomationRunRow(
  pool: IConnectionPool,
  tenantId: string | undefined,
  automation: Automation,
  scheduledFor: number,
  trigger: AutomationRunTrigger,
  listAutomationRuns: (automationId?: string) => Promise<AutomationRun[]>
): Promise<AutomationRun> {
  const existing = (await listAutomationRuns(automation.id)).find((run) => run.scheduledFor === scheduledFor)
  if (existing) {return existing}

  const allRunsForAutomation = await listAutomationRuns(automation.id)
  const runNumber = nextAutomationRunNumber(allRunsForAutomation)
  const now = Date.now()
  const run: AutomationRun = {
    id: randomUUID(),
    automationId: automation.id,
    runNumber,
    runContext: automation.runContext ?? null,
    sourceContext: automation.sourceContext ?? null,
    title: `${automation.name} run ${runNumber}`,
    scheduledFor,
    status: 'pending',
    trigger,
    workspaceId: automation.workspaceId,
    workspaceDisplayName: null,
    sessionKind: 'terminal',
    chatSessionId: null,
    terminalSessionId: null,
    terminalPaneKey: null,
    terminalPtyId: null,
    outputSnapshot: null,
    precheckResult: null,
    usage: null,
    error: null,
    startedAt: null,
    dispatchedAt: null,
    createdAt: now
  }

  await pool.withConnection((db) =>
    db.query(
      `INSERT INTO ${automationRunsTable(db.capabilities.dialect)}
         (id, tenant_id, automation_id, run_context_json, source_context_json, title,
          scheduled_for, status, trigger, workspace_id, workspace_display_name, session_kind,
          chat_session_id, terminal_session_id, terminal_pane_key, terminal_pty_id,
          output_snapshot_json, precheck_result_json, usage_json, error, run_number,
          started_at, dispatched_at, created_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      [
        run.id, tenantId ?? null, run.automationId,
        run.runContext ? JSON.stringify(run.runContext) : null,
        run.sourceContext ? JSON.stringify(run.sourceContext) : null,
        run.title, run.scheduledFor, run.status, run.trigger, run.workspaceId,
        run.workspaceDisplayName, run.sessionKind, run.chatSessionId, run.terminalSessionId,
        run.terminalPaneKey, run.terminalPtyId, null, null, null, run.error, run.runNumber ?? null,
        run.startedAt, run.dispatchedAt, run.createdAt
      ]
    )
  )

  // Why: mirrors Store.createAutomationRun's pruneAutomationRuns() call —
  // evict old *final* runs past MAX_AUTOMATION_RUNS_PER_AUTOMATION so a
  // long-lived automation's run history doesn't grow unbounded. Store prunes
  // an in-memory array before flush(); here we compute the same kept-id set
  // against the freshly-read run list and DELETE whatever pruneAutomationRuns()
  // dropped — same retention policy (shared/automation-run-retention.ts),
  // different storage mechanics.
  const allRuns = await listAutomationRuns(automation.id)
  const kept = new Set(pruneAutomationRuns(allRuns).map((r) => r.id))
  const toDelete = allRuns.filter((r) => !kept.has(r.id)).map((r) => r.id)
  if (toDelete.length > 0) {
    await pool.withConnection((db) =>
      db.query(
        `DELETE FROM ${automationRunsTable(db.capabilities.dialect)} WHERE id IN (${toDelete.map(() => '?').join(',')})`,
        toDelete
      )
    )
  }

  return run
}

export async function updateAutomationRunRow(
  pool: IConnectionPool,
  result: AutomationDispatchResult
): Promise<AutomationRun> {
  const [current] = await pool.withConnection((db) =>
    db.query<AutomationRunRow>(
      `SELECT ${AUTOMATION_RUN_COLUMNS} FROM ${automationRunsTable(db.capabilities.dialect)} WHERE id = ?`,
      [result.runId]
    )
  )
  if (!current) {throw new Error('Automation run not found.')}
  const existing = rowToAutomationRun(current)
  const now = Date.now()
  const updated: AutomationRun = {
    ...existing,
    status: result.status,
    workspaceId: result.workspaceId ?? existing.workspaceId,
    workspaceDisplayName: Object.hasOwn(result, 'workspaceDisplayName')
      ? (result.workspaceDisplayName ?? null)
      : existing.workspaceDisplayName,
    terminalSessionId: Object.hasOwn(result, 'terminalSessionId')
      ? (result.terminalSessionId ?? null)
      : existing.terminalSessionId,
    terminalPaneKey: Object.hasOwn(result, 'terminalPaneKey')
      ? (result.terminalPaneKey ?? null)
      : existing.terminalPaneKey,
    terminalPtyId: Object.hasOwn(result, 'terminalPtyId')
      ? (result.terminalPtyId ?? null)
      : existing.terminalPtyId,
    outputSnapshot: Object.hasOwn(result, 'outputSnapshot')
      ? (result.outputSnapshot ?? null)
      : existing.outputSnapshot,
    precheckResult: Object.hasOwn(result, 'precheckResult')
      ? (result.precheckResult ?? null)
      : existing.precheckResult,
    usage: Object.hasOwn(result, 'usage') ? (result.usage ?? null) : existing.usage,
    error: result.error ?? null,
    startedAt: existing.startedAt ?? now,
    dispatchedAt: result.status === 'dispatched' ? now : existing.dispatchedAt
  }

  await pool.withConnection((db) =>
    db.query(
      `UPDATE ${automationRunsTable(db.capabilities.dialect)}
       SET status = ?, workspace_id = ?, workspace_display_name = ?, terminal_session_id = ?,
           terminal_pane_key = ?, terminal_pty_id = ?, output_snapshot_json = ?,
           precheck_result_json = ?, usage_json = ?, error = ?, started_at = ?, dispatched_at = ?
       WHERE id = ?`,
      [
        updated.status, updated.workspaceId, updated.workspaceDisplayName, updated.terminalSessionId,
        updated.terminalPaneKey, updated.terminalPtyId,
        updated.outputSnapshot ? JSON.stringify(updated.outputSnapshot) : null,
        updated.precheckResult ? JSON.stringify(updated.precheckResult) : null,
        updated.usage ? JSON.stringify(updated.usage) : null,
        updated.error, updated.startedAt, updated.dispatchedAt, updated.id
      ]
    )
  )
  await pool.withConnection((db) =>
    db.query(
      `UPDATE ${automationsTable(db.capabilities.dialect)} SET last_run_at = ?, updated_at = ? WHERE id = ?`,
      [now, now, updated.automationId]
    )
  )
  return updated
}
