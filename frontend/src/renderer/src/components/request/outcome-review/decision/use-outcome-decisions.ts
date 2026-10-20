/**
 * useOutcomeDecisions — FE-OR-T301, T302
 *
 * The decisions of one delivered review, and the three things a person does with them: decide on an item, approve the review,
 * and tie sent-back items to the follow-up Request. The server owns the rules (who may, which decisions an item allows, whether
 * anything is still open); this hook only asks, sends the version of the review it saw, and reports the server's code. A write
 * that fails changes nothing on screen.
 *
 * @module components/request/outcome-review/decision/use-outcome-decisions
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { callRequestRpc } from '../../../../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../../../../shared/request-rpc-methods'
import {
  parseDecideOutcome,
  parseDecisionList
} from '../../../../../../shared/outcome-review-response-parsers'
import { parseOutcomeReview } from '../../../../../../shared/outcome-review-wire-parsers'
import { useRequestSubscription } from '../../../../hooks/useRequestSubscription'
import { needsDecisionItems } from '../outcome-review-view-model'
import type {
  OutcomeReview,
  ReviewDecision,
  ReviewDecisionValue
} from '../../../../../../shared/outcome-review-types'

export type DecisionFailure = { code: string; message: string }
export type DecisionResult = { ok: true } | { ok: false; error: DecisionFailure }

type Options = {
  requestId: string
  review: OutcomeReview | null
  /** Reads decisions only for a delivered review the viewer can see. */
  enabled: boolean
  /** A write changed the review (its version, its state): read it again. */
  onChanged: () => void
}

export type UseOutcomeDecisionsResult = {
  loaded: boolean
  decisions: ReviewDecision[]
  /** The decision in force for each item, by item key (orphaned ones are history of an item that is gone). */
  inForce: ReadonlyMap<string, ReviewDecision>
  /** Items that ask for a decision and have none in force. */
  openKeys: string[]
  busy: boolean
  decide: (itemKey: string, decision: ReviewDecisionValue, note: string) => Promise<DecisionResult>
  approve: (note: string) => Promise<DecisionResult>
  linkChild: (itemKeys: string[], childRequestId: string) => Promise<DecisionResult>
  refresh: () => void
}

function failure(error: { backendCode?: string; message: string }): DecisionResult {
  return { ok: false, error: { code: error.backendCode ?? '', message: error.message } }
}

export function useOutcomeDecisions({
  requestId,
  review,
  enabled,
  onChanged
}: Options): UseOutcomeDecisionsResult {
  const [decisions, setDecisions] = useState<ReviewDecision[]>([])
  const [loaded, setLoaded] = useState(false)
  const [busy, setBusy] = useState(false)
  const reviewId = review?.reviewId ?? ''
  // Why a maximum: a write answers with the version it produced, which is ahead of the review on screen until that is read
  // again; the next write in a row must carry the newer one. A different review starts from its own version.
  const version = useRef({ id: reviewId, value: review?.version ?? 0 })
  if (version.current.id !== reviewId) {
    version.current = { id: reviewId, value: review?.version ?? 0 }
  } else if ((review?.version ?? 0) > version.current.value) {
    version.current.value = review?.version ?? 0
  }
  const generation = useRef(0)

  const load = useCallback(async (): Promise<void> => {
    if (!enabled || !reviewId) {
      return
    }
    const mine = ++generation.current
    const res = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.REVIEW_DECISIONS, {
      id: requestId,
      reviewId
    })
    if (mine !== generation.current) {
      return
    }
    // Why: a failed read leaves the last list on screen; the write buttons still work and the server decides.
    if (res.ok) {
      setDecisions(parseDecisionList(res.value).decisions)
      setLoaded(true)
    }
  }, [enabled, requestId, reviewId])

  useEffect(() => {
    const reads = generation
    setDecisions([])
    setLoaded(false)
    void load()
    return () => {
      reads.current++
    }
  }, [load])

  useRequestSubscription({
    requestId,
    onEvent: (event) => {
      if (event.eventType.includes('review.')) {
        void load()
      }
    }
  })

  const inForce = useMemo(() => {
    const map = new Map<string, ReviewDecision>()
    for (const d of decisions) {
      if (d.active && !d.orphaned) {
        map.set(d.itemKey, d)
      }
    }
    return map
  }, [decisions])

  const openKeys = useMemo(
    () =>
      review
        ? needsDecisionItems(review)
            .filter((i) => !inForce.has(i.key))
            .map((i) => i.key)
        : [],
    [review, inForce]
  )

  const write = useCallback(
    async (method: 'decide' | 'approve' | 'link', params: object): Promise<DecisionResult> => {
      setBusy(true)
      const res = await callRequestRpc<unknown>(
        method === 'decide'
          ? REQUEST_RPC_METHODS.REVIEW_DECIDE
          : method === 'approve'
            ? REQUEST_RPC_METHODS.REVIEW_APPROVE
            : REQUEST_RPC_METHODS.REVIEW_LINK_CHILD,
        { id: requestId, reviewId, ...params }
      )
      setBusy(false)
      if (!res.ok) {
        return failure(res.error)
      }
      if (method === 'decide') {
        const outcome = parseDecideOutcome(res.value)
        if (!outcome) {
          return { ok: false, error: { code: '', message: 'unreadable answer' } }
        }
        // The next write in a row (sending several items back) must carry the version this one produced, not the one on screen.
        version.current.value = Math.max(version.current.value, outcome.reviewVersion)
      }
      if (method === 'approve') {
        const approved = parseOutcomeReview((res.value as { review?: unknown })?.review)
        if (!approved) {
          return { ok: false, error: { code: '', message: 'unreadable answer' } }
        }
        version.current.value = Math.max(version.current.value, approved.version)
      }
      void load()
      onChanged()
      return { ok: true }
    },
    [requestId, reviewId, load, onChanged]
  )

  const decide = useCallback(
    (itemKey: string, decision: ReviewDecisionValue, note: string) =>
      write('decide', {
        itemKey,
        decision,
        note: note.trim(),
        expectedVersion: version.current.value
      }),
    [write]
  )
  const approve = useCallback(
    (note: string) =>
      write('approve', { note: note.trim(), expectedVersion: version.current.value }),
    [write]
  )
  const linkChild = useCallback(
    (itemKeys: string[], childRequestId: string) => write('link', { itemKeys, childRequestId }),
    [write]
  )
  const refresh = useCallback(() => void load(), [load])

  return { loaded, decisions, inForce, openKeys, busy, decide, approve, linkChild, refresh }
}
