/**
 * useOutcomeReview — FE-OR-T112
 *
 * Reads the Outcome Review of a Request: the review (`request.review.get`) or, when there is none yet, what asking would do
 * (`request.review.estimate`), and the runs of the agent. It refreshes on the `review.changed` / `review.progress` events of the
 * request and, while a run is going, on a timer that stops when the page is hidden. Opening the tab only reads: a run starts
 * when a person asks for it (`generate`), never by itself.
 *
 * @module hooks/useOutcomeReview
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import {
  parseGenerateOutcome,
  parseGetReview,
  parseReviewEstimate,
  parseRuns
} from '../../../shared/outcome-review-response-parsers'
import type {
  GenerateOutcome,
  OutcomeReview,
  OutcomeReviewRun,
  ReviewEstimate
} from '../../../shared/outcome-review-types'
import type { RequestRpcError } from '../../../shared/request-errors'
import { useRequestSubscription } from './useRequestSubscription'

export const OUTCOME_REVIEW_POLL_MS = 4000

/**
 * loading: first read. none: the Request has no review yet (the estimate says what asking would do). ready: a review exists.
 * disabled / not_ready: the server refused with its documented code. unsupported: a gateway without the channels.
 */
export type OutcomeReviewStatus =
  | 'loading'
  | 'none'
  | 'ready'
  | 'disabled'
  | 'not_ready'
  | 'unsupported'
  | 'error'

export type OutcomeReviewFailure = { kind: RequestRpcError['kind']; code: string; message: string }

export type GenerateOptions = {
  /** False builds only the facts Orca measures, at no agent cost. */
  runContext?: boolean
  /** Reads the design (and the code) again even when a checked result is kept. */
  force?: boolean
  /** Lets a delivered review go ahead without the knowledge graph. */
  allowCodeOnly?: boolean
}

export type GenerateResult =
  | { ok: true; outcome: GenerateOutcome }
  | { ok: false; error: OutcomeReviewFailure }

type Options = {
  /** Reads happen only while enabled (the project has the review on and the Request has a Plan). */
  enabled?: boolean
  pollMs?: number
}

export type UseOutcomeReviewResult = {
  status: OutcomeReviewStatus
  review: OutcomeReview | null
  estimate: ReviewEstimate | null
  runs: OutcomeReviewRun[]
  /** The run of the agent that is going now (context or draw), if any. */
  activeRun: OutcomeReviewRun | null
  error: OutcomeReviewFailure | null
  generating: boolean
  generate: (options?: GenerateOptions) => Promise<GenerateResult>
  refresh: () => void
}

const DISABLED_CODE = 'REQUEST_REVIEW_DISABLED'
const NOT_READY_CODE = 'REQUEST_REVIEW_NOT_READY'

function failureOf(error: RequestRpcError): OutcomeReviewFailure {
  return { kind: error.kind, code: error.backendCode ?? '', message: error.message }
}

function statusOfFailure(error: RequestRpcError): OutcomeReviewStatus {
  if (error.kind === 'unsupported') {
    return 'unsupported'
  }
  if (error.backendCode === DISABLED_CODE) {
    return 'disabled'
  }
  return error.backendCode === NOT_READY_CODE ? 'not_ready' : 'error'
}

function pageVisible(): boolean {
  return typeof document === 'undefined' || document.visibilityState === 'visible'
}

function isActive(run: OutcomeReviewRun): boolean {
  return run.status === 'queued' || run.status === 'running'
}

function isReviewEvent(eventType: string): boolean {
  return eventType.includes('review.')
}

export function useOutcomeReview(
  requestId: string | null,
  options: Options = {}
): UseOutcomeReviewResult {
  const { enabled = true, pollMs = OUTCOME_REVIEW_POLL_MS } = options
  const [status, setStatus] = useState<OutcomeReviewStatus>('loading')
  const [review, setReview] = useState<OutcomeReview | null>(null)
  const [estimate, setEstimate] = useState<ReviewEstimate | null>(null)
  const [runs, setRuns] = useState<OutcomeReviewRun[]>([])
  const [error, setError] = useState<OutcomeReviewFailure | null>(null)
  const [generating, setGenerating] = useState(false)
  // Why: a read for another Request, or an older one, must never overwrite a newer answer.
  const generation = useRef(0)

  const load = useCallback(async (): Promise<void> => {
    if (!requestId || !enabled) {
      return
    }
    const mine = ++generation.current
    const got = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.REVIEW_GET, { id: requestId })
    if (mine !== generation.current) {
      return
    }
    if (!got.ok) {
      setStatus(statusOfFailure(got.error))
      setError(failureOf(got.error))
      return
    }
    const found = parseGetReview(got.value)
    setError(null)
    if (!found.review) {
      const est = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.REVIEW_ESTIMATE, {
        id: requestId
      })
      if (mine !== generation.current) {
        return
      }
      if (!est.ok) {
        setStatus(statusOfFailure(est.error))
        setError(failureOf(est.error))
        return
      }
      setReview(null)
      setRuns([])
      setEstimate(parseReviewEstimate(est.value))
      setStatus('none')
      return
    }
    const listed = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.REVIEW_RUNS, { id: requestId })
    if (mine !== generation.current) {
      return
    }
    setReview(found.review)
    setEstimate(null)
    // Why: the runs are context for the review; failing to list them must not hide the review itself.
    setRuns(listed.ok ? parseRuns(listed.value) : [])
    setStatus('ready')
  }, [requestId, enabled])

  useEffect(() => {
    const reads = generation
    setStatus('loading')
    setReview(null)
    setEstimate(null)
    setRuns([])
    setError(null)
    void load()
    return () => {
      reads.current++
    }
  }, [load])

  useRequestSubscription({
    requestId,
    onEvent: (event) => {
      if (isReviewEvent(event.eventType)) {
        void load()
      }
    }
  })

  const activeRun = runs.find(isActive) ?? null
  const polling = activeRun !== null
  useEffect(() => {
    if (!requestId || !enabled || pollMs <= 0 || !polling) {
      return undefined
    }
    const timer = setInterval(() => {
      if (pageVisible()) {
        void load()
      }
    }, pollMs)
    return () => clearInterval(timer)
  }, [requestId, enabled, pollMs, polling, load])

  const generate = useCallback(
    async (opts: GenerateOptions = {}): Promise<GenerateResult> => {
      if (!requestId) {
        return { ok: false, error: { kind: 'unknown', code: '', message: 'no request' } }
      }
      setGenerating(true)
      const res = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.REVIEW_GENERATE, {
        id: requestId,
        ...(opts.runContext === undefined ? {} : { runContext: opts.runContext }),
        ...(opts.force ? { force: true } : {}),
        ...(opts.allowCodeOnly ? { allowCodeOnly: true } : {})
      })
      setGenerating(false)
      if (!res.ok) {
        return { ok: false, error: failureOf(res.error) }
      }
      const outcome = parseGenerateOutcome(res.value)
      if (!outcome) {
        return { ok: false, error: { kind: 'unknown', code: '', message: 'unreadable answer' } }
      }
      void load()
      return { ok: true, outcome }
    },
    [requestId, load]
  )

  const refresh = useCallback(() => {
    void load()
  }, [load])

  return { status, review, estimate, runs, activeRun, error, generating, generate, refresh }
}
