/**
 * ExecutionNeedsYou — FE-REQ-TASK-037-05
 *
 * What the run waits for the viewer to do, one card each: answer the agent's question, decide a pre-deploy or step approval,
 * review drift, or see a task that ran out of attempts. The summary only lists what this viewer may act on.
 *
 * @module components/request/execution/ExecutionNeedsYou
 */

import React from 'react'
import { Hand } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { formatRequestRelativeTime } from '@/lib/request-relative-time'
import { ApprovalRiskGateSection, useApprovalRiskGate } from '../impact/ApprovalRiskGate'
import { planDecisionErrorMessage } from '../plan/plan-decision-error-message'
import { PlanDecisionButtons } from '../plan/PlanDecisionButtons'
import type { PlanDecision } from '../../../hooks/usePlanDecision'
import type { ExecutionNeedsYou as NeedsYouItem } from '../../../../../shared/request-execution-types'
import type { Approval } from '../../../../../shared/request-types'

const T = 'auto.components.request.execution.ExecutionNeedsYou.'

function dueText(item: NeedsYouItem): string | null {
  return item.dueAt
    ? translate(`${T}due`, 'Due {{when}}', { when: formatRequestRelativeTime(item.dueAt) })
    : null
}

function CardShell({
  item,
  title,
  children
}: {
  item: NeedsYouItem
  title: string
  children: React.ReactNode
}): React.JSX.Element {
  const due = dueText(item)
  return (
    <li
      className="flex flex-col gap-2 rounded-md border border-primary/40 px-3 py-2 text-sm"
      data-testid={`needs-you-${item.kind}`}
    >
      <div className="flex items-center gap-2">
        <Hand className="size-4 shrink-0 text-primary" aria-hidden />
        <span className="min-w-0 flex-1 font-medium">{title}</span>
        {due && <span className="text-xs text-muted-foreground">{due}</span>}
      </div>
      {children}
    </li>
  )
}

function ApprovalCard({
  item,
  approval,
  decision
}: {
  item: NeedsYouItem
  approval: Approval
  decision: PlanDecision
}): React.JSX.Element {
  const failure = decision.failure?.scope === approval.id ? decision.failure : null
  const risk = useApprovalRiskGate(approval, failure?.error ?? null)
  const preDeploy = item.kind === 'pre_deploy'
  const gateName = preDeploy ? 'pre_deploy' : 'phase'
  const title = preDeploy
    ? translate(`${T}preDeploy`, 'Approve before deploy{{what}}', {
        what: item.title ? `: ${item.title}` : ''
      })
    : translate(`${T}phase`, 'Approve step{{what}}', { what: item.title ? `: ${item.title}` : '' })
  return (
    <CardShell item={item} title={title}>
      <ApprovalRiskGateSection
        api={risk}
        approval={approval}
        gateName={gateName}
        disabled={decision.isBusy(`approve:${approval.id}`)}
      />
      <PlanDecisionButtons
        approval={approval}
        decision={decision}
        testIdPrefix={`needs-you-${gateName}`}
        approveBlockedReason={risk.blockedReason}
        approveExtras={risk.gate.approveExtras}
        hideApprove={risk.approverNotAllowed}
        approveLabel={translate(`${T}approve`, 'Approve')}
        rejectLabel={translate(`${T}reject`, 'Reject')}
        rejectTitle={translate(`${T}rejectTitle`, 'Reject this approval')}
      />
      {failure && (
        <p role="alert" className="text-xs text-destructive">
          {planDecisionErrorMessage(failure)}
        </p>
      )}
    </CardShell>
  )
}

type Props = {
  items: NeedsYouItem[]
  approvals: readonly Approval[]
  decision: PlanDecision
  onAnswerClarification: () => void
  onOpenTask: (taskId: string) => void
  /** Drift reviews have their own banner and sheet (PlanDriftSection), mounted by the tab. */
}

export function ExecutionNeedsYou({
  items,
  approvals,
  decision,
  onAnswerClarification,
  onOpenTask
}: Props): React.JSX.Element | null {
  if (items.length === 0) {
    return null
  }
  return (
    <section aria-label={translate(`${T}heading`, 'Needs you')} data-testid="execution-needs-you">
      <ul className="flex flex-col gap-2">
        {items.map((item) => {
          const key = `${item.kind}:${item.refId}`
          switch (item.kind) {
            case 'clarification':
              return (
                <CardShell
                  key={key}
                  item={item}
                  title={translate(`${T}clarification`, 'The agent has a question for you')}
                >
                  <div>
                    <Button
                      size="sm"
                      onClick={onAnswerClarification}
                      data-testid="needs-you-answer"
                    >
                      {translate(`${T}answer`, 'Answer')}
                    </Button>
                  </div>
                </CardShell>
              )
            case 'approval':
            case 'pre_deploy': {
              const approval = approvals.find((a) => a.id === item.refId && a.status === 'pending')
              return approval ? (
                <ApprovalCard key={key} item={item} approval={approval} decision={decision} />
              ) : null
            }
            case 'drift':
              // Why: PlanDriftSection owns the review sheet; this card only points at it.
              return (
                <CardShell
                  key={key}
                  item={item}
                  title={translate(`${T}drift`, 'The change drifted from the plan{{what}}', {
                    what: item.title ? `: ${item.title}` : ''
                  })}
                >
                  <p className="text-xs text-muted-foreground">
                    {translate(
                      `${T}driftHint`,
                      'Use “Review” in the banner above to accept or return it.'
                    )}
                  </p>
                </CardShell>
              )
            case 'failed_task':
              return (
                <CardShell
                  key={key}
                  item={item}
                  title={translate(`${T}failedTask`, '“{{task}}” ran out of attempts', {
                    task: item.title ?? ''
                  })}
                >
                  <div>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => onOpenTask(item.refId)}
                      data-testid="needs-you-view-log"
                    >
                      {translate(`${T}viewLog`, 'View last log')}
                    </Button>
                  </div>
                </CardShell>
              )
            default:
              return null
          }
        })}
      </ul>
    </section>
  )
}
