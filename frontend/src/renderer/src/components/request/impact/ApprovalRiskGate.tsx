/**
 * ApprovalRiskGate — FE-REQ-TASK-036-06
 *
 * Risk gate for the plan and phase approval bars (same rules as SolutionDecisionBar):
 * the impact section sits above Approve, high findings need an acceptance, and the
 * backend stays the source of truth (`approval.approve` re-checks everything).
 *
 * @module components/request/impact/ApprovalRiskGate
 */

import React, { useEffect } from 'react'
import { translate } from '@/i18n/i18n'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { useImpactAssessment, type ImpactAssessmentApi } from '../../../hooks/useImpactAssessment'
import { splitErrorCode } from '../../../../../shared/request-errors'
import { ImpactFindingList } from './ImpactFindingList'
import { RiskAcceptanceChecklist } from './RiskAcceptanceChecklist'
import { RiskOverrideMenu } from './RiskOverrideMenu'
import { RiskSummaryCard } from './RiskSummaryCard'
import { useRiskApprovalGate } from './useRiskApprovalGate'
import type { Approval } from '../../../../../shared/request-types'
import type { RequestRpcError } from '../../../../../shared/request-errors'

const T = 'auto.components.request.impact.'
// Why: these codes mean our local view of the assessment is outdated, so reload it.
const REFETCH_CODES = [
  'REQUEST_RISK_ACCEPTANCE_REQUIRED',
  'REQUEST_RISK_ASSESSMENT_STALE',
  'REQUEST_RISK_ASSESSMENT_PENDING'
]

export type ApprovalRiskGateApi = {
  impact: ImpactAssessmentApi
  gate: ReturnType<typeof useRiskApprovalGate>
  /** Text for the Approve tooltip while the gate blocks; null when Approve may be pressed. */
  blockedReason: string | null
  approverNotAllowed: boolean
}

export function useApprovalRiskGate(
  approval: Approval | null,
  failure: RequestRpcError | null
): ApprovalRiskGateApi {
  const pending = approval?.status === 'pending'
  const impact = useImpactAssessment({
    requestId: approval?.requestId ?? null,
    subjectType: approval?.subjectType ?? '',
    subjectId: pending && approval ? approval.subjectId : '',
    enabled: pending
  })
  const gate = useRiskApprovalGate(impact)
  const code = failure ? splitErrorCode(failure.message).code || failure.code : ''
  const { refetch } = impact
  useEffect(() => {
    if (REFETCH_CODES.includes(code)) {
      refetch()
    }
  }, [code, refetch])
  const key = gate.requirements.blockedReasonKey
  return {
    impact,
    gate,
    blockedReason: gate.requirements.canApprove
      ? null
      : translate(key ?? `${T}gate.viewImpactFirst`, 'Review the impact before approving'),
    approverNotAllowed: code === 'REQUEST_RISK_APPROVER_NOT_ALLOWED'
  }
}

type Props = {
  api: ApprovalRiskGateApi
  approval: Approval
  /** Gate name for `risk.override` ('plan' | 'phase'). */
  gateName: string
  disabled?: boolean
}

export function ApprovalRiskGateSection({
  api,
  approval,
  gateName,
  disabled
}: Props): React.JSX.Element | null {
  const { impact, gate } = api
  const reqs = gate.requirements
  if (approval.status !== 'pending' || impact.status === 'unsupported') {
    return null
  }
  return (
    <div className="flex flex-col gap-2" data-testid={`risk-gate-${gateName}`}>
      {reqs.requiresView ? (
        <Collapsible
          onOpenChange={(open) => {
            if (open) {
              gate.markViewed()
            }
          }}
        >
          <CollapsibleTrigger
            className="text-xs underline-offset-2 hover:underline"
            data-testid={`${gateName}-view-impact-trigger`}
          >
            {translate(`${T}gate.viewImpact`, 'View impact')}
          </CollapsibleTrigger>
          <CollapsibleContent className="flex flex-col gap-2 pt-2">
            <RiskSummaryCard
              assessment={impact}
              subjectType={approval.subjectType}
              subjectId={approval.subjectId}
            />
            <ImpactFindingList findings={impact.findings} />
          </CollapsibleContent>
        </Collapsible>
      ) : null}
      <RiskAcceptanceChecklist
        requirements={reqs}
        accepted={gate.state.acceptedFindingIds}
        info={gate.info}
        invalidated={gate.invalidated}
        onAccept={gate.accept}
        disabled={disabled}
      />
      {api.approverNotAllowed ? (
        <p
          role="status"
          className="text-xs text-muted-foreground"
          data-testid={`${gateName}-risk-approver-not-allowed`}
        >
          {translate(
            `${T}gate.approverNotAllowed`,
            'An approver from the designated team is required'
          )}
        </p>
      ) : null}
      <RiskOverrideMenu gate={gateName} allowed={impact.canOverride} onOverride={impact.override} />
    </div>
  )
}
