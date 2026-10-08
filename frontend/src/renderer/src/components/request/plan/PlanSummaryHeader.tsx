/**
 * PlanSummaryHeader — CR-REQ-021-03
 *
 * @module components/request/plan/PlanSummaryHeader
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { Progress } from '@/components/ui/progress'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { ApprovalStatusBadge } from '../ApprovalStatusBadge'
import { listPlanWorkTasks, resolveProgress } from './plan-approval-model'
import type { PlanSubtree } from '../../../../../shared/task-hierarchy'
import type { Approval } from '../../../../../shared/request-types'

type Props = {
  tree: PlanSubtree
  /** Latest plan / task_list approval, if any. */
  approval: Approval | null
  /** Optional tree/graph switch; omitted = no toggle (graph entry point, CR-REQ-032). */
  view?: 'tree' | 'graph'
  onViewChange?: (view: 'tree' | 'graph') => void
}

export function PlanSummaryHeader({ tree, approval, view, onViewChange }: Props): React.JSX.Element | null {
  if (!tree.plan) {
    return null
  }
  const work = listPlanWorkTasks(tree)
  const progress = resolveProgress(tree.plan, work)
  return (
    <div
      className="sticky top-0 z-10 flex flex-col gap-2 border-b border-border bg-background px-4 py-3"
      data-testid="plan-summary-header"
    >
      <div className="flex items-center gap-2">
        <h3 className="min-w-0 flex-1 truncate text-sm font-semibold">{tree.plan.title}</h3>
        {approval && <ApprovalStatusBadge status={approval.status} />}
        {view && onViewChange && (
          <ToggleGroup
            type="single"
            size="sm"
            variant="outline"
            value={view}
            onValueChange={(v) => {
              if (v) {onViewChange(v as 'tree' | 'graph')}
            }}
            aria-label={translate('auto.components.graph.Entry.planView', 'Plan view')}
          >
            <ToggleGroupItem value="tree">{translate('auto.components.graph.Entry.tree', 'Tree')}</ToggleGroupItem>
            <ToggleGroupItem value="graph">{translate('auto.components.graph.Entry.planGraph', 'Graph')}</ToggleGroupItem>
          </ToggleGroup>
        )}
      </div>
      <div className="flex items-center gap-2">
        <Progress
          value={progress.value}
          className="h-1.5 flex-1"
          aria-label={translate(
            'auto.components.request.plan.PlanSummaryHeader.progress',
            'Plan progress'
          )}
        />
        <span
          className="text-xs tabular-nums text-muted-foreground"
          data-testid="plan-progress-value"
        >
          {progress.value}%
          {progress.estimated &&
            ` (${translate('auto.components.request.plan.PlanSummaryHeader.estimated', 'estimated')})`}
        </span>
      </div>
      <p className="text-xs text-muted-foreground" data-testid="plan-counts">
        {translate(
          'auto.components.request.plan.PlanSummaryHeader.counts',
          '{{phases}} phases, {{tasks}} tasks',
          { phases: tree.phases.length, tasks: work.length }
        )}
      </p>
    </div>
  )
}
