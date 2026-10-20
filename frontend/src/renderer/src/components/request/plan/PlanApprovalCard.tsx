/**
 * PlanApprovalCard — FE-REQ-TASK-037-04
 *
 * The one approval of a plan that runs on its own: says what the agent will do on its own and when it will stop and ask, then
 * offers the usual decision bar. There is no per-step approval to look for afterwards.
 *
 * @module components/request/plan/PlanApprovalCard
 */

import React from 'react'
import { Info } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { PlanApprovalBar } from './PlanApprovalBar'
import type { PlanDecision } from '../../../hooks/usePlanDecision'
import type { RequestFlowView } from '../../../../../shared/request-execution-types'
import type { Approval } from '../../../../../shared/request-types'

const T = 'auto.components.request.plan.PlanApprovalCard.'

type Props = {
  approval: Approval
  decision: PlanDecision
  flow: RequestFlowView | null
  stepCount: number
  taskCount: number
  approveBlockedReason?: string | null
}

export function PlanApprovalCard({
  approval,
  decision,
  flow,
  stepCount,
  taskCount,
  approveBlockedReason
}: Props): React.JSX.Element {
  const hasPreDeploy =
    flow?.startGate === 'pre_deploy' || flow?.executionGates.includes('pre_deploy')
  // Why: the wire parser folds task_list into plan; the original kind survives in rawSubjectType.
  const isTaskList =
    approval.rawSubjectType === 'task_list' ||
    (approval.subjectType as string | undefined) === 'task_list'
  const unit =
    stepCount > 0
      ? translate(`${T}steps`, '{{count}} steps', { count: stepCount })
      : translate(`${T}tasks`, '{{count}} tasks', { count: taskCount })
  return (
    <div className="flex flex-col border-b border-border" data-testid="plan-approval-card">
      <div className="flex flex-col gap-2 px-4 pt-3 text-sm">
        <p className="font-medium">
          {isTaskList
            ? translate(`${T}titleTaskList`, 'Approve once and the agent runs all {{unit}}', {
                unit
              })
            : translate(`${T}title`, 'Approve once and the agent runs all {{unit}}', { unit })}
        </p>
        <div className="text-muted-foreground">
          <p>{translate(`${T}stopsWhen`, 'It stops and asks you only when:')}</p>
          <ul className="mt-1 list-disc pl-5">
            <li>{translate(`${T}needsInfo`, 'it needs information only you have')}</li>
            <li>
              {translate(
                `${T}failsTwice`,
                'a task keeps failing — the request then goes back to the backlog'
              )}
            </li>
            {stepCount > 0 && (
              <li>
                {translate(`${T}drift`, 'the change drifts from this plan and needs a review')}
              </li>
            )}
            {hasPreDeploy && (
              <li>{translate(`${T}preDeploy`, 'it reaches the pre-deploy approval')}</li>
            )}
          </ul>
        </div>
        {stepCount > 0 && (
          <p
            className="flex items-start gap-1.5 text-xs text-muted-foreground"
            data-testid="plan-risk-whole"
          >
            <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden />
            {translate(
              `${T}riskWhole`,
              'Risk is assessed for the plan as a whole, including every step. Review it below before approving.'
            )}
          </p>
        )}
      </div>
      <PlanApprovalBar
        approval={approval}
        decision={decision}
        approveBlockedReason={approveBlockedReason}
        approveExecutionMode="continuous"
        approveLabel={translate(`${T}approve`, 'Approve and start execution')}
        rejectLabel={translate(`${T}requestChanges`, 'Request changes')}
      />
    </div>
  )
}
