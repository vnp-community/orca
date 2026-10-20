/**
 * Automatic Plan generation (CR-REQ-044): the `autoPlan` block of `request.planProposal`, the gateway flag that says it exists,
 * and the tolerant parser the Plan tab reads it with.
 *
 * The parser never throws: a gateway older than CR-REQ-044 has no `autoPlan`, and proto3 zero values may be omitted.
 *
 * @module shared/request-plan-auto
 */

import { safeEnum, safeNum, safeOptStr, safeStr } from './request-wire-scalars'

/** `request.flowStatus.features` entry of a gateway that generates the Plan on its own and serves `request.retryPlan`. */
export const REQUEST_PLAN_AUTO_FEATURE = 'request.planAuto'

/** `retryPlan` refused because the automatic path is switched off for this deployment or project: fall back to the manual flow. */
export const PLAN_AUTO_DISABLED_CODE = 'REQUEST_PLAN_AUTO_DISABLED'

export type PlanAutoState =
  | 'pending'
  | 'running'
  | 'committing'
  | 'done'
  | 'gave_up'
  | 'skipped'
  | 'superseded'
  | ''

/** `revision` is the job of a reviewer's change request (CR-REQ-045). */
export type PlanAutoOrigin = 'event' | 'retry' | 'revision' | ''

export type PlanRunStatus = 'running' | 'succeeded' | 'failed' | ''

export type PlanAutoView = {
  /** The automatic path is on for this Request's project; false means the manual Generate button is the way. */
  enabled: boolean
  /** Empty when the Request has no job (or is no longer planning). */
  state: PlanAutoState
  origin: PlanAutoOrigin
  /** 1-based number of the attempt in flight or next; 0 when there is no job. */
  attempt: number
  /** Total attempts allowed (the first one plus the extra ones). */
  maxAttempts: number
  reasonCode: string
  nextAttemptAt?: string
}

export type PlanRunView = {
  runId: string
  status: PlanRunStatus
  errorCode: string
  errorMessage: string
  engine: string
  /** `null` is a gateway that predates CR-REQ-044; it is not the same as `enabled: false`. */
  autoPlan: PlanAutoView | null
}

const AUTO_STATES = new Set<PlanAutoState>([
  'pending',
  'running',
  'committing',
  'done',
  'gave_up',
  'skipped',
  'superseded',
  ''
])
const AUTO_ORIGINS = new Set<PlanAutoOrigin>(['event', 'retry', 'revision', ''])
const RUN_STATUSES = new Set<PlanRunStatus>(['running', 'succeeded', 'failed', ''])

function asRecord(raw: unknown): Record<string, unknown> | null {
  return raw !== null && typeof raw === 'object' && !Array.isArray(raw)
    ? (raw as Record<string, unknown>)
    : null
}

function count(value: unknown): number {
  return Math.max(safeNum(value, 0), 0)
}

function parsePlanAutoView(raw: unknown): PlanAutoView | null {
  const o = asRecord(raw)
  if (!o) {
    return null
  }
  const nextAttemptAt = safeOptStr(o.nextAttemptAt)
  return {
    enabled: o.enabled === true,
    state: safeEnum(o.state, AUTO_STATES, ''),
    origin: safeEnum(o.origin, AUTO_ORIGINS, ''),
    attempt: count(o.attempt),
    maxAttempts: count(o.maxAttempts),
    reasonCode: safeStr(o.reasonCode),
    ...(nextAttemptAt ? { nextAttemptAt } : {})
  }
}

/** Parses the answer of `request.planProposal` down to what the Plan tab needs; never throws. */
export function parsePlanRunView(raw: unknown): PlanRunView {
  const o = asRecord(raw) ?? {}
  return {
    runId: safeStr(o.runId),
    status: safeEnum(o.status, RUN_STATUSES, ''),
    errorCode: safeStr(o.errorCode),
    errorMessage: safeStr(o.errorMessage),
    engine: safeStr(o.engine),
    autoPlan: parsePlanAutoView(o.autoPlan)
  }
}
