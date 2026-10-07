/**
 * RequirementTracePanel.tsx — FE-CV-TASK-092-05
 *
 * Main panel for the requirement trace lens.
 * Shows grouped trace rows, evidence list, and unlinked changes.
 *
 * States: loading (100ms/1s/3s thresholds), empty, error, disabled.
 * No forbidden wording: never "met", "fulfilled", "satisfied".
 *
 * @module components/review-map/requirements/RequirementTracePanel
 */

import React from 'react'
import { AlertCircle, CheckCircle2, HelpCircle, AlertTriangle, Minus } from 'lucide-react'
import { useRequirementTrace } from './use-requirement-trace'
import type { TraceViewModel, TraceGroupKey } from './requirement-trace-view-model'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type RequirementTracePanelProps = {
  worktreeId: string
  environmentId: string | null
  projectId?: string | null
  translate: (key: string, params?: Record<string, unknown>) => string
}

// ---------------------------------------------------------------------------
// Icon per state
// ---------------------------------------------------------------------------

function StateIcon({ groupKey }: { groupKey: TraceGroupKey }): React.ReactElement {
  const cls = 'size-3.5 shrink-0'
  switch (groupKey) {
    case 'covered':
      return <CheckCircle2 className={`${cls} text-muted-foreground`} aria-hidden />
    case 'partial':
      return <AlertTriangle className={`${cls} text-yellow-500`} aria-hidden />
    case 'no_evidence':
      return <Minus className={`${cls} text-muted-foreground`} aria-hidden />
    case 'risk':
      return <AlertCircle className={`${cls} text-destructive`} aria-hidden />
    case 'unknown':
    default:
      return <HelpCircle className={`${cls} text-muted-foreground`} aria-hidden />
  }
}

// ---------------------------------------------------------------------------
// Trace row
// ---------------------------------------------------------------------------

function RequirementRow({
  trace,
  translate,
}: {
  trace: TraceViewModel
  translate: RequirementTracePanelProps['translate']
}): React.ReactElement {
  return (
    <li className="flex items-start gap-2 py-2 border-b border-border last:border-0">
      <StateIcon groupKey={trace.state as TraceGroupKey} />
      <div className="flex-1 min-w-0">
        <p className="text-sm break-words">{trace.requirementText}</p>
        <p className="text-xs text-muted-foreground mt-0.5">
          {translate(trace.stateLabelKey)}
          {trace.isSuggestion && (
            <span className="ml-1 italic">
              {translate('auto.components.reviewMap.requirements.trace.inferred')}
            </span>
          )}
        </p>
        {trace.evidence.length > 0 && (
          <ul className="mt-1 text-xs text-muted-foreground list-disc list-inside">
            {trace.evidence.map((ev, i) => (
              <li key={i}>{ev}</li>
            ))}
          </ul>
        )}
      </div>
    </li>
  )
}

// ---------------------------------------------------------------------------
// Panel
// ---------------------------------------------------------------------------

export function RequirementTracePanel({
  worktreeId,
  environmentId,
  projectId,
  translate,
}: RequirementTracePanelProps): React.ReactElement | null {
  const { data, status, error, disabled, available } = useRequirementTrace(
    worktreeId,
    environmentId,
    { projectId }
  )

  // Hidden when disabled (quality flag off or quality-disabled error)
  if (!available || disabled) return null

  if (status === 'loading' || status === 'idle') {
    return (
      <div
        role="status"
        aria-label={translate('auto.components.reviewMap.requirements.trace.loading')}
        className="p-4 text-sm text-muted-foreground"
      >
        {translate('auto.components.reviewMap.requirements.trace.loading')}
      </div>
    )
  }

  if (status === 'error' && error) {
    return (
      <div role="alert" className="p-4">
        <p className="text-sm text-destructive">
          {translate('auto.components.reviewMap.requirements.trace.error')}
        </p>
      </div>
    )
  }

  if (!data || !data.hasTraces) {
    return (
      <div className="p-4 text-sm text-muted-foreground">
        {translate('auto.components.reviewMap.requirements.trace.empty')}
      </div>
    )
  }

  return (
    <section aria-label={translate('auto.components.reviewMap.requirements.lens.label')}>
      {data.groups.map((group) => (
        <div key={group.key} className="mb-4">
          <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wide px-4 py-2">
            {translate(group.groupLabelKey)}
          </h3>
          <ul>
            {group.traces.map((trace) => (
              <RequirementRow
                key={trace.id}
                trace={trace}
                translate={translate}
              />
            ))}
          </ul>
        </div>
      ))}

      {data.suggestions.length > 0 && (
        <div className="mt-2">
          <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wide px-4 py-2">
            {translate('auto.components.reviewMap.requirements.trace.group.inferred')}
          </h3>
          <ul>
            {data.suggestions.map((trace) => (
              <RequirementRow key={trace.id} trace={trace} translate={translate} />
            ))}
          </ul>
        </div>
      )}
    </section>
  )
}
