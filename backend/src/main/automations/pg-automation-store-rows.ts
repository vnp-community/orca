/**
 * Row shapes + SQL column lists + row→domain mappers for `PgAutomationStore`
 * (migration 0021_automation_schema.ts). Split out of pg-automation-store.ts
 * purely to stay under the repo's max-lines lint rule — no behavior here,
 * just the `automation.automations`/`automation.automation_runs` column
 * mapping.
 *
 * @module main/automations/pg-automation-store-rows
 */

import type { Automation, AutomationRun, AutomationRunTrigger } from '../../shared/automations-types'

export type AutomationRow = {
  id: string
  name: string
  prompt: string
  precheckJson: string | null
  agentId: string
  runContextJson: string | null
  sourceContextJson: string | null
  projectId: string
  executionTargetType: string
  executionTargetId: string
  schedulerOwner: string
  workspaceMode: string
  workspaceId: string | null
  baseBranch: string | null
  setupDecisionJson: string | null
  reuseSession: number
  timezone: string
  rrule: string
  dtstart: number
  enabled: number
  nextRunAt: number
  lastRunAt: number | null
  missedRunPolicy: string
  missedRunGraceMinutes: number
  createdAt: number
  updatedAt: number
}

export type AutomationRunRow = {
  id: string
  automationId: string
  runContextJson: string | null
  sourceContextJson: string | null
  title: string
  scheduledFor: number
  status: string
  trigger: string
  workspaceId: string | null
  workspaceDisplayName: string | null
  sessionKind: string
  chatSessionId: string | null
  terminalSessionId: string | null
  terminalPaneKey: string | null
  terminalPtyId: string | null
  outputSnapshotJson: string | null
  precheckResultJson: string | null
  usageJson: string | null
  error: string | null
  runNumber: number | null
  startedAt: number | null
  dispatchedAt: number | null
  createdAt: number
}

export const AUTOMATION_COLUMNS = `id, name, prompt, precheck_json as precheckJson,
  agent_id as agentId, run_context_json as runContextJson,
  source_context_json as sourceContextJson, project_id as projectId,
  execution_target_type as executionTargetType, execution_target_id as executionTargetId,
  scheduler_owner as schedulerOwner, workspace_mode as workspaceMode,
  workspace_id as workspaceId, base_branch as baseBranch,
  setup_decision_json as setupDecisionJson, reuse_session as reuseSession,
  timezone, rrule, dtstart, enabled, next_run_at as nextRunAt,
  last_run_at as lastRunAt, missed_run_policy as missedRunPolicy,
  missed_run_grace_minutes as missedRunGraceMinutes,
  created_at as createdAt, updated_at as updatedAt`

export const AUTOMATION_RUN_COLUMNS = `id, automation_id as automationId,
  run_context_json as runContextJson, source_context_json as sourceContextJson,
  title, scheduled_for as scheduledFor, status, trigger, workspace_id as workspaceId,
  workspace_display_name as workspaceDisplayName, session_kind as sessionKind,
  chat_session_id as chatSessionId, terminal_session_id as terminalSessionId,
  terminal_pane_key as terminalPaneKey, terminal_pty_id as terminalPtyId,
  output_snapshot_json as outputSnapshotJson, precheck_result_json as precheckResultJson,
  usage_json as usageJson, error, run_number as runNumber,
  started_at as startedAt, dispatched_at as dispatchedAt, created_at as createdAt`

export function rowToAutomation(row: AutomationRow): Automation {
  return {
    id: row.id,
    name: row.name,
    prompt: row.prompt,
    precheck: row.precheckJson ? JSON.parse(row.precheckJson) : null,
    agentId: row.agentId as Automation['agentId'],
    runContext: row.runContextJson ? JSON.parse(row.runContextJson) : null,
    sourceContext: row.sourceContextJson ? JSON.parse(row.sourceContextJson) : null,
    projectId: row.projectId,
    executionTargetType: row.executionTargetType as Automation['executionTargetType'],
    executionTargetId: row.executionTargetId,
    schedulerOwner: row.schedulerOwner as Automation['schedulerOwner'],
    workspaceMode: row.workspaceMode as Automation['workspaceMode'],
    workspaceId: row.workspaceId,
    baseBranch: row.baseBranch,
    setupDecision: row.setupDecisionJson ? JSON.parse(row.setupDecisionJson) : undefined,
    reuseSession: Boolean(row.reuseSession),
    timezone: row.timezone,
    rrule: row.rrule,
    dtstart: row.dtstart,
    enabled: Boolean(row.enabled),
    nextRunAt: row.nextRunAt,
    lastRunAt: row.lastRunAt ?? undefined,
    missedRunPolicy: row.missedRunPolicy as Automation['missedRunPolicy'],
    missedRunGraceMinutes: row.missedRunGraceMinutes,
    createdAt: row.createdAt,
    updatedAt: row.updatedAt
  }
}

export function rowToAutomationRun(row: AutomationRunRow): AutomationRun {
  return {
    id: row.id,
    automationId: row.automationId,
    runContext: row.runContextJson ? JSON.parse(row.runContextJson) : null,
    sourceContext: row.sourceContextJson ? JSON.parse(row.sourceContextJson) : null,
    title: row.title,
    scheduledFor: row.scheduledFor,
    status: row.status as AutomationRun['status'],
    trigger: row.trigger as AutomationRunTrigger,
    workspaceId: row.workspaceId,
    workspaceDisplayName: row.workspaceDisplayName,
    sessionKind: 'terminal',
    chatSessionId: row.chatSessionId,
    terminalSessionId: row.terminalSessionId,
    terminalPaneKey: row.terminalPaneKey,
    terminalPtyId: row.terminalPtyId,
    outputSnapshot: row.outputSnapshotJson ? JSON.parse(row.outputSnapshotJson) : null,
    precheckResult: row.precheckResultJson ? JSON.parse(row.precheckResultJson) : null,
    usage: row.usageJson ? JSON.parse(row.usageJson) : null,
    error: row.error,
    startedAt: row.startedAt,
    dispatchedAt: row.dispatchedAt,
    createdAt: row.createdAt,
    runNumber: row.runNumber ?? undefined
  }
}
