/**
 * Parsers of the execution summary and the activity feed — FE-REQ-TASK-037-02
 *
 * Tolerant like every request parser: a field the server leaves out gets a neutral value, an enum it adds later is kept or
 * turned into 'unknown', and nothing throws.
 *
 * @module shared/request-execution-parsers
 */

import type {
  ActivityActorKind,
  ActivitySeverity,
  ExecutionFailedTask,
  ExecutionHandover,
  ExecutionNeedsYou,
  ExecutionProgress,
  ExecutionRunningTask,
  ExecutionSummary,
  FailureCause,
  HandoverState,
  NeedsYouKind,
  RequestActivityEntry,
  RequestActivityPage
} from './request-execution-types'
import type { RequestStatus } from './request-types'

const FAILURE_CAUSES = new Set<FailureCause>([
  'timed_out',
  'model_error',
  'verify_failed',
  'env_defect',
  'spec_defect',
  'agent_error',
  'needs_info',
  'cancelled',
  'unknown'
])
const NEEDS_YOU_KINDS = new Set<NeedsYouKind>([
  'approval',
  'clarification',
  'failed_task',
  'pre_deploy',
  'drift'
])
const HANDOVER_STATES = new Set<HandoverState>(['none', 'archiving', 'ready', 'failed'])
const SEVERITIES = new Set<ActivitySeverity>(['info', 'attention', 'error'])
const ACTORS = new Set<ActivityActorKind>(['user', 'ai', 'agent', 'system'])

function obj(raw: unknown): Record<string, unknown> {
  return raw && typeof raw === 'object' && !Array.isArray(raw)
    ? (raw as Record<string, unknown>)
    : {}
}
function str(v: unknown): string {
  return typeof v === 'string' ? v : ''
}
function optStr(v: unknown): string | undefined {
  return typeof v === 'string' && v !== '' ? v : undefined
}
function num(v: unknown): number {
  return typeof v === 'number' && Number.isFinite(v) ? v : 0
}
function list<T>(v: unknown, map: (x: unknown) => T): T[] {
  return Array.isArray(v) ? v.map(map) : []
}

function parseProgress(raw: unknown): ExecutionProgress {
  const r = obj(raw)
  return {
    phasesTotal: num(r.phasesTotal),
    phasesDone: num(r.phasesDone),
    tasksTotal: num(r.tasksTotal),
    tasksDone: num(r.tasksDone),
    tasksFailed: num(r.tasksFailed),
    tasksRunning: num(r.tasksRunning)
  }
}

function parseRunning(raw: unknown): ExecutionRunningTask {
  const r = obj(raw)
  return {
    taskId: str(r.taskId),
    phaseId: optStr(r.phaseId),
    title: str(r.title),
    attempt: Math.max(1, num(r.attempt)),
    maxAttempts: num(r.maxAttempts),
    startedAt: optStr(r.startedAt)
  }
}

function parseFailed(raw: unknown): ExecutionFailedTask {
  const r = obj(raw)
  return {
    taskId: str(r.taskId),
    phaseId: optStr(r.phaseId),
    title: str(r.title),
    attempts: num(r.attempts),
    maxAttempts: num(r.maxAttempts),
    lastCause: FAILURE_CAUSES.has(r.lastCause as FailureCause)
      ? (r.lastCause as FailureCause)
      : 'unknown',
    lastError: optStr(r.lastError),
    lastAt: optStr(r.lastAt)
  }
}

function parseNeedsYou(raw: unknown): ExecutionNeedsYou {
  const r = obj(raw)
  return {
    kind: NEEDS_YOU_KINDS.has(r.kind as NeedsYouKind) ? (r.kind as NeedsYouKind) : 'unknown',
    refId: str(r.refId),
    subjectType: optStr(r.subjectType),
    title: optStr(r.title),
    phaseId: optStr(r.phaseId),
    since: optStr(r.since),
    dueAt: optStr(r.dueAt)
  }
}

function parseHandover(raw: unknown): ExecutionHandover {
  const r = obj(raw)
  return {
    state: HANDOVER_STATES.has(r.state as HandoverState) ? (r.state as HandoverState) : 'none',
    archiveAttempts: num(r.archiveAttempts),
    summaryRef: optStr(r.summaryRef)
  }
}

/** Parses the `summary` of `request.executionSummary`; null when there is none. */
export function parseExecutionSummary(raw: unknown): ExecutionSummary | null {
  const r = obj(raw)
  if (typeof r.requestId !== 'string' || r.requestId === '') {
    return null
  }
  return {
    requestId: r.requestId,
    status: (typeof r.status === 'string' ? r.status : 'unknown') as RequestStatus,
    engine: r.engine === 'native' || r.engine === 'openspec' ? r.engine : undefined,
    executionMode:
      r.executionMode === 'step_by_step' || r.executionMode === 'continuous'
        ? r.executionMode
        : undefined,
    progress: parseProgress(r.progress),
    currentPhaseId: optStr(r.currentPhaseId),
    nextPhaseId: optStr(r.nextPhaseId),
    running: list(r.running, parseRunning).filter((t) => t.taskId !== ''),
    failed: list(r.failed, parseFailed).filter((t) => t.taskId !== ''),
    needsYou: list(r.needsYou, parseNeedsYou).filter((n) => n.refId !== ''),
    handover: parseHandover(r.handover),
    maxAttempts: num(r.maxAttempts),
    startedAt: optStr(r.startedAt)
  }
}

function parseActivityEntry(raw: unknown): RequestActivityEntry {
  const r = obj(raw)
  const attempt = num(r.attempt)
  return {
    id: str(r.id),
    kind: str(r.kind),
    occurredAt: str(r.occurredAt),
    actorKind: ACTORS.has(r.actorKind as ActivityActorKind)
      ? (r.actorKind as ActivityActorKind)
      : 'system',
    actorId: optStr(r.actorId),
    subjectType: optStr(r.subjectType),
    subjectId: optStr(r.subjectId),
    summary: str(r.summary),
    attempt: attempt > 0 ? attempt : undefined,
    severity: SEVERITIES.has(r.severity as ActivitySeverity)
      ? (r.severity as ActivitySeverity)
      : 'info',
    details: obj(r.details)
  }
}

/** Parses the answer of `request.activity`. */
export function parseActivityPage(raw: unknown): RequestActivityPage {
  const r = obj(raw)
  return {
    entries: list(r.entries, parseActivityEntry).filter((e) => e.id !== '' && e.kind !== ''),
    nextPageToken: optStr(r.nextPageToken)
  }
}
