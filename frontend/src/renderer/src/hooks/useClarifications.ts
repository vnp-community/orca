/**
 * useClarifications — FE-REQ-TASK-036-02
 *
 * One open Clarification per Request. Drafts live in hook state only: answers
 * may be sensitive, so nothing goes to localStorage and the server draft
 * (`complete=false`) is never called automatically.
 *
 * @module hooks/useClarifications
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { callRequestRpc, type Result } from '../runtime/request-rpc-client'
import { useRefetchOnRequestEvent } from './useRefetchOnRequestEvent'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseClarification } from '../../../shared/request-artifact-parsers'
import type { AnswerPayload, Clarification } from '../../../shared/request-artifact-types'

const EVENTS = ['clarification.requested', 'clarification.answered', 'clarification.expired', 'clarification.cancelled', 'request.status_changed'] as const

export type AnswerOutcome = { stillMissing: boolean; requestStatus?: string }

function unwrap(raw: unknown): unknown {
  const o = raw as { clarification?: unknown } | null
  return o && typeof o === 'object' && 'clarification' in o ? o.clarification : raw
}

export function useClarifications(requestId: string | null) {
  const [open, setOpen] = useState<Clarification | null>(null)
  const [history, setHistory] = useState<Clarification[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [draft, setDraft] = useState<ReadonlyMap<string, unknown>>(new Map())
  const [tick, setTick] = useState(0)
  const submitting = useRef(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const draftOwner = useRef<string | null>(null)

  const refetch = useCallback(() => setTick((n) => n + 1), [])
  const onEvent = useCallback(() => refetch(), [refetch])
  useRefetchOnRequestEvent(requestId, EVENTS, onEvent)

  useEffect(() => {
    if (!requestId) {
      setOpen(null)
      setHistory([])
      return
    }
    let cancelled = false
    setLoading(true)
    void (async () => {
      const listed = await callRequestRpc<{ clarifications?: unknown[] }>(REQUEST_RPC_METHODS.CLARIFICATION_LIST, { requestId })
      if (cancelled) {return}
      if (!listed.ok) {
        setLoading(false)
        setError(listed.error.kind)
        // Why: an unsupported runtime simply has no clarifications; do not surface an error.
        if (listed.error.kind === 'unsupported') {setError(null)}
        setOpen(null)
        return
      }
      setError(null)
      const all = (listed.value.clarifications ?? []).map(parseClarification).filter((c): c is Clarification => c !== null)
      const openOne = all.find((c) => c.status === 'open') ?? null
      let detailed = openOne
      if (openOne && openOne.questions.length === 0) {
        const got = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.CLARIFICATION_GET, { clarificationId: openOne.id })
        if (cancelled) {return}
        if (got.ok) {detailed = parseClarification(unwrap(got.value)) ?? openOne}
      }
      setHistory(all.filter((c) => c.status !== 'open'))
      setOpen(detailed)
      setLoading(false)
    })()
    return () => { cancelled = true }
  }, [requestId, tick])

  // Why: a draft belongs to one Clarification; a new round starts clean.
  useEffect(() => {
    const id = open?.id ?? null
    if (draftOwner.current !== id) {
      draftOwner.current = id
      setDraft(new Map())
    }
  }, [open?.id])

  const setDraftValue = useCallback((questionId: string, value: unknown) => {
    setDraft((prev) => new Map(prev).set(questionId, value))
  }, [])
  const clearDraft = useCallback(() => setDraft(new Map()), [])

  const answer = useCallback(
    async (payload: AnswerPayload): Promise<Result<AnswerOutcome>> => {
      if (submitting.current) {
        return { ok: false, error: { kind: 'invalid_state', code: 'SUBMIT_IN_FLIGHT', message: 'A submit is already in flight' } }
      }
      submitting.current = true
      setIsSubmitting(true)
      try {
        const res = await callRequestRpc<{ stillMissing?: boolean; still_missing?: boolean; requestStatus?: string; request_status?: string }>(
          REQUEST_RPC_METHODS.CLARIFICATION_ANSWER,
          payload
        )
        if (!res.ok) {return res}
        refetch()
        return {
          ok: true,
          value: {
            stillMissing: (res.value.stillMissing ?? res.value.still_missing) === true,
            requestStatus: res.value.requestStatus ?? res.value.request_status
          }
        }
      } finally {
        submitting.current = false
        setIsSubmitting(false)
      }
    },
    [refetch]
  )

  const cancel = useCallback(
    async (clarificationId: string, expectedVersion: number): Promise<Result<unknown>> => {
      const res = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.CLARIFICATION_CANCEL, { clarificationId, expectedVersion })
      if (res.ok) {refetch()}
      return res
    },
    [refetch]
  )

  return { open, history, loading, error, refetch, answer, cancel, submitting: isSubmitting, draft, setDraftValue, clearDraft, hasDraft: draft.size > 0 }
}
