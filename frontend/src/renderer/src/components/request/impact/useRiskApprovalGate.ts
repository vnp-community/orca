/**
 * useRiskApprovalGate — FE-REQ-TASK-036-06
 *
 * Holds the approver's local progress (impact viewed, findings accepted) and
 * derives the requirements. Acceptances die when the assessment digest changes.
 *
 * @module components/request/impact/useRiskApprovalGate
 */

import { useCallback, useEffect, useMemo, useState } from 'react'
import { acceptedIdsForApprove, getApprovalRequirements, NO_ACCEPTANCES, type ApprovalAcceptanceState } from './risk-approval-rules'
import type { ImpactAssessmentApi } from '../../../hooks/useImpactAssessment'
import type { Result } from '../../../runtime/request-rpc-client'

export type AcceptedInfo = { acceptedBy?: string; createdAt?: string }

export function useRiskApprovalGate(impact: ImpactAssessmentApi) {
  const [state, setState] = useState<ApprovalAcceptanceState>(NO_ACCEPTANCES)
  const [info, setInfo] = useState<Record<string, AcceptedInfo>>({})
  const [invalidated, setInvalidated] = useState(false)
  const digest = impact.summary?.digest ?? null

  useEffect(() => {
    if (state.digestAtAcceptance !== null && digest !== state.digestAtAcceptance) {
      setState((s) => ({ ...s, acceptedFindingIds: new Set(), digestAtAcceptance: null }))
      setInfo({})
      setInvalidated(true)
    }
  }, [digest, state.digestAtAcceptance])

  const markViewed = useCallback(() => setState((s) => (s.viewedImpact ? s : { ...s, viewedImpact: true })), [])

  const accept = useCallback(
    async (findingId: string, rationale: string): Promise<Result<unknown>> => {
      const res = await impact.acceptRisk({ findingId, rationale })
      if (res.ok && digest) {
        const acc = (res.value as { acceptance?: { acceptedBy?: string; createdAt?: string } } | null)?.acceptance
        setInvalidated(false)
        setState((s) => ({ ...s, acceptedFindingIds: new Set(s.acceptedFindingIds).add(findingId), digestAtAcceptance: digest }))
        setInfo((prev) => ({ ...prev, [findingId]: { acceptedBy: acc?.acceptedBy, createdAt: acc?.createdAt } }))
      }
      return res
    },
    [impact, digest]
  )

  const requirements = useMemo(
    () => getApprovalRequirements(impact.summary, impact.findings, state),
    [impact.summary, impact.findings, state]
  )

  return {
    state,
    requirements,
    info,
    invalidated,
    markViewed,
    accept,
    approveExtras: {
      viewedImpactDigest: requirements.requiresView && state.viewedImpact && impact.summary ? impact.summary.digest : undefined,
      acceptedFindingIds: acceptedIdsForApprove(impact.summary, state)
    }
  }
}
