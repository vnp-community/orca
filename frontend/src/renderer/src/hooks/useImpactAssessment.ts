/**
 * useImpactAssessment — FE-REQ-TASK-036-02
 *
 * Summary, findings (medium and above) and option comparison, plus the write
 * paths (request, accept, override) with client-side minimum-length guards.
 *
 * @module hooks/useImpactAssessment
 */

import { useCallback, useEffect, useState } from 'react'
import { callRequestRpc, type Result } from '../runtime/request-rpc-client'
import { useRefetchOnRequestEvent } from './useRefetchOnRequestEvent'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { splitErrorCode } from '../../../shared/request-errors'
import { parseImpactComparison, parseImpactFinding, parseImpactSummary } from '../../../shared/request-artifact-parsers'
import type { ImpactComparison, ImpactFinding, ImpactSummary } from '../../../shared/request-artifact-types'

export const RISK_ACCEPT_RATIONALE_MIN = 10
export const RISK_OVERRIDE_REASON_MIN = 20
const EVENTS = ['impact.assessed', 'risk.accepted'] as const

export type ImpactAssessmentStatus = 'idle' | 'loading' | 'collecting' | 'ready' | 'unsupported' | 'noDevServer' | 'forbidden' | 'error'

type Params = { requestId?: string | null; subjectType: string; subjectId: string; solutionId?: string; enabled?: boolean }

const validation = (code: string, message: string): Result<never> => ({ ok: false, error: { kind: 'validation', code, message } })

export function useImpactAssessment({ requestId = null, subjectType, subjectId, solutionId, enabled = true }: Params) {
  const [summary, setSummary] = useState<ImpactSummary | null>(null)
  const [findings, setFindings] = useState<ImpactFinding[]>([])
  const [comparison, setComparison] = useState<ImpactComparison[] | null>(null)
  const [canOverride, setCanOverride] = useState<boolean | undefined>(undefined)
  const [status, setStatus] = useState<ImpactAssessmentStatus>('idle')
  const [tick, setTick] = useState(0)

  const refetch = useCallback(() => setTick((n) => n + 1), [])
  useRefetchOnRequestEvent(requestId, EVENTS, refetch)

  useEffect(() => {
    if (!enabled || !subjectId) {
      setStatus('idle')
      return
    }
    let cancelled = false
    setStatus('loading')
    const load = async (): Promise<void> => {
      const got = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.IMPACT_GET, { subjectType, subjectId })
      if (cancelled) {return}
      if (!got.ok) {
        const code = splitErrorCode(got.error.message).code || got.error.code
        setSummary(null)
        if (got.error.kind === 'unsupported') {setStatus('unsupported')}
        else if (got.error.kind === 'forbidden') {setStatus('forbidden')}
        else if (got.error.kind === 'no_dev_server' || code === 'REQUEST_IMPACT_NO_CONNECTION') {setStatus('noDevServer')}
        else if (got.error.kind === 'pending') {setStatus('collecting')}
        else {setStatus('error')}
        return
      }
      const raw = got.value as { summary?: unknown; assessment?: unknown } | null
      const parsed = parseImpactSummary(raw?.summary ?? raw?.assessment ?? got.value)
      // Why: permission comes only from the backend; absent flag keeps the override menu hidden.
      const viewer = (got.value as { viewerCan?: { override?: unknown }; viewer_can?: { override?: unknown } } | null)
      setCanOverride((viewer?.viewerCan ?? viewer?.viewer_can)?.override === true ? true : undefined)
      setSummary(parsed)
      if (!parsed) {
        // Why: no assessment is "not assessed", not an error.
        setFindings([])
        setStatus('ready')
        return
      }
      if (parsed.status === 'collecting') {setStatus('collecting')}
      const [f, c] = await Promise.all([
        callRequestRpc<{ findings?: unknown[] }>(REQUEST_RPC_METHODS.IMPACT_FINDINGS, { assessmentId: parsed.assessmentId, minLevel: 'medium' }),
        solutionId ? callRequestRpc<{ comparison?: unknown[]; options?: unknown[] }>(REQUEST_RPC_METHODS.IMPACT_COMPARE, { solutionId }) : Promise.resolve(null)
      ])
      if (cancelled) {return}
      if (f.ok) {setFindings((f.value.findings ?? []).map(parseImpactFinding).filter((x): x is ImpactFinding => x !== null))}
      if (c && c.ok) {setComparison((c.value.comparison ?? c.value.options ?? []).map(parseImpactComparison).filter((x): x is ImpactComparison => x !== null))}
      if (parsed.status !== 'collecting') {setStatus('ready')}
    }
    // Why: a rejected call must degrade to an error state, never an unhandled rejection.
    load().catch(() => { if (!cancelled) {setStatus('error')} })
    return () => { cancelled = true }
  }, [enabled, subjectType, subjectId, solutionId, tick])

  const request = useCallback(async (): Promise<Result<unknown>> => {
    const res = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.IMPACT_REQUEST, { subjectType, subjectId })
    if (res.ok) {refetch()}
    return res
  }, [subjectType, subjectId, refetch])

  const acceptRisk = useCallback(
    async ({ findingId, rationale }: { findingId: string; rationale: string }): Promise<Result<unknown>> => {
      if (!summary) {return validation('NO_ASSESSMENT', 'No assessment to accept against')}
      if (rationale.trim().length < RISK_ACCEPT_RATIONALE_MIN) {return validation('RATIONALE_TOO_SHORT', 'Rationale too short')}
      const res = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.IMPACT_ACCEPT, {
        assessmentId: summary.assessmentId, findingId, rationale: rationale.trim(), assessmentDigest: summary.digest
      })
      // Why: a stale digest invalidates earlier acceptances; reload so the UI rebuilds the list.
      if (!res.ok && (res.error.kind === 'conflict' || splitErrorCode(res.error.message).code === 'REQUEST_RISK_ASSESSMENT_STALE')) {refetch()}
      return res
    },
    [summary, refetch]
  )

  const override = useCallback(
    async ({ gate, reason }: { gate: string; reason: string }): Promise<Result<unknown>> => {
      if (!requestId) {return validation('NO_REQUEST', 'Request id required')}
      if (reason.trim().length < RISK_OVERRIDE_REASON_MIN) {return validation('REASON_TOO_SHORT', 'Reason too short')}
      return callRequestRpc<unknown>(REQUEST_RPC_METHODS.RISK_OVERRIDE, { requestId, gate, reason: reason.trim() })
    },
    [requestId]
  )

  return { summary, findings, comparison, canOverride, status, request, acceptRisk, override, refetch }
}

export type ImpactAssessmentApi = ReturnType<typeof useImpactAssessment>
