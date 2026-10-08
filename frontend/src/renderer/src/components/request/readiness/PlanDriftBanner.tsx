/**
 * PlanDriftBanner — FE-REQ-TASK-036-07
 *
 * Shown for a pending phase approval at stage `drift_review`. In shadow mode it
 * is advisory (no alarming styling). No keyboard shortcut on the action.
 *
 * @module components/request/readiness/PlanDriftBanner
 */

import React from 'react'
import { TriangleAlert } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import type { Approval } from '../../../../../shared/request-types'

export function findDriftApproval(approvals: readonly Approval[]): Approval | null {
  return approvals.find((a) => a.subjectType === 'phase' && a.status === 'pending' && a.stage === 'drift_review') ?? null
}

type Props = { phaseName?: string; advisory?: boolean; onReview: () => void }

export function PlanDriftBanner({ phaseName, advisory, onReview }: Props): React.JSX.Element {
  return (
    <div
      role={advisory ? 'status' : 'alert'}
      data-testid="plan-drift-banner"
      className={cn(
        'flex flex-wrap items-center gap-2 border-b px-4 py-2 text-sm',
        advisory ? 'border-border text-muted-foreground' : 'border-risk-high-border bg-risk-high-background'
      )}
    >
      <TriangleAlert className="size-4 shrink-0" aria-hidden />
      <span className="min-w-0 flex-1">
        {advisory
          ? translate('auto.components.request.readiness.drift.shadow', 'Advisory: actual changes exceed the plan.')
          : translate('auto.components.request.readiness.drift.banner', 'Actual changes exceed the plan. Phase {{name}} is paused.', { name: phaseName ?? '' })}
      </span>
      <Button size="sm" onClick={onReview}>
        {translate('auto.components.request.readiness.drift.review', 'Review and decide')}
      </Button>
    </div>
  )
}
