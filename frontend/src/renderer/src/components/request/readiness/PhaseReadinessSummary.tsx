/**
 * PhaseReadinessSummary — FE-REQ-TASK-036-07
 *
 * @module components/request/readiness/PhaseReadinessSummary
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import type { PhaseReadinessSummary as Summary } from '../../../hooks/useTaskReadiness'

export function PhaseReadinessSummary({ summary }: { summary: Summary }): React.JSX.Element | null {
  const checked = summary.ready + summary.needsInfo + summary.specDefect + summary.envDefect
  if (checked === 0) {return null}
  const parts = [translate('auto.components.request.readiness.phase.ready', '{{count}} ready', { count: summary.ready })]
  if (summary.needsInfo > 0) {parts.push(translate('auto.components.request.readiness.phase.needsInfo', '{{count}} need information', { count: summary.needsInfo }))}
  if (summary.specDefect > 0) {parts.push(translate('auto.components.request.readiness.phase.specDefect', '{{count}} spec defects', { count: summary.specDefect }))}
  if (summary.envDefect > 0) {parts.push(translate('auto.components.request.readiness.phase.envDefect', '{{count}} environment issues', { count: summary.envDefect }))}
  return <span className="text-xs text-muted-foreground" data-testid="phase-readiness-summary">{parts.join(', ')}</span>
}
