/**
 * usePlanRevision — CR-REQ-045
 *
 * "Revise the plan by prompt": at the Plan gate a reviewer types what should change and the AI writes the next version of the
 * plan. It calls `request.generatePlan` with the text as feedback and never rejects the plan on screen (a rejection sends the
 * Request back to the backlog).
 *
 * The AI turn runs in the background, so `busy` outlives the call. It is judged from the Request and from the server's run
 * state (usePlanRunState, shared with the rest of the tab), never from this component remembering that it clicked, so a
 * reloaded page and another reviewer's tab agree.
 *
 * @module components/request/plan/revision/usePlanRevision
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { createBrowserUuid } from '@/lib/browser-uuid'
import { REQUEST_POLL_EVENT_TYPE } from '../../../../hooks/useRequestEvents'
import { useRequestSubscription } from '../../../../hooks/useRequestSubscription'
import {
  PLAN_RUN_OPENSPEC_TIMEOUT_MS,
  PLAN_RUN_TIMEOUT_MS,
  generatePlanPropose,
  generatePlanProposeCommit
} from '../../../../hooks/request-plan-generation'
import type { PlanRunView } from '../../../../../../shared/request-plan-auto'
import type { PlanKind } from '../../../../../../shared/request-flow-registry'
import type { OrcaRequest } from '../../../../../../shared/request-types'
import { FEEDBACK_MAX_LENGTH } from '../../solution/solution-view-model'
import type { PlanAutoDisplay } from '../plan-auto-display'
import {
  classifyPlanRevisionError,
  planRevisionMessage,
  type PlanRevisionError
} from './plan-revision-errors'

// Same shape the version list refreshes on: a revision ends with a status change, a new Plan or a new Approval.
const PROGRESS_EVENT_RE = /(?:^|\.)(?:plan\.[a-z_]+|approval\.[a-z_]+|request\.status_changed)$/

export type UsePlanRevisionParams = {
  request: Pick<OrcaRequest, 'id' | 'status' | 'planTaskId' | 'engine'> &
    Partial<Pick<OrcaRequest, 'version' | 'updatedAt'>>
  /** The kind of plan the flow of this type produces; only a plan or a task list can be revised. */
  flowPlanKind: PlanKind | undefined
  /** The server saves the revised plan (the gateway has `request.planAuto`); otherwise this tab saves it. */
  serverCommit: boolean
  /** What usePlanAutoGeneration reports about the server's Plan run. */
  runState: {
    display: PlanAutoDisplay
    busy: boolean
    run: PlanRunView | null
    refresh: () => void
  }
  /** Number of the version the gate is about; the one being written is the next. */
  currentRevision: number | undefined
  /** The reader is not looking at an older version. */
  viewingCurrent: boolean
  /** Re-reads the Request, the Plan and the Approvals. */
  onSettled: () => void
}

export type PlanRevisionOutcome = { ok: true } | { ok: false; error: PlanRevisionError }

// What the Request looked like when the revision was accepted: busy lasts while it has not moved past that.
type Submitted = {
  requestId: string
  version: number | undefined
  updatedAt: string | undefined
  planTaskId: string | undefined
  /** The run the server reported before this one, so its failure is not mistaken for the new run's. */
  priorRunId: string
  at: number
}

function hasRevisablePlan(kind: PlanKind | undefined): boolean {
  return kind === 'plan' || kind === 'task_list'
}

function stillRevising(
  submitted: Submitted | null,
  request: UsePlanRevisionParams['request'],
  runState: UsePlanRevisionParams['runState']
): boolean {
  if (!submitted || submitted.requestId !== request.id) {
    return false
  }
  if (request.status === 'planning') {
    const failure = runState.display.kind === 'failed'
    return !(failure && (runState.run?.runId ?? '') !== submitted.priorRunId)
  }
  if (request.status !== 'awaiting_plan_approval') {
    return false
  }
  // Back at the gate with nothing changed: the Request has not even entered `planning` on screen yet. A changed version,
  // timestamp or Plan means a round has come and gone, so a missed `planning` cannot hang the panel.
  return (
    request.version === submitted.version &&
    request.updatedAt === submitted.updatedAt &&
    request.planTaskId === submitted.planTaskId
  )
}

export function usePlanRevision({
  request,
  flowPlanKind,
  serverCommit,
  runState,
  currentRevision,
  viewingCurrent,
  onSettled
}: UsePlanRevisionParams) {
  const [submitted, setSubmitted] = useState<Submitted | null>(null)
  const [calling, setCalling] = useState(false)
  const [error, setError] = useState<PlanRevisionError | null>(null)
  const [draftState, setDraftState] = useState<{ requestId: string; text: string } | null>(null)
  const [retrying, setRetrying] = useState(false)
  const [retryError, setRetryError] = useState<string | null>(null)
  const [slow, setSlow] = useState(false)
  // Why: a second click must not start a second AI turn; state alone lags a render behind.
  const inFlight = useRef(false)
  const retryInFlight = useRef(false)

  const draft = draftState?.requestId === request.id ? draftState.text : ''
  const revising = stillRevising(submitted, request, runState)
  const serverRewrite =
    runState.busy && runState.display.kind === 'writing' && runState.display.replan
  const busy = calling || retrying || revising || serverRewrite
  // A failed round leaves the Request in `planning` with its plan: the reviewer can ask again from there.
  const atGate =
    request.status === 'awaiting_plan_approval' ||
    (request.status === 'planning' && !!request.planTaskId)
  const canShow = hasRevisablePlan(flowPlanKind) && atGate && !!request.planTaskId && viewingCurrent
  const visible = viewingCurrent && (canShow || (busy && hasRevisablePlan(flowPlanKind)))
  const nextVersion = currentRevision !== undefined ? currentRevision + 1 : undefined

  useRequestSubscription({
    requestId: request.id,
    onEvent: (event) => {
      if (
        busy &&
        (PROGRESS_EVENT_RE.test(event.eventType) || event.eventType === REQUEST_POLL_EVENT_TYPE)
      ) {
        onSettled()
      }
    }
  })

  // Past the budget the wait is reported, never turned into a failure: the server decides when a run has failed.
  useEffect(() => {
    if (!busy) {
      return undefined
    }
    const budget =
      request.engine === 'openspec' ? PLAN_RUN_OPENSPEC_TIMEOUT_MS : PLAN_RUN_TIMEOUT_MS
    const left = Math.max(0, budget - (submitted ? Date.now() - submitted.at : 0))
    const timer = setTimeout(() => setSlow(true), left)
    return () => {
      clearTimeout(timer)
      setSlow(false)
    }
  }, [busy, request.engine, submitted])

  const setDraft = useCallback(
    (text: string) => {
      setDraftState({ requestId: request.id, text })
      setError(null)
    },
    [request.id]
  )

  const markSubmitted = useCallback((): void => {
    setSubmitted({
      requestId: request.id,
      version: request.version,
      updatedAt: request.updatedAt,
      planTaskId: request.planTaskId,
      priorRunId: runState.run?.runId ?? '',
      at: Date.now()
    })
  }, [request.id, request.version, request.updatedAt, request.planTaskId, runState.run?.runId])

  const startRun = useCallback(
    (feedback: string | undefined, idempotencyKey: string | undefined) =>
      serverCommit
        ? generatePlanPropose(request.id, { feedback, idempotencyKey })
        : generatePlanProposeCommit(request.id, { feedback, idempotencyKey }),
    [serverCommit, request.id]
  )

  const submit = useCallback(
    async (text?: string): Promise<PlanRevisionOutcome> => {
      const fail = (failure: PlanRevisionError): PlanRevisionOutcome => {
        setError(failure)
        return { ok: false, error: failure }
      }
      const message = (kind: 'empty' | 'tooLong' | 'wrongState'): string =>
        planRevisionMessage(kind, { serverCommit }) ?? ''
      if (inFlight.current || busy) {
        return { ok: false, error: { kind: 'wrongState', message: '' } }
      }
      const feedback = (text ?? draft).trim()
      if (feedback === '') {
        return fail({ kind: 'empty', message: message('empty') })
      }
      if ([...feedback].length > FEEDBACK_MAX_LENGTH) {
        return fail({ kind: 'tooLong', message: message('tooLong') })
      }
      if (!canShow) {
        return fail({ kind: 'wrongState', message: message('wrongState') })
      }

      inFlight.current = true
      setCalling(true)
      setError(null)
      markSubmitted()
      try {
        // A fresh key per submit: a retry after a failure is a new request for a new version, not a replay.
        const result = await startRun(feedback, createBrowserUuid())
        if (!result.ok) {
          const failure = classifyPlanRevisionError(result.error, { serverCommit })
          // The text stays in the box: nothing here clears the draft on a failure.
          setSubmitted(null)
          if (failure.kind === 'wrongState') {
            onSettled()
          }
          return fail(failure)
        }
        setDraftState({ requestId: request.id, text: '' })
        runState.refresh()
        onSettled()
        return { ok: true }
      } catch (thrown) {
        // The RPC wrappers return a Result, so this is a bug or a broken transport: report it, keep the text.
        console.error('[usePlanRevision] request.generatePlan threw', thrown)
        setSubmitted(null)
        return fail(
          classifyPlanRevisionError(
            {
              kind: 'unknown',
              code: 'unknown',
              message: thrown instanceof Error ? thrown.message : ''
            },
            { serverCommit }
          )
        )
      } finally {
        inFlight.current = false
        setCalling(false)
      }
    },
    [busy, canShow, draft, markSubmitted, onSettled, request.id, runState, serverCommit, startRun]
  )

  // Try again after a failed round: no new text, the server hands the last request on to the new run.
  const retry = useCallback(async (): Promise<void> => {
    if (retryInFlight.current || inFlight.current) {
      return
    }
    retryInFlight.current = true
    setRetrying(true)
    setRetryError(null)
    markSubmitted()
    try {
      const result = await startRun(undefined, undefined)
      if (!result.ok) {
        setSubmitted(null)
        setRetryError(classifyPlanRevisionError(result.error, { serverCommit }).message)
        return
      }
      runState.refresh()
      onSettled()
    } finally {
      retryInFlight.current = false
      setRetrying(false)
    }
  }, [markSubmitted, onSettled, runState, serverCommit, startRun])

  return {
    /** The panel may take a new request: a revisable plan at its gate (or after a failed round), on the current version. */
    canShow,
    /** The panel is on screen: it takes a request, or it says a version is being written. */
    visible,
    /** A call is in flight, or the AI is writing the next version. Approve, Reject and Revise stay locked. */
    busy,
    /** The version being written (n+1) while busy, when the current one is known. */
    nextVersion,
    /** The round has outlasted what an engine normally needs; the server still decides when it failed. */
    slow: busy && slow,
    draft,
    setDraft,
    error,
    submit,
    retry,
    retrying,
    retryError
  }
}

export type PlanRevisionApi = ReturnType<typeof usePlanRevision>
