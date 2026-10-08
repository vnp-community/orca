/**
 * useDecisions — FE-REQ-TASK-036-02
 *
 * There is no `decision.record`: a Decision is written by `solution.choose`.
 * This hook lists Decisions and performs the second confirmation.
 *
 * @module hooks/useDecisions
 */

import { useCallback, useEffect, useMemo, useState } from 'react'
import { callRequestRpc, type Result } from '../runtime/request-rpc-client'
import { useRefetchOnRequestEvent } from './useRefetchOnRequestEvent'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseDecision } from '../../../shared/request-artifact-parsers'
import type { Decision } from '../../../shared/request-artifact-types'

const EVENTS = ['decision.recorded', 'decision.confirmed', 'solution.approved', 'solution.proposed', 'approval.decided'] as const

export function useDecisions(requestId: string | null, solutionId?: string) {
  const [decisions, setDecisions] = useState<Decision[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [tick, setTick] = useState(0)

  const refetch = useCallback(() => setTick((n) => n + 1), [])
  useRefetchOnRequestEvent(requestId, EVENTS, refetch)

  useEffect(() => {
    if (!requestId) {
      setDecisions([])
      return
    }
    let cancelled = false
    setLoading(true)
    void callRequestRpc<{ decisions?: unknown[] }>(REQUEST_RPC_METHODS.DECISION_LIST, { requestId }).then((res) => {
      if (cancelled) {return}
      setLoading(false)
      if (!res.ok) {
        setError(res.error.kind === 'unsupported' ? null : res.error.kind)
        return
      }
      setError(null)
      setDecisions((res.value.decisions ?? []).map(parseDecision).filter((d): d is Decision => d !== null))
    })
    return () => { cancelled = true }
  }, [requestId, tick])

  const current = useMemo(() => {
    const scoped = decisions.filter((d) => d.status !== 'superseded' && (solutionId ? d.subjectId === solutionId : d.subjectKind === 'solution_option' || d.subjectKind === 'solution'))
    return scoped.at(-1) ?? null
  }, [decisions, solutionId])

  const confirm = useCallback(
    async (decisionId: string, confirmationText: string, expectedVersion: number): Promise<Result<unknown>> => {
      const res = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.DECISION_CONFIRM, { decisionId, confirmationText, expectedVersion })
      if (res.ok) {refetch()}
      return res
    },
    [refetch]
  )

  return { decisions, current, loading, error, refetch, confirm }
}
