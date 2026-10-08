/**
 * useSolutionDecision — CR-REQ-020-04
 *
 * Orchestrates choose -> approve, reject (+ optional regenerate) and bare
 * regenerate. If the CONTRACT ever folds `choose` into `approve`, this is the
 * only place that changes.
 *
 * @module components/request/solution/useSolutionDecision
 */

import { useCallback, useRef, useState } from 'react'
import type { Result } from '../../../runtime/request-rpc-client'
import type { RequestRpcError } from '../../../../../shared/request-errors'
import type { Approval, Solution } from '../../../../../shared/request-types'
import { classifySolutionError } from './solution-error-classification'
import type { SolutionErrorClass } from './solution-error-classification'
import { clampFeedback } from './solution-view-model'
import { parseDecision } from '../../../../../shared/request-artifact-parsers'
import type { Decision } from '../../../../../shared/request-artifact-types'

export type SolutionDecisionActions = {
  choose: (p: { solutionId: string; optionId: string; comment?: string; rationale?: string }) => Promise<Result<unknown>>
  generate: (p?: { feedback?: string; idempotencyKey?: string }) => Promise<Result<unknown>>
  approve: (p: { approval: Approval; comment?: string; viewedImpactDigest?: string; acceptedFindingIds?: string[] }) => Promise<Result<unknown>>
  reject: (p: { approval: Approval; comment: string }) => Promise<Result<unknown>>
}

export type DecisionOutcome = {
  ok: boolean
  error?: RequestRpcError
  /** Set when `solution.choose` recorded a high-risk Decision that needs the second confirmation before approving. */
  needsConfirmation?: boolean
  decision?: Decision
}

export type ApproveExtras = { rationale?: string; viewedImpactDigest?: string; acceptedFindingIds?: string[] }

export type SolutionDecisionError = { errorClass: SolutionErrorClass; error: RequestRpcError }

export type UseSolutionDecisionParams = {
  solution: Solution
  approval: Approval | null
  actions: SolutionDecisionActions
  /** Refetch solutions/approvals (and the request) after any outcome that may change them. */
  onSettled: () => void
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null
}

export function useSolutionDecision({ solution, approval, actions, onSettled }: UseSolutionDecisionParams) {
  const [submitting, setSubmitting] = useState(false)
  const [failure, setFailure] = useState<SolutionDecisionError | null>(null)
  const [chosenButNotApproved, setChosenButNotApproved] = useState(false)
  const [conflicted, setConflicted] = useState(false)
  const [deniedApprovalIds, setDeniedApprovalIds] = useState<ReadonlySet<string>>(new Set())
  const busy = useRef(false)
  // Step-1 memory so a retry after a failed approve never calls choose twice.
  const choseRef = useRef<{ solutionId: string; optionId: string; digest?: string } | null>(null)

  const fail = useCallback(
    (error: RequestRpcError, forApproval?: Approval | null): DecisionOutcome => {
      const errorClass = classifySolutionError(error)
      setFailure({ errorClass, error })
      if (errorClass === 'conflict') {setConflicted(true)}
      if (errorClass === 'notApprover' && forApproval) {
        setDeniedApprovalIds((prev) => new Set(prev).add(forApproval.id))
      }
      if (errorClass === 'conflict' || errorClass === 'alreadyDecided' || errorClass === 'expired') {onSettled()}
      return { ok: false, error }
    },
    [onSettled]
  )

  const run = useCallback(async (fn: () => Promise<DecisionOutcome>): Promise<DecisionOutcome> => {
    // Why: block double submits synchronously; state alone lags a render behind.
    if (busy.current) {return { ok: false }}
    busy.current = true
    setSubmitting(true)
    setFailure(null)
    try {
      return await fn()
    } finally {
      busy.current = false
      setSubmitting(false)
    }
  }, [])

  const approveSelected = useCallback(
    (optionId?: string, comment?: string, extras?: ApproveExtras): Promise<DecisionOutcome> =>
      run(async () => {
        if (!approval) {return { ok: false }}
        let digest = approval.subjectDigest
        if (solution.kind === 'solution') {
          if (!optionId) {return { ok: false }}
          const done = choseRef.current
          const alreadyChosen = solution.chosenOptionId === optionId
          if (done && done.solutionId === solution.id && done.optionId === optionId) {
            digest = done.digest ?? digest
          } else if (!alreadyChosen) {
            const rationale = extras?.rationale?.trim()
            const res = await actions.choose({ solutionId: solution.id, optionId, ...(rationale ? { rationale } : {}) })
            if (!res.ok) {return fail(res.error, approval)}
            // choose returns the fresh digest that approve must echo back.
            const fresh = isRecord(res.value) && typeof res.value.approvalDigest === 'string' ? res.value.approvalDigest : undefined
            choseRef.current = { solutionId: solution.id, optionId, digest: fresh }
            digest = fresh ?? digest
            // Why: the Decision is written by `choose`; a high-risk one must be confirmed before approve.
            const recorded = isRecord(res.value) ? parseDecision(res.value.decision) : null
            if (recorded && recorded.riskLevel === 'high' && recorded.status !== 'effective') {
              return { ok: true, needsConfirmation: true, decision: recorded }
            }
          }
        }
        const res = await actions.approve({
          approval: { ...approval, subjectDigest: digest },
          comment,
          ...(extras?.viewedImpactDigest ? { viewedImpactDigest: extras.viewedImpactDigest } : {}),
          ...(extras?.acceptedFindingIds?.length ? { acceptedFindingIds: extras.acceptedFindingIds } : {})
        })
        if (!res.ok) {
          setChosenButNotApproved(solution.kind === 'solution')
          return fail(res.error, approval)
        }
        setChosenButNotApproved(false)
        setConflicted(false)
        choseRef.current = null
        onSettled()
        return { ok: true }
      }),
    [run, approval, solution, actions, fail, onSettled]
  )

  const reject = useCallback(
    (comment: string, regenerate: boolean): Promise<DecisionOutcome> =>
      run(async () => {
        if (!approval) {return { ok: false }}
        const res = await actions.reject({ approval, comment })
        if (!res.ok) {return fail(res.error, approval)}
        if (regenerate) {
          const gen = await actions.generate({
            feedback: clampFeedback(comment),
            idempotencyKey: crypto.randomUUID()
          })
          if (!gen.ok) {
            // The rejection itself succeeded: surface the regenerate failure without reopening the dialog.
            setFailure({ errorClass: classifySolutionError(gen.error), error: gen.error })
          }
        }
        onSettled()
        return { ok: true }
      }),
    [run, approval, actions, fail, onSettled]
  )

  const regenerate = useCallback(
    (): Promise<DecisionOutcome> =>
      run(async () => {
        const res = await actions.generate({ idempotencyKey: crypto.randomUUID() })
        if (!res.ok) {return fail(res.error)}
        onSettled()
        return { ok: true }
      }),
    [run, actions, fail, onSettled]
  )

  return {
    submitting,
    failure,
    chosenButNotApproved,
    conflicted,
    acknowledgeConflict: () => setConflicted(false),
    isDenied: (approvalId: string | undefined) => (approvalId ? deniedApprovalIds.has(approvalId) : false),
    approveSelected,
    reject,
    regenerate
  }
}

export type SolutionDecision = ReturnType<typeof useSolutionDecision>
