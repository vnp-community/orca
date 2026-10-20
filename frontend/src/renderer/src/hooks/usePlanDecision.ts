/**
 * usePlanDecision — CR-REQ-021-04
 *
 * Approve / reject Plan, Phase and pre_deploy gates, generate the first Plan and start a
 * Phase. Changing a Plan that exists is usePlanRevision's job (CR-REQ-045), not a rejection. Double-submit is blocked per approval id / phase id.
 *
 * @module hooks/usePlanDecision
 */

import { useCallback, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { generatePlanProposeCommit } from './request-plan-generation'
import { useRequestCooldown } from './useRequestCooldown'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import {
  isRunAllAlreadyPinned,
  isRunAllProgrammingError,
  parseApproveResult,
  RUN_ALL_EXECUTION_MODE
} from '../../../shared/request-run-all'
import {
  backendErrorCode,
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

export type PlanDecisionOutcome = {
  ok: boolean
  error?: RequestRpcError
  /** Only when run-all was asked for: whether the server pinned `continuous`. False = a gateway that dropped the field. */
  runAllApplied?: boolean
}

export type ApproveOptions = { executionMode?: typeof RUN_ALL_EXECUTION_MODE }

const IN_FLIGHT: RequestRpcError = { kind: 'unknown', code: 'IN_FLIGHT', message: 'In flight' }

export function usePlanDecision(request: Pick<OrcaRequest, 'id'>, onSettled: () => void) {
  const [busyKeys, setBusyKeys] = useState<ReadonlySet<string>>(new Set())
  const [failure, setFailure] = useState<PlanDecisionFailure | null>(null)
  const [deniedApprovalIds, setDeniedApprovalIds] = useState<ReadonlySet<string>>(new Set())
  // Why: a refused run-all means the Request is pinned to step by step; remember it across refreshes so the choice stays hidden.
  const [runAllPinnedIds, setRunAllPinnedIds] = useState<ReadonlySet<string>>(new Set())
  const inFlight = useRef(new Set<string>())
  // Why: the limiter told us to wait; gate buttons read these instead of firing into it.
  const writeCooldown = useRequestCooldown('write')
  const aiCooldown = useRequestCooldown('ai')

  const run = useCallback(
    async (
      key: string,
      scope: string,
      call: () => Promise<Result<unknown>>,
      forApproval?: Approval,
      outcomeOf?: (value: unknown) => Partial<PlanDecisionOutcome>
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
          if (isRunAllAlreadyPinned(res.error)) {
            setRunAllPinnedIds((prev) => new Set(prev).add(request.id))
          }
          if (isRunAllProgrammingError(res.error)) {
            // Why: the UI offered run-all where it does not apply; log the code only, never request content.
            console.warn('[plan-decision] run-all refused:', backendErrorCode(res.error))
          }
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
        return { ok: true, ...outcomeOf?.(res.value) } as PlanDecisionOutcome
      } finally {
        inFlight.current.delete(key)
        setBusyKeys(new Set(inFlight.current))
      }
    },
    [onSettled, request.id]
  )

  const approve = useCallback(
    (
      approval: Approval,
      comment?: string,
      risk?: { viewedImpactDigest?: string; acceptedFindingIds?: string[] },
      options?: ApproveOptions
    ) =>
      run(
        `approve:${approval.id}`,
        approval.id,
        () =>
          callRequestRpc(REQUEST_RPC_METHODS.APPROVAL_APPROVE, {
            // Why: CONTRACT 2.3 keys the approval as `id`; `approvalId` kept like useApprovals for older gateways.
            id: approval.id,
            approvalId: approval.id,
            expectedVersion: approval.version,
            expectedDigest: approval.subjectDigest,
            comment,
            // Why: CR-030 risk gate fields (camelCase names are provisional); omitted when empty.
            ...(risk?.viewedImpactDigest ? { viewedImpactDigest: risk.viewedImpactDigest } : {}),
            ...(risk?.acceptedFindingIds?.length
              ? { acceptedFindingIds: risk.acceptedFindingIds }
              : {}),
            // Why: CR-REQ-043; sent only when asked, so a plain approval keeps its exact arguments.
            ...(options?.executionMode ? { executionMode: options.executionMode } : {})
          }),
        approval,
        options?.executionMode
          ? (value) => ({
              runAllApplied: parseApproveResult(value).executionMode === RUN_ALL_EXECUTION_MODE
            })
          : undefined
      ),
    [run]
  )

  const regeneratePlan = useCallback(
    () => run('generatePlan', 'plan', () => generatePlanProposeCommit(request.id)),
    [run, request.id]
  )

  const reject = useCallback(
    async (approval: Approval, comment: string): Promise<PlanDecisionOutcome> => {
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
      return run(
        `reject:${approval.id}`,
        approval.id,
        () =>
          callRequestRpc(REQUEST_RPC_METHODS.APPROVAL_REJECT, {
            id: approval.id,
            approvalId: approval.id,
            expectedVersion: approval.version,
            expectedDigest: approval.subjectDigest,
            comment
          }),
        approval
      )
    },
    [run]
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
    /** True while approve/reject/start-phase calls are being throttled. */
    coolingDown: writeCooldown.coolingDown,
    /** True while plan generation (an AI call) is being throttled. */
    generateCoolingDown: aiCooldown.coolingDown,
    isBusy: (key: string) => busyKeys.has(key),
    failure,
    clearFailure: () => setFailure(null),
    /** True once a run-all approval of this Request was refused because it is pinned to step by step. */
    runAllPinned: runAllPinnedIds.has(request.id),
    isDenied: (approvalId: string | undefined) =>
      approvalId ? deniedApprovalIds.has(approvalId) : false,
    approve,
    reject,
    regeneratePlan,
    startPhase
  }
}

export type PlanDecision = ReturnType<typeof usePlanDecision>
