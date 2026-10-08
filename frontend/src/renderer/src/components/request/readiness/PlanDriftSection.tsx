/**
 * PlanDriftSection — FE-REQ-TASK-036-07
 *
 * Mounted at the top of RequestPlanTab: banner + review sheet wired to approval
 * approve/reject. Renders nothing when no drift_review approval is pending.
 *
 * @module components/request/readiness/PlanDriftSection
 */

import React, { useState } from 'react'
import { useApprovals } from '../../../hooks/useApprovals'
import { useImpactAssessment } from '../../../hooks/useImpactAssessment'
import { PlanDriftBanner, findDriftApproval } from './PlanDriftBanner'
import { PlanDriftReviewSheet } from './PlanDriftReviewSheet'
import type { Approval, OrcaRequest } from '../../../../../shared/request-types'
import type { OrcaTask } from '../../../../../shared/task-types'

type Props = { request: OrcaRequest; approvals: readonly Approval[]; phases: readonly OrcaTask[]; onChanged: () => void }

export function PlanDriftSection({ request, approvals, phases, onChanged }: Props): React.JSX.Element | null {
  const [open, setOpen] = useState(false)
  const drift = findDriftApproval(approvals)
  const actions = useApprovals({ requestId: request.id })
  const impact = useImpactAssessment({ requestId: request.id, subjectType: 'plan', subjectId: request.planTaskId ?? '', enabled: drift !== null && Boolean(request.planTaskId) })
  if (!drift) {return null}
  const phaseName = phases.find((p) => p.id === drift.subjectId)?.title

  return (
    <>
      <PlanDriftBanner phaseName={phaseName} advisory={impact.summary?.mode === 'shadow'} onReview={() => setOpen(true)} />
      <PlanDriftReviewSheet
        open={open}
        onOpenChange={setOpen}
        phaseId={drift.subjectId}
        phaseName={phaseName}
        onAccept={async (comment) => {
          const res = await actions.approve({ approval: drift, comment })
          if (res.ok) {onChanged()}
          return res
        }}
        onReturn={async (comment) => {
          const res = await actions.reject({ approval: drift, comment })
          if (res.ok) {onChanged()}
          return res
        }}
      />
    </>
  )
}
