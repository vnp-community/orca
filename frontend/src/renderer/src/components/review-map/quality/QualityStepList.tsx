/**
 * QualityStepList.tsx — FE-CV-TASK-087-05
 *
 * Every step of the runs behind the verdict. A step that failed, timed out or was not ready is
 * labelled as incomplete: it must never be read as "0 findings".
 *
 * @module components/review-map/quality/QualityStepList
 */

import { useState } from 'react'
import { ChevronRight } from 'lucide-react'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { SeverityGlyph } from '../../quality-charts/SeverityGlyph'
import type { EncodingShape } from '../../quality-charts/severity-encoding'
import type { QualityStep } from '../../../../../shared/code-intel-quality-types'
import type { ScorecardStepRow } from './quality-scorecard-model'
import { scorecardCopy } from './quality-scorecard-copy'
import type { QualityScorecardCopyKey } from './quality-scorecard-copy'

const STEP_VISUAL: Record<string, { shape: EncodingShape; className: string }> = {
  passed: { shape: 'circle-check', className: 'text-quality-pass' },
  findings: { shape: 'triangle', className: 'text-quality-warning' },
  failed: { shape: 'octagon', className: 'text-quality-error' },
  timeout: { shape: 'octagon', className: 'text-quality-error' },
  env_not_ready: { shape: 'circle-dashed', className: 'text-quality-unknown' },
  cancelled: { shape: 'circle-dashed', className: 'text-quality-unknown' },
  skipped: { shape: 'circle-dashed', className: 'text-quality-unknown' },
  unknown: { shape: 'circle-dashed', className: 'text-quality-unknown' }
}

function statusLabel(status: QualityStep['status']): string {
  return scorecardCopy(`step.${status}` as QualityScorecardCopyKey)
}

function StepRow({ row }: { row: ScorecardStepRow }): React.JSX.Element {
  const { step, incomplete } = row
  const visual = STEP_VISUAL[step.status] ?? STEP_VISUAL.unknown
  const tool = [step.tool, step.toolVersion].filter(Boolean).join(' ')
  return (
    <li
      className="flex items-start gap-2 py-1 text-xs"
      data-status={step.status}
      data-incomplete={incomplete || undefined}
    >
      <SeverityGlyph shape={visual.shape} className={`mt-0.5 shrink-0 ${visual.className}`} />
      <div className="min-w-0 flex-1">
        <p className="break-words text-foreground">
          {step.id || step.profileId}
          {tool ? <span className="text-muted-foreground"> · {tool}</span> : null}
        </p>
        <p className="text-muted-foreground">
          {statusLabel(step.status)}
          {step.failureKind
            ? ` · ${scorecardCopy(`step.failure.${step.failureKind}` as QualityScorecardCopyKey)}`
            : ''}
          {step.durationMs > 0
            ? ` · ${scorecardCopy('step.duration', { seconds: Math.round(step.durationMs / 100) / 10 })}`
            : ''}
        </p>
        {step.envReason ? (
          <p className="break-words text-muted-foreground">{step.envReason}</p>
        ) : null}
        {incomplete ? (
          <p className="text-foreground">{scorecardCopy('step.noResult')}</p>
        ) : (
          <p className="text-muted-foreground">
            {scorecardCopy('step.counts', {
              errors: step.errorCount,
              warnings: step.warningCount,
              infos: step.infoCount
            })}
            {step.truncated ? ` · ${scorecardCopy('step.truncated')}` : ''}
            {step.outsideScopeCount > 0
              ? ` · ${scorecardCopy('step.outside', { count: step.outsideScopeCount })}`
              : ''}
          </p>
        )}
      </div>
    </li>
  )
}

export function QualityStepList({
  steps
}: {
  steps: readonly ScorecardStepRow[]
}): React.JSX.Element {
  const [open, setOpen] = useState(steps.some((s) => s.incomplete))
  return (
    <Collapsible open={open} onOpenChange={setOpen} data-testid="quality-steps">
      <CollapsibleTrigger className="flex items-center gap-1 text-xs font-medium text-foreground">
        <ChevronRight
          className={`size-3.5 transition-transform ${open ? 'rotate-90' : ''}`}
          aria-hidden
        />
        {scorecardCopy('steps.title', { count: steps.length })}
      </CollapsibleTrigger>
      <CollapsibleContent>
        {steps.length === 0 ? (
          <p className="py-1 text-xs text-muted-foreground">{scorecardCopy('steps.none')}</p>
        ) : (
          <ul className="divide-y divide-border">
            {steps.map((row) => (
              <StepRow key={row.key} row={row} />
            ))}
          </ul>
        )}
      </CollapsibleContent>
    </Collapsible>
  )
}
