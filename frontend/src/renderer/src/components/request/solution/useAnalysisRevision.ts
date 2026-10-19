/**
 * useAnalysisRevision — CR-REQ-041
 *
 * "Revise the analysis by prompt": at the Analysis gate an approver types what should change and the AI writes the next
 * version. It calls the same `solution.generate` action as Regenerate, with the typed text as feedback, and never
 * rejects the version on screen (a light edit is not a rejection).
 *
 * The AI turn runs in the background, so `busy` outlives the call: it lasts until the Request has gone through
 * `analyzing` and is back (or has moved on), judged from the Request itself, not from the call returning.
 *
 * @module components/request/solution/useAnalysisRevision
 */

import { useCallback, useRef, useState } from 'react'
import { createBrowserUuid } from '@/lib/browser-uuid'
import { REQUEST_POLL_EVENT_TYPE } from '../../../hooks/useRequestEvents'
import { useRequestSubscription } from '../../../hooks/useRequestSubscription'
import { REQUEST_FLOW_REGISTRY } from '../../../../../shared/request-flow-registry'
import type { OrcaRequest, Solution } from '../../../../../shared/request-types'
import {
  analysisRevisionMessage,
  classifyAnalysisRevisionError,
  type AnalysisRevisionError
} from './analysis-revision-errors'
import { FEEDBACK_MAX_LENGTH } from './solution-view-model'
import type { SolutionDecisionActions } from './useSolutionDecision'

// Same shape SolutionPanel refetches on: the revision is finished by a status change or a new Solution/Approval.
const PROGRESS_EVENT_RE = /(?:^|\.)(?:solution\.[a-z_]+|approval\.[a-z_]+|request\.status_changed)$/

export type UseAnalysisRevisionParams = {
  // version and updatedAt are how "the Request moved on" is told; both may be missing on an older gateway.
  request: Pick<OrcaRequest, 'id' | 'type' | 'status'> &
    Partial<Pick<OrcaRequest, 'version' | 'updatedAt'>>
  /** The version on screen. Only a version still waiting for approval can be revised. */
  solution: Pick<Solution, 'id' | 'status' | 'seq'> | undefined
  actions: Pick<SolutionDecisionActions, 'generate'>
  /** Re-reads the Request, the Solutions and the Approvals. */
  onSettled: () => void
}

export type AnalysisRevisionOutcome = { ok: true } | { ok: false; error: AnalysisRevisionError }

// What the Request looked like when the revision was accepted: busy lasts while it has not moved past that.
type Submitted = {
  requestId: string
  version: number | undefined
  updatedAt: string | undefined
  solutionId: string
  fromSeq: number | undefined
}

function hasAnalysisGate(type: OrcaRequest['type']): boolean {
  if (type === 'unknown') {
    return false
  }
  return REQUEST_FLOW_REGISTRY[type].analysisGate !== null
}

function stillRevising(
  submitted: Submitted | null,
  request: UseAnalysisRevisionParams['request'],
  solution: UseAnalysisRevisionParams['solution']
): boolean {
  if (!submitted || submitted.requestId !== request.id) {
    return false
  }
  if (request.status === 'analyzing') {
    return true
  }
  if (request.status !== 'awaiting_analysis_approval') {
    return false
  }
  // Back at the gate with nothing changed: the Request has not even entered `analyzing` on screen yet. A changed
  // version, timestamp or current Solution means a round has come and gone, so a missed `analyzing` cannot hang the panel.
  return (
    request.version === submitted.version &&
    request.updatedAt === submitted.updatedAt &&
    solution?.id === submitted.solutionId
  )
}

export function useAnalysisRevision({
  request,
  solution,
  actions,
  onSettled
}: UseAnalysisRevisionParams) {
  const [submitted, setSubmitted] = useState<Submitted | null>(null)
  const [calling, setCalling] = useState(false)
  const [error, setError] = useState<AnalysisRevisionError | null>(null)
  const [draftState, setDraftState] = useState<{ requestId: string; text: string } | null>(null)
  // Why: a second click must not start a second AI turn; state alone lags a render behind.
  const inFlight = useRef(false)

  const draft = draftState?.requestId === request.id ? draftState.text : ''
  const revising = stillRevising(submitted, request, solution)
  const busy = calling || revising
  const canShow =
    hasAnalysisGate(request.type) &&
    request.status === 'awaiting_analysis_approval' &&
    solution?.status === 'ready'
  const nextVersion =
    revising && submitted?.fromSeq !== undefined ? submitted.fromSeq + 1 : undefined

  useRequestSubscription({
    requestId: request.id,
    onEvent: (event) => {
      if (
        revising &&
        (PROGRESS_EVENT_RE.test(event.eventType) || event.eventType === REQUEST_POLL_EVENT_TYPE)
      ) {
        onSettled()
      }
    }
  })

  const clearError = useCallback(() => setError(null), [])
  const setDraft = useCallback(
    (text: string) => {
      setDraftState({ requestId: request.id, text })
      setError(null)
    },
    [request.id]
  )

  const submit = useCallback(
    async (text?: string): Promise<AnalysisRevisionOutcome> => {
      const fail = (failure: AnalysisRevisionError): AnalysisRevisionOutcome => {
        setError(failure)
        return { ok: false, error: failure }
      }
      if (inFlight.current || busy) {
        return { ok: false, error: { kind: 'wrongState', message: '' } }
      }
      const feedback = (text ?? draft).trim()
      if (feedback === '') {
        return fail({ kind: 'empty', message: analysisRevisionMessage('empty') ?? '' })
      }
      if ([...feedback].length > FEEDBACK_MAX_LENGTH) {
        return fail({ kind: 'tooLong', message: analysisRevisionMessage('tooLong') ?? '' })
      }
      if (!canShow || !solution) {
        return fail({ kind: 'wrongState', message: analysisRevisionMessage('wrongState') ?? '' })
      }

      inFlight.current = true
      setCalling(true)
      setError(null)
      try {
        // A fresh key per submit: a retry after a failure is a new request for a new version, not a replay.
        const result = await actions.generate({ feedback, idempotencyKey: createBrowserUuid() })
        if (!result.ok) {
          const failure = classifyAnalysisRevisionError(result.error)
          // The text stays in the box: nothing here clears the draft on a failure.
          if (failure.kind === 'wrongState') {
            onSettled()
          }
          return fail(failure)
        }
        setSubmitted({
          requestId: request.id,
          version: request.version,
          updatedAt: request.updatedAt,
          solutionId: solution.id,
          fromSeq: solution.seq
        })
        setDraftState({ requestId: request.id, text: '' })
        onSettled()
        return { ok: true }
      } catch (thrown) {
        // The action wrappers return a Result, so this is a bug or a broken transport: report it, keep the text.
        console.error('[useAnalysisRevision] solution.generate threw', thrown)
        return fail(
          classifyAnalysisRevisionError({
            kind: 'unknown',
            code: 'unknown',
            message: thrown instanceof Error ? thrown.message : ''
          })
        )
      } finally {
        inFlight.current = false
        setCalling(false)
      }
    },
    [
      actions,
      busy,
      canShow,
      draft,
      onSettled,
      request.id,
      request.updatedAt,
      request.version,
      solution
    ]
  )

  return {
    /** The panel belongs on screen: a revisable version at the Analysis gate of a flow that has one. */
    canShow,
    /** A call is in flight, or the AI is writing the next version. Approve, Revise, Choose and Reject stay locked. */
    busy,
    /** The version being written (n+1) while busy, when the current one carries its number. */
    nextVersion,
    draft,
    setDraft,
    error,
    clearError,
    submit
  }
}

export type AnalysisRevision = ReturnType<typeof useAnalysisRevision>
