/**
 * usePlanDecision — CR-REQ-021-04
 *
 * Approve / reject Plan, Phase and pre_deploy gates, regenerate the Plan and start a
 * Phase. Double-submit is blocked per approval id / phase id.
 *
 * @module hooks/usePlanDecision
 */

import { useCallback, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { generatePlanProposeCommit } from './request-plan-generation'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import {
  classifySolutionError,
  type SolutionErrorClass
} from '../components/request/solution/solution-error-classification'
import { validateRejectReason } from '../components/request/solution/solution-view-model'
import type { RequestRpcError } from '../../../shared/request-errors'
import type { Result } from '../runtime/request-rpc-client'
import type { Approval, OrcaRequest } from '../../../shared/request-types'

export type PlanDecisionFailure = {
  /** Approval id / phase task id / 'plan' the failure belongs to. */
  scope: string
  errorClass: SolutionErrorClass
  error: RequestRpcError
}

export type PlanDecisionOutcome = { ok: boolean; error?: RequestRpcError }

const IN_FLIGHT: RequestRpcError = { kind: 'unknown', code: 'IN_FLIGHT', message: 'In flight' }

export function usePlanDecision(request: Pick<OrcaRequest, 'id'>, onSettled: () => void) {
  const [busyKeys, setBusyKeys] = useState<ReadonlySet<string>>(new Set())
  const [failure, setFailure] = useState<PlanDecisionFailure | null>(null)
  const [deniedApprovalIds, setDeniedApprovalIds] = useState<ReadonlySet<string>>(new Set())
  const inFlight = useRef(new Set<string>())

  const run = useCallback(
    async (
      key: string,
      scope: string,
      call: () => Promise<Result<unknown>>,
      forApproval?: Approval
    ) => {
      // Why: refs block a second click synchronously; state lags one render behind.
      if (inFlight.current.has(key)) {
        return { ok: false, error: IN_FLIGHT } as PlanDecisionOutcome
      }
      inFlight.current.add(key)
      setBusyKeys(new Set(inFlight.current))
      setFailure(null)
      try {
        const res = await call()
        if (!res.ok) {
          const errorClass = classifySolutionError(res.error)
          setFailure({ scope, errorClass, error: res.error })
          if ((errorClass === 'notApprover' || errorClass === 'forbidden') && forApproval) {
            setDeniedApprovalIds((prev) => new Set(prev).add(forApproval.id))
          }
          if (
            errorClass === 'conflict' ||
            errorClass === 'alreadyDecided' ||
            errorClass === 'expired'
          ) {
            onSettled()
          }
          return { ok: false, error: res.error } as PlanDecisionOutcome
        }
        onSettled()
        return { ok: true } as PlanDecisionOutcome
      } finally {
        inFlight.current.delete(key)
        setBusyKeys(new Set(inFlight.current))
      }
    },
    [onSettled]
  )

  const approve = useCallback(
    (approval: Approval, comment?: string) =>
      run(
        `approve:${approval.id}`,
        approval.id,
        () =>
          callRequestRpc(REQUEST_RPC_METHODS.APPROVAL_APPROVE, {
            approvalId: approval.id,
            expectedVersion: approval.version,
            expectedDigest: approval.subjectDigest,
            comment
          }),
        approval
      ),
    [run]
  )

  const regeneratePlan = useCallback(
    () =>
      run('generatePlan', 'plan', () =>
        generatePlanProposeCommit(request.id)
      ),
    [run, request.id]
  )

  const reject = useCallback(
    async (
      approval: Approval,
      comment: string,
      options?: { regenerate?: boolean }
    ): Promise<PlanDecisionOutcome> => {
      // Why: a rejection without a reason must never reach the backend.
      if (!validateRejectReason(comment).ok) {
        return {
          ok: false,
          error: {
            kind: 'validation',
            code: 'APPROVAL_COMMENT_REQUIRED',
            message: 'Reason required'
          }
        }
      }
      const res = await run(
        `reject:${approval.id}`,
        approval.id,
        () =>
          callRequestRpc(REQUEST_RPC_METHODS.APPROVAL_REJECT, {
            approvalId: approval.id,
            expectedVersion: approval.version,
            expectedDigest: approval.subjectDigest,
            comment
          }),
        approval
      )
      // Regenerate failure must not reopen the dialog: the rejection itself succeeded.
      if (res.ok && options?.regenerate) {
        await regeneratePlan()
      }
      return res
    },
    [run, regeneratePlan]
  )

  const startPhase = useCallback(
    (phaseTaskId: string) =>
      run(`startPhase:${phaseTaskId}`, phaseTaskId, () =>
        callRequestRpc(REQUEST_RPC_METHODS.START_PHASE, { id: request.id, phaseTaskId })
      ),
    [run, request.id]
  )

  return {
    busyKeys,
    isBusy: (key: string) => busyKeys.has(key),
    failure,
    clearFailure: () => setFailure(null),
    isDenied: (approvalId: string | undefined) =>
      approvalId ? deniedApprovalIds.has(approvalId) : false,
    approve,
    reject,
    regeneratePlan,
    startPhase
  }
}

export type PlanDecision = ReturnType<typeof usePlanDecision>
