/**
 * plan-run-all-model.ts — FE-REQ-TASK-043-02
 *
 * When the Plan approval offers "Approve and run all steps", and what its confirmation dialog has to say.
 *
 * @module components/request/plan/plan-run-all-model
 */

import type { RequestFlowView } from '../../../../../shared/request-execution-types'
import type { Approval, OrcaRequest } from '../../../../../shared/request-types'

export type RunAllOfferInput = {
  request: Pick<OrcaRequest, 'executionMode'>
  approval: Approval | null
  flow: RequestFlowView | null
  /** The gateway lists `approval.runAll`; an older one would drop the choice silently. */
  supported: boolean
  /** The risk gate hides Approve for this person, so no variant of it may be offered. */
  approverHidden: boolean
  /** A refused run-all said the Request is pinned to step by step. */
  alreadyPinned: boolean
}

function isPlanOrTaskList(approval: Approval): boolean {
  // Why: the wire parser folds task_list into plan; the original kind survives in rawSubjectType.
  const kind = approval.rawSubjectType ?? approval.subjectType
  return kind === 'plan' || kind === 'task_list'
}

/**
 * Backend flow only: the built-in table has drifted from the backend (CR-REQ-038), and this action has consequences, so
 * it is never offered on the strength of a guess.
 */
export function canOfferRunAll(input: RunAllOfferInput): boolean {
  const { request, approval, flow, supported, approverHidden, alreadyPinned } = input
  return (
    supported &&
    !approverHidden &&
    !alreadyPinned &&
    approval?.status === 'pending' &&
    isPlanOrTaskList(approval) &&
    flow?.source === 'backend' &&
    flow.hasPhases &&
    request.executionMode !== 'continuous'
  )
}

export type RunAllDialogModel = { stepCount: number; mentionsPreDeploy: boolean }

export function runAllDialogModel(input: {
  stepCount: number
  flow: RequestFlowView | null
}): RunAllDialogModel {
  const { stepCount, flow } = input
  return {
    stepCount,
    mentionsPreDeploy:
      flow?.startGate === 'pre_deploy' || (flow?.executionGates.includes('pre_deploy') ?? false)
  }
}
