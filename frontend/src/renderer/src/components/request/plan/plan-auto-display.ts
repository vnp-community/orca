/**
 * plan-auto-display.ts — CR-REQ-044
 *
 * Pure decision of what the Plan tab says about the Plan being written, from the Request and the latest Plan run: derived, not
 * remembered, so a reloaded page and a run started from another tab read the same.
 *
 * @module components/request/plan/plan-auto-display
 */

import type { PlanAutoState, PlanRunView } from '../../../../../shared/request-plan-auto'
import type { OrcaRequest } from '../../../../../shared/request-types'

export type PlanAutoDisplay =
  | { kind: 'none' }
  | {
      kind: 'writing'
      attempt: number
      maxAttempts: number
      /** A failed attempt is waiting for its turn to run again. */
      waiting: boolean
      /** The Request already has a Plan: this run rewrites it. */
      replan: boolean
      /** Nothing has started yet; the server is expected to start in a moment. */
      starting: boolean
    }
  | { kind: 'failed'; code: string; viaServer: boolean }
  | { kind: 'skipped'; reasonCode: string }

export type PlanAutoDisplayInput = {
  request: Pick<OrcaRequest, 'status' | 'planTaskId'>
  /** The latest Plan run as read from `request.planProposal`; null when there is none (or the read failed). */
  run: PlanRunView | null
  supportsPlanAuto: boolean
  /** The start grace period ran out with nothing started (see usePlanRunState). */
  graceElapsed: boolean
}

/** Job states in which the server is still working on the Plan. */
const ACTIVE_STATES: ReadonlySet<PlanAutoState> = new Set(['pending', 'running', 'committing'])

export function isPlanRunBusy(run: PlanRunView | null): boolean {
  return run?.status === 'running' || (!!run?.autoPlan && ACTIVE_STATES.has(run.autoPlan.state))
}

/** The same code the manual flow reports when a run fails without one. */
const UNKNOWN_FAILURE_CODE = 'REQUEST_INTERNAL'

export function derivePlanAutoDisplay({
  request,
  run,
  supportsPlanAuto,
  graceElapsed
}: PlanAutoDisplayInput): PlanAutoDisplay {
  if (request.status !== 'planning') {
    return { kind: 'none' }
  }
  // Why: a gateway without the flag must keep the old behaviour even if a block slipped into the answer.
  const auto = supportsPlanAuto ? (run?.autoPlan ?? null) : null
  const replan = !!request.planTaskId

  if (isPlanRunBusy(run)) {
    const attempt = auto && auto.attempt > 0 ? auto.attempt : 1
    return {
      kind: 'writing',
      attempt,
      maxAttempts: auto?.maxAttempts ?? 0,
      waiting: auto?.state === 'pending' && attempt > 1,
      replan,
      starting: false
    }
  }
  if (auto?.state === 'gave_up' || run?.status === 'failed') {
    return {
      kind: 'failed',
      code:
        (auto?.state === 'gave_up' && auto.reasonCode) ||
        run?.errorCode ||
        auto?.reasonCode ||
        UNKNOWN_FAILURE_CODE,
      viaServer: auto?.enabled === true
    }
  }
  if (auto?.state === 'skipped') {
    return { kind: 'skipped', reasonCode: auto.reasonCode }
  }
  if (auto?.enabled && auto.state === '' && !run?.runId && !run?.status && !graceElapsed) {
    return { kind: 'writing', attempt: 1, maxAttempts: 0, waiting: false, replan, starting: true }
  }
  return { kind: 'none' }
}
