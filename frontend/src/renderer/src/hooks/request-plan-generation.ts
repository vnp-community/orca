/**
 * Two-phase request.generatePlan (propose, then commit) per CONTRACT-request-ui-api.
 *
 * `propose` starts a background AI run and returns its id; the proposal (or the run's error
 * code) is read with `request.planProposal`. Only then can `commit` write the Plan.
 *
 * @module hooks/request-plan-generation
 */

import { callRequestRpc } from '../runtime/request-rpc-client'
import { classifyRequestRpcError } from '../../../shared/request-errors'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import type { Result } from '../runtime/request-rpc-client'

type ProposeValue = {
  runId?: string
  status?: string
  errorCode?: string
  errorMessage?: string
  proposal?: unknown
  rawAiResponse?: string
  alreadyExists?: boolean
}

export const PLAN_RUN_POLL_INTERVAL_MS = 2000
// Why: the AI call plus its one corrective retry (the backend suite waits the same 240s).
export const PLAN_RUN_TIMEOUT_MS = 240_000
const MAX_CONSECUTIVE_POLL_FAILURES = 3

export type PlanGenerationOptions = {
  feedback?: string
  /** Seam for tests; defaults to a real timer. */
  sleep?: (ms: number) => Promise<void>
  now?: () => number
}

const realSleep = (ms: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, ms))

function hasProposal(v: ProposeValue | undefined): boolean {
  return v?.proposal !== undefined && v.proposal !== null
}

/** The run's failure as a normal request error, so callers classify it like any other. */
function runFailure(view: ProposeValue): Result<never> {
  const code = view.errorCode || 'REQUEST_INTERNAL'
  const detail = view.errorMessage ? ` ${view.errorMessage}` : ''
  return {
    ok: false,
    error: classifyRequestRpcError({
      code: 'runtime_error',
      message: `${code}:${detail}`.trimEnd()
    })
  }
}

async function waitForProposal(
  id: string,
  runId: string,
  { sleep = realSleep, now = Date.now }: PlanGenerationOptions
): Promise<Result<ProposeValue>> {
  const deadline = now() + PLAN_RUN_TIMEOUT_MS
  let failures = 0
  for (;;) {
    await sleep(PLAN_RUN_POLL_INTERVAL_MS)
    const polled = await callRequestRpc<ProposeValue>(REQUEST_RPC_METHODS.PLAN_PROPOSAL, {
      id,
      runId
    })
    if (!polled.ok) {
      failures += 1
      if (failures >= MAX_CONSECUTIVE_POLL_FAILURES) {
        return polled
      }
    } else {
      failures = 0
      const view = polled.value
      if (view.status === 'failed') {
        return runFailure(view)
      }
      if (view.status !== 'running' && hasProposal(view)) {
        return polled
      }
      if (view.status === 'succeeded') {
        return runFailure({
          errorCode: 'REQUEST_PLAN_AI_INVALID_JSON',
          errorMessage: 'the run returned no proposal'
        })
      }
    }
    if (now() >= deadline) {
      return runFailure({
        errorCode: 'REQUEST_AI_COMPLETE_TIMEOUT',
        errorMessage: 'the plan run did not finish in time'
      })
    }
  }
}

// Why: `propose` writes nothing and `commit` requires the proposal, so a one-click
// "generate plan" must run both; an existing plan or a failed run stops after propose.
export async function generatePlanProposeCommit(
  id: string,
  options: PlanGenerationOptions = {}
): Promise<Result<unknown>> {
  const proposed = await callRequestRpc<ProposeValue>(REQUEST_RPC_METHODS.GENERATE_PLAN, {
    id,
    mode: 'propose',
    ...(options.feedback ? { feedback: options.feedback } : {})
  })
  if (!proposed.ok || proposed.value?.alreadyExists) {
    return proposed
  }
  let ready: Result<ProposeValue> = proposed
  if (!hasProposal(proposed.value)) {
    // Servers before the background run answered with the proposal itself; none and no run id is a dead end.
    if (!proposed.value?.runId) {
      return proposed
    }
    ready = await waitForProposal(id, proposed.value.runId, options)
    if (!ready.ok) {
      return ready
    }
  }
  return callRequestRpc(
    REQUEST_RPC_METHODS.GENERATE_PLAN,
    {
      id,
      mode: 'commit',
      proposal: ready.value.proposal,
      ...(ready.value.rawAiResponse ? { rawAiResponse: ready.value.rawAiResponse } : {})
    },
    // Why: commit answers `alreadyExists` for a repeat, so it is safe to retry after a throttle.
    { idempotent: true }
  )
}
